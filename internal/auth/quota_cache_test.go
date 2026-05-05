package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQuotaChecker_tryCachedQuotaVerdict(t *testing.T) {
	qc := &QuotaChecker{
		resultCacheTTL:    30 * time.Second,
		transientCacheMax: 5 * time.Second,
	}
	now := time.Now()
	acc := &Account{
		QuotaInfo: &QuotaInfo{
			StatusCode: 200,
			Valid:      true,
		},
		QuotaCheckedAt: now.Add(-10 * time.Second),
	}
	if v, ok := qc.tryCachedQuotaVerdict(acc); !ok || v != 1 {
		t.Fatalf("expected cached valid, got ok=%v v=%d", ok, v)
	}
	acc.QuotaCheckedAt = now.Add(-40 * time.Second)
	if _, ok := qc.tryCachedQuotaVerdict(acc); ok {
		t.Fatal("expected stale miss")
	}
}

func TestQuotaCheckerCheckAccountUpdatesPlanTypeFromUsageSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"plan_type": "free",
			"rate_limit": {
				"allowed": true,
				"primary_window": {
					"used_percent": 55,
					"limit_window_seconds": 604800
				},
				"secondary_window": null
			}
		}`))
	}))
	defer server.Close()

	qc := &QuotaChecker{
		httpClient: server.Client(),
		usageURL:   server.URL,
	}
	acc := &Account{
		Token: TokenData{
			AccessToken: "access-token",
			AccountID:   "acct_123",
			Email:       "downgraded@example.com",
			PlanType:    "plus",
		},
	}

	verdict, status := qc.checkAccount(context.Background(), acc)
	if verdict != 1 || status != http.StatusOK {
		t.Fatalf("expected valid quota response, got verdict=%d status=%d", verdict, status)
	}
	if got := acc.TokenSnapshot().PlanType; got != "free" {
		t.Fatalf("expected quota snapshot to update token plan type to free, got %q", got)
	}
}
