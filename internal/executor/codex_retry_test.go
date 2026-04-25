package executor

import (
	"context"
	"fmt"
	"net/http"
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
