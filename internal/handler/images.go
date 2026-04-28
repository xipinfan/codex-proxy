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
