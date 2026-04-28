package handler

import (
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
