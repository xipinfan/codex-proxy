package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codex-proxy/internal/auth"
)

func TestExecuteImageGenerationUsesImageModelForPickingAndCodexResponses(t *testing.T) {
	var pickedModel string
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %s, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q, want text/event-stream", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aW1hZ2U="}}` + "\n\n"))
	}))
	defer upstream.Close()

	exec := NewExecutor(upstream.URL, "", HTTPPoolConfig{})
	acc := &auth.Account{
		FilePath: "acc.json",
		Token: auth.TokenData{
			Email:       "a@example.com",
			AccessToken: "token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()
	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			pickedModel = model
			return acc, nil
		},
		MaxRetry: 0,
	}

	body := []byte(`{"model":"gpt-5.5","stream":true,"store":false}`)
	got, usedAccount, err := exec.ExecuteImageGeneration(context.Background(), rc, body, "gpt-image-2")
	if err != nil {
		t.Fatalf("ExecuteImageGeneration() error = %v", err)
	}
	if string(got) == "" {
		t.Fatalf("expected SSE body")
	}
	if usedAccount != acc {
		t.Fatalf("used account mismatch")
	}
	if pickedModel != "gpt-image-2" {
		t.Fatalf("picked model = %q, want gpt-image-2", pickedModel)
	}
	if upstreamBody["model"] != "gpt-5.5" {
		t.Fatalf("upstream model = %v", upstreamBody["model"])
	}
}
