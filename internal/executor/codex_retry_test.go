package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/upstream"

	"github.com/klauspost/compress/zstd"
)

func TestConcurrentRetryAfter429DoesNotIgnoreCooldownAccounts(t *testing.T) {
	executor := &Executor{httpClient: &http.Client{}}
	ignoreCooldownPicks := 0

	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return nil, fmt.Errorf("no regular accounts")
		},
		PickIgnoringCooldownFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			ignoreCooldownPicks++
			return &auth.Account{FilePath: fmt.Sprintf("cooldown-%d", ignoreCooldownPicks)}, nil
		},
	}

	_, _, _, _ = executor.concurrentRetryAfter429(context.Background(), rc, "gpt-5", "http://127.0.0.1/", []byte("{}"), false, nil)

	if ignoreCooldownPicks != 0 {
		t.Fatalf("expected concurrent 429 retry to respect cooldown, picked ignoring cooldown %d times", ignoreCooldownPicks)
	}
}

func TestModelAccessErrorDoesNotCooldownWholeAccount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"You do not have access to model gpt-5.5"}}`))
	}))
	defer upstream.Close()

	acc := &auth.Account{
		FilePath: "model-access.json",
		Token: auth.TokenData{
			Email:       "free@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()
	manager := auth.NewManager(t.TempDir(), nil, "", 3000, auth.NewRoundRobinSelector(), false, nil)
	executor := &Executor{httpClient: upstream.Client()}
	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
		OnAfterUpstreamErrFn: func(acc *auth.Account, model string, statusCode int, errBody []byte) bool {
			return manager.RecordModelFailureIfAccessError(acc, model, statusCode, errBody)
		},
	}

	_, _, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstream.URL, []byte("{}"), false)
	if err == nil {
		t.Fatalf("expected upstream 403 error")
	}
	stats := acc.GetStats()
	if stats.Status != "active" || !stats.Pickable {
		t.Fatalf("model access error should not cooldown whole account, status=%s pickable=%v", stats.Status, stats.Pickable)
	}
}

func TestImageGenerationToolUnavailableSwitchesAccount(t *testing.T) {
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Tool choice 'image_generation' not found in 'tools' parameter."}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aW1hZ2U="}}` + "\n\n"))
	}))
	defer upstream.Close()

	acc1 := &auth.Account{
		FilePath: "image-no-tool.json",
		Token: auth.TokenData{
			Email:       "image-no-tool@example.com",
			AccessToken: "access-token-1",
		},
		Status: auth.StatusActive,
	}
	acc1.SetActive()
	acc2 := &auth.Account{
		FilePath: "image-ok.json",
		Token: auth.TokenData{
			Email:       "image-ok@example.com",
			AccessToken: "access-token-2",
		},
		Status: auth.StatusActive,
	}
	acc2.SetActive()

	executor := &Executor{httpClient: upstream.Client()}
	rc := RetryConfig{
		MaxRetry: 1,
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			if !excluded[acc1.FilePath] {
				return acc1, nil
			}
			return acc2, nil
		},
	}

	resp, used, attempts, err := executor.sendWithRetry(context.Background(), rc, "gpt-image-2", upstream.URL, []byte("{}"), true)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if used != acc2 {
		t.Fatalf("used account = %s, want %s", used.GetEmail(), acc2.GetEmail())
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestSendWithRetryCompressesLargeUpstreamBody(t *testing.T) {
	wantBody := bytes.Repeat([]byte(`{"input":"multi-image"}`), 1024)
	var sawRequest bool

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		if got := r.Header.Get("Content-Encoding"); got != "zstd" {
			t.Fatalf("Content-Encoding = %q, want zstd", got)
		}

		decoder, err := zstd.NewReader(r.Body)
		if err != nil {
			t.Fatalf("create zstd decoder: %v", err)
		}
		defer decoder.Close()

		gotBody, err := io.ReadAll(decoder)
		if err != nil {
			t.Fatalf("read zstd body: %v", err)
		}
		if !bytes.Equal(gotBody, wantBody) {
			t.Fatalf("decoded body mismatch")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstreamServer.Close()

	acc := &auth.Account{
		FilePath: "compressed.json",
		Token: auth.TokenData{
			Email:       "compressed@example.com",
			AccessToken: "access-token",
		},
		Status: auth.StatusActive,
	}
	acc.SetActive()

	executor := NewExecutor(upstreamServer.URL, "", HTTPPoolConfig{
		UpstreamRequestCompression: upstream.CompressionConfig{
			Mode:     upstream.CompressionAuto,
			MinBytes: 1,
		},
	})
	rc := RetryConfig{
		PickFn: func(model string, excluded map[string]bool) (*auth.Account, error) {
			return acc, nil
		},
	}

	resp, _, _, err := executor.sendWithRetry(context.Background(), rc, "gpt-5.5", upstreamServer.URL, wantBody, false)
	if err != nil {
		t.Fatalf("sendWithRetry() error = %v", err)
	}
	defer resp.Body.Close()
	if !sawRequest {
		t.Fatal("upstream server did not receive request")
	}
}
