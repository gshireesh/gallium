package main

import (
	"bytes"
	"embed"
	"io"

	"shireesh.com/gallium/cmd"
	"shireesh.com/gallium/internal/compressor"
)

//go:embed artifacts/*
var templatesZip embed.FS

func ZipExists(zipLoc string) error {
	_, err := templatesZip.ReadFile(zipLoc)
	if err != nil {
		return err
	}
	return nil
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

	err = compressor.UnzipFromReader(bytes.NewReader(b), int64(len(b)), "~/gallium/templates")
	if err != nil {
		panic(err)
	}
	cmd.Execute()
}
