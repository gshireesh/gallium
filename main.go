package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"shireesh.com/gallium/cmd"
)

//go:embed all:templates
var embeddedTemplates embed.FS

func createTemporaryDir() (string, error) {
	tempDir, err := os.MkdirTemp("", "gallium-templates")
	if err != nil {
		return "", err
	}
	return tempDir, nil
}

func copyTemplates(dst string) error {
	return fs.WalkDir(embeddedTemplates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "templates" {
			return nil
		}

		relPath, err := filepath.Rel("templates", path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)
		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return err
		}

		src, err := embeddedTemplates.Open(path)
		if err != nil {
			return err
		}

		fileMode := info.Mode().Perm()
		if fileMode == 0 {
			fileMode = 0644
		}

		out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
		if err != nil {
			src.Close()
			return err
		}

		_, copyErr := io.Copy(out, src)
		closeErr := out.Close()
		srcErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return srcErr
	})
}

func run() error {
	tempDir, err := createTemporaryDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	if err := copyTemplates(tempDir); err != nil {
		return fmt.Errorf("failed to prepare embedded templates: %w", err)
	}

	cmd.Execute(tempDir)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gallium: %v\n", err)
		os.Exit(1)
	}
}
