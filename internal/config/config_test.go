package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestLoadConfigDefaultsFreeAccountPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.FreeAccountRole != "fallback" {
		t.Fatalf("FreeAccountRole = %q, want fallback", cfg.FreeAccountRole)
	}
	if cfg.FreeAccountCutoff != 70 {
		t.Fatalf("FreeAccountCutoff = %d, want 70", cfg.FreeAccountCutoff)
	}
}

func TestLoadConfigKeepsExplicitZeroFreeAccountCutoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("free-account-cutoff: 0\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.FreeAccountCutoff != 0 {
		t.Fatalf("FreeAccountCutoff = %d, want explicit 0", cfg.FreeAccountCutoff)
	}
}

func TestSanitizeNormalizesFreeAccountPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		role       string
		cutoff     int
		wantRole   string
		wantCutoff int
	}{
		{name: "shared role", role: " Shared ", cutoff: 0, wantRole: "shared", wantCutoff: 0},
		{name: "invalid role and high cutoff", role: "unexpected", cutoff: 101, wantRole: "fallback", wantCutoff: 100},
		{name: "empty role and low cutoff", role: "", cutoff: -1, wantRole: "fallback", wantCutoff: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				FreeAccountRole:   tc.role,
				FreeAccountCutoff: tc.cutoff,
			}
			cfg.Sanitize()
			if cfg.FreeAccountRole != tc.wantRole {
				t.Fatalf("FreeAccountRole = %q, want %q", cfg.FreeAccountRole, tc.wantRole)
			}
			if cfg.FreeAccountCutoff != tc.wantCutoff {
				t.Fatalf("FreeAccountCutoff = %d, want %d", cfg.FreeAccountCutoff, tc.wantCutoff)
			}
		})
	}
}
