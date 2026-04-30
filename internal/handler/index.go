/**
 * 首页处理
 */
package handler

import (
	"codex-proxy/internal/static"
	"path"
	"strings"

	"github.com/valyala/fasthttp"
)

/**
 * handleIndex 返回静态首页
 */
func (h *ProxyHandler) handleIndex(ctx *fasthttp.RequestCtx) {
	if len(h.indexHTML) == 0 {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	ctx.SetContentType("text/html; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(h.indexHTML)
}

func (h *ProxyHandler) handleStaticAsset(ctx *fasthttp.RequestCtx) {
	filepath, _ := ctx.UserValue("filepath").(string)
	assetName := strings.TrimPrefix(filepath, "/")
	assetName = strings.TrimPrefix(assetName, "assets/")
	assetPath := path.Join("assets", assetName)
	data, contentType, err := static.ReadAsset(assetPath)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}

	ctx.SetContentType(contentType)
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(data)
}
