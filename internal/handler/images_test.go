package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/executor"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

type imageKeepaliveTestWriter struct {
	bytes.Buffer
	flushes int
}

func (w *imageKeepaliveTestWriter) Flush() error {
	w.flushes++
	return nil
}

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

	authDir := t.TempDir()
	accountPath := filepath.Join(authDir, "acc.json")
	if err := os.WriteFile(accountPath, []byte(`{"access_token":"token","email":"a@example.com","type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write account: %v", err)
	}
	manager := auth.NewManager(authDir, nil, "", 3000, auth.NewRoundRobinSelector(), false, nil)
	if err := manager.LoadAccounts(); err != nil {
		t.Fatalf("load accounts: %v", err)
	}
	h := NewProxyHandler(manager, executor.NewExecutor(upstream.URL, "", executor.HTTPPoolConfig{}), nil, 0, false, "", upstream.URL, false, "", "", 1, 0, nil, false, 0, false, true, true, true, false, false, false, 0, nil)

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
	accounts := manager.GetAccounts()
	if len(accounts) != 1 {
		t.Fatalf("accounts len = %d, want 1", len(accounts))
	}
	if got := accounts[0].GetStats().TotalRequests; got != 1 {
		t.Fatalf("account total requests = %d, want 1", got)
	}
}

func TestImageEditsRouteForwardsMultipartImageAsReference(t *testing.T) {
	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"ZWRpdGVk"}}` + "\n\n"))
	}))
	defer upstream.Close()

	authDir := t.TempDir()
	accountPath := filepath.Join(authDir, "acc.json")
	if err := os.WriteFile(accountPath, []byte(`{"access_token":"token","email":"a@example.com","type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write account: %v", err)
	}
	manager := auth.NewManager(authDir, nil, "", 3000, auth.NewRoundRobinSelector(), false, nil)
	if err := manager.LoadAccounts(); err != nil {
		t.Fatalf("load accounts: %v", err)
	}
	h := NewProxyHandler(manager, executor.NewExecutor(upstream.URL, "", executor.HTTPPoolConfig{}), nil, 0, false, "", upstream.URL, false, "", "", 1, 0, nil, false, 0, false, true, true, true, false, false, false, 0, nil)

	r := fasthttprouter.New()
	h.RegisterRoutes(r)

	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	if err := mw.WriteField("model", "gpt-image-2"); err != nil {
		t.Fatalf("write model field: %v", err)
	}
	if err := mw.WriteField("prompt", "edit this"); err != nil {
		t.Fatalf("write prompt field: %v", err)
	}
	fw, err := mw.CreateFormFile("image", "reference.png")
	if err != nil {
		t.Fatalf("create image field: %v", err)
	}
	if _, err := fw.Write([]byte("png-bytes")); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/v1/images/edits")
	ctx.Request.Header.SetContentType(mw.FormDataContentType())
	ctx.Request.SetBody(form.Bytes())
	r.Handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d body=%s", ctx.Response.StatusCode(), ctx.Response.Body())
	}
	if got := gjson.GetBytes(ctx.Response.Body(), "data.0.b64_json").String(); got != "ZWRpdGVk" {
		t.Fatalf("b64_json = %q", got)
	}
	root := gjson.ParseBytes(upstreamBody)
	if got := root.Get("input.0.content.1.type").String(); got != "input_image" {
		t.Fatalf("upstream image content type = %q, want input_image; body=%s", got, upstreamBody)
	}
	if got := root.Get("input.0.content.1.image_url").String(); got != "data:image/png;base64,cG5nLWJ5dGVz" {
		t.Fatalf("upstream image_url = %q", got)
	}
}

func TestImageGenerationsRouteRecordsModelBlockFromSSEAccessError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.failed","error":{"message":"You do not have access to model gpt-image-2"}}` + "\n\n"))
	}))
	defer upstream.Close()

	authDir := t.TempDir()
	accountPath := filepath.Join(authDir, "acc.json")
	if err := os.WriteFile(accountPath, []byte(`{"access_token":"token","email":"a@example.com","type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write account: %v", err)
	}
	manager := auth.NewManager(authDir, nil, "", 3000, auth.NewRoundRobinSelector(), false, nil)
	if err := manager.LoadAccounts(); err != nil {
		t.Fatalf("load accounts: %v", err)
	}
	h := NewProxyHandler(manager, executor.NewExecutor(upstream.URL, "", executor.HTTPPoolConfig{}), nil, 0, false, "", upstream.URL, false, "", "", 1, 0, nil, false, 0, false, true, true, true, false, false, false, 0, nil)
	r := fasthttprouter.New()
	h.RegisterRoutes(r)

	for i := 0; i < 3; i++ {
		var ctx fasthttp.RequestCtx
		ctx.Request.Header.SetMethod("POST")
		ctx.Request.SetRequestURI("/v1/images/generations")
		ctx.Request.SetBodyString(`{"model":"gpt-image-2","prompt":"draw","n":1}`)
		r.Handler(&ctx)
		if ctx.Response.StatusCode() != fasthttp.StatusBadGateway {
			t.Fatalf("attempt %d status = %d body=%s", i+1, ctx.Response.StatusCode(), ctx.Response.Body())
		}
	}

	accounts := manager.GetAccounts()
	if len(accounts) != 1 {
		t.Fatalf("accounts len = %d, want 1", len(accounts))
	}
	if !accounts[0].IsModelBlocked("gpt-image-2", time.Now()) {
		t.Fatalf("gpt-image-2 should be model-blocked after three SSE access errors")
	}
}

func TestImageGenerationKeepaliveStreamsWhitespaceBeforeDelayedJSON(t *testing.T) {
	results := make(chan imageGenerationHTTPResult, 1)
	writer := &imageKeepaliveTestWriter{}

	go func() {
		time.Sleep(30 * time.Millisecond)
		results <- imageGenerationHTTPResult{
			statusCode: fasthttp.StatusOK,
			body:       []byte(`{"created":1,"data":[{"b64_json":"aW1hZ2U="}]}`),
		}
	}()

	streamImageGenerationJSONWithKeepalive("test-image", writer, results, 10*time.Millisecond)

	got := writer.String()
	if !bytes.HasPrefix([]byte(got), []byte("\n")) {
		t.Fatalf("response should start with JSON whitespace keepalive, got %q", got)
	}
	if !gjson.Valid(got) {
		t.Fatalf("streamed body should remain valid JSON, got %q", got)
	}
	if writer.flushes < 2 {
		t.Fatalf("flushes = %d, want at least 2", writer.flushes)
	}
}

func TestBuildImageRetryConfigPrefersRecentlySuccessfulImageAccount(t *testing.T) {
	authDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(authDir, "a.json"), []byte(`{"access_token":"token-a","email":"a@example.com","type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write first account: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "b.json"), []byte(`{"access_token":"token-b","email":"b@example.com","type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write second account: %v", err)
	}
	manager := auth.NewManager(authDir, nil, "", 3000, auth.NewRoundRobinSelector(), false, nil)
	if err := manager.LoadAccounts(); err != nil {
		t.Fatalf("load accounts: %v", err)
	}
	accounts := manager.GetAccounts()
	if len(accounts) != 2 {
		t.Fatalf("accounts len = %d, want 2", len(accounts))
	}
	var successful *auth.Account
	for _, acc := range accounts {
		if acc.GetEmail() == "b@example.com" {
			successful = acc
			break
		}
	}
	if successful == nil {
		t.Fatalf("b@example.com account not loaded")
	}
	successful.RecordSuccess()

	h := NewProxyHandler(manager, executor.NewExecutor("http://127.0.0.1", "", executor.HTTPPoolConfig{}), nil, 2, true, "", "", false, "", "", 1, 0, nil, false, 0, false, true, true, true, false, false, false, 0, nil)
	rc := h.buildImageRetryConfig()

	picked, err := rc.PickFn("gpt-image-2", map[string]bool{})
	if err != nil {
		t.Fatalf("PickFn() error = %v", err)
	}
	if picked.GetEmail() != "b@example.com" {
		t.Fatalf("picked account = %s, want b@example.com", picked.GetEmail())
	}
}
