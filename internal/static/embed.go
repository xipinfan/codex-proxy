package static

import (
	"embed"
	"fmt"
	"mime"
	"path"
	"strings"
)

//go:embed assets assets/*
var Assets embed.FS

//go:embed assets/index.html
var IndexHTML []byte

func ReadAsset(name string) ([]byte, string, error) {
	clean := path.Clean(strings.TrimPrefix(name, "/"))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "..") {
		return nil, "", fmt.Errorf("invalid asset path")
	}

	data, err := Assets.ReadFile(clean)
	if err != nil {
		return nil, "", err
	}

	contentType := mime.TypeByExtension(path.Ext(clean))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return data, contentType, nil
}
