package upstream

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestEncodeRequestBodyOffLeavesBodyUnchanged(t *testing.T) {
	body := []byte(`{"input":"hello"}`)

	encoded, err := EncodeRequestBody(body, CompressionConfig{
		Mode:     CompressionOff,
		MinBytes: 1,
	})
	if err != nil {
		t.Fatalf("EncodeRequestBody() error = %v", err)
	}
	if !bytes.Equal(encoded.Body, body) {
		t.Fatalf("body changed: got %q, want %q", encoded.Body, body)
	}
	if len(encoded.Headers) != 0 {
		t.Fatalf("headers = %v, want none", encoded.Headers)
	}
}

func TestEncodeRequestBodyAutoLeavesSmallBodyUnchanged(t *testing.T) {
	body := []byte(`{"input":"small"}`)

	encoded, err := EncodeRequestBody(body, CompressionConfig{
		Mode:     CompressionAuto,
		MinBytes: len(body) + 1,
	})
	if err != nil {
		t.Fatalf("EncodeRequestBody() error = %v", err)
	}
	if !bytes.Equal(encoded.Body, body) {
		t.Fatalf("body changed: got %q, want %q", encoded.Body, body)
	}
	if len(encoded.Headers) != 0 {
		t.Fatalf("headers = %v, want none", encoded.Headers)
	}
}

func TestEncodeRequestBodyAutoCompressesLargeBody(t *testing.T) {
	body := bytes.Repeat([]byte(`{"input":"large"}`), 1024)

	encoded, err := EncodeRequestBody(body, CompressionConfig{
		Mode:     CompressionAuto,
		MinBytes: len(body),
	})
	if err != nil {
		t.Fatalf("EncodeRequestBody() error = %v", err)
	}
	if encoded.Headers["Content-Encoding"] != "zstd" {
		t.Fatalf("Content-Encoding = %q, want zstd", encoded.Headers["Content-Encoding"])
	}
	assertZstdBodyEquals(t, encoded.Body, body)
}

func TestEncodeRequestBodyAlwaysCompressesSmallBody(t *testing.T) {
	body := []byte(`{"input":"small"}`)

	encoded, err := EncodeRequestBody(body, CompressionConfig{
		Mode:     CompressionAlways,
		MinBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("EncodeRequestBody() error = %v", err)
	}
	if encoded.Headers["Content-Encoding"] != "zstd" {
		t.Fatalf("Content-Encoding = %q, want zstd", encoded.Headers["Content-Encoding"])
	}
	assertZstdBodyEquals(t, encoded.Body, body)
}

func assertZstdBodyEquals(t *testing.T, compressed, want []byte) {
	t.Helper()

	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("create zstd decoder: %v", err)
	}
	defer decoder.Close()

	got, err := io.ReadAll(decoder)
	if err != nil {
		t.Fatalf("read zstd body: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded body mismatch: got %q, want %q", got, want)
	}
}
