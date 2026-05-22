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

func TestSessionAffinityKeyLogLabel(t *testing.T) {
	if got := sessionAffinityKeyLogLabel(""); got != "" {
		t.Fatalf("sessionAffinityKeyLogLabel(empty) = %q, want empty", got)
	}

	got := sessionAffinityKeyLogLabel("explicit-session-with-user-context")
	if got == "" {
		t.Fatal("sessionAffinityKeyLogLabel() returned empty label")
	}
	if got == "explicit-session-with-user-context" {
		t.Fatal("sessionAffinityKeyLogLabel() should not expose the raw session key")
	}
	if got != sessionAffinityKeyLogLabel("explicit-session-with-user-context") {
		t.Fatal("sessionAffinityKeyLogLabel() changed for the same session key")
	}
}

func TestSessionAffinityStoreBindCleansExpiredEntries(t *testing.T) {
	now := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	store := newSessionAffinityStore(time.Minute)
	store.Bind("session-expired", "account-a", now)

	store.Bind("session-live", "account-b", now.Add(2*time.Minute))

	if _, ok := store.entries["session-expired"]; ok {
		t.Fatal("Bind() should opportunistically clear expired affinity")
	}
	if got, ok := store.Lookup("session-live", now.Add(2*time.Minute)); !ok || got != "account-b" {
		t.Fatalf("live Lookup() after cleanup = (%q, %v), want account-b true", got, ok)
	}
}

func TestSessionAffinityStoreBindCleanupIsBoundedByTTL(t *testing.T) {
	now := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	store := newSessionAffinityStore(time.Minute)
	store.Bind("session-live", "account-a", now)
	store.entries["session-expired"] = sessionAffinityEntry{
		accountKey: "account-expired",
		expiresAt:  now.Add(-time.Second),
	}

	store.Bind("session-next", "account-b", now.Add(30*time.Second))

	if _, ok := store.entries["session-expired"]; !ok {
		t.Fatal("Bind() should not sweep on every insert within cleanup interval")
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

func TestManagerPickSessionAccountHonorsFreeFallbackPoolSemantics(t *testing.T) {
	resetFreeAccountPolicy(t)

	m := newSessionAffinityPolicyTestManager(t)
	m.selector = NewFillFirstSelector()
	free := addPolicyTestAccount(m, "a-free-bound@example.com")
	free.Token.PlanType = "free"
	paid := addPolicyTestAccount(m, "b-paid-replacement@example.com")
	paid.Token.PlanType = "plus"
	m.BindSessionAccount("session-a", free)

	picked, err := m.PickSessionAccount("session-a", "gpt-5", nil)
	if err != nil {
		t.Fatalf("PickSessionAccount() error = %v", err)
	}
	if picked != paid {
		t.Fatalf("picked %s, want paid replacement while free is fallback-only", picked.GetEmail())
	}
	if _, ok := m.sessionAffinity.Lookup("session-a", time.Now()); ok {
		t.Fatal("fallback-hidden free binding should be evicted")
	}
}

func TestManagerLookupAvailableSessionAccountUsesCurrentIndexIdentity(t *testing.T) {
	m := newSessionAffinityPolicyTestManager(t)
	stale := addPolicyTestAccount(m, "stale-snapshot@example.com")

	m.mu.Lock()
	delete(m.accountIndex, stale.FilePath)
	m.mu.Unlock()

	if got := m.lookupAvailableSessionAccount(stale.FilePath, "gpt-5"); got != nil {
		t.Fatalf("lookupAvailableSessionAccount() = %p, want nil for account missing from index", got)
	}
}
