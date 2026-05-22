# Session Account Affinity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep repeated Codex conversation requests on the same eligible account so upstream prompt cache locality improves without replacing current retry and selector fallbacks.

**Architecture:** Add a process-local TTL affinity store owned by `auth.Manager`, then expose session-aware bind, evict, and pick helpers that reuse current account eligibility checks before falling back to `PickExcluding`. Let the executor derive a request-scoped session key after request conversion because it has the final Codex body, pass manager callbacks through `RetryConfig`, and bind only after upstream success while evicting compare-safely when the current request excludes a bound account.

**Tech Stack:** Go, `fasthttp`, `net/http`, YAML config tests, existing `auth` and `executor` unit tests.

---

## File Map

- Create `internal/auth/session_affinity.go`: process-local TTL affinity store and manager-facing bind, lookup, evict helpers.
- Create `internal/auth/session_affinity_test.go`: store TTL, compare-and-evict, session-aware pick behavior.
- Modify `internal/auth/manager.go`: initialize the store from `ManagerOptions`, resolve bound accounts against current account snapshots, and expose session-aware manager methods.
- Modify `internal/executor/codex.go`: derive account-independent session keys, carry request-scoped affinity callbacks in `RetryConfig`, bind after upstream success, evict when retries exclude a bound account, and reuse the session key for `Session_id`.
- Modify `internal/executor/codex_retry_test.go`: session-key derivation, retry binding, eviction, explicit session header coverage.
- Modify `internal/handler/proxy.go`: pass inbound explicit session IDs and manager affinity callbacks into the otherwise cached retry config.
- Modify `internal/config/config.go`, `internal/config/config_test.go`, `main.go`, `config.example.yaml`, and `docs/CONFIGURATION.md`: expose `session-affinity-ttl-sec`.

### Task 1: Add The TTL Affinity Store

**Files:**
- Create: `internal/auth/session_affinity.go`
- Create: `internal/auth/session_affinity_test.go`

- [ ] **Step 1: Write failing store tests**

Add focused tests for success bind, expiry, disabled TTL, and compare-and-evict:

```go
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
```

- [ ] **Step 2: Run the store tests to verify they fail**

Run:

```powershell
go test ./internal/auth -run 'TestSessionAffinityStore' -count=1
```

Expected: FAIL because `newSessionAffinityStore` does not exist.

- [ ] **Step 3: Implement the minimal store**

Create a mutex-protected process-local store:

```go
type sessionAffinityEntry struct {
	accountKey string
	expiresAt  time.Time
}

type sessionAffinityStore struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]sessionAffinityEntry
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
	s.entries[sessionKey] = sessionAffinityEntry{accountKey: accountKey, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()
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
```

Keep cleanup lazy in this first store implementation; add bounded cleanup only if tests or review show a real hot-path growth concern.

- [ ] **Step 4: Run the store tests to verify they pass**

Run:

```powershell
go test ./internal/auth -run 'TestSessionAffinityStore' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the store**

```powershell
git add internal/auth/session_affinity.go internal/auth/session_affinity_test.go
git commit -m "feat: add session affinity store" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

### Task 2: Make Manager Picking Session-Aware

**Files:**
- Modify: `internal/auth/manager.go`
- Modify: `internal/auth/session_affinity.go`
- Modify: `internal/auth/session_affinity_test.go`

- [ ] **Step 1: Write failing manager affinity tests**

Use the policy-test manager helpers and the stable fill-first selector to show sticky reuse, excluded-account fallback, stale account replacement, and model availability checks:

```go
func TestManagerPickSessionAccountReusesSuccessfulBinding(t *testing.T) {
	m := newPolicyTestManager(t)
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
	m := newPolicyTestManager(t)
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
```

Add cases where the bound account is disabled or model-blocked and verify the replacement is picked and the stale binding is no longer returned.

- [ ] **Step 2: Run manager affinity tests to verify they fail**

Run:

```powershell
go test ./internal/auth -run 'TestManagerPickSessionAccount' -count=1
```

Expected: FAIL because manager session affinity methods and options do not exist.

- [ ] **Step 3: Initialize affinity in manager options**

Extend options and manager state:

```go
type ManagerOptions struct {
    // existing fields...
    SessionAffinityTTL time.Duration
}

type Manager struct {
    // existing fields...
    sessionAffinity *sessionAffinityStore
}
```

Initialize it in `NewManager`:

```go
ttl := time.Duration(0)
if opts != nil {
    ttl = opts.SessionAffinityTTL
}
m.sessionAffinity = newSessionAffinityStore(ttl)
```

- [ ] **Step 4: Implement manager bind, evict, and pick helpers**

Use `FilePath` as the account key because current retry exclusion and snapshot lookup already treat it as the stable loaded identity:

```go
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
```

Implement `PickSessionAccount` by resolving the binding against `accountsPtr`, checking `excluded` and `accountPickableAt(time.Now().UnixMilli(), model, acc)`, evicting stale bindings, and falling back to `PickExcluding(model, excluded)`.

- [ ] **Step 5: Run auth tests to verify manager behavior**

Run:

```powershell
go test ./internal/auth -run 'TestSessionAffinityStore|TestManagerPickSessionAccount' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit manager affinity**

```powershell
git add internal/auth/manager.go internal/auth/session_affinity.go internal/auth/session_affinity_test.go
git commit -m "feat: add session-aware account picking" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

### Task 3: Thread Request Affinity Through Executor Retries

**Files:**
- Modify: `internal/executor/codex.go`
- Modify: `internal/executor/codex_retry_test.go`
- Modify: `internal/handler/proxy.go`

- [ ] **Step 1: Write failing session-key tests**

Split account-independent session key derivation away from the existing account-dependent session hint:

```go
func TestDeriveAffinitySessionKeyIgnoresAccountIdentity(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"hello"}]}`)

	first := deriveAffinitySessionKey("", body)
	if first == "" {
		t.Fatal("deriveAffinitySessionKey() returned empty key")
	}
	if first != deriveAffinitySessionKey("", body) {
		t.Fatal("fallback affinity key changed for same stable prefix")
	}
}

func TestDeriveAffinitySessionKeyPrefersExplicitHeader(t *testing.T) {
	body := []byte(`{"instructions":"prefix","input":[{"type":"message","content":"hello"}]}`)
	if got := deriveAffinitySessionKey(" explicit-session ", body); got != "explicit-session" {
		t.Fatalf("deriveAffinitySessionKey() = %q, want explicit-session", got)
	}
}
```

Update the `deriveSessionHint` tests so the new `Session_id` hint derives from the explicit or fallback affinity key rather than adding account identity back into the affinity key.

- [ ] **Step 2: Run session-key tests to verify they fail**

Run:

```powershell
go test ./internal/executor -run 'TestDeriveAffinitySessionKey|TestDeriveSessionHint|TestApplyCodexHeaders' -count=1
```

Expected: FAIL because `deriveAffinitySessionKey` does not exist or current hint expectations still include account identity.

- [ ] **Step 3: Add request-scoped affinity callbacks to RetryConfig**

Extend `RetryConfig` with request-scoped values and callbacks:

```go
type RetryConfig struct {
    // existing fields...
    ExplicitSessionID string
    PickSessionAccountFn func(sessionKey, model string, excluded map[string]bool) (*auth.Account, error)
    BindSessionAccountFn  func(sessionKey string, acc *auth.Account)
    EvictSessionAccountFn func(sessionKey string, acc *auth.Account)
}
```

Add a minimal helper in `sendWithRetry`:

```go
sessionKey := deriveAffinitySessionKey(rc.ExplicitSessionID, codexBody)

bindSuccess := func(acc *auth.Account) {
    if sessionKey != "" && rc.BindSessionAccountFn != nil && acc != nil {
        rc.BindSessionAccountFn(sessionKey, acc)
    }
}

evict := func(acc *auth.Account) {
    if sessionKey != "" && rc.EvictSessionAccountFn != nil && acc != nil {
        rc.EvictSessionAccountFn(sessionKey, acc)
    }
}
```

Call `bindSuccess(account)` immediately before returning a successful upstream response. Call `evict(account)` when a failed attempt adds the account to `excluded`, when quota precheck rejects it, and when token freshness rejects it.

- [ ] **Step 4: Write failing executor retry tests for bind and eviction**

Use `httptest` and callback counters:

```go
func TestSendWithRetryBindsSuccessfulSessionAccount(t *testing.T) {
	// one upstream 2xx response
	// rc.ExplicitSessionID = "session-a"
	// BindSessionAccountFn captures account
	// assert callback receives success account once
}

func TestSendWithRetryEvictsFailedBoundSessionAccountBeforeReplacement(t *testing.T) {
	// first account returns 429, second returns 2xx
	// EvictSessionAccountFn captures first account
	// BindSessionAccountFn captures second account
	// assert both callbacks match request session key
}
```

- [ ] **Step 5: Run retry tests to verify they fail before implementation**

Run:

```powershell
go test ./internal/executor -run 'TestSendWithRetryBindsSuccessfulSessionAccount|TestSendWithRetryEvictsFailedBoundSessionAccountBeforeReplacement' -count=1
```

Expected: FAIL because callbacks are not wired into the retry loop yet.

- [ ] **Step 6: Derive the session key before picking and use it for upstream session hints**

Make session derivation account-independent:

```go
func deriveAffinitySessionKey(explicit string, body []byte) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if len(body) == 0 {
		return ""
	}
	// Hash instructions prefix plus first input prefix.
}
```

Pass a stable derived key into `applyCodexHeaders` for `Session_id`. Keep `applyCodexHeaders` preference for explicit `X-Session-Id`.

- [ ] **Step 7: Use session-aware pick from the executor retry loop**

Update `pickForAttempt` so the normal primary pick path uses affinity when a session key and callback are available, while the healthy retry and last-attempt branches keep their existing behavior:

```go
pickPrimary := func(model string, excluded map[string]bool) (*auth.Account, error) {
	if sessionKey != "" && rc.PickSessionAccountFn != nil {
		return rc.PickSessionAccountFn(sessionKey, model, excluded)
	}
	return rc.PickFn(model, excluded)
}
```

Use `pickPrimary` where the current code calls `rc.PickFn` for the ordinary path and for the fallback after a failed healthy pick. Keep `HealthyPickFn`, `FallbackRecentPickFn`, and `LastAttemptPickFn` unchanged.

- [ ] **Step 8: Pass explicit session IDs and callbacks from handler**

Keep `buildRetryConfig()` as the cached base policy, then add a small helper in `ProxyHandler` that copies the cached config and attaches request-specific affinity fields:

```go
func (h *ProxyHandler) retryConfigForRequest(ctx *fasthttp.RequestCtx) executor.RetryConfig {
	rc := h.buildRetryConfig()
	rc.ExplicitSessionID = strings.TrimSpace(string(ctx.Request.Header.Peek("X-Session-Id")))
	rc.PickSessionAccountFn = h.manager.PickSessionAccount
	rc.BindSessionAccountFn = h.manager.BindSessionAccount
	rc.EvictSessionAccountFn = h.manager.EvictSessionAccount
	return rc
}
```

Use this helper for normal chat/responses/compact conversation paths. Do not wrap image generation or admin paths in the first version.

- [ ] **Step 9: Run executor and handler package tests**

Run:

```powershell
go test ./internal/executor ./internal/handler -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit retry integration**

```powershell
git add internal/executor/codex.go internal/executor/codex_retry_test.go internal/handler/proxy.go
git commit -m "feat: keep conversation retries account-affine" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

### Task 4: Add Configuration And Final Verification

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `main.go`
- Modify: `config.example.yaml`
- Modify: `docs/CONFIGURATION.md`

- [ ] **Step 1: Write failing config test**

```go
func TestLoadConfigDefaultsSessionAffinityTTLAndAllowsDisablingIt(t *testing.T) {
	dir := t.TempDir()
	defaultPath := filepath.Join(dir, "default.yaml")
	if err := os.WriteFile(defaultPath, []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write default config: %v", err)
	}
	cfg, err := LoadConfig(defaultPath)
	if err != nil {
		t.Fatalf("LoadConfig(default) error = %v", err)
	}
	if cfg.SessionAffinityTTLSec != 1800 {
		t.Fatalf("SessionAffinityTTLSec = %d, want 1800", cfg.SessionAffinityTTLSec)
	}

	disabledPath := filepath.Join(dir, "disabled.yaml")
	if err := os.WriteFile(disabledPath, []byte("session-affinity-ttl-sec: 0\n"), 0600); err != nil {
		t.Fatalf("write disabled config: %v", err)
	}
	cfg, err = LoadConfig(disabledPath)
	if err != nil {
		t.Fatalf("LoadConfig(disabled) error = %v", err)
	}
	if cfg.SessionAffinityTTLSec != 0 {
		t.Fatalf("SessionAffinityTTLSec = %d, want explicit 0", cfg.SessionAffinityTTLSec)
	}
}
```

- [ ] **Step 2: Run config test to verify it fails**

Run:

```powershell
go test ./internal/config -run TestLoadConfigDefaultsSessionAffinityTTLAndAllowsDisablingIt -count=1
```

Expected: FAIL because `SessionAffinityTTLSec` does not exist.

- [ ] **Step 3: Add config plumbing**

Add config field, default, and manager option wiring:

```go
SessionAffinityTTLSec int `yaml:"session-affinity-ttl-sec"`
```

```go
SessionAffinityTTLSec: 1800,
```

```go
SessionAffinityTTL: time.Duration(cfg.SessionAffinityTTLSec) * time.Second,
```

Preserve an explicit YAML zero so users can disable affinity.

- [ ] **Step 4: Document the new field**

Add to `config.example.yaml` near selector and retry settings:

```yaml
# 同一会话成功后优先复用同账号的粘性时长（秒），提升 Prompt Cache 局部性；0 关闭
# session-affinity-ttl-sec: 1800
```

Add a `docs/CONFIGURATION.md` row explaining that it is a process-local conversation cache-locality optimization.

- [ ] **Step 5: Run focused config and auth tests**

Run:

```powershell
go test ./internal/config ./internal/auth -count=1
```

Expected: PASS.

- [ ] **Step 6: Run full Go verification**

Run:

```powershell
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Review diff and commit**

Run:

```powershell
git diff --stat
git status --short
```

Then commit:

```powershell
git add internal/config/config.go internal/config/config_test.go main.go config.example.yaml docs/CONFIGURATION.md
git commit -m "docs: configure session account affinity" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

## Final Manual Check

- [ ] Start the proxy with at least two paid accounts and `log-cache-metrics: true`.
- [ ] Send several continuation requests from one conversation with the same explicit `X-Session-Id` or stable request prefix.
- [ ] Confirm request summary logs show the same account until that account is deliberately cooled down or quota-rejected.
- [ ] Compare `upstream_cache_metric` `cached_tokens` and `hit_ratio` against the pre-affinity baseline for that repeated conversation.
