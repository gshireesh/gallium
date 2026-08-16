package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"shireesh.com/gallium/cmd"
)

// The directory is underscore-prefixed so Go tooling ignores template
// source files (e.g. go-basic's main.go); all: still embeds them.
//
//go:embed all:_templates
var embeddedTemplates embed.FS

func main() {
	templates, err := fs.Sub(embeddedTemplates, "_templates")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gallium: %v\n", err)
		os.Exit(1)
	}
	cmd.Execute(templates)
}
