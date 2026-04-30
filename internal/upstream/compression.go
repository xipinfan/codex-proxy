package upstream

import (
	"bytes"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

type CompressionMode string

const (
	CompressionOff    CompressionMode = "off"
	CompressionAuto   CompressionMode = "auto"
	CompressionAlways CompressionMode = "always"

	DefaultCompressionMinBytes = 1024 * 1024
)

type CompressionConfig struct {
	Mode     CompressionMode
	MinBytes int
}

type EncodedBody struct {
	Body    []byte
	Headers map[string]string
}

func EncodeRequestBody(body []byte, cfg CompressionConfig) (EncodedBody, error) {
	if !shouldCompress(body, cfg) {
		return EncodedBody{Body: body}, nil
	}

	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		return EncodedBody{}, fmt.Errorf("create zstd encoder: %w", err)
	}
	if _, err = encoder.Write(body); err != nil {
		encoder.Close()
		return EncodedBody{}, fmt.Errorf("write zstd body: %w", err)
	}
	if err = encoder.Close(); err != nil {
		return EncodedBody{}, fmt.Errorf("close zstd encoder: %w", err)
	}

	return EncodedBody{
		Body: buf.Bytes(),
		Headers: map[string]string{
			"Content-Encoding": "zstd",
		},
	}, nil
}

func shouldCompress(body []byte, cfg CompressionConfig) bool {
	if len(body) == 0 {
		return false
	}

	switch cfg.Mode {
	case CompressionOff:
		return false
	case CompressionAlways:
		return true
	case CompressionAuto, "":
		minBytes := cfg.MinBytes
		if minBytes <= 0 {
			minBytes = DefaultCompressionMinBytes
		}
		return len(body) >= minBytes
	default:
		minBytes := cfg.MinBytes
		if minBytes <= 0 {
			minBytes = DefaultCompressionMinBytes
		}
		return len(body) >= minBytes
	}
}
