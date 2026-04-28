package handler

import (
	"testing"

	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

func TestHandleModelsIncludesGPTImage2(t *testing.T) {
	h := &ProxyHandler{}
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetMethod("GET")

	h.handleModels(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("status = %d, want 200", ctx.Response.StatusCode())
	}
	data := gjson.GetBytes(ctx.Response.Body(), "data").Array()
	found := false
	for _, item := range data {
		if item.Get("id").String() == "gpt-image-2" {
			found = true
		}
		if item.Get("id").String() == "gpt-image-2-fast" {
			t.Fatalf("image model should not get text suffix variants")
		}
	}
	if !found {
		t.Fatalf("gpt-image-2 not found in /v1/models")
	}
}
