package translator

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestBuildCodexImageGenerationRequest(t *testing.T) {
	req := ImageGenerationRequest{
		Model:             "gpt-image-2",
		Prompt:            "Draw a small green lighthouse",
		Size:              "1024x1536",
		Quality:           "low",
		OutputFormat:      "jpeg",
		Background:        "opaque",
		OutputCompression: 55,
		HasCompression:    true,
	}

	body, err := BuildCodexImageGenerationRequest(req)
	if err != nil {
		t.Fatalf("BuildCodexImageGenerationRequest() error = %v", err)
	}

	root := gjson.ParseBytes(body)
	if got := root.Get("model").String(); got != "gpt-5.5" {
		t.Fatalf("outer model = %q, want gpt-5.5", got)
	}
	if got := root.Get("instructions").String(); got != "You are an image generation assistant." {
		t.Fatalf("instructions = %q", got)
	}
	if !root.Get("stream").Bool() {
		t.Fatalf("stream should be true")
	}
	if root.Get("store").Bool() {
		t.Fatalf("store should be false")
	}
	if got := root.Get("input.0.content.0.text").String(); got != req.Prompt {
		t.Fatalf("prompt = %q", got)
	}
	tool := root.Get("tools.0")
	if got := tool.Get("type").String(); got != "image_generation" {
		t.Fatalf("tool type = %q", got)
	}
	if got := tool.Get("model").String(); got != "gpt-image-2" {
		t.Fatalf("tool model = %q", got)
	}
	if got := root.Get("tool_choice.type").String(); got != "allowed_tools" {
		t.Fatalf("tool_choice.type = %q", got)
	}
	if got := root.Get("tool_choice.mode").String(); got != "required" {
		t.Fatalf("tool_choice.mode = %q", got)
	}
	if got := root.Get("tool_choice.tools.0.type").String(); got != "image_generation" {
		t.Fatalf("tool_choice.tools.0.type = %q", got)
	}
	if got := tool.Get("size").String(); got != "1024x1536" {
		t.Fatalf("tool size = %q", got)
	}
	if got := tool.Get("quality").String(); got != "low" {
		t.Fatalf("tool quality = %q", got)
	}
	if got := tool.Get("output_format").String(); got != "jpeg" {
		t.Fatalf("tool output_format = %q", got)
	}
	if got := tool.Get("background").String(); got != "opaque" {
		t.Fatalf("tool background = %q", got)
	}
	if got := tool.Get("output_compression").Int(); got != 55 {
		t.Fatalf("tool output_compression = %d", got)
	}
}

func TestBuildCodexImageGenerationRequestWithReferenceImage(t *testing.T) {
	image := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	req := ImageGenerationRequest{
		Model:  "gpt-image-2",
		Prompt: "Edit this image",
		Images: []ImageInput{{
			MIMEType: "image/png",
			Base64:   image,
		}},
	}

	body, err := BuildCodexImageGenerationRequest(req)
	if err != nil {
		t.Fatalf("BuildCodexImageGenerationRequest() error = %v", err)
	}

	root := gjson.ParseBytes(body)
	if got := root.Get("input.0.content.0.type").String(); got != "input_text" {
		t.Fatalf("first content type = %q, want input_text", got)
	}
	imagePart := root.Get("input.0.content.1")
	if got := imagePart.Get("type").String(); got != "input_image" {
		t.Fatalf("image content type = %q, want input_image", got)
	}
	wantURL := "data:image/png;base64," + image
	if got := imagePart.Get("image_url").String(); got != wantURL {
		t.Fatalf("image_url = %q, want %q", got, wantURL)
	}
}

func TestParseCodexImageGenerationSSEOutputItem(t *testing.T) {
	image := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	body := `data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"` + image + `","revised_prompt":"clean prompt"}}` + "\n\n" +
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1}}}` + "\n\n"

	result, err := ParseCodexImageGenerationSSE([]byte(body))
	if err != nil {
		t.Fatalf("ParseCodexImageGenerationSSE() error = %v", err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("images len = %d, want 1", len(result.Images))
	}
	if result.Images[0].Base64 != image {
		t.Fatalf("image base64 mismatch")
	}
	if result.Images[0].RevisedPrompt != "clean prompt" {
		t.Fatalf("revised prompt = %q", result.Images[0].RevisedPrompt)
	}
}

func TestParseCodexImageGenerationSSECompletedOutput(t *testing.T) {
	image := base64.StdEncoding.EncodeToString([]byte("completed-png-bytes"))
	body := `data: {"type":"response.completed","response":{"output":[{"type":"image_generation_call","result":"` + image + `"}]}}` + "\n\n"

	result, err := ParseCodexImageGenerationSSE([]byte(body))
	if err != nil {
		t.Fatalf("ParseCodexImageGenerationSSE() error = %v", err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("images len = %d, want 1", len(result.Images))
	}
	if result.Images[0].Base64 != image {
		t.Fatalf("image base64 mismatch")
	}
}

func TestParseCodexImageGenerationSSEFailure(t *testing.T) {
	body := `data: {"type":"response.failed","error":{"message":"no model access"}}` + "\n\n"
	_, err := ParseCodexImageGenerationSSE([]byte(body))
	if err == nil {
		t.Fatalf("expected failure error")
	}
}

func TestSummarizeCodexImageGenerationRequest(t *testing.T) {
	body, err := BuildCodexImageGenerationRequest(ImageGenerationRequest{
		Model:        "gpt-image-2",
		Prompt:       "Draw a small green lighthouse",
		Size:         "1024x1536",
		Quality:      "low",
		OutputFormat: "png",
	})
	if err != nil {
		t.Fatalf("BuildCodexImageGenerationRequest() error = %v", err)
	}

	got := SummarizeCodexImageGenerationRequest(body)
	want := "outer_model=gpt-5.5 tool_type=image_generation tool_model=gpt-image-2 size=1024x1536 quality=low output_format=png tool_choice_type=allowed_tools tool_choice_mode=required"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestSummarizeCodexImageGenerationSSE(t *testing.T) {
	body := `data: {"type":"response.failed","error":{"message":"bad image tool"}}` + "\n\n"
	got := SummarizeCodexImageGenerationSSE([]byte(body), 200)
	want := `event=response.failed error="bad image tool" bytes=71`
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestSummarizeCodexImageGenerationSSERedactsImageResult(t *testing.T) {
	body := `data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"SECRET_BASE64"}}` + "\n\n" +
		`data: {"type":"response.completed","response":{"output":[]}}` + "\n\n"

	got := SummarizeCodexImageGenerationSSE([]byte(body), 200)
	want := "events=2 first=response.output_item.done last=response.completed image_calls=1 has_image_result=true bytes=171"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if strings.Contains(got, "SECRET_BASE64") {
		t.Fatalf("summary leaked image result: %q", got)
	}
}
