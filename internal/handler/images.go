package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/executor"
	"codex-proxy/internal/translator"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

const (
	imageGenerationKeepaliveDelay    = 15 * time.Second
	imageGenerationKeepaliveInterval = 15 * time.Second
)

type imageGenerationHTTPResult struct {
	statusCode int
	body       []byte
}

type imageGenerationFlushWriter interface {
	Write([]byte) (int, error)
	Flush() error
}

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

	reqID := fmt.Sprintf("img-%x", time.Now().UnixNano())
	start := time.Now()
	log.Infof("image generation request accepted req_id=%s model=%s count=%d size=%s quality=%s output_format=%s", reqID, model, count, imgReq.Size, imgReq.Quality, imgReq.OutputFormat)

	resultCh := make(chan imageGenerationHTTPResult, 1)
	go func() {
		resultCh <- h.executeImageGenerations(reqID, imgReq, model, count)
	}()

	select {
	case result := <-resultCh:
		log.Infof("image generation response ready req_id=%s status=%d bytes=%d total=%v", reqID, result.statusCode, len(result.body), time.Since(start))
		writeImageGenerationResult(ctx, result)
	case <-time.After(imageGenerationKeepaliveDelay):
		log.Infof("image generation keepalive enabled req_id=%s delay=%v", reqID, imageGenerationKeepaliveDelay)
		ctx.Response.Header.Set("Content-Type", "application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			streamImageGenerationJSONWithKeepalive(reqID, w, resultCh, imageGenerationKeepaliveInterval)
			log.Infof("image generation streamed response finished req_id=%s total=%v", reqID, time.Since(start))
		})
	}
}

func (h *ProxyHandler) executeImageGenerations(reqID string, imgReq translator.ImageGenerationRequest, model string, count int) imageGenerationHTTPResult {
	rc := h.buildImageRetryConfig()
	images := make([]translator.CodexImage, 0, count)
	for i := 0; i < count; i++ {
		codexBody, err := translator.BuildCodexImageGenerationRequest(imgReq)
		if err != nil {
			return imageGenerationErrorResult(fasthttp.StatusBadRequest, err.Error(), "invalid_request_error")
		}
		log.Infof("image generation upstream request req_id=%s iteration=%d/%d %s", reqID, i+1, count, translator.SummarizeCodexImageGenerationRequest(codexBody))
		raw, account, err := h.executor.ExecuteImageGeneration(context.Background(), rc, codexBody, model)
		if err != nil {
			log.Warnf("image generation upstream transport failed req_id=%s iteration=%d/%d model=%s err=%v", reqID, i+1, count, model, err)
			return imageGenerationExecutorErrorResult(err)
		}
		parsed, err := translator.ParseCodexImageGenerationSSE(raw)
		if err != nil {
			accountEmail := ""
			if account != nil {
				accountEmail = account.GetEmail()
			}
			log.Warnf("image generation parse failed req_id=%s iteration=%d/%d model=%s account=%s err=%v upstream=%s", reqID, i+1, count, model, accountEmail, err, translator.SummarizeCodexImageGenerationSSE(raw, 2048))
			h.recordImageModelFailureFromSSEError(account, model, err)
			return imageGenerationErrorResult(fasthttp.StatusBadGateway, err.Error(), "bad_gateway")
		}
		if account != nil {
			account.ClearModelAccessFailure(model)
			account.RecordSuccess()
		}
		images = append(images, parsed.Images...)
		if len(images) >= count {
			images = images[:count]
			break
		}
	}

	resp, err := translator.MarshalOpenAIImageResponse(time.Now().Unix(), images)
	if err != nil {
		return imageGenerationErrorResult(fasthttp.StatusInternalServerError, "json编码失败", "server_error")
	}
	RecordRequest()
	return imageGenerationHTTPResult{statusCode: fasthttp.StatusOK, body: resp}
}

func (h *ProxyHandler) buildImageRetryConfig() executor.RetryConfig {
	rc := h.buildRetryConfig()
	if h == nil || h.manager == nil || rc.PickFn == nil {
		return rc
	}
	basePick := rc.PickFn
	rc.PickFn = func(model string, excluded map[string]bool) (*auth.Account, error) {
		if acc, err := h.manager.PickRecentlySuccessfulOnly(model, excluded); err == nil {
			log.Debugf("图片生成优先选择最近成功账号 account=%s model=%s", acc.GetEmail(), model)
			return acc, nil
		}
		return basePick(model, excluded)
	}
	return rc
}

func writeImageGenerationResult(ctx *fasthttp.RequestCtx, result imageGenerationHTTPResult) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.SetStatusCode(result.statusCode)
	ctx.SetBody(result.body)
}

func streamImageGenerationJSONWithKeepalive(reqID string, w imageGenerationFlushWriter, results <-chan imageGenerationHTTPResult, interval time.Duration) {
	if interval <= 0 {
		interval = imageGenerationKeepaliveInterval
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		log.Warnf("image generation initial keepalive write failed req_id=%s err=%v", reqID, err)
		return
	}
	if err := w.Flush(); err != nil {
		log.Warnf("image generation initial keepalive flush failed req_id=%s err=%v", reqID, err)
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case result := <-results:
			if result.statusCode < 200 || result.statusCode >= 300 {
				log.Warnf("image generation completed after keepalive with error req_id=%s status=%d", reqID, result.statusCode)
			}
			_, _ = w.Write(result.body)
			_ = w.Flush()
			return
		case <-ticker.C:
			if _, err := w.Write([]byte("\n")); err != nil {
				log.Warnf("image generation keepalive write failed req_id=%s err=%v", reqID, err)
				return
			}
			if err := w.Flush(); err != nil {
				log.Warnf("image generation keepalive flush failed req_id=%s err=%v", reqID, err)
				return
			}
		}
	}
}

func imageGenerationExecutorErrorResult(err error) imageGenerationHTTPResult {
	if errors.Is(err, context.Canceled) {
		return imageGenerationErrorResult(fasthttp.StatusBadGateway, "请求已取消或客户端断开", "request_cancelled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return imageGenerationErrorResult(fasthttp.StatusGatewayTimeout, "请求处理超时", "timeout")
	}
	if errors.Is(err, executor.ErrEmptyResponse) {
		return imageGenerationErrorResult(fasthttp.StatusBadGateway, "上游未返回有效内容（空响应）", "bad_gateway")
	}
	var statusErr *executor.StatusError
	if errors.As(err, &statusErr) {
		if gjson.ValidBytes(statusErr.Body) && gjson.GetBytes(statusErr.Body, "error").Exists() {
			return imageGenerationHTTPResult{statusCode: statusErr.Code, body: statusErr.Body}
		}
		return imageGenerationErrorResult(statusErr.Code, summarizeUpstreamError(statusErr.Body), "api_error")
	}
	return imageGenerationErrorResult(fasthttp.StatusBadGateway, err.Error(), "bad_gateway")
}

func imageGenerationErrorResult(status int, message, errType string) imageGenerationHTTPResult {
	body, err := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errType,
		},
	})
	if err != nil {
		body = []byte(fmt.Sprintf(`{"error":{"message":%q,"type":"server_error"}}`, "json编码失败"))
		status = fasthttp.StatusInternalServerError
	}
	return imageGenerationHTTPResult{statusCode: status, body: body}
}

func (h *ProxyHandler) recordImageModelFailureFromSSEError(account *auth.Account, model string, err error) {
	if h == nil || h.manager == nil || account == nil || err == nil {
		return
	}
	body, marshalErr := json.Marshal(map[string]any{
		"error": map[string]any{"message": err.Error()},
	})
	if marshalErr != nil {
		body = []byte(err.Error())
	}
	h.manager.RecordModelFailureIfAccessError(account, model, fasthttp.StatusForbidden, body)
}
