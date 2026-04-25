package auth

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newPolicyTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(t.TempDir(), nil, "", 3000, NewRoundRobinSelector(), false, nil)
	return m
}

func addPolicyTestAccount(m *Manager, email string) *Account {
	acc := &Account{
		FilePath: filepath.Join("test", email+".json"),
		Token: TokenData{
			Email:        email,
			RefreshToken: "rt-" + email,
		},
		Status: StatusActive,
	}
	acc.atomicStatus.Store(int32(StatusActive))
	m.mu.Lock()
	m.accounts = append(m.accounts, acc)
	m.accountIndex[acc.FilePath] = acc
	m.publishSnapshot()
	m.mu.Unlock()
	return acc
}

func TestApplyQuotaUsageLegacyUsesCooldownFor4xx(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "legacy@example.com")

	out := m.applyQuotaUsageHTTPLegacy(acc, 401, -1)
	if out != QuotaApplyCooldown {
		t.Fatalf("expected cooldown outcome, got %v", out)
	}
	if !m.AccountInPool(acc) {
		t.Fatalf("account should remain in pool")
	}
	if acc.GetStats().Status != "cooldown" {
		t.Fatalf("expected account cooldown status, got %s", acc.GetStats().Status)
	}
}

func TestHandleRefreshHTTPErrorNonHTTPKeepsAccount(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "refresh@example.com")

	recovered, out := m.handleRefreshHTTPError(context.Background(), acc, acc.GetEmail(), fmt.Errorf("dial timeout"), true)
	if recovered {
		t.Fatalf("unexpected recovered=true")
	}
	if out != QuotaApplyCooldown {
		t.Fatalf("expected cooldown outcome, got %v", out)
	}
	if !m.AccountInPool(acc) {
		t.Fatalf("account should remain in pool")
	}
}

func TestBackgroundRefreshFailureDoesNotCooldownFreshToken(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "fresh-refresh@example.com")
	acc.Token.AccessToken = "access-token"
	acc.Token.Expire = time.Now().Add(time.Hour).Format(time.RFC3339)
	acc.SyncAccessExpireFromToken()

	recovered, out := m.handleRefreshHTTPError(context.Background(), acc, acc.GetEmail(), fmt.Errorf("dial timeout"), true)
	if recovered {
		t.Fatalf("unexpected recovered=true")
	}
	if out != QuotaApplyNone {
		t.Fatalf("expected no scheduling cooldown for fresh token, got %v", out)
	}
	stats := acc.GetStats()
	if stats.Status != "active" || !stats.Pickable {
		t.Fatalf("expected fresh-token account to remain pickable, status=%s pickable=%v", stats.Status, stats.Pickable)
	}
}
