package translator

import "testing"

func TestExtractResponseUsageFromSSELine(t *testing.T) {
	line := []byte(`data: {"type":"response.completed","response":{"usage":{"input_tokens":1200,"output_tokens":3400,"total_tokens":4600,"input_tokens_details":{"cached_tokens":800}}}}`)

	usage := ExtractResponseUsageFromSSELine(line)
	if !usage.FoundCompleted {
		t.Fatalf("expected completed event to be detected")
	}
	if !usage.FoundUsage {
		t.Fatalf("expected usage to be detected")
	}
	if usage.InputTokens != 1200 || usage.OutputTokens != 3400 || usage.TotalTokens != 4600 || usage.CachedTokens != 800 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestExtractResponseUsageFromResponseObjectIncludesCachedTokens(t *testing.T) {
	raw := []byte(`{"usage":{"input_tokens":2000,"output_tokens":100,"total_tokens":2100,"input_tokens_details":{"cached_tokens":1536}}}`)

	usage := ExtractResponseUsageFromResponseObjectJSON(raw)
	if !usage.FoundUsage {
		t.Fatalf("expected usage to be detected")
	}
	if usage.CachedTokens != 1536 {
		t.Fatalf("CachedTokens = %d, want 1536", usage.CachedTokens)
	}
}

func TestExtractResponseUsageFromCompletedJSONBackfillsTotal(t *testing.T) {
	raw := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":15,"output_tokens":25,"total_tokens":0}}}`)

	usage := ExtractResponseUsageFromCompletedJSON(raw)
	if usage.TotalTokens != 40 {
		t.Fatalf("expected total_tokens to fall back to input+output, got %+v", usage)
	}
}
