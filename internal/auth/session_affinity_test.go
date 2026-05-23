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

func TestResponseContinuationStoreBindLookupAndExpiry(t *testing.T) {
	now := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	store := newResponseContinuationStore(30 * time.Minute)

	store.Bind("resp_1", "session-a", "account-a", now)

	got, ok := store.Lookup("resp_1", now.Add(time.Minute))
	if !ok || got.sessionKey != "session-a" || got.accountKey != "account-a" {
		t.Fatalf("Lookup() = (%+v, %v), want session-a/account-a true", got, ok)
	}
	if _, ok := store.Lookup("resp_1", now.Add(31*time.Minute)); ok {
		t.Fatal("expired continuation should miss")
	}
}

func TestManagerPickSessionAccountReusesSuccessfulBinding(t *testing.T) {
	m := newPolicyTestManager(t)
	m.sessionAffinity = newSessionAffinityStore(time.Hour)
	first := addPolicyTestAccount(m, "first@example.com")
	_ = addPolicyTestAccount(m, "second@example.com")

	m.BindSessionAccount("session-a", first)

	picked, err := m.PickSessionAccount("session-a", "gpt-5.5", nil)
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != first {
		t.Fatalf("picked %s, want sticky first account", picked.GetEmail())
	}
}

func TestManagerPickSessionAccountSkipsExcludedBinding(t *testing.T) {
	m := newPolicyTestManager(t)
	m.sessionAffinity = newSessionAffinityStore(time.Hour)
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

func TestManagerPickSessionAccountEvictsUnavailableBinding(t *testing.T) {
	m := newPolicyTestManager(t)
	m.sessionAffinity = newSessionAffinityStore(time.Hour)
	first := addPolicyTestAccount(m, "unavailable@example.com")
	second := addPolicyTestAccount(m, "replacement@example.com")
	m.BindSessionAccount("session-a", first)
	first.SetCooldown(time.Minute)

	picked, err := m.PickSessionAccount("session-a", "gpt-5.5", nil)
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != second {
		t.Fatalf("picked %s, want replacement account", picked.GetEmail())
	}
	if got, ok := m.sessionAffinity.Lookup("session-a", time.Now()); ok && got == first.FilePath {
		t.Fatal("stale binding should be evicted")
	}
}

func TestManagerResolveSessionKeyFromResponseIDRebindsAffinity(t *testing.T) {
	m := newPolicyTestManager(t)
	m.sessionAffinity = newSessionAffinityStore(time.Hour)
	m.responseContinuation = newResponseContinuationStore(time.Hour)
	first := addPolicyTestAccount(m, "reply@example.com")

	m.BindResponseContinuation("resp_123", "session-a", first)

	if got := m.ResolveSessionKeyFromResponseID("resp_123"); got != "session-a" {
		t.Fatalf("ResolveSessionKeyFromResponseID() = %q, want session-a", got)
	}
	if picked, err := m.PickSessionAccount("session-a", "gpt-5.5", nil); err != nil || picked != first {
		t.Fatalf("PickSessionAccount() = (%v, %v), want sticky bound account", picked, err)
	}
}
