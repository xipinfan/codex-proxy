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
	"github.com/tidwall/gjson"
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

func TestDeriveAffinitySessionKeyPrefersExplicitHeader(t *testing.T) {
	body := []byte(`{"instructions":"keep a stable prefix","input":[{"type":"message","content":[{"type":"input_text","text":"hello"}]}]}`)

	if got := deriveAffinitySessionKey(" explicit-session ", "", body); got != "explicit-session" {
		t.Fatalf("deriveAffinitySessionKey() = %q, want explicit-session", got)
	}
}

func TestDeriveAffinitySessionKeyIgnoresAccountIdentity(t *testing.T) {
	body := []byte(`{"instructions":"keep a stable prefix","input":[{"type":"message","content":[{"type":"input_text","text":"hello"}]}]}`)

	got := deriveAffinitySessionKey("", "", body)
	if got == "" {
		t.Fatal("deriveAffinitySessionKey() returned empty key")
	}
	if got != deriveAffinitySessionKey("", "", body) {
		t.Fatal("deriveAffinitySessionKey() changed for the same request body")
	}
}

func TestDeriveAffinitySessionKeyUsesResolvedPreviousResponseSession(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"new user turn"}]}`)

	if got := deriveAffinitySessionKey("", "resolved-session", body); got != "resolved-session" {
		t.Fatalf("deriveAffinitySessionKey() = %q, want resolved-session", got)
	}
}

func TestDeriveSessionHintIsStableForBody(t *testing.T) {
	body := []byte(`{"instructions":"keep a stable prefix","input":[{"type":"message","content":[{"type":"input_text","text":"hello"}]}]}`)

	got := deriveSessionHint(body)
	if got == "" {
		t.Fatal("deriveSessionHint() returned empty hint")
	}
	if len(got) != 32 {
		t.Fatalf("deriveSessionHint() length = %d, want 32", len(got))
	}
	if got != deriveSessionHint(body) {
		t.Fatal("deriveSessionHint() changed for the same body")
	}
}

func TestDeriveSessionHintChangesWithBodyOnly(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"hello"}]}`)
	otherBody := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"goodbye"}]}`)

	got := deriveSessionHint(body)
	if got == deriveSessionHint(otherBody) {
		t.Fatal("deriveSessionHint() should change for a different request body")
	}
}

func TestDeriveSessionHintEmptyBody(t *testing.T) {
	if got := deriveSessionHint(nil); got != "" {
		t.Fatalf("deriveSessionHint() = %q, want empty for empty body", got)
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

func TestSendWithRetryBindsSessionAccountOnSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	acc := &auth.Account{
		FilePath: "sticky.json",
		Token: auth.TokenData{
			Email:       "sticky@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()

	var boundSession string
	var boundAccount string
	executor := &Executor{httpClient: upstream.Client()}
	rc := RetryConfig{
		ExplicitSessionID: "session-a",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
		BindSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			boundSession = sessionKey
			boundAccount = acc.FilePath
		},
	}

	resp, _, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if boundSession != "session-a" {
		t.Fatalf("bound session = %q, want session-a", boundSession)
	}
	if boundAccount != acc.FilePath {
		t.Fatalf("bound account = %q, want %q", boundAccount, acc.FilePath)
	}
}

func TestSendWithRetryEvictsFailedBoundAccountBeforeRetry(t *testing.T) {
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	first := &auth.Account{
		FilePath: "first.json",
		Token: auth.TokenData{
			Email:       "first@example.com",
			AccessToken: "access-token-1",
		},
		Status: auth.StatusActive,
	}
	first.SetActive()
	second := &auth.Account{
		FilePath: "second.json",
		Token: auth.TokenData{
			Email:       "second@example.com",
			AccessToken: "access-token-2",
		},
		Status: auth.StatusActive,
	}
	second.SetActive()

	var evicted []string
	executor := &Executor{httpClient: upstream.Client()}
	rc := RetryConfig{
		ExplicitSessionID: "session-a",
		MaxRetry:          1,
		PickSessionAccountFn: func(sessionKey, model string, excluded map[string]bool) (*auth.Account, error) {
			if !excluded[first.FilePath] {
				return first, nil
			}
			return second, nil
		},
		EvictSessionAccountFn: func(sessionKey string, acc *auth.Account) {
			evicted = append(evicted, sessionKey+":"+acc.FilePath)
		},
	}

	resp, used, attempts, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte(`{"input":"hello"}`), false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if used != second {
		t.Fatalf("used account = %s, want %s", used.GetEmail(), second.GetEmail())
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if len(evicted) == 0 || evicted[0] != "session-a:first.json" {
		t.Fatalf("evicted = %v, want first bound account eviction", evicted)
	}
}

func TestExecuteResponsesNonStreamBindsResponseContinuation(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	acc := &auth.Account{
		FilePath: "sticky.json",
		Token: auth.TokenData{
			Email:       "sticky@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()

	var boundResponseID string
	var boundSession string
	executor := &Executor{
		baseURL:    upstream.URL,
		httpClient: upstream.Client(),
	}
	rc := RetryConfig{
		ResolvedSessionKey: "resolved-session",
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
		BindResponseContinuationFn: func(responseID, sessionKey string, acc *auth.Account) {
			boundResponseID = responseID
			boundSession = sessionKey
		},
	}

	resp, err := executor.ExecuteResponsesNonStream(context.Background(), rc, []byte(`{"model":"gpt-5.5","input":"hello"}`), "gpt-5.5")
	if err != nil {
		t.Fatalf("ExecuteResponsesNonStream() error = %v", err)
	}
	if gjson.GetBytes(resp, "id").String() != "resp_123" {
		t.Fatalf("response id = %q, want resp_123", gjson.GetBytes(resp, "id").String())
	}
	if boundResponseID != "resp_123" || boundSession != "resolved-session" {
		t.Fatalf("bound continuation = (%q, %q), want (resp_123, resolved-session)", boundResponseID, boundSession)
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
