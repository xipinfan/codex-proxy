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

func selectorTestAccount(path, plan string, usedPercent float64) *Account {
	acc := &Account{
		FilePath: path,
		Token: TokenData{
			PlanType: plan,
		},
		Status: StatusActive,
	}
	acc.atomicStatus.Store(int32(StatusActive))
	acc.atomicUsedPct.Store(int64(usedPercent * 100))
	return acc
}

func resetFreeAccountPolicy(t *testing.T) {
	t.Helper()
	SetFreeAccountPolicy(70, "fallback")
	t.Cleanup(func() {
		SetFreeAccountPolicy(70, "fallback")
	})
}

func TestSortByTierThenUsedPercentPrefersPaidThenUnknownThenFree(t *testing.T) {
	paid := selectorTestAccount("paid.json", "pro", 90)
	unknown := selectorTestAccount("unknown.json", "", 10)
	free := selectorTestAccount("free.json", "free", 1)
	accounts := []*Account{free, unknown, paid}

	sortByTierThenUsedPercent(accounts)

	want := []*Account{paid, unknown, free}
	for i, acc := range want {
		if accounts[i] != acc {
			t.Fatalf("accounts[%d] = %s, want %s", i, accounts[i].FilePath, acc.FilePath)
		}
	}
}

func TestFilterAvailableDropsFreeAccountAtCutoff(t *testing.T) {
	resetFreeAccountPolicy(t)
	SetFreeAccountPolicy(70, "shared")

	belowCutoff := selectorTestAccount("below.json", "free", 69)
	atCutoff := selectorTestAccount("at.json", "free", 70)

	got := filterAvailable("gpt-5", []*Account{belowCutoff, atCutoff})
	if len(got) != 1 || got[0] != belowCutoff {
		t.Fatalf("filterAvailable() = %#v, want only free account below cutoff", got)
	}
}

func TestFilterAvailableKeepsFreeAccountWhenCutoffDisabled(t *testing.T) {
	resetFreeAccountPolicy(t)
	SetFreeAccountPolicy(0, "shared")

	full := selectorTestAccount("full.json", "free", 100)
	got := filterAvailable("gpt-5", []*Account{full})
	if len(got) != 1 || got[0] != full {
		t.Fatalf("filterAvailable() = %#v, want free account when cutoff is disabled", got)
	}
}

func TestFallbackFilterHidesFreeWhilePrimaryTierAvailable(t *testing.T) {
	resetFreeAccountPolicy(t)

	paid := selectorTestAccount("paid.json", "plus", 60)
	unknown := selectorTestAccount("unknown.json", "", 60)
	free := selectorTestAccount("free.json", "free", 60)

	got := filterAvailable("gpt-5", []*Account{free, unknown, paid})
	if len(got) != 2 || got[0] != unknown || got[1] != paid {
		t.Fatalf("filterAvailable() = %#v, want paid and unknown primary tier only", got)
	}
}

func TestFallbackFilterRevealsFreeWhenPaidUnavailable(t *testing.T) {
	resetFreeAccountPolicy(t)

	disabled := selectorTestAccount("disabled.json", "team", 20)
	disabled.SetDisabled(nil)
	cooling := selectorTestAccount("cooling.json", "pro", 20)
	cooling.SetCooldown(time.Minute)
	free := selectorTestAccount("free.json", "free", 20)

	got := filterAvailable("gpt-5", []*Account{disabled, free, cooling})
	if len(got) != 1 || got[0] != free {
		t.Fatalf("filterAvailable() = %#v, want fallback free account", got)
	}
}

func TestFallbackFilterPreservesModelBlockFiltering(t *testing.T) {
	resetFreeAccountPolicy(t)

	paid := selectorTestAccount("paid.json", "plus", 20)
	free := selectorTestAccount("free.json", "free", 20)
	now := time.Now()
	for i := 0; i < modelAccessFailureThreshold; i++ {
		paid.RecordModelAccessFailure("gpt-5.5", now.Add(time.Duration(i)*time.Second))
	}

	got := filterAvailable("gpt-5.5", []*Account{paid, free})
	if len(got) != 1 || got[0] != free {
		t.Fatalf("blocked model filterAvailable() = %#v, want fallback free account", got)
	}

	got = filterAvailable("gpt-5.4", []*Account{paid, free})
	if len(got) != 1 || got[0] != paid {
		t.Fatalf("other model filterAvailable() = %#v, want primary paid account", got)
	}
}

func TestPickRecentlySuccessfulKeepsFreeInFallbackTier(t *testing.T) {
	resetFreeAccountPolicy(t)

	m := newPolicyTestManager(t)
	paid := addPolicyTestAccount(m, "recent-paid@example.com")
	paid.Token.PlanType = "plus"
	paid.lastSuccessUnixMs.Store(time.Now().Add(-time.Minute).UnixMilli())
	free := addPolicyTestAccount(m, "recent-free@example.com")
	free.Token.PlanType = "free"
	free.lastSuccessUnixMs.Store(time.Now().UnixMilli())

	picked, err := m.PickRecentlySuccessful("gpt-5", nil)
	if err != nil {
		t.Fatalf("PickRecentlySuccessful() error = %v", err)
	}
	if picked != paid {
		t.Fatalf("PickRecentlySuccessful() = %s, want paid primary tier", picked.GetEmail())
	}
}

func TestPickRecentlySuccessfulOnlyKeepsFreeInFallbackTier(t *testing.T) {
	resetFreeAccountPolicy(t)

	m := newPolicyTestManager(t)
	paid := addPolicyTestAccount(m, "recent-only-paid@example.com")
	paid.Token.PlanType = "plus"
	paid.lastSuccessUnixMs.Store(time.Now().Add(-time.Minute).UnixMilli())
	free := addPolicyTestAccount(m, "recent-only-free@example.com")
	free.Token.PlanType = "free"
	free.lastSuccessUnixMs.Store(time.Now().UnixMilli())

	picked, err := m.PickRecentlySuccessfulOnly("gpt-5", nil)
	if err != nil {
		t.Fatalf("PickRecentlySuccessfulOnly() error = %v", err)
	}
	if picked != paid {
		t.Fatalf("PickRecentlySuccessfulOnly() = %s, want paid primary tier", picked.GetEmail())
	}
}
