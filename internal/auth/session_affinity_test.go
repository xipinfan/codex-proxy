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
