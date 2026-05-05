package auth

import (
	"encoding/json"
	"os"
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

func TestGetStatsDerivesFreePlanFromQuotaSnapshot(t *testing.T) {
	raw := json.RawMessage(`{
		"plan_type": "free",
		"rate_limit": {
			"primary_window": {
				"used_percent": 55,
				"limit_window_seconds": 604800
			},
			"secondary_window": null
		}
	}`)
	acc := &Account{
		Token: TokenData{
			Email:    "downgraded@example.com",
			PlanType: "plus",
		},
		Status: StatusActive,
		QuotaInfo: &QuotaInfo{
			Valid:      true,
			StatusCode: 200,
			RawData:    raw,
			CheckedAt:  time.Now(),
		},
	}

	stats := acc.GetStats()
	if stats.PlanType != "free" {
		t.Fatalf("expected quota snapshot to report downgraded plan as free, got %q", stats.PlanType)
	}
}

func TestSaveTokenToFilePersistsPlanTypeOverride(t *testing.T) {
	dir := t.TempDir()
	acc := &Account{
		FilePath: filepath.Join(dir, "account.json"),
		Token: TokenData{
			IDToken:      "old-id-token",
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			AccountID:    "acct_123",
			Email:        "downgraded@example.com",
			Expire:       time.Now().Add(time.Hour).Format(time.RFC3339),
			PlanType:     "free",
		},
		LastRefreshedAt: time.Now(),
	}
	m := NewManager(dir, nil, "", 300, nil, false, nil)

	if err := m.saveTokenToFile(acc); err != nil {
		t.Fatalf("saveTokenToFile() error = %v", err)
	}
	data, err := os.ReadFile(acc.FilePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if stored["plan_type"] != "free" {
		t.Fatalf("expected saved plan_type free, got %#v", stored["plan_type"])
	}
	reloaded, err := loadAccountFromFile(acc.FilePath)
	if err != nil {
		t.Fatalf("loadAccountFromFile() error = %v", err)
	}
	if got := reloaded.TokenSnapshot().PlanType; got != "free" {
		t.Fatalf("expected reloaded plan_type free, got %q", got)
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
