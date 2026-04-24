package translator

import (
	"strings"
	"testing"
)

func TestExtractResponseOutputTextFromSSELine(t *testing.T) {
	line := []byte(`data: {"type":"response.output_text.delta","delta":"hello world"}`)
	got := ExtractResponseOutputTextFromSSELine(line)
	if got == "" {
		t.Fatalf("expected non-empty output text from sse line")
	}
}

func TestExtractResponseOutputTextFromCompletedJSON(t *testing.T) {
	raw := []byte(`{
  "type":"response.completed",
  "response":{
    "output":[
      {"type":"reasoning","summary":[{"type":"summary_text","text":"step by step"}]},
      {"type":"message","content":[{"type":"output_text","text":"final answer"}]},
      {"type":"function_call","arguments":"{\"k\":\"v\"}"}
    ]
  }
}`)
	got := ExtractResponseOutputTextFromCompletedJSON(raw)
	if got == "" {
		t.Fatalf("expected non-empty output text from completed response")
	}
}

func TestResponseOutputTextAccumulatorDeduplicatesReasoningDone(t *testing.T) {
	acc := NewResponseOutputTextAccumulator()
	acc.AddSSELine([]byte(`data: {"type":"response.reasoning_text.delta","item_id":"r1","delta":"hello "}`))
	acc.AddSSELine([]byte(`data: {"type":"response.reasoning_text.delta","item_id":"r1","delta":"world"}`))
	acc.AddSSELine([]byte(`data: {"type":"response.reasoning_text.done","item_id":"r1","text":"hello world"}`))

	got := acc.Text()
	if strings.Count(got, "hello world") > 0 {
		t.Fatalf("expected done event to avoid duplicating full reasoning text, got %q", got)
	}
	if strings.Count(got, "hello ") != 1 || strings.Count(got, "world") != 1 {
		t.Fatalf("expected reasoning deltas to remain once, got %q", got)
	}
}

func TestResponseOutputTextAccumulatorDeduplicatesFunctionCallDone(t *testing.T) {
	acc := NewResponseOutputTextAccumulator()
	acc.AddSSELine([]byte(`data: {"type":"response.function_call_arguments.delta","call_id":"call_1","delta":"{\"a\":"}`))
	acc.AddSSELine([]byte(`data: {"type":"response.function_call_arguments.delta","call_id":"call_1","delta":"\"b\"}"}`))
	acc.AddSSELine([]byte(`data: {"type":"response.function_call_arguments.done","call_id":"call_1","arguments":"{\"a\":\"b\"}"}`))
	acc.AddSSELine([]byte(`data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_1","arguments":"{\"a\":\"b\"}"}}`))

	got := acc.Text()
	if strings.Count(got, `{"a":"b"}`) > 0 {
		t.Fatalf("expected full function arguments not to be duplicated after deltas, got %q", got)
	}
	if strings.Count(got, `{"a":`) != 1 || strings.Count(got, `"b"}`) != 1 {
		t.Fatalf("expected function argument deltas to remain once, got %q", got)
	}
}
