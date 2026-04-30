package handler

import (
	"codex-proxy/internal/static"
	"regexp"
	"testing"

	fasthttprouter "github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

func TestHandleStaticAssetServesEmbeddedBundleAcrossPathVariants(t *testing.T) {
	handler := &ProxyHandler{}
	assetName := currentBundleAssetName(t)

	for _, filepath := range []string{
		assetName,
		"/" + assetName,
		"assets/" + assetName,
		"/assets/" + assetName,
	} {
		t.Run(filepath, func(t *testing.T) {
			var ctx fasthttp.RequestCtx
			ctx.SetUserValue("filepath", filepath)

			handler.handleStaticAsset(&ctx)

			if got := ctx.Response.StatusCode(); got != fasthttp.StatusOK {
				t.Fatalf("expected status 200 for embedded asset %q, got %d", filepath, got)
			}
		})
	}
}

func currentBundleAssetName(t *testing.T) string {
	t.Helper()

	matches := regexp.MustCompile(`/assets/([^"]+\.js)`).FindSubmatch(static.IndexHTML)
	if len(matches) != 2 {
		t.Fatalf("expected embedded index.html to reference a bundle asset, got %q", string(static.IndexHTML))
	}

	return string(matches[1])
}

func TestAssetsRouteProvidesUsableFilepathParam(t *testing.T) {
	router := fasthttprouter.New()

	var got any
	router.GET("/assets/{filepath:*}", func(ctx *fasthttp.RequestCtx) {
		got = ctx.UserValue("filepath")
		ctx.SetStatusCode(fasthttp.StatusNoContent)
	})

	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/assets/index-B3jxcYWG.js")
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)

	router.Handler(&ctx)

	if ctx.Response.StatusCode() != fasthttp.StatusNoContent {
		t.Fatalf("expected route match, got status %d", ctx.Response.StatusCode())
	}

	switch value := got.(type) {
	case string:
		if value == "" {
			t.Fatal("expected non-empty filepath string")
		}
	case []byte:
		if len(value) == 0 {
			t.Fatal("expected non-empty filepath bytes")
		}
	default:
		t.Fatalf("unexpected filepath param type %T", got)
	}
}
