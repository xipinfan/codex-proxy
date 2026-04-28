# Account Model Block Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add runtime-only account-model blocking after three qualifying model access failures.

**Architecture:** Store model block state on `auth.Account` under the existing account mutex. Selection remains account-list based, but all model-aware pick paths skip accounts blocked for the requested model. Executor passes model and error body into the existing handler error hook so the handler can classify model-access failures without changing storage or config.

**Tech Stack:** Go, existing `internal/auth`, `internal/executor`, `internal/handler`, `go test`.

---

## File Structure

- Modify `internal/auth/types.go`: add `modelBlockState`, constants, model normalization, account helpers for failure recording, success clearing, blocking, and availability.
- Modify `internal/auth/selector.go`: make selector filtering model-aware while keeping `Selector.Pick(model, accounts)` unchanged.
- Modify `internal/auth/manager.go`: apply model-aware filtering in direct manager pick loops and expose a manager helper that records qualifying model access failures and invalidates selector cache on first block transition.
- Modify `internal/executor/codex.go`: extend `RetryConfig.OnAfterUpstreamErrFn` signature and call it with model plus error body; clear account-model state on successful upstream open.
- Modify `internal/handler/proxy.go`: update the callback implementation to call the manager helper and keep existing cache invalidation for cooldown statuses.
- Create `internal/auth/model_block_test.go`: unit tests for blocking threshold, per-model isolation, status classification, success reset, and expiry.

## Task 1: Account Model Block State

**Files:**
- Create: `internal/auth/model_block_test.go`
- Modify: `internal/auth/types.go`

- [ ] **Step 1: Write failing tests for account-local block state**

Add tests that call the desired API directly:

```go
func TestAccountModelBlockAfterThreeQualifyingFailures(t *testing.T) {
	acc := &Account{}
	for i := 0; i < 2; i++ {
		blocked := acc.RecordModelAccessFailure("gpt-5.5", time.Date(2026, 4, 28, 0, 0, i, 0, time.UTC))
		if blocked {
			t.Fatalf("failure %d blocked too early", i+1)
		}
		if acc.IsModelBlocked("gpt-5.5", time.Date(2026, 4, 28, 0, 0, i, 0, time.UTC)) {
			t.Fatalf("model blocked before threshold")
		}
	}
	if !acc.RecordModelAccessFailure("gpt-5.5", time.Date(2026, 4, 28, 0, 0, 3, 0, time.UTC)) {
		t.Fatalf("third qualifying failure should create a new block")
	}
	if !acc.IsModelBlocked("gpt-5.5", time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("model should be blocked during 7 day window")
	}
	if acc.IsModelBlocked("gpt-5.5", time.Date(2026, 5, 6, 0, 0, 4, 0, time.UTC)) {
		t.Fatalf("model block should expire after 7 days")
	}
}

func TestAccountModelBlockIsPerModelAndSuccessClears(t *testing.T) {
	acc := &Account{}
	now := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		acc.RecordModelAccessFailure("gpt-5.5", now.Add(time.Duration(i)*time.Second))
	}
	if !acc.IsModelBlocked("gpt-5.5", now.Add(time.Hour)) {
		t.Fatalf("gpt-5.5 should be blocked")
	}
	if acc.IsModelBlocked("gpt-5.4", now.Add(time.Hour)) {
		t.Fatalf("different model should remain available")
	}
	acc.ClearModelAccessFailure("gpt-5.5")
	if acc.IsModelBlocked("gpt-5.5", now.Add(time.Hour)) {
		t.Fatalf("success clear should unblock the model")
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/auth -run 'TestAccountModelBlock'`

Expected: FAIL because `RecordModelAccessFailure`, `IsModelBlocked`, and `ClearModelAccessFailure` do not exist.

- [ ] **Step 3: Implement minimal account-local state**

Add runtime-only state and helpers in `internal/auth/types.go`:

```go
const (
	modelAccessFailureThreshold = 3
	modelAccessBlockDuration    = 7 * 24 * time.Hour
)

type modelBlockState struct {
	failures     int
	blockedUntil time.Time
}

// Account gains:
modelBlocks map[string]modelBlockState
```

Implement `normalizeModelBlockKey`, `RecordModelAccessFailure`, `ClearModelAccessFailure`, and `IsModelBlocked`.

- [ ] **Step 4: Run tests and verify GREEN**

Run: `go test ./internal/auth -run 'TestAccountModelBlock'`

Expected: PASS.

## Task 2: Model-Aware Selection

**Files:**
- Modify: `internal/auth/model_block_test.go`
- Modify: `internal/auth/selector.go`
- Modify: `internal/auth/manager.go`

- [ ] **Step 1: Write failing tests for selection skip behavior**

Add tests:

```go
func TestPickExcludingSkipsBlockedAccountOnlyForRequestedModel(t *testing.T) {
	m := newPolicyTestManager(t)
	blocked := addPolicyTestAccount(m, "blocked@example.com")
	other := addPolicyTestAccount(m, "other@example.com")
	now := time.Now()
	for i := 0; i < 3; i++ {
		blocked.RecordModelAccessFailure("gpt-5.5", now)
	}

	picked, err := m.PickExcluding("gpt-5.5", nil)
	if err != nil {
		t.Fatalf("pick blocked model: %v", err)
	}
	if picked != other {
		t.Fatalf("expected other account for blocked model, got %s", picked.GetEmail())
	}

	picked, err = m.PickExcluding("gpt-5.4", nil)
	if err != nil {
		t.Fatalf("pick different model: %v", err)
	}
	if picked != blocked {
		t.Fatalf("blocked account should remain eligible for other models")
	}
}

func TestPickRecentlySuccessfulSkipsModelBlockedAccount(t *testing.T) {
	m := newPolicyTestManager(t)
	blocked := addPolicyTestAccount(m, "recent-blocked@example.com")
	other := addPolicyTestAccount(m, "recent-other@example.com")
	blocked.lastSuccessUnixMs.Store(time.Now().UnixMilli())
	other.lastSuccessUnixMs.Store(time.Now().Add(-time.Minute).UnixMilli())
	for i := 0; i < 3; i++ {
		blocked.RecordModelAccessFailure("gpt-5.5", time.Now())
	}

	picked, err := m.PickRecentlySuccessful("gpt-5.5", nil)
	if err != nil {
		t.Fatalf("pick recently successful: %v", err)
	}
	if picked != other {
		t.Fatalf("expected non-blocked recent account, got %s", picked.GetEmail())
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/auth -run 'TestPick.*Blocked'`

Expected: FAIL because selection still ignores model block state.

- [ ] **Step 3: Implement model-aware filtering**

Change helper signatures in `internal/auth/selector.go`:

```go
func accountPickableAt(nowMs int64, model string, acc *Account) bool
func filterAvailable(model string, accounts []*Account) []*Account
```

Call `acc.IsModelBlocked(model, time.UnixMilli(nowMs))` after account-level availability passes. Update all selector and manager pick paths listed in the spec.

- [ ] **Step 4: Run tests and verify GREEN**

Run: `go test ./internal/auth -run 'TestPick.*Blocked|TestGetStatsReportsExpiredCooldownAsPickable|TestPickRecentlySuccessfulUsesExpiredCooldownAccount'`

Expected: PASS.

## Task 3: Failure Classification and Error Hook

**Files:**
- Modify: `internal/auth/model_block_test.go`
- Modify: `internal/auth/manager.go`
- Modify: `internal/executor/codex.go`
- Modify: `internal/handler/proxy.go`

- [ ] **Step 1: Write failing tests for manager classification**

Add tests:

```go
func TestRecordModelFailureIfAccessErrorOnlyBlocksQualifyingErrors(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "qualifying@example.com")
	body := []byte(`{"error":{"message":"You do not have access to model gpt-5.5"}}`)
	for i := 0; i < 3; i++ {
		m.RecordModelFailureIfAccessError(acc, "gpt-5.5", 403, body)
	}
	if !acc.IsModelBlocked("gpt-5.5", time.Now()) {
		t.Fatalf("qualifying model access errors should block model")
	}
}

func TestRecordModelFailureIfAccessErrorIgnoresQuotaAndServerErrors(t *testing.T) {
	m := newPolicyTestManager(t)
	acc := addPolicyTestAccount(m, "ignored@example.com")
	body := []byte(`{"error":{"message":"rate limited for model gpt-5.5"}}`)
	for _, status := range []int{401, 429, 500} {
		for i := 0; i < 3; i++ {
			m.RecordModelFailureIfAccessError(acc, "gpt-5.5", status, body)
		}
	}
	if acc.IsModelBlocked("gpt-5.5", time.Now()) {
		t.Fatalf("non-qualifying statuses should not block model")
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/auth -run 'TestRecordModelFailureIfAccessError'`

Expected: FAIL because `RecordModelFailureIfAccessError` does not exist.

- [ ] **Step 3: Implement classification and hook signature**

Add `Manager.RecordModelFailureIfAccessError(acc, model string, statusCode int, errBody []byte)` in `internal/auth/manager.go`. It should return without action unless status is `400` or `403` and the lowercased summary has model context plus access/availability language. If `acc.RecordModelAccessFailure` returns true, log a warning and call `m.InvalidateSelectorCache()`.

Change `executor.RetryConfig.OnAfterUpstreamErrFn` to:

```go
OnAfterUpstreamErrFn func(acc *auth.Account, model string, statusCode int, errBody []byte)
```

Pass `model` and `errBody` from `sendWithRetry`. Update `internal/handler/proxy.go` callback to call the new manager helper and preserve existing cooldown cache invalidation.

- [ ] **Step 4: Run tests and verify GREEN**

Run: `go test ./internal/auth -run 'TestRecordModelFailureIfAccessError|TestAccountModelBlock|TestPick.*Blocked'`

Expected: PASS.

## Task 4: Success Reset and Full Verification

**Files:**
- Modify: `internal/executor/codex.go`
- Test: all touched packages

- [ ] **Step 1: Add failing integration-level expectation if needed**

If no existing executor test can cheaply prove success reset through the executor without network-heavy setup, rely on the account-level success-clear test from Task 1 and implement the executor call directly at each successful upstream open.

- [ ] **Step 2: Clear model failure state on successful upstream response**

In `sendWithRetry`, just before returning a 2xx response, call:

```go
account.ClearModelAccessFailure(model)
```

This is intentionally tied to successful upstream open, while existing `RecordSuccess` remains responsible for completed response accounting.

- [ ] **Step 3: Run focused tests**

Run:

```powershell
go test ./internal/auth ./internal/executor ./internal/handler
```

Expected: PASS.

- [ ] **Step 4: Run full Go test suite**

Run:

```powershell
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Review git diff**

Run:

```powershell
git diff --stat
git diff -- internal/auth/types.go internal/auth/selector.go internal/auth/manager.go internal/executor/codex.go internal/handler/proxy.go internal/auth/model_block_test.go
```

Expected: only runtime model-block changes, tests, and no storage/config/UI changes.
