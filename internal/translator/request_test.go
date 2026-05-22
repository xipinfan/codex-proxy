package translator

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIRequestToCodexPreservesPreviousResponseIDAndPromptCacheKey(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.5",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"previous_response_id":"resp_123",
		"prompt_cache_key":"cache-key-abc"
	}`)

	got := ConvertOpenAIRequestToCodex("gpt-5.5", raw, true, false)

	if value := gjson.GetBytes(got, "previous_response_id").String(); value != "resp_123" {
		t.Fatalf("previous_response_id = %q, want resp_123", value)
	}
	if value := gjson.GetBytes(got, "prompt_cache_key").String(); value != "cache-key-abc" {
		t.Fatalf("prompt_cache_key = %q, want cache-key-abc", value)
	}
}
