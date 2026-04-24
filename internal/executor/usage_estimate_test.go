package executor

import "testing"

func TestEstimateTokensFromOutputText(t *testing.T) {
	text := "Hello world. 这是一段用于估算 token 的中文文本。"
	got := estimateTokensFromOutputText(text, "gpt-5.4")
	if got <= 0 {
		t.Fatalf("expected estimated output tokens > 0, got %d", got)
	}
}

func TestEstimatePromptTokensFromRequest(t *testing.T) {
	body := []byte(`{
  "model":"gpt-5.4",
  "instructions":"你是一个乐于助人的助手",
  "input":[
    {"role":"user","content":[{"type":"input_text","text":"请帮我总结一下今天的会议内容"}]}
  ],
  "tools":[{"type":"function","function":{"name":"search","description":"search docs"}}]
}`)
	got := estimatePromptTokensFromRequest(body, "gpt-5.4")
	if got <= 0 {
		t.Fatalf("expected estimated prompt tokens > 0, got %d", got)
	}
}
