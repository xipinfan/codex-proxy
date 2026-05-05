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
	choice := `{"type":"allowed_tools","mode":"required","tools":[{"type":"image_generation"}]}`
	out, _ = sjson.SetRaw(out, "tool_choice", choice)
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

func SummarizeCodexImageGenerationRequest(body []byte) string {
	root := gjson.ParseBytes(body)
	parts := []string{
		"outer_model=" + root.Get("model").String(),
		"tool_type=" + root.Get("tools.0.type").String(),
		"tool_model=" + root.Get("tools.0.model").String(),
		"size=" + root.Get("tools.0.size").String(),
		"quality=" + root.Get("tools.0.quality").String(),
		"output_format=" + root.Get("tools.0.output_format").String(),
		"tool_choice_type=" + root.Get("tool_choice.type").String(),
		"tool_choice_mode=" + root.Get("tool_choice.mode").String(),
	}
	return strings.Join(parts, " ")
}

func SummarizeCodexImageGenerationSSE(body []byte, maxPrefix int) string {
	eventCount := 0
	firstType := ""
	lastType := ""
	imageCalls := 0
	hasImageResult := false
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" || !gjson.Valid(data) {
			continue
		}
		event := gjson.Parse(data)
		typ := event.Get("type").String()
		if typ != "" {
			eventCount++
			if firstType == "" {
				firstType = typ
			}
			lastType = typ
		}
		item := event.Get("item")
		if item.Get("type").String() == "image_generation_call" {
			imageCalls++
			if item.Get("result").String() != "" {
				hasImageResult = true
			}
		}
		for _, output := range event.Get("response.output").Array() {
			if output.Get("type").String() != "image_generation_call" {
				continue
			}
			imageCalls++
			if output.Get("result").String() != "" {
				hasImageResult = true
			}
		}
		if typ == "response.failed" || typ == "error" {
			msg := event.Get("error.message").String()
			if msg == "" {
				msg = event.Get("message").String()
			}
			if msg != "" {
				return fmt.Sprintf("event=%s error=%q bytes=%d", typ, msg, len(body))
			}
			return fmt.Sprintf("event=%s bytes=%d", typ, len(body))
		}
	}
	if eventCount > 0 {
		return fmt.Sprintf("events=%d first=%s last=%s image_calls=%d has_image_result=%v bytes=%d", eventCount, firstType, lastType, imageCalls, hasImageResult, len(body))
	}
	prefix := strings.TrimSpace(string(body))
	if maxPrefix > 0 && len(prefix) > maxPrefix {
		prefix = prefix[:maxPrefix] + "...(truncated)"
	}
	return fmt.Sprintf("bytes=%d prefix=%q", len(body), prefix)
}
