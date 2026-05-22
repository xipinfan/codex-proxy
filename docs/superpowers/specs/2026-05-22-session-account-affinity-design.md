# Session Account Affinity Design

## Context

`codex-proxy` currently chooses an account for each upstream request through the configured selector:

- `ProxyHandler` builds a cached `executor.RetryConfig`.
- `RetryConfig.PickFn` calls `Manager.PickExcluding(model, excluded)`.
- The default selector is round-robin, with quota-first as a supported alternative.
- `sendWithRetry` excludes accounts that already failed within the current request and can switch accounts on quota, auth, network, and upstream retry paths.

This spreads sequential requests from the same Codex conversation across accounts. Upstream prompt caching still works for a stable prefix, but a conversation that rotates through multiple accounts warms more cache routes and loses some locality. The current stable `Session_id` hint is derived after account selection and cannot by itself keep account selection sticky.

## Goal

Add runtime-only session affinity for normal Codex conversation requests so a stable conversation prefers the same account across requests. When the bound account stops being a good candidate, the request must fall back to the existing selector and retry behavior, then update affinity after a successful replacement.

The feature should follow the common sticky-session pattern:

1. Derive a stable session key.
2. Look up a TTL-bound affinity entry for that key.
3. Prefer the bound backend when it is still eligible.
4. Evict affinity when that backend fails in a way that requires account switching.
5. Rebind only after a replacement succeeds.

## Non-Goals

- No global fill-first behavior that concentrates all conversations on one account.
- No database schema or account JSON changes.
- No management UI changes in the first version.
- No cross-process affinity sharing.
- No changes to the account cooldown, OAuth refresh, model-block, or quota policy semantics.
- No dependency on inbound TCP connection lifetime.

## Session Key

Affinity must be based on conversation identity, not the inbound HTTP connection.

Session key derivation should be centralized near the request path that already has the translated Codex request body available. The first version should use this priority:

1. Explicit inbound `X-Session-Id`, when present and non-empty.
2. A deterministic fallback based on stable request-prefix material already used for upstream session hinting.

The fallback must be conservative. It may use request instructions and the first input item prefix to group continuation requests whose stable front matter matches, but it must not hash the full request body because the changing tail would defeat affinity. Account identity must not be part of the affinity session key; otherwise the key cannot decide which account to reuse.

The same derived session key should feed upstream cache locality signals where this proxy already controls them:

- `Session_id` should remain stable for a session.
- If a supported Codex upstream request path can safely preserve or set a prompt cache routing key, it should be derived from the same session key and documented in implementation notes. Unsupported upstream endpoints should remain unchanged.

## Runtime State

Add a small in-memory session affinity store in the auth or routing layer.

Each entry contains:

- session key
- bound account identity, using an identity that can be resolved against the current manager snapshot
- last successful bind timestamp or expiry timestamp

The store is process-local and TTL-bound. The first implementation should expose a configuration duration in seconds and use a default of 30 minutes. A non-positive configured duration should disable session affinity without changing existing selector behavior.

Expired or stale entries can be removed lazily during lookup. The store should avoid unbounded growth with lazy expiry and a bounded cleanup strategy suitable for hot-path use.

## Selection Flow

Normal conversation selection should keep current selector behavior as the fallback.

For the first attempt of a request with a session key:

1. Look up the affinity entry.
2. Resolve its bound account against the manager snapshot.
3. Reuse that account only if it is still pickable for the requested model and it is not already excluded for this request.
4. If lookup fails, the entry is stale, or the account is no longer pickable, evict the entry and call the existing selector path.

Retries after a bound account fails must continue to honor `excluded`, healthy retry, last-attempt fallback, quota precheck, and existing account policy behavior. Affinity must not force a failed account back into the same request.

When the upstream request succeeds on an account, record success affinity for the session key and refresh the TTL. This must happen after upstream success, not after account selection.

## Eviction Rules

Affinity should be evicted when a failure means the request path intentionally switches accounts:

- quota precheck definitively rejects the bound account
- upstream `429` or quota-exhausted payload triggers account switching
- `401` recovery path decides to switch accounts
- account becomes disabled, cooled down, token-ineligible, or model-blocked before pick
- retryable network or upstream failures exclude that account for the current request

Eviction should be keyed by session key and bound account identity so an old failure cannot delete a newer successful rebind.

Non-switching client/request errors should keep existing retry semantics and should not aggressively evict affinity unless the current path would exclude or mark the account unavailable.

## Component Boundaries

### Affinity Store

The store owns TTL entries, compare-and-evict behavior, and account rebinding. It does not decide whether an account is pickable.

### Manager

The manager remains the source of truth for account snapshots and eligibility. It should provide a session-aware pick entry point or a small wrapper over existing pick logic that:

- tries a bound account first
- applies the same model and availability checks used by current picks
- falls back to existing `PickExcluding`

### Handler And Executor

The handler/executor boundary should carry a request-scoped affinity context containing the derived session key and callbacks needed to bind or evict around retry outcomes. The executor keeps retry sequencing; it should not own long-lived affinity state.

## Configuration

Add one YAML configuration field for the first version:

- `session-affinity-ttl-sec`

Default:

- `1800`

Semantics:

- `> 0`: enable process-local session affinity with that TTL.
- `<= 0`: disable affinity and preserve current selector behavior.

The example config and configuration documentation should describe this as a cache-locality optimization for conversation requests, not a quota-balancing selector replacement.

## Observability

Keep hot-path logs low-volume.

- Debug log affinity lookup outcome when debugging selection paths is already enabled.
- Debug log stale-entry eviction and failure-driven eviction.
- Existing `upstream_cache_metric` remains the main signal for cache improvement.

The first version does not add stats UI fields. Log analysis should compare repeated session `cached_tokens` and hit ratio before and after affinity is enabled.

## Error Handling

- Missing session material should skip affinity and use the current selector.
- A binding to a removed account should be treated as stale and evicted.
- Affinity store failures should degrade to existing selector behavior; they must not block a request.
- Quota and auth recovery callbacks retain their current ownership of account state transitions.

## Testing

Backend tests should cover:

1. The same derived session key reuses a successful account on the next request.
2. Different session keys can still fall back through the configured selector independently.
3. Account identity is not part of fallback session-key derivation.
4. A bound account already present in `excluded` is not repicked within the same request.
5. A bound account that is cooled down, disabled, blocked for the model, or missing from the manager snapshot is evicted and replaced.
6. A quota precheck rejection or upstream switch-worthy failure evicts only the matching current binding.
7. Rebinding occurs after a successful replacement and compare-and-evict protects the newer binding.
8. TTL expiry and disabled configuration fall back to existing selection behavior.
9. Existing selector, retry, and cache metric tests remain green.

## Open Decisions

- Runtime-only affinity is acceptable for the first version; restart clears bindings.
- TTL configuration is preferred over a permanent sticky map.
- Conversation affinity should target normal Codex conversation paths first; image generation and unrelated admin/health paths should keep their current selection behavior unless implementation review finds they share the same safe request context.
