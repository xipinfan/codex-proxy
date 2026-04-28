# Account Model Block Design

## Context

`codex-proxy` already passes the requested model name through the account selection path:

- `ProxyHandler` builds a cached `executor.RetryConfig`.
- `RetryConfig.PickFn` calls `Manager.PickExcluding(model, excluded)`.
- `Manager` passes `model` into the configured selector.
- Existing selectors currently ignore `model` and only filter by account-level availability.

The current account-level failure handling is intentionally broad: 429 can cool down an account, 401 can trigger token recovery, and hard failures may remove or disable the account. That is too coarse for plan-limited models such as `gpt-5.5`, where a free account may fail for that model while still being valid for other models.

## Goal

Add a minimal, runtime-only guard that prevents repeated requests to a model on an account that appears not to have permission for that model.

After an account receives three qualifying failures for the same model, that account-model pair is blocked for 7 days. Other models on the same account remain eligible, and other accounts remain eligible for the blocked model.

## Non-Goals

- No database schema changes.
- No account JSON format changes.
- No management UI changes.
- No new configuration fields.
- No persistence across process restarts.
- No changes to quota cooldown, 401 recovery, disabled credential recovery, or account removal semantics.

## Runtime State

Add a small in-memory map on `auth.Account`, protected by the account mutex:

- key: normalized model name
- value:
  - consecutive qualifying failure count
  - blocked-until timestamp

The model key should use the incoming logical model string passed through selection and retry, trimmed and lowercased. It should not collapse different public model variants unless the existing request path already does so for selection. This keeps the feature conservative and avoids accidentally blocking sibling variants.

## Qualifying Failures

Only clear model-permission style upstream responses should increment the account-model counter.

The first implementation should count only HTTP `400` and `403` responses whose error body summary contains model-access language. The matcher should be intentionally narrow and case-insensitive. It should require model context plus an access or availability clue, using terms such as:

- `model`
- `permission`
- `access`
- `entitled`
- `available`
- `not found`
- `unsupported`

Network errors, empty responses, `401`, `429`, and `5xx` must not count. Those paths should continue to use existing retry, cooldown, recovery, and account-level failure logic. Non-qualifying failures neither increment nor reset the model failure counter; a successful request for the same account-model pair resets it.

## Blocking Behavior

When a qualifying failure is observed:

1. Increment the account-model failure count.
2. If the count reaches 3, set `blockedUntil = now + 7 days`.
3. Invalidate the selector cache so future picks stop returning that account for the blocked model quickly.

When a request succeeds for an account-model pair, clear that pair's failure count and block state. This can be called before or alongside the existing `RecordSuccess` flow.

When a block has expired, the selection path should treat the account-model pair as eligible again. Expired entries can be cleaned lazily during the availability check.

## Selection Behavior

Add model-aware availability filtering with minimal changes:

- Keep existing account-level `IsAvailable` / `accountPickableAt` semantics intact.
- Add an account helper such as `IsModelAvailable(model, now)` or `IsModelBlocked(model, now)`.
- Update `filterAvailable`, `accountPickableAt`, and direct manager selection loops to include the requested model.
- Keep the selector interface unchanged because `Pick(model, accounts)` already receives the model.

All account selection paths used by normal retry should skip a blocked account-model pair:

- round-robin selector
- quota-first selector
- fill-first selector
- `Manager.PickExcluding`
- `Manager.PickRecentlySuccessful`
- `Manager.PickIgnoringCooldown`

## Error Hook

Extend the executor error callback with enough context to classify a model-specific failure:

Current callback:

```go
OnAfterUpstreamErrFn func(acc *auth.Account, statusCode int)
```

Proposed callback:

```go
OnAfterUpstreamErrFn func(acc *auth.Account, model string, statusCode int, errBody []byte)
```

The executor already has the model and error body at the call site, so this is a small local signature change. The handler can then call a manager/account helper such as `RecordModelFailureIfAccessError(acc, model, statusCode, errBody)`.

## Observability

Log only on state transitions, not every failed attempt:

- Debug log for qualifying failures before threshold.
- Warn log when an account-model pair is blocked for 7 days.
- Debug log when an expired block is lazily cleared.

No `/stats` or frontend field changes in the minimal version.

## Testing

Backend unit tests should cover:

1. Three qualifying `400` or `403` model-access errors block only that account-model pair.
2. A blocked account is skipped for the blocked model but can still be picked for a different model.
3. Non-qualifying statuses such as `429`, `401`, and `500` do not create model blocks.
4. A success clears the failure count for the same account-model pair.
5. Expired blocks become eligible again.

The existing test suite should continue to pass with no config or fixture changes.

## Open Decisions

- Model blocks are always enabled in this minimal implementation.
- The threshold is fixed at 3 consecutive qualifying failures.
- The block duration is fixed at 7 days.
- Runtime-only behavior is acceptable; process restart clears all model block state.
