package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
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
	sessionLabel := sessionAffinityKeyLogLabel(sessionKey)
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[sessionKey]
	if !ok {
		log.WithFields(log.Fields{
			"session": sessionLabel,
		}).Debug("session_affinity_miss")
		return "", false
	}
	if !entry.expiresAt.After(now) {
		delete(s.entries, sessionKey)
		log.WithFields(log.Fields{
			"session": sessionLabel,
			"account": entry.accountKey,
		}).Debug("session_affinity_expired")
		return "", false
	}
	log.WithFields(log.Fields{
		"session": sessionLabel,
		"account": entry.accountKey,
	}).Debug("session_affinity_lookup")
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
	log.WithFields(log.Fields{
		"session": sessionAffinityKeyLogLabel(sessionKey),
		"account": accountKey,
		"ttl":     s.ttl.String(),
	}).Debug("session_affinity_bind")
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
	sessionLabel := sessionAffinityKeyLogLabel(sessionKey)
	s.mu.Lock()
	if entry, ok := s.entries[sessionKey]; ok && entry.accountKey == accountKey {
		delete(s.entries, sessionKey)
		s.mu.Unlock()
		log.WithFields(log.Fields{
			"session": sessionLabel,
			"account": accountKey,
		}).Debug("session_affinity_evict")
		return
	}
	s.mu.Unlock()
}

func sessionAffinityKeyLogLabel(sessionKey string) string {
	if sessionKey = strings.TrimSpace(sessionKey); sessionKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sessionKey))
	return hex.EncodeToString(sum[:])[:12]
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
	sessionLabel := sessionAffinityKeyLogLabel(sessionKey)
	if accountKey, ok := m.sessionAffinity.Lookup(sessionKey, now); ok {
		if excluded != nil && excluded[accountKey] {
			log.WithFields(log.Fields{
				"session": sessionLabel,
				"account": accountKey,
				"model":   model,
				"reason":  "excluded",
			}).Debug("session_affinity_reject")
			m.sessionAffinity.EvictIfBound(sessionKey, accountKey)
			return m.PickExcluding(model, excluded)
		}

		if acc := m.lookupAvailableSessionAccount(accountKey, model); acc != nil {
			log.WithFields(log.Fields{
				"session": sessionLabel,
				"account": accountKey,
				"model":   model,
			}).Debug("session_affinity_pick")
			return acc, nil
		}
		log.WithFields(log.Fields{
			"session": sessionLabel,
			"account": accountKey,
			"model":   model,
			"reason":  "not_available",
		}).Debug("session_affinity_reject")
		m.sessionAffinity.EvictIfBound(sessionKey, accountKey)
	}

	if sessionLabel != "" {
		log.WithFields(log.Fields{
			"session": sessionLabel,
			"model":   model,
		}).Debug("session_affinity_fallback_pick")
	}
	return m.PickExcluding(model, excluded)
}

func (m *Manager) lookupAvailableSessionAccount(accountKey, model string) *Account {
	if m == nil || strings.TrimSpace(accountKey) == "" {
		return nil
	}
	m.mu.RLock()
	acc := m.accountIndex[accountKey]
	var accounts []*Account
	if acc != nil {
		accounts = make([]*Account, len(m.accounts))
		copy(accounts, m.accounts)
	}
	m.mu.RUnlock()
	if acc == nil {
		return nil
	}
	for _, available := range filterAvailable(model, accounts) {
		if available == acc {
			return acc
		}
	}
	return nil
}
