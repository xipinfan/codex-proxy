# Multi Image Request Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve multi-image request reliability by raising the inbound request body limit, returning clear JSON parse-layer errors, and optionally compressing large upstream Codex request bodies.

**Architecture:** Keep `main.go` thin by moving HTTP server construction and parse-layer error handling into `internal/httpserver`. Move upstream body compression into `internal/upstream`, and let the executor call a small helper before constructing outbound requests. Configuration stays in `internal/config`, with conservative defaults and explicit YAML examples.

**Tech Stack:** Go, fasthttp, standard `compress/gzip` only for inbound decompression if later enabled, `github.com/klauspost/compress/zstd` for upstream zstd request compression.

---

## Context And Root Cause

Current multi-image failures are most likely caused by the inbound fasthttp default body limit. The project creates `fasthttp.Server` in `main.go` without setting `MaxRequestBodySize`, so fasthttp uses its default 4 MiB limit. Multi-image requests from Codex clients commonly encode local images as `data:image/...;base64,...` inside `input_image.image_url`, which can exceed 4 MiB after base64 expansion.

The optimization should not add image parsing or resizing in this pass. It should focus on transport limits and request compression, keeping behavior compatible with existing Chat Completions, Responses, Claude Messages, and Images API routes.

## Desired Behavior

- Requests larger than 4 MiB but below configured limit should reach existing route handlers.
- Requests above configured limit should return OpenAI-style JSON instead of fasthttp's plain text `Error when parsing request`.
- Large Codex upstream request bodies should be compressed with zstd when configured as `auto` or `always`.
- Compression should be isolated from executor retry/account-selection logic.
- Existing default behavior should remain safe for normal text requests.

## New Configuration

Add fields to `internal/config.Config`:

```yaml
# Maximum inbound HTTP request body size in bytes.
# Default: 134217728 (128 MiB)
listen-max-request-body-bytes: 134217728

# Compress request bodies sent from this proxy to the Codex backend.
# Values: off | auto | always
# Default: auto
upstream-request-compression: "auto"

# Minimum uncompressed request body size for auto compression.
# Default: 1048576 (1 MiB)
upstream-request-compression-min-bytes: 1048576
```

Validation rules:

- `listen-max-request-body-bytes <= 0` uses default `128 MiB`.
- Values below `4 MiB` are raised to `4 MiB` to preserve existing fasthttp default behavior.
- `upstream-request-compression` accepts `off`, `auto`, or `always`; unknown values become `auto` with a warning.
- `upstream-request-compression-min-bytes <= 0` uses default `1 MiB`.

## File Structure

Create:

- `internal/httpserver/options.go`
  - Owns HTTP server options derived from config.
  - Keeps `main.go` free of limit/default/error details.

- `internal/httpserver/server.go`
  - Exposes `New(handler fasthttp.RequestHandler, opts Options) *fasthttp.Server`.
  - Builds the fasthttp server with existing timeout/connection behavior plus `MaxRequestBodySize` and `ErrorHandler`.

- `internal/httpserver/error_handler.go`
  - Converts parse-layer errors into OpenAI-compatible JSON where possible.
  - Handles request-body-too-large style parse failures.

- `internal/httpserver/server_test.go`
  - Verifies server defaults, body limit wiring, and JSON error body shape.

- `internal/upstream/compression.go`
  - Owns outbound request compression policy and zstd encoding.
  - Provides a small API that returns encoded body bytes and headers.

- `internal/upstream/compression_test.go`
  - Verifies `off`, `auto`, and `always` modes.

Modify:

- `internal/config/config.go`
  - Add config fields, defaults, and normalization.

- `config.example.yaml`
  - Document new options near listen/upstream transport options.

- `docs/CONFIGURATION.md`
  - Document new options and recommended values for multi-image requests.

- `main.go`
  - Replace inline `fasthttp.Server{...}` construction with `httpserver.New(...)`.
  - Main file should still own route/middleware composition and lifecycle only.

- `internal/executor/codex.go`
  - Add compression config to `Executor`.
  - Use `internal/upstream` helper inside outbound request construction.
  - Keep retry/account/error logic unchanged.

## Public APIs To Add

`internal/httpserver`:

```go
type Options struct {
    Name                          string
    Concurrency                   int
    IdleTimeout                   time.Duration
    ListenReadHeaderTimeout       time.Duration
    TCPKeepalive                  bool
    TCPKeepalivePeriod            time.Duration
    ReadBufferSize                int
    MaxRequestBodySize            int
}

func New(handler fasthttp.RequestHandler, opts Options) *fasthttp.Server
```

`internal/upstream`:

```go
type CompressionMode string

const (
    CompressionOff    CompressionMode = "off"
    CompressionAuto   CompressionMode = "auto"
    CompressionAlways CompressionMode = "always"
)

type CompressionConfig struct {
    Mode        CompressionMode
    MinBytes    int
}

type EncodedBody struct {
    Body    []byte
    Headers map[string]string
}

func EncodeRequestBody(body []byte, cfg CompressionConfig) (EncodedBody, error)
```

Executor construction:

```go
type HTTPPoolConfig struct {
    ...
    UpstreamRequestCompression upstream.CompressionConfig
}
```

`main.go` already passes `executor.HTTPPoolConfig`; add the new nested config there instead of adding more parameters to `NewExecutor`.

## Task 1: HTTP Server Module And Body Limit

**Files:**
- Create: `internal/httpserver/options.go`
- Create: `internal/httpserver/server.go`
- Create: `internal/httpserver/error_handler.go`
- Create: `internal/httpserver/server_test.go`
- Modify: `main.go`
- Modify: `internal/config/config.go`
- Modify: `config.example.yaml`
- Modify: `docs/CONFIGURATION.md`

- [ ] **Step 1: Write failing tests for server limit wiring**

Add tests in `internal/httpserver/server_test.go`:

- `TestNewSetsMaxRequestBodySize`
- `TestNewUsesJSONErrorHandlerForParseErrors`

Expected before implementation: package does not exist or tests fail because `httpserver.New` is undefined.

- [ ] **Step 2: Implement `internal/httpserver.Options` and `New`**

Move only server-construction concerns into the new package. Preserve the existing values from `main.go`:

- `Name`
- `DisableKeepalive: false`
- `Concurrency`
- `IdleTimeout`
- `ReadTimeout: 0`
- `WriteTimeout: 0`
- `HeaderReceived` read timeout
- `TCPKeepalive`
- `TCPKeepalivePeriod`
- `ReadBufferSize`
- `MaxConnsPerIP: 0`
- `MaxRequestsPerConn: 0`

Add:

- `MaxRequestBodySize: opts.MaxRequestBodySize`
- `ErrorHandler: parseErrorHandler`

- [ ] **Step 3: Implement OpenAI-style parse error JSON**

Use a small JSON helper local to `internal/httpserver` to avoid importing `internal/handler`.

Default parse-layer response:

```json
{
  "error": {
    "message": "Invalid HTTP request. If this request contains images, verify the request body size and encoding.",
    "type": "invalid_request_error",
    "code": "invalid_http_request"
  }
}
```

For body-size-like errors, return:

```json
{
  "error": {
    "message": "Request body too large. Increase listen-max-request-body-bytes or reduce/compress images.",
    "type": "invalid_request_error",
    "code": "request_body_too_large"
  }
}
```

Detection should be conservative:

- If the parse error string contains `body size exceeds`
- Or contains `too large`
- Or contains `cannot read request body`

- [ ] **Step 4: Add config fields and defaults**

In `internal/config/config.go`, add:

- `ListenMaxRequestBodyBytes int`

Default:

- `134217728`

Normalize:

- If zero/negative: set default.
- If positive but below `4 * 1024 * 1024`: raise to `4 * 1024 * 1024`.

- [ ] **Step 5: Replace inline server literal in `main.go`**

Keep route and middleware setup in `main.go`. Replace the `srv := &fasthttp.Server{...}` literal with:

```go
srv := httpserver.New(appHandler, httpserver.Options{
    Name: "Codex Proxy",
    Concurrency: cfg.ListenConcurrency,
    IdleTimeout: time.Duration(cfg.ListenIdleTimeoutSec) * time.Second,
    ListenReadHeaderTimeout: time.Duration(cfg.ListenReadHeaderTimeoutSec) * time.Second,
    TCPKeepalive: cfg.ListenTCPKeepaliveSec > 0,
    TCPKeepalivePeriod: time.Duration(cfg.ListenTCPKeepaliveSec) * time.Second,
    ReadBufferSize: cfg.ListenMaxHeaderBytes,
    MaxRequestBodySize: cfg.ListenMaxRequestBodyBytes,
})
```

This should be the only `main.go` behavior change.

- [ ] **Step 6: Run focused tests**

Run:

```powershell
go test ./internal/httpserver ./internal/config
```

Expected: all tests pass.

## Task 2: Upstream Zstd Compression

**Files:**
- Create: `internal/upstream/compression.go`
- Create: `internal/upstream/compression_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/executor/codex.go`
- Modify: `internal/executor/codex_image_test.go`
- Modify: `config.example.yaml`
- Modify: `docs/CONFIGURATION.md`

- [ ] **Step 1: Write failing compression tests**

Add tests in `internal/upstream/compression_test.go`:

- `TestEncodeRequestBodyOffLeavesBodyUnchanged`
- `TestEncodeRequestBodyAutoLeavesSmallBodyUnchanged`
- `TestEncodeRequestBodyAutoCompressesLargeBody`
- `TestEncodeRequestBodyAlwaysCompressesSmallBody`

Expected before implementation: package or functions are undefined.

- [ ] **Step 2: Implement `internal/upstream.EncodeRequestBody`**

Behavior:

- `off`: return original body and no extra headers.
- `auto`: compress only when `len(body) >= MinBytes`.
- `always`: compress any non-empty body.
- Empty body should never be compressed.

When compressed:

- Use zstd level 3 or default encoder level.
- Add header `Content-Encoding: zstd`.
- Keep `Content-Type` owned by executor headers, not this helper.

- [ ] **Step 3: Add config fields and defaults**

In `internal/config/config.go`, add:

- `UpstreamRequestCompression string`
- `UpstreamRequestCompressionMinBytes int`

Defaults:

- mode: `auto`
- min bytes: `1048576`

Normalize mode to lowercase. Unknown mode should become `auto` and log a warning.

- [ ] **Step 4: Thread compression config into executor**

Extend `executor.HTTPPoolConfig` with:

```go
UpstreamRequestCompression upstream.CompressionConfig
```

Store it on `Executor`.

In `main.go`, populate from config:

```go
UpstreamRequestCompression: upstream.CompressionConfig{
    Mode: upstream.CompressionMode(cfg.UpstreamRequestCompression),
    MinBytes: cfg.UpstreamRequestCompressionMinBytes,
}
```

- [ ] **Step 5: Apply compression in outbound request construction**

Inside `sendWithRetry`, before `http.NewRequestWithContext`, call:

```go
encoded, encErr := upstream.EncodeRequestBody(codexBody, e.upstreamRequestCompression)
if encErr != nil {
    return nil, fmt.Errorf("%w: encode upstream request body: %w", errCodexBuildRequest, encErr)
}
bodyReader := bytes.NewReader(encoded.Body)
```

After `applyCodexHeaders`, set returned headers:

```go
for k, v := range encoded.Headers {
    httpReq.Header.Set(k, v)
}
```

Important: ensure retries use fresh readers over the encoded body. Avoid recompressing on every account retry unless the implementation is simpler and benchmark-neutral. Preferred: encode once before the retry loop and reuse `encoded.Body`.

- [ ] **Step 6: Update executor tests**

Add or extend an executor test so a large request sent through `sendWithRetry` reaches the upstream test server with:

- `Content-Encoding: zstd`
- body that decompresses to the original JSON

Keep the test local to `internal/executor` and avoid real network.

- [ ] **Step 7: Run focused tests**

Run:

```powershell
go test ./internal/upstream ./internal/executor
```

Expected: all tests pass.

## Task 3: Documentation And End-To-End Verification

**Files:**
- Modify: `config.example.yaml`
- Modify: `docs/CONFIGURATION.md`

- [ ] **Step 1: Document multi-image recommended settings**

Add a short section explaining:

- Why multi-image requests can exceed 4 MiB.
- Recommended `listen-max-request-body-bytes`.
- When to disable upstream compression.
- That this feature does not resize or rewrite image content.

- [ ] **Step 2: Run full test suite**

Run:

```powershell
go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Manual verification with synthetic large body**

Start the service locally and send a request body between 5 MiB and the configured limit.

Expected:

- The response should be produced by a route handler or upstream, not the fasthttp parse-layer text `Error when parsing request`.
- If request is intentionally malformed JSON but below body limit, the route handler should handle it according to existing behavior.

- [ ] **Step 4: Manual verification with over-limit body**

Send a request body larger than `listen-max-request-body-bytes`.

Expected:

- HTTP 400 or 413 is acceptable, but body must be JSON.
- JSON error code should be `request_body_too_large` when fasthttp exposes enough error detail to classify it.
- Plain text `Error when parsing request` must not appear.

## Non-Goals

- Do not resize, re-encode, or validate image data in this implementation.
- Do not change translator message semantics.
- Do not alter account selection, retry, quota, OAuth, or model suffix behavior.
- Do not add multipart upload or file-id upload support.
- Do not change `/v1/images/generations` image generation request behavior beyond shared upstream compression if it uses the same executor path.

## Risks And Mitigations

- **Risk:** Larger inbound body limit increases memory pressure.
  - **Mitigation:** Default to 128 MiB, keep it configurable, and document deployment-specific tuning.

- **Risk:** Codex backend may reject zstd in some environments.
  - **Mitigation:** `upstream-request-compression: off` disables it immediately without code changes.

- **Risk:** Recompressing per retry wastes CPU.
  - **Mitigation:** Encode once per proxy request and reuse bytes across account retries.

- **Risk:** Error classification may not catch every fasthttp parse error.
  - **Mitigation:** Always return JSON parse-layer errors, even when the exact cause is unknown.

## Acceptance Checklist

- [ ] `main.go` has no new compression or parse-error logic.
- [ ] `main.go` only wires `httpserver.New` and config values.
- [ ] `internal/httpserver` owns fasthttp server construction and parse-layer JSON errors.
- [ ] `internal/upstream` owns zstd compression policy.
- [ ] Requests above 4 MiB but below configured max no longer fail at fasthttp parsing.
- [ ] Oversized requests return JSON instead of plain text.
- [ ] Large upstream Codex requests use `Content-Encoding: zstd` in `auto` mode.
- [ ] `go test ./...` passes.
