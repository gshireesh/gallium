package main

import (
	"bytes"
	"embed"
	"io"
	"os"
	"path/filepath"
	"strings"

	"shireesh.com/gallium/cmd"
	"shireesh.com/gallium/internal/compressor"
)

//go:embed artifacts/*
var templatesZip embed.FS

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}
	return path, nil
}

func createTemporaryDir() (string, error) {
	tempDir, err := os.MkdirTemp("", "gallium-templates")
	if err != nil {
		return "", err
	}
	return tempDir, nil
}

func main() {

	f, err := templatesZip.Open("artifacts/templates.zip")
	if err != nil {
		panic("Template zip not found")
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	// Create a temporary directory to extract the templates
	tempDir, err := createTemporaryDir()
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir) // Clean up the temporary directory after use
	err = compressor.UnzipFromReader(bytes.NewReader(b), int64(len(b)), tempDir)
	if err != nil {
		panic(err)
	}
	cmd.Execute()
}
