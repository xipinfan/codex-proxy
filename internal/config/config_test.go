package config

import "testing"

func TestSanitizeDefaultsListenMaxRequestBodyBytes(t *testing.T) {
	cfg := &Config{}
	cfg.Sanitize()

	if cfg.ListenMaxRequestBodyBytes != DefaultListenMaxRequestBodyBytes {
		t.Fatalf("ListenMaxRequestBodyBytes = %d, want %d", cfg.ListenMaxRequestBodyBytes, DefaultListenMaxRequestBodyBytes)
	}
}

func TestSanitizeRaisesListenMaxRequestBodyBytesToFasthttpDefault(t *testing.T) {
	cfg := &Config{ListenMaxRequestBodyBytes: 1024}
	cfg.Sanitize()

	if cfg.ListenMaxRequestBodyBytes != MinListenMaxRequestBodyBytes {
		t.Fatalf("ListenMaxRequestBodyBytes = %d, want %d", cfg.ListenMaxRequestBodyBytes, MinListenMaxRequestBodyBytes)
	}
}

func TestSanitizeDefaultsUpstreamRequestCompression(t *testing.T) {
	cfg := &Config{}
	cfg.Sanitize()

	if cfg.UpstreamRequestCompression != DefaultUpstreamRequestCompression {
		t.Fatalf("UpstreamRequestCompression = %q, want %q", cfg.UpstreamRequestCompression, DefaultUpstreamRequestCompression)
	}
	if cfg.UpstreamRequestCompressionMinBytes != DefaultUpstreamRequestCompressionMinBytes {
		t.Fatalf("UpstreamRequestCompressionMinBytes = %d, want %d", cfg.UpstreamRequestCompressionMinBytes, DefaultUpstreamRequestCompressionMinBytes)
	}
}

func TestSanitizeUnknownUpstreamRequestCompressionFallsBackToAuto(t *testing.T) {
	cfg := &Config{
		UpstreamRequestCompression:         "unexpected",
		UpstreamRequestCompressionMinBytes: -1,
	}
	cfg.Sanitize()

	if cfg.UpstreamRequestCompression != DefaultUpstreamRequestCompression {
		t.Fatalf("UpstreamRequestCompression = %q, want %q", cfg.UpstreamRequestCompression, DefaultUpstreamRequestCompression)
	}
	if cfg.UpstreamRequestCompressionMinBytes != DefaultUpstreamRequestCompressionMinBytes {
		t.Fatalf("UpstreamRequestCompressionMinBytes = %d, want %d", cfg.UpstreamRequestCompressionMinBytes, DefaultUpstreamRequestCompressionMinBytes)
	}
}
