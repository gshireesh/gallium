package main

import (
	"bytes"
	"embed"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	// remove existing templates directory if it exists
	err = compressor.RemoveDir("~/gallium/templates")
	if err != nil {
		panic(err)
	}

	path, err := expandPath("~/gallium/templates")
	if err != nil {
		panic(err)
	}
	err = compressor.UnzipFromReader(bytes.NewReader(b), int64(len(b)), path)
	if err != nil {
		panic(err)
	}
	//cmd.Execute()
}
