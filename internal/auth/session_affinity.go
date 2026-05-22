package auth

import (
	"strings"
	"sync"
	"time"
)

type sessionAffinityEntry struct {
	accountKey string
	expiresAt  time.Time
}

type sessionAffinityStore struct {
	mu          sync.Mutex
	ttl         time.Duration
	entries     map[string]sessionAffinityEntry
	lastCleanup time.Time
}

func newSessionAffinityStore(ttl time.Duration) *sessionAffinityStore {
	return &sessionAffinityStore{
		ttl:     ttl,
		entries: make(map[string]sessionAffinityEntry),
	}
}

func (s *sessionAffinityStore) Lookup(sessionKey string, now time.Time) (string, bool) {
	if s == nil || s.ttl <= 0 || strings.TrimSpace(sessionKey) == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[sessionKey]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.After(now) {
		delete(s.entries, sessionKey)
		return "", false
	}
	return entry.accountKey, true
}

func (s *sessionAffinityStore) Bind(sessionKey, accountKey string, now time.Time) {
	if s == nil || s.ttl <= 0 || strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(accountKey) == "" {
		return
	}
	s.mu.Lock()
	s.cleanupExpiredLocked(now)
	s.entries[sessionKey] = sessionAffinityEntry{accountKey: accountKey, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()
}

func (s *sessionAffinityStore) cleanupExpiredLocked(now time.Time) {
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < s.ttl {
		return
	}
	for sessionKey, entry := range s.entries {
		if !entry.expiresAt.After(now) {
			delete(s.entries, sessionKey)
		}
	}
	s.lastCleanup = now
}

func (s *sessionAffinityStore) EvictIfBound(sessionKey, accountKey string) {
	if s == nil || sessionKey == "" || accountKey == "" {
		return
	}
	s.mu.Lock()
	if entry, ok := s.entries[sessionKey]; ok && entry.accountKey == accountKey {
		delete(s.entries, sessionKey)
	}
	s.mu.Unlock()
}

func (m *Manager) BindSessionAccount(sessionKey string, acc *Account) {
	if m == nil || acc == nil {
		return
	}
	m.sessionAffinity.Bind(sessionKey, acc.FilePath, time.Now())
}

func (m *Manager) EvictSessionAccount(sessionKey string, acc *Account) {
	if m == nil || acc == nil {
		return
	}
	m.sessionAffinity.EvictIfBound(sessionKey, acc.FilePath)
}

func (m *Manager) PickSessionAccount(sessionKey, model string, excluded map[string]bool) (*Account, error) {
	now := time.Now()
	if accountKey, ok := m.sessionAffinity.Lookup(sessionKey, now); ok {
		if excluded != nil && excluded[accountKey] {
			m.sessionAffinity.EvictIfBound(sessionKey, accountKey)
			return m.PickExcluding(model, excluded)
		}

		if acc := m.lookupSnapshotAccount(accountKey); acc != nil && accountPickableAt(now.UnixMilli(), model, acc) {
			return acc, nil
		}
		m.sessionAffinity.EvictIfBound(sessionKey, accountKey)
	}

	return m.PickExcluding(model, excluded)
}

func (m *Manager) lookupSnapshotAccount(accountKey string) *Account {
	if m == nil || strings.TrimSpace(accountKey) == "" {
		return nil
	}
	m.mu.RLock()
	acc := m.accountIndex[accountKey]
	m.mu.RUnlock()
	return acc
}
