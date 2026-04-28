# Codex OAuth Image Generation Design

## Goal

Expose a minimal OpenAI-compatible image generation endpoint backed by the existing Codex OAuth account pool, so clients can call `POST /v1/images/generations` with `model: "gpt-image-2"` and receive base64 image results.

## Scope

First version supports only non-streaming image generation:

- Endpoint: `POST /v1/images/generations`
- Model: `gpt-image-2`
- Required input: `prompt`
- Optional inputs: `n`, `size`, `quality`, `output_format`, `background`
- Output: OpenAI Images API style JSON with `b64_json`

The first version intentionally does not support image edits, multipart reference images, response streaming, hosted image URLs, masks, or direct OpenAI Platform API-key routing.

## Existing Context

The project already routes text requests through `https://chatgpt.com/backend-api/codex/responses` using Codex OAuth accounts. The executor handles account selection, retry, OAuth headers, 401 recovery, 429 handling, and model-specific short-term blocking.

OpenClaw's current implementation confirms the compatible image path: send a Codex Responses request with outer model `gpt-5.5`, add an `image_generation` tool using `gpt-image-2`, force `tool_choice` to that tool, stream the upstream SSE, then extract the generated image base64 from `image_generation_call.result`.

## Architecture

Add a thin Images API adapter at the HTTP boundary and keep all upstream calls inside the existing Codex executor.

Client request flow:

```text
POST /v1/images/generations
  -> handler validates OpenAI Images request
  -> translator builds Codex Responses image tool request
  -> executor sends through existing Codex OAuth retry path
  -> translator parses Codex SSE image result
  -> handler returns OpenAI-compatible Images JSON
```

The Codex upstream request shape is:

```json
{
  "model": "gpt-5.5",
  "instructions": "You are an image generation assistant.",
  "input": [
    {
      "role": "user",
      "content": [
        {"type": "input_text", "text": "prompt text"}
      ]
    }
  ],
  "tools": [
    {
      "type": "image_generation",
      "model": "gpt-image-2",
      "size": "1024x1024"
    }
  ],
  "tool_choice": {"type": "image_generation"},
  "stream": true,
  "store": false
}
```

## Components

### Handler

Create `internal/handler/images.go` for `handleImageGenerations`. It validates the request body, enforces `model == "gpt-image-2"`, caps `n` to a small maximum, calls the executor, and writes the OpenAI Images response.

The handler reuses existing API key middleware by registering the route in `ProxyHandler.RegisterRoutes`.

### Translator

Create `internal/translator/image.go` for pure request and response conversion:

- Convert a validated image generation request into the Codex Responses body.
- Parse Codex SSE lines.
- Extract base64 from `response.output_item.done.item.result` when `item.type == "image_generation_call"`.
- Fall back to `response.completed.response.output[].result` for completed-response payloads.
- Surface `response.failed` and `error` events as normal errors.

This keeps image-specific protocol details out of handler and executor code.

### Executor

Add `ExecuteImageGeneration` to `internal/executor/codex.go`. It reuses `sendWithRetry` with the requested image model as the account-selection and model-block key, sends to `/responses`, reads the streamed body with bounded size, and returns the raw SSE body for translator parsing.

The outbound `stream` flag should be true so Codex receives `Accept: text/event-stream`, matching the proven upstream route.

### Account Behavior

Use `gpt-image-2` as the model key passed into selection, retry, and model-specific block recording. If a free account lacks access to `gpt-image-2`, repeated qualifying model-access failures block only that account-model pair for seven days and do not cool down the account for text models.

## Validation Rules

- Empty body returns `400 invalid_request_error`.
- Missing `prompt` returns `400 invalid_request_error`.
- Missing `model` defaults to `gpt-image-2`.
- Any model other than `gpt-image-2` returns `400 invalid_request_error`.
- `n` defaults to `1`, accepts positive integers, and is capped at `4`.
- `size` defaults to `1024x1024`.
- `quality`, `output_format`, and `background` are passed through only when present.
- `response_format` is accepted only as omitted or `b64_json`; URL output is not supported in the first version.

## Response Shape

Successful responses use:

```json
{
  "created": 1777392000,
  "data": [
    {
      "b64_json": "base64-image-data"
    }
  ]
}
```

If Codex returns `revised_prompt`, include it beside `b64_json`.

## Testing

Add focused Go tests for:

- Request conversion includes outer model `gpt-5.5`, `image_generation` tool model `gpt-image-2`, `tool_choice`, `stream: true`, and `store: false`.
- SSE parsing extracts base64 from `response.output_item.done`.
- SSE parsing extracts base64 from `response.completed.response.output`.
- Handler rejects unsupported models and unsupported `response_format: "url"`.
- Handler returns OpenAI Images JSON for a mocked Codex SSE image result.
- Model-access errors from the image route use `gpt-image-2` as the model-specific block key and do not apply account-wide cooldown.

## Rollout

Keep the feature enabled by route availability only. No new config, database, OAuth, or UI changes are required for the first version. If future compatibility requires disabling the endpoint, add a config switch in a later change.
