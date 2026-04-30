package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	codexOAuthAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	codexOAuthRedirectURI  = "http://localhost:1455/auth/callback"
	codexOAuthScope        = "openid profile email offline_access"
	codexOAuthStateTTL     = 5 * time.Minute
)

type oauthStateEntry struct {
	Verifier     string
	ExpireAt     time.Time
	Completed    bool
	TokenData    *TokenData
	ErrorMessage string
}

type OAuthAuthorizationStatus string

const (
	OAuthAuthorizationPending   OAuthAuthorizationStatus = "pending"
	OAuthAuthorizationCompleted OAuthAuthorizationStatus = "completed"
	OAuthAuthorizationFailed    OAuthAuthorizationStatus = "failed"
)

type OAuthAuthorizationResult struct {
	Status       OAuthAuthorizationStatus
	TokenData    *TokenData
	ErrorMessage string
}

type CodexOAuthFlow struct {
	refresher          *Refresher
	mu                 sync.Mutex
	stateMap           map[string]oauthStateEntry
	codeExchange       func(ctx context.Context, code string, verifier string) (*TokenData, error)
	callbackServerOnce sync.Once
	callbackServerErr  error
	skipCallbackServer bool
}

func NewCodexOAuthFlow(proxyURL string, enableHTTP2 bool) *CodexOAuthFlow {
	flow := &CodexOAuthFlow{
		refresher: NewRefresher(proxyURL, enableHTTP2),
		stateMap:  make(map[string]oauthStateEntry),
	}
	flow.codeExchange = flow.exchangeAuthorizationCode
	return flow
}

func (f *CodexOAuthFlow) StartAuthorization() (authorizeURL string, state string, expiresInSec int, err error) {
	if err := f.ensureCallbackServer(); err != nil {
		return "", "", 0, err
	}
	verifier, err := newCodeVerifier()
	if err != nil {
		return "", "", 0, err
	}
	state, err = randomHex(16)
	if err != nil {
		return "", "", 0, err
	}
	challenge := codeChallengeS256(verifier)
	expireAt := time.Now().Add(codexOAuthStateTTL)

	f.mu.Lock()
	for key, item := range f.stateMap {
		if time.Now().After(item.ExpireAt) {
			delete(f.stateMap, key)
		}
	}
	f.stateMap[state] = oauthStateEntry{
		Verifier: verifier,
		ExpireAt: expireAt,
	}
	f.mu.Unlock()

	q := url.Values{}
	q.Set("client_id", ClientID)
	q.Set("redirect_uri", codexOAuthRedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", codexOAuthScope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("codex_cli_simplified_flow", "true")

	return codexOAuthAuthorizeURL + "?" + q.Encode(), state, int(codexOAuthStateTTL.Seconds()), nil
}

func (f *CodexOAuthFlow) ExchangeCallbackURL(ctx context.Context, callbackURL string) (*TokenData, error) {
	_, code, state, err := parseCallbackURL(strings.TrimSpace(callbackURL))
	if err != nil {
		return nil, err
	}

	verifier, ok := f.consumeStateVerifier(state)
	if !ok {
		return nil, fmt.Errorf("state 无效或已过期，请重新发起授权")
	}

	return f.codeExchange(ctx, code, verifier)
}

func (f *CodexOAuthFlow) HandleCallbackURL(ctx context.Context, callbackURL string) error {
	_, code, state, err := parseCallbackURL(strings.TrimSpace(callbackURL))
	if err != nil {
		return err
	}

	verifier, ok := f.peekStateVerifier(state)
	if !ok {
		return fmt.Errorf("state 无效或已过期，请重新发起授权")
	}

	td, exchangeErr := f.codeExchange(ctx, code, verifier)
	f.storeAuthorizationResult(state, td, exchangeErr)
	if exchangeErr != nil {
		return exchangeErr
	}
	return nil
}

func (f *CodexOAuthFlow) PollAuthorizationResult(state string) (*OAuthAuthorizationResult, error) {
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, item := range f.stateMap {
		if now.After(item.ExpireAt) {
			delete(f.stateMap, key)
		}
	}

	item, ok := f.stateMap[state]
	if !ok {
		return nil, fmt.Errorf("state 无效或已过期，请重新发起授权")
	}
	if !item.Completed {
		return &OAuthAuthorizationResult{Status: OAuthAuthorizationPending}, nil
	}

	delete(f.stateMap, state)
	if item.ErrorMessage != "" {
		return &OAuthAuthorizationResult{
			Status:       OAuthAuthorizationFailed,
			ErrorMessage: item.ErrorMessage,
		}, nil
	}

	return &OAuthAuthorizationResult{
		Status:    OAuthAuthorizationCompleted,
		TokenData: item.TokenData,
	}, nil
}

func (f *CodexOAuthFlow) exchangeAuthorizationCode(ctx context.Context, code string, verifier string) (*TokenData, error) {
	form := url.Values{}
	form.Set("client_id", ClientID)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", codexOAuthRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建授权换 token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := f.refresher.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("授权换 token 请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取授权换 token 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := truncateLogExcerpt(string(body), 200)
		return nil, fmt.Errorf("授权换 token 失败 [%d]: %s", resp.StatusCode, msg)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析授权换 token 响应失败: %w", err)
	}

	accountID, email, planType := parseIDTokenClaims(tokenResp.IDToken)
	return &TokenData{
		IDToken:      tokenResp.IDToken,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		AccountID:    accountID,
		Email:        email,
		Expire:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339),
		PlanType:     planType,
	}, nil
}

func (f *CodexOAuthFlow) ensureCallbackServer() error {
	if f.skipCallbackServer {
		return nil
	}
	f.callbackServerOnce.Do(func() {
		handler := http.NewServeMux()
		handler.HandleFunc("/auth/callback", f.handleBrowserCallback)

		server := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		}

		listeners := make([]net.Listener, 0, 2)
		var errs []string
		for _, addr := range []string{"127.0.0.1:1455", "[::1]:1455"} {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", addr, err))
				continue
			}
			listeners = append(listeners, ln)
		}
		if len(listeners) == 0 {
			f.callbackServerErr = fmt.Errorf("启动 OAuth 本地回调监听失败: %s", strings.Join(errs, "; "))
			return
		}

		for _, ln := range listeners {
			go func(listener net.Listener) {
				if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
					log.Warnf("OAuth 本地回调监听异常 [%s]: %v", listener.Addr().String(), err)
				}
			}(ln)
		}
	})
	return f.callbackServerErr
}

func (f *CodexOAuthFlow) handleBrowserCallback(w http.ResponseWriter, r *http.Request) {
	callbackURL := fmt.Sprintf("http://%s%s", r.Host, r.URL.RequestURI())
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	err := f.HandleCallbackURL(ctx, callbackURL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(callbackHTML(false, err.Error())))
		return
	}
	_, _ = w.Write([]byte(callbackHTML(true, "授权完成，现在可以回到控制台继续导入。")))
}

func callbackHTML(success bool, message string) string {
	title := "授权已完成"
	background := "#e8fff4"
	border := "#1f9d64"
	if !success {
		title = "授权失败"
		background = "#fff1ef"
		border = "#c2410c"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
</head>
<body style="margin:0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f7f4ee;color:#1f2937;">
  <main style="max-width:560px;margin:10vh auto;padding:24px;">
    <section style="border:1px solid %s;background:%s;border-radius:20px;padding:24px;box-shadow:0 20px 60px rgba(31,41,55,0.08);">
      <h1 style="margin:0 0 12px;font-size:28px;">%s</h1>
      <p style="margin:0 0 16px;line-height:1.7;">%s</p>
      <p style="margin:0;color:#6b7280;">这个页面可以直接关闭。</p>
    </section>
  </main>
</body>
</html>`, title, border, background, title, htmlEscape(message))
}

func htmlEscape(input string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(input)
}

func parseCallbackURL(raw string) (*url.URL, string, string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, "", "", fmt.Errorf("回调 URL 解析失败: %w", err)
	}
	params := parsedURL.Query()
	if e := strings.TrimSpace(params.Get("error")); e != "" {
		msg := strings.TrimSpace(params.Get("error_description"))
		if msg == "" {
			msg = e
		}
		return nil, "", "", fmt.Errorf("授权失败: %s", msg)
	}

	code := strings.TrimSpace(params.Get("code"))
	state := strings.TrimSpace(params.Get("state"))
	if code == "" || state == "" {
		return nil, "", "", fmt.Errorf("回调 URL 缺少 code 或 state")
	}
	return parsedURL, code, state, nil
}

func (f *CodexOAuthFlow) peekStateVerifier(state string) (string, bool) {
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, item := range f.stateMap {
		if now.After(item.ExpireAt) {
			delete(f.stateMap, key)
		}
	}
	item, ok := f.stateMap[state]
	if !ok || now.After(item.ExpireAt) {
		delete(f.stateMap, state)
		return "", false
	}
	return item.Verifier, true
}

func (f *CodexOAuthFlow) storeAuthorizationResult(state string, td *TokenData, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.stateMap[state]
	if !ok {
		return
	}
	item.Completed = true
	item.TokenData = td
	if err != nil {
		item.ErrorMessage = err.Error()
	}
	f.stateMap[state] = item
}

func (f *CodexOAuthFlow) consumeStateVerifier(state string) (string, bool) {
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, item := range f.stateMap {
		if now.After(item.ExpireAt) {
			delete(f.stateMap, key)
		}
	}
	item, ok := f.stateMap[state]
	if !ok || now.After(item.ExpireAt) {
		delete(f.stateMap, state)
		return "", false
	}
	delete(f.stateMap, state)
	return item.Verifier, true
}

func newCodeVerifier() (string, error) {
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成 code_verifier 失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomHex(n int) (string, error) {
	if n <= 0 {
		n = 16
	}
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成随机 state 失败: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
