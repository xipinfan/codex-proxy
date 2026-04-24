package executor

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"codex-proxy/internal/auth"
)

func TestPumpRawSSERecordsUsageFromCompletedEvent(t *testing.T) {
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1200,\"output_tokens\":3400,\"total_tokens\":4600}}}\n\n"

	acc := &auth.Account{
		Token: auth.TokenData{
			Email: "stream@example.com",
		},
	}
	stream := &CodexResponsesStream{
		body:       io.NopCloser(bytes.NewBufferString(body)),
		account:    acc,
		BaseModel:  "gpt-5.4",
		Attempts:   1,
		pumpRounds: 1,
	}

	var out bytes.Buffer
	if err := stream.PumpRawSSE(&out, nil); err != nil {
		t.Fatalf("PumpRawSSE returned error: %v", err)
	}

	if got := acc.TotalRequests.Load(); got != 1 {
		t.Fatalf("expected one successful request, got %d", got)
	}
	if got := acc.TotalInputTokens.Load(); got != 1200 {
		t.Fatalf("expected input tokens to be recorded, got %d", got)
	}
	if got := acc.TotalOutputTokens.Load(); got != 3400 {
		t.Fatalf("expected output tokens to be recorded, got %d", got)
	}
	if got := acc.TotalTokens.Load(); got != 4600 {
		t.Fatalf("expected total tokens to be recorded, got %d", got)
	}
}

func TestPumpRawSSEEstimatesUsageWhenCompletedHasNoUsage(t *testing.T) {
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"this is a long enough output for estimating tokens\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\"}}\n\n"

	acc := &auth.Account{
		Token: auth.TokenData{
			Email: "stream-estimate@example.com",
		},
	}
	stream := &CodexResponsesStream{
		body:                  io.NopCloser(bytes.NewBufferString(body)),
		account:               acc,
		BaseModel:             "gpt-5.4",
		Attempts:              1,
		pumpRounds:            1,
		estimatedPromptTokens: 120,
	}

	var out bytes.Buffer
	if err := stream.PumpRawSSE(&out, nil); err != nil {
		t.Fatalf("PumpRawSSE returned error: %v", err)
	}

	if got := acc.TotalRequests.Load(); got != 1 {
		t.Fatalf("expected one successful request, got %d", got)
	}
	if got := acc.TotalOutputTokens.Load(); got <= 0 {
		t.Fatalf("expected estimated output tokens to be recorded, got %d", got)
	}
	if got := acc.TotalInputTokens.Load(); got <= 0 {
		t.Fatalf("expected estimated prompt tokens to be recorded, got %d", got)
	}
	if got := acc.TotalTokens.Load(); got < acc.TotalInputTokens.Load()+acc.TotalOutputTokens.Load() {
		t.Fatalf("expected total tokens to cover input+output, got total=%d input=%d output=%d", got, acc.TotalInputTokens.Load(), acc.TotalOutputTokens.Load())
	}
}

func TestCompactPumpParsesUsageWithoutTrailingNewline(t *testing.T) {
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1200,\"output_tokens\":3400,\"total_tokens\":4600}}}"

	compact := &CodexCompactStream{
		Resp: &http.Response{
			Body: io.NopCloser(bytes.NewBufferString(body)),
		},
		BaseModel:             "gpt-5.4",
		estimatedPromptTokens: 100,
	}

	var out bytes.Buffer
	if err := compact.PumpBody(&out, nil); err != nil {
		t.Fatalf("PumpBody returned error: %v", err)
	}

	usage := compact.Usage()
	if usage.InputTokens != 1200 || usage.OutputTokens != 3400 || usage.TotalTokens != 4600 {
		t.Fatalf("expected usage from trailing completed event, got %+v", usage)
	}
}
