package auth

import (
	"testing"
	"time"
)

func TestSessionAffinityStoreBindLookupAndExpiry(t *testing.T) {
	now := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	store := newSessionAffinityStore(30 * time.Minute)

	store.Bind("session-a", "account-a", now)

	if got, ok := store.Lookup("session-a", now.Add(time.Minute)); !ok || got != "account-a" {
		t.Fatalf("Lookup() = (%q, %v), want account-a true", got, ok)
	}
	if _, ok := store.Lookup("session-a", now.Add(31*time.Minute)); ok {
		t.Fatal("expired affinity should miss")
	}
}

func TestSessionAffinityStoreDisabledTTLMisses(t *testing.T) {
	store := newSessionAffinityStore(0)
	store.Bind("session-a", "account-a", time.Now())
	if _, ok := store.Lookup("session-a", time.Now()); ok {
		t.Fatal("disabled store should not retain affinity")
	}
}

func TestSessionAffinityStoreEvictOnlyMatchingBinding(t *testing.T) {
	now := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	store := newSessionAffinityStore(time.Hour)
	store.Bind("session-a", "account-a", now)
	store.Bind("session-a", "account-b", now.Add(time.Second))

	store.EvictIfBound("session-a", "account-a")

	if got, ok := store.Lookup("session-a", now.Add(time.Minute)); !ok || got != "account-b" {
		t.Fatalf("Lookup() after stale eviction = (%q, %v), want account-b true", got, ok)
	}
}

func newSessionAffinityPolicyTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(t.TempDir(), nil, "", 3000, NewRoundRobinSelector(), false, &ManagerOptions{
		SessionAffinityTTL: time.Hour,
	})
}

func TestManagerPickSessionAccountReusesSuccessfulBinding(t *testing.T) {
	m := newSessionAffinityPolicyTestManager(t)
	m.selector = NewRoundRobinSelector()
	first := addPolicyTestAccount(m, "first@example.com")
	second := addPolicyTestAccount(m, "second@example.com")

	m.BindSessionAccount("session-a", first)

	picked, err := m.PickSessionAccount("session-a", "gpt-5.5", nil)
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != first || picked == second {
		t.Fatalf("picked %s, want sticky first account", picked.GetEmail())
	}
}

func TestManagerPickSessionAccountSkipsExcludedBinding(t *testing.T) {
	m := newSessionAffinityPolicyTestManager(t)
	m.selector = NewFillFirstSelector()
	first := addPolicyTestAccount(m, "a-first@example.com")
	second := addPolicyTestAccount(m, "b-second@example.com")
	m.BindSessionAccount("session-a", first)

	picked, err := m.PickSessionAccount("session-a", "gpt-5.5", map[string]bool{first.FilePath: true})
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != second {
		t.Fatalf("picked %s, want replacement account", picked.GetEmail())
	}
}

func TestManagerPickSessionAccountEvictsDisabledBinding(t *testing.T) {
	m := newSessionAffinityPolicyTestManager(t)
	m.selector = NewFillFirstSelector()
	first := addPolicyTestAccount(m, "a-disabled@example.com")
	second := addPolicyTestAccount(m, "b-replacement@example.com")
	m.BindSessionAccount("session-a", first)
	first.SetDisabled(nil)

	picked, err := m.PickSessionAccount("session-a", "gpt-5.5", nil)
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != second {
		t.Fatalf("picked %s, want enabled replacement account", picked.GetEmail())
	}
	if _, ok := m.sessionAffinity.Lookup("session-a", time.Now()); ok {
		t.Fatal("disabled bound account should be evicted")
	}
}

func TestManagerPickSessionAccountEvictsModelBlockedBinding(t *testing.T) {
	m := newSessionAffinityPolicyTestManager(t)
	m.selector = NewFillFirstSelector()
	first := addPolicyTestAccount(m, "a-model-blocked@example.com")
	second := addPolicyTestAccount(m, "b-model-replacement@example.com")
	m.BindSessionAccount("session-a", first)
	for i := 0; i < 3; i++ {
		first.RecordModelAccessFailure("gpt-5.5", time.Now().Add(time.Duration(i)*time.Second))
	}

	picked, err := m.PickSessionAccount("session-a", "gpt-5.5", nil)
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != second {
		t.Fatalf("picked %s, want model-capable replacement account", picked.GetEmail())
	}
	if _, ok := m.sessionAffinity.Lookup("session-a", time.Now()); ok {
		t.Fatal("model-blocked bound account should be evicted")
	}
}
