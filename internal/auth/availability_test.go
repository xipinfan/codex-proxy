package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGetStatsReportsExpiredCooldownAsPickable(t *testing.T) {
	acc := &Account{
		FilePath: filepath.Join("test", "expired.json"),
		Token: TokenData{
			Email: "expired@example.com",
		},
		Status:        StatusCooldown,
		CooldownUntil: time.Now().Add(-time.Minute),
	}
	acc.atomicStatus.Store(int32(StatusCooldown))
	acc.atomicCooldownMs.Store(acc.CooldownUntil.UnixMilli())

	stats := acc.GetStats()
	if stats.Status != "active" {
		t.Fatalf("expected effective status active after cooldown expiry, got %s", stats.Status)
	}
	if !stats.Pickable {
		t.Fatalf("expected expired cooldown account to be pickable")
	}
	if stats.UnavailableReason != "" {
		t.Fatalf("expected no unavailable reason, got %q", stats.UnavailableReason)
	}
}

func TestPickRecentlySuccessfulUsesExpiredCooldownAccount(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "recent-expired@example.com")
	acc.SetCooldown(time.Minute)
	acc.mu.Lock()
	acc.CooldownUntil = time.Now().Add(-time.Minute)
	acc.mu.Unlock()
	acc.atomicCooldownMs.Store(acc.CooldownUntil.UnixMilli())
	acc.lastSuccessUnixMs.Store(time.Now().Add(-time.Second).UnixMilli())

	picked, err := m.PickRecentlySuccessful("gpt-5", nil)
	if err != nil {
		t.Fatalf("expected expired cooldown recent account to be picked: %v", err)
	}
	if picked != acc {
		t.Fatalf("expected picked account %p, got %p", acc, picked)
	}
}
