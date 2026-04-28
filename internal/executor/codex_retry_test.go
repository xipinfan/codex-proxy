package executor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codex-proxy/internal/auth"
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
