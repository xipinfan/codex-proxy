package handler

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
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

func (h *ProxyHandler) handleImageEdits(ctx *fasthttp.RequestCtx) {
	imgReq, model, count, errResult := parseImageEditRequest(ctx)
	if errResult != nil {
		writeImageGenerationResult(ctx, *errResult)
		return
	}

	reqID := fmt.Sprintf("img-edit-%x", time.Now().UnixNano())
	start := time.Now()
	log.Infof("image edit request accepted req_id=%s model=%s count=%d size=%s quality=%s output_format=%s images=%d", reqID, model, count, imgReq.Size, imgReq.Quality, imgReq.OutputFormat, len(imgReq.Images))

	resultCh := make(chan imageGenerationHTTPResult, 1)
	go func() {
		resultCh <- h.executeImageGenerations(reqID, imgReq, model, count)
	}()

	select {
	case result := <-resultCh:
		log.Infof("image edit response ready req_id=%s status=%d bytes=%d total=%v", reqID, result.statusCode, len(result.body), time.Since(start))
		writeImageGenerationResult(ctx, result)
	case <-time.After(imageGenerationKeepaliveDelay):
		log.Infof("image edit keepalive enabled req_id=%s delay=%v", reqID, imageGenerationKeepaliveDelay)
		ctx.Response.Header.Set("Content-Type", "application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			streamImageGenerationJSONWithKeepalive(reqID, w, resultCh, imageGenerationKeepaliveInterval)
			log.Infof("image edit streamed response finished req_id=%s total=%v", reqID, time.Since(start))
		})
	}
}

func parseImageEditRequest(ctx *fasthttp.RequestCtx) (translator.ImageGenerationRequest, string, int, *imageGenerationHTTPResult) {
	contentType := strings.ToLower(string(ctx.Request.Header.ContentType()))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartImageEditRequest(ctx)
	}
	return parseJSONImageEditRequest(ctx.PostBody())
}

func parseMultipartImageEditRequest(ctx *fasthttp.RequestCtx) (translator.ImageGenerationRequest, string, int, *imageGenerationHTTPResult) {
	form, err := ctx.MultipartForm()
	if err != nil {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, "读取 multipart 请求体失败", "invalid_request_error")
		return translator.ImageGenerationRequest{}, "", 0, &result
	}
	value := func(key string) string {
		values := form.Value[key]
		if len(values) == 0 {
			return ""
		}
		return strings.TrimSpace(values[0])
	}
	imgReq := translator.ImageGenerationRequest{
		Model:        value("model"),
		Prompt:       value("prompt"),
		Size:         value("size"),
		Quality:      value("quality"),
		OutputFormat: value("output_format"),
		Background:   value("background"),
	}
	if imgReq.Model == "" {
		imgReq.Model = translator.DefaultImageModel
	}
	if errResult := validateImageEditBase(&imgReq, value("response_format")); errResult != nil {
		return translator.ImageGenerationRequest{}, "", 0, errResult
	}
	if compression := value("output_compression"); compression != "" {
		if n := gjson.Parse(compression); n.Exists() {
			imgReq.OutputCompression = int(n.Int())
			imgReq.HasCompression = true
		}
	}
	images, err := imageInputsFromMultipartForm(form)
	if err != nil {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, err.Error(), "invalid_request_error")
		return translator.ImageGenerationRequest{}, "", 0, &result
	}
	imgReq.Images = images
	count := imageCountFromString(value("n"))
	return imgReq, imgReq.Model, count, nil
}

func parseJSONImageEditRequest(body []byte) (translator.ImageGenerationRequest, string, int, *imageGenerationHTTPResult) {
	if len(body) == 0 {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, "读取请求体失败", "invalid_request_error")
		return translator.ImageGenerationRequest{}, "", 0, &result
	}
	imgReq := translator.ImageGenerationRequest{
		Model:        strings.TrimSpace(gjson.GetBytes(body, "model").String()),
		Prompt:       strings.TrimSpace(gjson.GetBytes(body, "prompt").String()),
		Size:         strings.TrimSpace(gjson.GetBytes(body, "size").String()),
		Quality:      strings.TrimSpace(gjson.GetBytes(body, "quality").String()),
		OutputFormat: strings.TrimSpace(gjson.GetBytes(body, "output_format").String()),
		Background:   strings.TrimSpace(gjson.GetBytes(body, "background").String()),
	}
	if imgReq.Model == "" {
		imgReq.Model = translator.DefaultImageModel
	}
	if v := gjson.GetBytes(body, "output_compression"); v.Exists() {
		imgReq.OutputCompression = int(v.Int())
		imgReq.HasCompression = true
	}
	if errResult := validateImageEditBase(&imgReq, strings.TrimSpace(gjson.GetBytes(body, "response_format").String())); errResult != nil {
		return translator.ImageGenerationRequest{}, "", 0, errResult
	}
	imgReq.Images = imageInputsFromJSON(gjson.GetBytes(body, "image"))
	if len(imgReq.Images) == 0 {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, "缺少 image 字段", "invalid_request_error")
		return translator.ImageGenerationRequest{}, "", 0, &result
	}
	count := int(gjson.GetBytes(body, "n").Int())
	if count <= 0 {
		count = 1
	}
	if count > translator.MaxImageResults {
		count = translator.MaxImageResults
	}
	return imgReq, imgReq.Model, count, nil
}

func validateImageEditBase(imgReq *translator.ImageGenerationRequest, responseFormat string) *imageGenerationHTTPResult {
	if imgReq.Prompt == "" {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, "缺少 prompt 字段", "invalid_request_error")
		return &result
	}
	if imgReq.Model != translator.DefaultImageModel {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, "仅支持 gpt-image-2 图像模型", "invalid_request_error")
		return &result
	}
	if responseFormat != "" && responseFormat != "b64_json" {
		result := imageGenerationErrorResult(fasthttp.StatusBadRequest, "当前仅支持 response_format=b64_json", "invalid_request_error")
		return &result
	}
	return nil
}

func imageInputsFromMultipartForm(form *multipart.Form) ([]translator.ImageInput, error) {
	if form == nil {
		return nil, errors.New("缺少 image 字段")
	}
	var inputs []translator.ImageInput
	for _, key := range []string{"image", "image[]"} {
		for _, fh := range form.File[key] {
			if fh == nil {
				continue
			}
			file, err := fh.Open()
			if err != nil {
				return nil, fmt.Errorf("读取 image 文件失败: %w", err)
			}
			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if readErr != nil {
				return nil, fmt.Errorf("读取 image 文件失败: %w", readErr)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("关闭 image 文件失败: %w", closeErr)
			}
			if len(data) == 0 {
				return nil, errors.New("image 文件为空")
			}
			inputs = append(inputs, translator.ImageInput{
				MIMEType: detectImageMIMEType(fh.Header.Get("Content-Type"), fh.Filename, data),
				Base64:   base64.StdEncoding.EncodeToString(data),
			})
		}
	}
	if len(inputs) == 0 {
		return nil, errors.New("缺少 image 字段")
	}
	return inputs, nil
}

func detectImageMIMEType(headerValue, filename string, data []byte) string {
	if mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(headerValue)); err == nil && strings.HasPrefix(mediaType, "image/") {
		return mediaType
	}
	if extType := mime.TypeByExtension(filepath.Ext(filename)); strings.HasPrefix(extType, "image/") {
		if mediaType, _, err := mime.ParseMediaType(extType); err == nil {
			return mediaType
		}
		return extType
	}
	if len(data) > 0 {
		if detected := http.DetectContentType(data); strings.HasPrefix(detected, "image/") {
			return detected
		}
	}
	return "image/png"
}

func imageInputsFromJSON(node gjson.Result) []translator.ImageInput {
	if !node.Exists() {
		return nil
	}
	if node.IsArray() {
		var out []translator.ImageInput
		for _, item := range node.Array() {
			out = append(out, imageInputsFromJSON(item)...)
		}
		return out
	}
	if node.Type == gjson.String {
		if input, ok := imageInputFromString(node.String()); ok {
			return []translator.ImageInput{input}
		}
		return nil
	}
	if node.IsObject() {
		for _, key := range []string{"image_url.url", "image_url", "url"} {
			if input, ok := imageInputFromString(node.Get(key).String()); ok {
				return []translator.ImageInput{input}
			}
		}
		for _, key := range []string{"b64_json", "base64", "data"} {
			if input, ok := imageInputFromString(node.Get(key).String()); ok {
				return []translator.ImageInput{input}
			}
		}
	}
	return nil
}

func imageInputFromString(value string) (translator.ImageInput, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return translator.ImageInput{}, false
	}
	if strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return translator.ImageInput{URL: value}, true
	}
	return translator.ImageInput{Base64: value}, true
}

func imageCountFromString(value string) int {
	count := int(gjson.Parse(value).Int())
	if count <= 0 {
		count = 1
	}
	if count > translator.MaxImageResults {
		count = translator.MaxImageResults
	}
	return count
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
