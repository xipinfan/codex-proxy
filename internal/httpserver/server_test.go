package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestNewSetsMaxRequestBodySize(t *testing.T) {
	const maxBody = 32 * 1024 * 1024

	srv := New(func(ctx *fasthttp.RequestCtx) {}, Options{
		Name:               "test server",
		MaxRequestBodySize: maxBody,
	})

	if srv.MaxRequestBodySize != maxBody {
		t.Fatalf("MaxRequestBodySize = %d, want %d", srv.MaxRequestBodySize, maxBody)
	}
}

func TestNewUsesJSONErrorHandlerForParseErrors(t *testing.T) {
	srv := New(func(ctx *fasthttp.RequestCtx) {}, Options{})
	if srv.ErrorHandler == nil {
		t.Fatal("ErrorHandler is nil")
	}

	var ctx fasthttp.RequestCtx
	srv.ErrorHandler(&ctx, errors.New("body size exceeds the given limit"))

	if got := ctx.Response.StatusCode(); got != fasthttp.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", got, fasthttp.StatusRequestEntityTooLarge)
	}
	if got := string(ctx.Response.Header.ContentType()); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var payload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(ctx.Response.Body(), &payload); err != nil {
		t.Fatalf("unmarshal response body: %v; body=%s", err, ctx.Response.Body())
	}
	if payload.Error.Type != "invalid_request_error" {
		t.Fatalf("error.type = %q, want invalid_request_error", payload.Error.Type)
	}
	if payload.Error.Code != "request_body_too_large" {
		t.Fatalf("error.code = %q, want request_body_too_large", payload.Error.Code)
	}
	if payload.Error.Message == "" {
		t.Fatal("error.message is empty")
	}
}

func TestServerHandlesConfiguredBodyLimitWithJSONParseError(t *testing.T) {
	var handlerCalls atomic.Int32
	srv := New(func(ctx *fasthttp.RequestCtx) {
		handlerCalls.Add(1)
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyString("ok")
	}, Options{
		MaxRequestBodySize: 64,
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Shutdown()
		select {
		case err := <-errCh:
			if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				t.Errorf("server returned error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	})

	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://" + ln.Addr().String() + "/v1/responses"

	belowResp, err := client.Post(baseURL, "application/json", bytes.NewReader(bytes.Repeat([]byte("a"), 32)))
	if err != nil {
		t.Fatalf("post below limit: %v", err)
	}
	belowBody, _ := io.ReadAll(belowResp.Body)
	_ = belowResp.Body.Close()
	if belowResp.StatusCode != http.StatusOK {
		t.Fatalf("below-limit status = %d, want %d; body=%s", belowResp.StatusCode, http.StatusOK, belowBody)
	}
	if string(belowBody) != "ok" {
		t.Fatalf("below-limit body = %q, want ok", belowBody)
	}
	if got := handlerCalls.Load(); got != 1 {
		t.Fatalf("handler calls after below-limit request = %d, want 1", got)
	}

	overResp, err := client.Post(baseURL, "application/json", bytes.NewReader(bytes.Repeat([]byte("b"), 128)))
	if err != nil {
		t.Fatalf("post over limit: %v", err)
	}
	defer overResp.Body.Close()
	overBody, err := io.ReadAll(overResp.Body)
	if err != nil {
		t.Fatalf("read over-limit response: %v", err)
	}
	if overResp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-limit status = %d, want %d; body=%s", overResp.StatusCode, http.StatusRequestEntityTooLarge, overBody)
	}
	if got := overResp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("over-limit Content-Type = %q, want application/json", got)
	}
	if bytes.Contains(overBody, []byte("Error when parsing request")) {
		t.Fatalf("over-limit response contains fasthttp default parse error: %s", overBody)
	}
	if got := handlerCalls.Load(); got != 1 {
		t.Fatalf("handler calls after over-limit request = %d, want 1", got)
	}

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(overBody, &payload); err != nil {
		t.Fatalf("unmarshal over-limit response: %v; body=%s", err, overBody)
	}
	if payload.Error.Code != "request_body_too_large" {
		t.Fatalf("error.code = %q, want request_body_too_large", payload.Error.Code)
	}
}
