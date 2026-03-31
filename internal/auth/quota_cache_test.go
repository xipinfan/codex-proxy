package auth

import (
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
