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
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]sessionAffinityEntry
}

type responseContinuationEntry struct {
	sessionKey string
	accountKey string
	expiresAt  time.Time
}

type responseContinuationStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]responseContinuationEntry
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
	s.entries[sessionKey] = sessionAffinityEntry{
		accountKey: accountKey,
		expiresAt:  now.Add(s.ttl),
	}
	s.mu.Unlock()
}

func (s *sessionAffinityStore) EvictIfBound(sessionKey, accountKey string) {
	if s == nil || strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(accountKey) == "" {
		return
	}
	s.mu.Lock()
	if entry, ok := s.entries[sessionKey]; ok && entry.accountKey == accountKey {
		delete(s.entries, sessionKey)
	}
	s.mu.Unlock()
}

func newResponseContinuationStore(ttl time.Duration) *responseContinuationStore {
	return &responseContinuationStore{
		ttl:     ttl,
		entries: make(map[string]responseContinuationEntry),
	}
}

func (s *responseContinuationStore) Lookup(responseID string, now time.Time) (responseContinuationEntry, bool) {
	if s == nil || s.ttl <= 0 || strings.TrimSpace(responseID) == "" {
		return responseContinuationEntry{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[responseID]
	if !ok {
		return responseContinuationEntry{}, false
	}
	if !entry.expiresAt.After(now) {
		delete(s.entries, responseID)
		return responseContinuationEntry{}, false
	}
	return entry, true
}

func (s *responseContinuationStore) Bind(responseID, sessionKey, accountKey string, now time.Time) {
	if s == nil || s.ttl <= 0 || strings.TrimSpace(responseID) == "" || strings.TrimSpace(sessionKey) == "" || strings.TrimSpace(accountKey) == "" {
		return
	}
	s.mu.Lock()
	s.entries[responseID] = responseContinuationEntry{
		sessionKey: sessionKey,
		accountKey: accountKey,
		expiresAt:  now.Add(s.ttl),
	}
	s.mu.Unlock()
}
