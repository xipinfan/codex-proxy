# Codex OAuth Image Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `POST /v1/images/generations` backed by Codex OAuth and `gpt-image-2`.

**Architecture:** Add an OpenAI Images API handler, a pure image translator, and a small executor method that reuses the existing Codex `/responses` retry path. The handler validates public API shape, the translator owns Codex image tool JSON/SSE parsing, and the executor owns account selection and upstream transport.

**Tech Stack:** Go, fasthttp, net/http, `tidwall/gjson`, `tidwall/sjson`, existing Codex executor and auth manager.

---

## File Structure

- Create `internal/translator/image.go`: image request conversion and Codex SSE image result parsing.
- Create `internal/translator/image_test.go`: unit tests for image request conversion and SSE parsing.
- Create `internal/handler/images.go`: OpenAI-compatible `/v1/images/generations` handler.
- Create `internal/handler/images_test.go`: route/handler tests using a mocked Codex upstream server.
- Modify `internal/executor/codex.go`: add `ExecuteImageGeneration`.
- Modify `internal/handler/proxy.go`: register the authenticated images route.
- Modify or add executor tests if the model-block hook needs explicit coverage for the image method.

---

### Task 1: Translator Request Builder and Parser

**Files:**
- Create: `internal/translator/image.go`
- Test: `internal/translator/image_test.go`

- [ ] **Step 1: Write failing translator tests**

Create `internal/translator/image_test.go` with tests that assert the exact Codex request shape and both supported SSE result forms:

```go
package translator

import (
	"encoding/base64"
	"testing"

	"github.com/tidwall/gjson"
)

func TestBuildCodexImageGenerationRequest(t *testing.T) {
	req := ImageGenerationRequest{
		Model:            "gpt-image-2",
		Prompt:           "Draw a small green lighthouse",
		Size:             "1024x1536",
		Quality:          "low",
		OutputFormat:     "jpeg",
		Background:       "opaque",
		OutputCompression: 55,
	}

	body, err := BuildCodexImageGenerationRequest(req)
	if err != nil {
		t.Fatalf("BuildCodexImageGenerationRequest() error = %v", err)
	}

	root := gjson.ParseBytes(body)
	if got := root.Get("model").String(); got != "gpt-5.5" {
		t.Fatalf("outer model = %q, want gpt-5.5", got)
	}
	if got := root.Get("instructions").String(); got != "You are an image generation assistant." {
		t.Fatalf("instructions = %q", got)
	}
	if !root.Get("stream").Bool() {
		t.Fatalf("stream should be true")
	}
	if root.Get("store").Bool() {
		t.Fatalf("store should be false")
	}
	if got := root.Get("input.0.content.0.text").String(); got != req.Prompt {
		t.Fatalf("prompt = %q", got)
	}
	tool := root.Get("tools.0")
	if got := tool.Get("type").String(); got != "image_generation" {
		t.Fatalf("tool type = %q", got)
	}
	if got := tool.Get("model").String(); got != "gpt-image-2" {
		t.Fatalf("tool model = %q", got)
	}
	if got := tool.Get("size").String(); got != "1024x1536" {
		t.Fatalf("tool size = %q", got)
	}
	if got := tool.Get("quality").String(); got != "low" {
		t.Fatalf("tool quality = %q", got)
	}
	if got := tool.Get("output_format").String(); got != "jpeg" {
		t.Fatalf("tool output_format = %q", got)
	}
	if got := tool.Get("background").String(); got != "opaque" {
		t.Fatalf("tool background = %q", got)
	}
	if got := tool.Get("output_compression").Int(); got != 55 {
		t.Fatalf("tool output_compression = %d", got)
	}
	if got := root.Get("tool_choice.type").String(); got != "image_generation" {
		t.Fatalf("tool_choice.type = %q", got)
	}
}

func TestParseCodexImageGenerationSSEOutputItem(t *testing.T) {
	image := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	body := `data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"` + image + `","revised_prompt":"clean prompt"}}` + "\n\n" +
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1}}}` + "\n\n"

	result, err := ParseCodexImageGenerationSSE([]byte(body))
	if err != nil {
		t.Fatalf("ParseCodexImageGenerationSSE() error = %v", err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("images len = %d, want 1", len(result.Images))
	}
	if result.Images[0].Base64 != image {
		t.Fatalf("image base64 mismatch")
	}
	if result.Images[0].RevisedPrompt != "clean prompt" {
		t.Fatalf("revised prompt = %q", result.Images[0].RevisedPrompt)
	}
}

func TestParseCodexImageGenerationSSECompletedOutput(t *testing.T) {
	image := base64.StdEncoding.EncodeToString([]byte("completed-png-bytes"))
	body := `data: {"type":"response.completed","response":{"output":[{"type":"image_generation_call","result":"` + image + `"}]}}` + "\n\n"

	result, err := ParseCodexImageGenerationSSE([]byte(body))
	if err != nil {
		t.Fatalf("ParseCodexImageGenerationSSE() error = %v", err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("images len = %d, want 1", len(result.Images))
	}
	if result.Images[0].Base64 != image {
		t.Fatalf("image base64 mismatch")
	}
}

func TestParseCodexImageGenerationSSEFailure(t *testing.T) {
	body := `data: {"type":"response.failed","error":{"message":"no model access"}}` + "\n\n"
	_, err := ParseCodexImageGenerationSSE([]byte(body))
	if err == nil {
		t.Fatalf("expected failure error")
	}
}
```

- [ ] **Step 2: Run translator tests and verify they fail**

Run: `go test ./internal/translator -run 'Test(BuildCodexImageGenerationRequest|ParseCodexImageGenerationSSE)'`

Expected: FAIL because `ImageGenerationRequest`, `BuildCodexImageGenerationRequest`, and `ParseCodexImageGenerationSSE` do not exist.

- [ ] **Step 3: Implement translator**

Create `internal/translator/image.go` with:

```go
package translator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	CodexImageResponsesModel = "gpt-5.5"
	CodexImageInstructions   = "You are an image generation assistant."
	DefaultImageModel        = "gpt-image-2"
	DefaultImageSize         = "1024x1024"
	MaxImageResults          = 4
	maxCodexImageEvents      = 512
)

type ImageGenerationRequest struct {
	Model             string
	Prompt            string
	Size              string
	Quality           string
	OutputFormat      string
	Background        string
	OutputCompression int
	HasCompression    bool
}

type CodexImageGenerationResult struct {
	Images []CodexImage
}

type CodexImage struct {
	Base64        string
	RevisedPrompt string
}

func BuildCodexImageGenerationRequest(req ImageGenerationRequest) ([]byte, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = DefaultImageModel
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = DefaultImageSize
	}

	out := `{}`
	out, _ = sjson.Set(out, "model", CodexImageResponsesModel)
	out, _ = sjson.Set(out, "instructions", CodexImageInstructions)
	out, _ = sjson.SetRaw(out, "input", `[]`)
	item := `{"role":"user","content":[]}`
	content := `{}`
	content, _ = sjson.Set(content, "type", "input_text")
	content, _ = sjson.Set(content, "text", prompt)
	item, _ = sjson.SetRaw(item, "content.-1", content)
	out, _ = sjson.SetRaw(out, "input.-1", item)

	tool := `{}`
	tool, _ = sjson.Set(tool, "type", "image_generation")
	tool, _ = sjson.Set(tool, "model", model)
	tool, _ = sjson.Set(tool, "size", size)
	if req.Quality != "" {
		tool, _ = sjson.Set(tool, "quality", req.Quality)
	}
	if req.OutputFormat != "" {
		tool, _ = sjson.Set(tool, "output_format", req.OutputFormat)
	}
	if req.Background != "" {
		tool, _ = sjson.Set(tool, "background", req.Background)
	}
	if req.HasCompression {
		tool, _ = sjson.Set(tool, "output_compression", req.OutputCompression)
	}
	out, _ = sjson.SetRaw(out, "tools", `[]`)
	out, _ = sjson.SetRaw(out, "tools.-1", tool)
	out, _ = sjson.Set(out, "tool_choice.type", "image_generation")
	out, _ = sjson.Set(out, "stream", true)
	out, _ = sjson.Set(out, "store", false)
	return []byte(out), nil
}

func ParseCodexImageGenerationSSE(body []byte) (CodexImageGenerationResult, error) {
	var events []gjson.Result
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}
		if !gjson.Valid(data) {
			continue
		}
		event := gjson.Parse(data)
		events = append(events, event)
		if len(events) > maxCodexImageEvents {
			return CodexImageGenerationResult{}, errors.New("codex image response exceeded event limit")
		}
		if typ := event.Get("type").String(); typ == "response.failed" || typ == "error" {
			msg := event.Get("error.message").String()
			if msg == "" {
				msg = event.Get("message").String()
			}
			if msg == "" {
				msg = "codex image generation failed"
			}
			return CodexImageGenerationResult{}, errors.New(msg)
		}
	}

	images := make([]CodexImage, 0, MaxImageResults)
	for _, event := range events {
		if event.Get("type").String() != "response.output_item.done" {
			continue
		}
		item := event.Get("item")
		if item.Get("type").String() != "image_generation_call" {
			continue
		}
		if img := imageFromResult(item); img.Base64 != "" {
			images = append(images, img)
			if len(images) >= MaxImageResults {
				break
			}
		}
	}
	if len(images) == 0 {
		for _, event := range events {
			if event.Get("type").String() != "response.completed" {
				continue
			}
			for _, item := range event.Get("response.output").Array() {
				if item.Get("type").String() != "image_generation_call" {
					continue
				}
				if img := imageFromResult(item); img.Base64 != "" {
					images = append(images, img)
					if len(images) >= MaxImageResults {
						break
					}
				}
			}
		}
	}
	if len(images) == 0 {
		return CodexImageGenerationResult{}, fmt.Errorf("codex image generation returned no image")
	}
	return CodexImageGenerationResult{Images: images}, nil
}

func imageFromResult(item gjson.Result) CodexImage {
	result := item.Get("result").String()
	if result == "" {
		return CodexImage{}
	}
	return CodexImage{
		Base64:        result,
		RevisedPrompt: item.Get("revised_prompt").String(),
	}
}

func MarshalOpenAIImageResponse(created int64, images []CodexImage) ([]byte, error) {
	payload := map[string]any{
		"created": created,
		"data":    make([]map[string]string, 0, len(images)),
	}
	data := payload["data"].([]map[string]string)
	for _, image := range images {
		item := map[string]string{"b64_json": image.Base64}
		if image.RevisedPrompt != "" {
			item["revised_prompt"] = image.RevisedPrompt
		}
		data = append(data, item)
	}
	payload["data"] = data
	return json.Marshal(payload)
}
```

- [ ] **Step 4: Run translator tests and verify they pass**

Run: `go test ./internal/translator -run 'Test(BuildCodexImageGenerationRequest|ParseCodexImageGenerationSSE)'`

Expected: PASS.

- [ ] **Step 5: Commit translator**

Run:

```bash
git add internal/translator/image.go internal/translator/image_test.go
git commit -m "feat: add codex image translator" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

---

### Task 2: Executor Image Generation Method

**Files:**
- Modify: `internal/executor/codex.go`
- Test: `internal/executor/codex_image_test.go`

- [ ] **Step 1: Write failing executor test**

Create `internal/executor/codex_image_test.go`:

```go
package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codex-proxy/internal/auth"
)

func TestExecuteImageGenerationUsesImageModelForPickingAndCodexResponses(t *testing.T) {
	var pickedModel string
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %s, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q, want text/event-stream", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aW1hZ2U="}}` + "\n\n"))
	}))
	defer upstream.Close()

	exec := NewExecutor(upstream.URL, "", HTTPPoolConfig{})
	acc := &auth.Account{AccessToken: "token", FilePath: "acc.json"}
	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			pickedModel = model
			return acc, nil
		},
		MaxRetry: 0,
	}

	body := []byte(`{"model":"gpt-5.5","stream":true,"store":false}`)
	got, err := exec.ExecuteImageGeneration(context.Background(), rc, body, "gpt-image-2")
	if err != nil {
		t.Fatalf("ExecuteImageGeneration() error = %v", err)
	}
	if string(got) == "" {
		t.Fatalf("expected SSE body")
	}
	if pickedModel != "gpt-image-2" {
		t.Fatalf("picked model = %q, want gpt-image-2", pickedModel)
	}
	if upstreamBody["model"] != "gpt-5.5" {
		t.Fatalf("upstream model = %v", upstreamBody["model"])
	}
}
```

- [ ] **Step 2: Run executor test and verify it fails**

Run: `go test ./internal/executor -run TestExecuteImageGenerationUsesImageModelForPickingAndCodexResponses`

Expected: FAIL because `ExecuteImageGeneration` does not exist.

- [ ] **Step 3: Implement executor method**

In `internal/executor/codex.go`, add:

```go
const maxImageGenerationSSEBytes = 64 * 1024 * 1024

func (e *Executor) ExecuteImageGeneration(ctx context.Context, rc RetryConfig, requestBody []byte, model string) ([]byte, error) {
	apiURL := e.baseURL + "/responses"
	resp, account, _, err := e.sendWithRetry(ctx, rc, model, apiURL, requestBody, true)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Body == nil {
		return nil, ErrEmptyResponse
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageGenerationSSEBytes+1))
	if err != nil {
		if account != nil {
			account.RecordFailure()
		}
		return nil, wrapReadErr(err)
	}
	if len(body) > maxImageGenerationSSEBytes {
		return nil, fmt.Errorf("codex image generation response exceeded size limit")
	}
	return body, nil
}
```

- [ ] **Step 4: Run executor test and verify it passes**

Run: `go test ./internal/executor -run TestExecuteImageGenerationUsesImageModelForPickingAndCodexResponses`

Expected: PASS.

- [ ] **Step 5: Commit executor**

Run:

```bash
git add internal/executor/codex.go internal/executor/codex_image_test.go
git commit -m "feat: add codex image executor" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

---

### Task 3: Images Handler and Route

**Files:**
- Create: `internal/handler/images.go`
- Modify: `internal/handler/proxy.go`
- Test: `internal/handler/images_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `internal/handler/images_test.go` with tests for validation and success response. Use a local upstream server and `ProxyHandler.RegisterRoutes` so authentication and route registration are exercised.

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/executor"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

func TestHandleImageGenerationsRejectsUnsupportedModel(t *testing.T) {
	h := &ProxyHandler{}
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBodyString(`{"model":"gpt-image-1","prompt":"draw"}`)

	h.handleImageGenerations(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.Response.StatusCode())
	}
	if got := gjson.GetBytes(ctx.Response.Body(), "error.type").String(); got != "invalid_request_error" {
		t.Fatalf("error.type = %q", got)
	}
}

func TestHandleImageGenerationsRejectsURLResponseFormat(t *testing.T) {
	h := &ProxyHandler{}
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBodyString(`{"model":"gpt-image-2","prompt":"draw","response_format":"url"}`)

	h.handleImageGenerations(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Fatalf("status = %d, want 400", ctx.Response.StatusCode())
	}
}

func TestImageGenerationsRouteReturnsB64JSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aW1hZ2U=","revised_prompt":"revised"}}` + "\n\n"))
	}))
	defer upstream.Close()

	manager := auth.NewManager("round-robin")
	manager.AddAccount(&auth.Account{AccessToken: "token", FilePath: "acc.json", Email: "a@example.com"})
	h := NewProxyHandler(manager, executor.NewExecutor(upstream.URL, "", executor.HTTPPoolConfig{}), nil, 0, false, "", upstream.URL, false, "", "", 1, nil, false, 0, false, true, true, true, false, false, false, 0, nil)

	r := fasthttprouter.New()
	h.RegisterRoutes(r)

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/v1/images/generations")
	ctx.Request.SetBodyString(`{"model":"gpt-image-2","prompt":"draw","n":1}`)
	r.Handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if got := gjson.GetBytes(ctx.Response.Body(), "data.0.b64_json").String(); got != "aW1hZ2U=" {
		t.Fatalf("b64_json = %q", got)
	}
	if got := gjson.GetBytes(ctx.Response.Body(), "data.0.revised_prompt").String(); got != "revised" {
		t.Fatalf("revised_prompt = %q", got)
	}
}
```

- [ ] **Step 2: Run handler tests and verify they fail**

Run: `go test ./internal/handler -run 'TestHandleImageGenerations|TestImageGenerationsRoute'`

Expected: FAIL because `handleImageGenerations` and the route do not exist.

- [ ] **Step 3: Implement handler**

Create `internal/handler/images.go`:

```go
package handler

import (
	"context"
	"strings"
	"time"

	"codex-proxy/internal/translator"

	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

func (h *ProxyHandler) handleImageGenerations(ctx *fasthttp.RequestCtx) {
	body := ctx.PostBody()
	if len(body) == 0 {
		sendError(ctx, fasthttp.StatusBadRequest, "读取请求体失败", "invalid_request_error")
		return
	}
	prompt := strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	if prompt == "" {
		sendError(ctx, fasthttp.StatusBadRequest, "缺少 prompt 字段", "invalid_request_error")
		return
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		model = translator.DefaultImageModel
	}
	if model != translator.DefaultImageModel {
		sendError(ctx, fasthttp.StatusBadRequest, "仅支持 gpt-image-2 图像模型", "invalid_request_error")
		return
	}
	responseFormat := strings.TrimSpace(gjson.GetBytes(body, "response_format").String())
	if responseFormat != "" && responseFormat != "b64_json" {
		sendError(ctx, fasthttp.StatusBadRequest, "当前仅支持 response_format=b64_json", "invalid_request_error")
		return
	}

	count := int(gjson.GetBytes(body, "n").Int())
	if count <= 0 {
		count = 1
	}
	if count > translator.MaxImageResults {
		count = translator.MaxImageResults
	}

	imgReq := translator.ImageGenerationRequest{
		Model:        model,
		Prompt:       prompt,
		Size:         strings.TrimSpace(gjson.GetBytes(body, "size").String()),
		Quality:      strings.TrimSpace(gjson.GetBytes(body, "quality").String()),
		OutputFormat: strings.TrimSpace(gjson.GetBytes(body, "output_format").String()),
		Background:   strings.TrimSpace(gjson.GetBytes(body, "background").String()),
	}
	if v := gjson.GetBytes(body, "output_compression"); v.Exists() {
		imgReq.OutputCompression = int(v.Int())
		imgReq.HasCompression = true
	}

	rc := h.buildRetryConfig()
	images := make([]translator.CodexImage, 0, count)
	for i := 0; i < count; i++ {
		codexBody, err := translator.BuildCodexImageGenerationRequest(imgReq)
		if err != nil {
			sendError(ctx, fasthttp.StatusBadRequest, err.Error(), "invalid_request_error")
			return
		}
		raw, err := h.executor.ExecuteImageGeneration(context.Background(), rc, codexBody, model)
		if err != nil {
			handleExecutorError(ctx, err)
			return
		}
		parsed, err := translator.ParseCodexImageGenerationSSE(raw)
		if err != nil {
			sendError(ctx, fasthttp.StatusBadGateway, err.Error(), "bad_gateway")
			return
		}
		images = append(images, parsed.Images...)
		if len(images) >= count {
			images = images[:count]
			break
		}
	}

	resp, err := translator.MarshalOpenAIImageResponse(time.Now().Unix(), images)
	if err != nil {
		sendError(ctx, fasthttp.StatusInternalServerError, "json编码失败", "server_error")
		return
	}
	RecordRequest()
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(resp)
}
```

- [ ] **Step 4: Register route**

In `ProxyHandler.RegisterRoutes`, add an authenticated `POST /v1/images/generations` route using the same API key middleware pattern as `/v1/responses`.

```go
	apiImageGenerations := h.handleImageGenerations
	if len(h.apiKeys) > 0 {
		apiImageGenerations = h.authMiddleware(h.handleImageGenerations)
	}
	r.POST("/v1/images/generations", apiImageGenerations)
```

- [ ] **Step 5: Run handler tests and verify they pass**

Run: `go test ./internal/handler -run 'TestHandleImageGenerations|TestImageGenerationsRoute'`

Expected: PASS.

- [ ] **Step 6: Commit handler**

Run:

```bash
git add internal/handler/images.go internal/handler/images_test.go internal/handler/proxy.go
git commit -m "feat: add images generations route" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

---

### Task 4: Integration Verification and Full Test Run

**Files:**
- Modify only if tests expose gaps.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./internal/translator ./internal/executor ./internal/handler ./internal/auth
```

Expected: PASS.

- [ ] **Step 2: Run full suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Inspect git diff**

Run:

```bash
git diff --stat HEAD
git diff --check
```

Expected: no whitespace errors and only image-generation related files changed.

- [ ] **Step 4: Commit any final fixes**

If changes were needed after Task 3, commit them:

```bash
git add <changed-files>
git commit -m "test: verify codex image generation" -m "Co-Authored-By: Codex <noreply@openai.com>"
```

If no changes were needed, skip this commit.

---

## Self-Review

- Spec coverage: route, non-streaming scope, request conversion, SSE parsing, account-model block key, validation, and response shape are all mapped to tasks.
- Placeholder scan: no task contains unspecified implementation work.
- Type consistency: plan uses `ImageGenerationRequest`, `BuildCodexImageGenerationRequest`, `ParseCodexImageGenerationSSE`, `ExecuteImageGeneration`, and `handleImageGenerations` consistently across tests and implementation.
