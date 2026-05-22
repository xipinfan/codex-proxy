package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/upstream"

	"github.com/klauspost/compress/zstd"
)

func TestConcurrentRetryAfter429DoesNotIgnoreCooldownAccounts(t *testing.T) {
	executor := &Executor{httpClient: &http.Client{}}
	ignoreCooldownPicks := 0

	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return nil, fmt.Errorf("no regular accounts")
		},
		PickIgnoringCooldownFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			ignoreCooldownPicks++
			return &auth.Account{FilePath: fmt.Sprintf("cooldown-%d", ignoreCooldownPicks)}, nil
		},
	}

	_, _, _, _ = executor.concurrentRetryAfter429(context.Background(), rc, "gpt-5", "http://127.0.0.1/", []byte("{}"), false, nil)

	if ignoreCooldownPicks != 0 {
		t.Fatalf("expected concurrent 429 retry to respect cooldown, picked ignoring cooldown %d times", ignoreCooldownPicks)
	}
}

func TestModelAccessErrorDoesNotCooldownWholeAccount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"You do not have access to model gpt-5.5"}}`))
	}))
	defer upstream.Close()

	acc := &auth.Account{
		FilePath: "model-access.json",
		Token: auth.TokenData{
			Email:       "free@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()
	manager := auth.NewManager(t.TempDir(), nil, "", 3000, auth.NewRoundRobinSelector(), false, nil)
	executor := &Executor{httpClient: upstream.Client()}
	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
		OnAfterUpstreamErrFn: func(acc *auth.Account, model string, statusCode int, errBody []byte) bool {
			return manager.RecordModelFailureIfAccessError(acc, model, statusCode, errBody)
		},
	}

	_, _, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte("{}"), false)
	if err == nil {
		t.Fatalf("expected upstream 403 error")
	}
	stats := acc.GetStats()
	if stats.Status != "active" || !stats.Pickable {
		t.Fatalf("model access error should not cooldown whole account, status=%s pickable=%v", stats.Status, stats.Pickable)
	}
}

func TestImageGenerationToolUnavailableSwitchesAccount(t *testing.T) {
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Tool choice 'image_generation' not found in 'tools' parameter."}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aW1hZ2U="}}` + "\n\n"))
	}))
	defer upstream.Close()

	acc1 := &auth.Account{
		FilePath: "image-no-tool.json",
		Token: auth.TokenData{
			Email:       "image-no-tool@example.com",
			AccessToken: "access-token-1",
		},
		Status: auth.StatusActive,
	}
	acc1.SetActive()
	acc2 := &auth.Account{
		FilePath: "image-ok.json",
		Token: auth.TokenData{
			Email:       "image-ok@example.com",
			AccessToken: "access-token-2",
		},
		Status: auth.StatusActive,
	}
	acc2.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	rc := RetryConfig{
		MaxRetry: 1,
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			if !excluded[acc1.FilePath] {
				return acc1, nil
			}
			return acc2, nil
		},
	}

	resp, used, attempts, err := executor.sendWithRetry(context.Background(), rc, "gpt-image-2", upstream.URL, []byte("{}"), true)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if used != acc2 {
		t.Fatalf("used account = %s, want %s", used.GetEmail(), acc2.GetEmail())
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestSendWithRetryCompressesLargeUpstreamBody(t *testing.T) {
	wantBody := bytes.Repeat([]byte(`{"input":"multi-image"}`), 1024)
	var sawRequest bool

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if got := r.Header.Get("Content-Encoding"); got != "zstd" {
			t.Fatalf("Content-Encoding = %q, want zstd", got)
		}

		decoder, err := zstd.NewReader(r.Body)
		if err != nil {
			t.Fatalf("create zstd decoder: %v", err)
		}
		defer decoder.Close()

		gotBody, err := io.ReadAll(decoder)
		if err != nil {
			t.Fatalf("read zstd body: %v", err)
		}
		if !bytes.Equal(gotBody, wantBody) {
			t.Fatalf("decoded body mismatch")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstreamServer.Close()

	acc := &auth.Account{
		FilePath: "compressed.json",
		Token: auth.TokenData{
			Email:       "compressed@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()

	executor := NewExecutor(upstreamServer.URL, "", HTTPPoolConfig{
		UpstreamRequestCompression: upstream.CompressionConfig{
			Mode:     upstream.CompressionAuto,
			MinBytes: 1,
		},
	})
	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
	}

	resp, _, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstreamServer.URL, wantBody, false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if !sawRequest {
		t.Fatal("upstream server did not receive request")
	}
}

func TestDeriveSessionHintIsStableForAccountAndBody(t *testing.T) {
	body := []byte(`{"instructions":"keep a stable prefix","input":[{"type":"message","content":[{"type":"input_text","text":"hello"}]}]}`)

	got := deriveSessionHint("account-a", body)
	if got == "" {
		t.Fatal("deriveSessionHint() returned empty hint")
	}
	if len(got) != 32 {
		t.Fatalf("deriveSessionHint() length = %d, want 32", len(got))
	}
	if got != deriveSessionHint("account-a", body) {
		t.Fatal("deriveSessionHint() changed for the same account and body")
	}
}

func TestDeriveSessionHintIgnoresAccountIdentity(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"hello"}]}`)

	got := deriveSessionHint("account-a", body)
	if got != deriveSessionHint("account-b", body) {
		t.Fatal("deriveSessionHint() should ignore account identity")
	}
}

func TestDeriveSessionHintEmptyBody(t *testing.T) {
	if got := deriveSessionHint("account-a", nil); got != "" {
		t.Fatalf("deriveSessionHint() = %q, want empty for empty body", got)
	}
}

func TestDeriveAffinitySessionKeyIgnoresAccountIdentity(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"hello"}]}`)

	first := deriveAffinitySessionKey("", body)
	if first == "" {
		t.Fatal("deriveAffinitySessionKey() returned empty key")
	}
	if first != deriveAffinitySessionKey("", body) {
		t.Fatal("fallback affinity key changed for the same stable prefix")
	}
}

func TestDeriveAffinitySessionKeyPrefersExplicitHeader(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"hello"}]}`)

	if got := deriveAffinitySessionKey(" explicit-session ", body); got != "explicit-session" {
		t.Fatalf("deriveAffinitySessionKey() = %q, want explicit-session", got)
	}
}

func TestApplyCodexHeadersPrefersExplicitSessionID(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("X-Session-Id", "explicit-session")

	applyCodexHeaders(req, testHeaderAccount(), false, "derived-hint")

	if got := req.Header.Get("Session_id"); got != "explicit-session" {
		t.Fatalf("Session_id = %q, want explicit-session", got)
	}
}

func TestApplyCodexHeadersUsesDerivedSessionHint(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	applyCodexHeaders(req, testHeaderAccount(), false, "derived-hint")

	if got := req.Header.Get("Session_id"); got != "derived-hint" {
		t.Fatalf("Session_id = %q, want derived-hint", got)
	}
}

func TestApplyCodexHeadersFallsBackToSessionID(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/responses", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	applyCodexHeaders(req, testHeaderAccount(), false, "")

	if got := req.Header.Get("Session_id"); got == "" {
		t.Fatal("Session_id fallback should not be empty")
	}
}

func TestSendWithRetryUsesSessionAwarePick(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Session_id"); got != "session-a" {
			t.Fatalf("Session_id = %q, want session-a", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	acc := &auth.Account{
		FilePath: "session-aware.json",
		Token: auth.TokenData{
			Email:       "session-aware@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	var pickSessionKey string
	var regularPicks int
	rc := RetryConfig{
		ExplicitSessionID: "session-a",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			regularPicks++
			return nil, fmt.Errorf("regular pick should not be used")
		},
		PickSessionAccountFn: func(sessionKey, model string, excluded map[string]bool) (*auth.Account, error) {
			pickSessionKey = sessionKey
			return acc, nil
		},
	}

	resp, used, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if pickSessionKey != "session-a" {
		t.Fatalf("PickSessionAccountFn sessionKey = %q, want session-a", pickSessionKey)
	}
	if regularPicks != 0 {
		t.Fatalf("regular PickFn called %d times, want 0", regularPicks)
	}
	if used != acc {
		t.Fatalf("used account = %v, want session-aware account", used)
	}
}

func TestSendWithRetryBindsSuccessfulSessionAccount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	acc := &auth.Account{
		FilePath: "success.json",
		Token: auth.TokenData{
			Email:       "success@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	var boundKey string
	var boundAccount *auth.Account
	rc := RetryConfig{
		ExplicitSessionID: "session-a",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
		BindSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			boundKey = sessionKey
			boundAccount = acc
		},
	}

	resp, used, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if used != acc {
		t.Fatalf("used account = %v, want success account", used)
	}
	if boundKey != "session-a" {
		t.Fatalf("bound sessionKey = %q, want session-a", boundKey)
	}
	if boundAccount != acc {
		t.Fatalf("bound account = %v, want success account", boundAccount)
	}
}

func TestSendWithRetryEvictsFailedBoundSessionAccountBeforeReplacement(t *testing.T) {
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	acc1 := &auth.Account{
		FilePath: "rate-limited.json",
		Token: auth.TokenData{
			Email:       "rate-limited@example.com",
			AccessToken: "access-token-1",
		},
		Status: auth.StatusActive,
	}
	acc1.SetActive()
	acc2 := &auth.Account{
		FilePath: "replacement.json",
		Token: auth.TokenData{
			Email:       "replacement@example.com",
			AccessToken: "access-token-2",
		},
		Status: auth.StatusActive,
	}
	acc2.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	var evictedKey string
	var evictedAccount *auth.Account
	var boundKey string
	var boundAccount *auth.Account
	rc := RetryConfig{
		MaxRetry:          1,
		ExplicitSessionID: "session-a",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			if !excluded[acc1.FilePath] {
				return acc1, nil
			}
			return acc2, nil
		},
		BindSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			boundKey = sessionKey
			boundAccount = acc
		},
		EvictSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			evictedKey = sessionKey
			evictedAccount = acc
		},
	}

	resp, used, attempts, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if used != acc2 {
		t.Fatalf("used account = %v, want replacement account", used)
	}
	if evictedKey != "session-a" {
		t.Fatalf("evicted sessionKey = %q, want session-a", evictedKey)
	}
	if evictedAccount != acc1 {
		t.Fatalf("evicted account = %v, want first account", evictedAccount)
	}
	if boundKey != "session-a" {
		t.Fatalf("bound sessionKey = %q, want session-a", boundKey)
	}
	if boundAccount != acc2 {
		t.Fatalf("bound account = %v, want replacement account", boundAccount)
	}
}

func TestSendWithRetryEvictsQuotaRejectedSessionAccount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	acc1 := &auth.Account{
		FilePath: "quota-rejected.json",
		Token: auth.TokenData{
			Email:       "quota-rejected@example.com",
			AccessToken: "access-token-1",
		},
		Status: auth.StatusActive,
	}
	acc1.SetActive()
	acc2 := &auth.Account{
		FilePath: "quota-ok.json",
		Token: auth.TokenData{
			Email:       "quota-ok@example.com",
			AccessToken: "access-token-2",
		},
		Status: auth.StatusActive,
	}
	acc2.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	var evictedAccount *auth.Account
	rc := RetryConfig{
		ExplicitSessionID: "session-a",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			if !excluded[acc1.FilePath] {
				return acc1, nil
			}
			return acc2, nil
		},
		QuotaCheckFn: func(ctx context.Context, acc *auth.Account) bool {
			return acc != acc1
		},
		EvictSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			if sessionKey == "session-a" {
				evictedAccount = acc
			}
		},
	}

	resp, used, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if used != acc2 {
		t.Fatalf("used account = %v, want quota-ok account", used)
	}
	if evictedAccount != acc1 {
		t.Fatalf("evicted account = %v, want quota-rejected account", evictedAccount)
	}
}

func TestSendWithRetryEvictsTokenRejectedSessionAccount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	acc1 := &auth.Account{
		FilePath: "stale-token.json",
		Token: auth.TokenData{
			Email:       "stale-token@example.com",
			AccessToken: "access-token-1",
		},
		Status: auth.StatusActive,
	}
	acc1.SetActive()
	acc2 := &auth.Account{
		FilePath: "fresh-token.json",
		Token: auth.TokenData{
			Email:       "fresh-token@example.com",
			AccessToken: "access-token-2",
		},
		Status: auth.StatusActive,
	}
	acc2.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	var evictedAccount *auth.Account
	rc := RetryConfig{
		MaxRetry:          1,
		ExplicitSessionID: "session-a",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			if !excluded[acc1.FilePath] {
				return acc1, nil
			}
			return acc2, nil
		},
		EnsureTokenFreshFn: func(ctx context.Context, acc *auth.Account) bool {
			return acc != acc1
		},
		EvictSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			if sessionKey == "session-a" {
				evictedAccount = acc
			}
		},
	}

	resp, used, attempts, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if used != acc2 {
		t.Fatalf("used account = %v, want fresh-token account", used)
	}
	if evictedAccount != acc1 {
		t.Fatalf("evicted account = %v, want stale-token account", evictedAccount)
	}
}

func testHeaderAccount() *auth.Account {
	return &auth.Account{
		Token: auth.TokenData{
			AccountID:   "account-a",
			AccessToken: "access-token",
		},
	}
}
