package main

import (
	"embed"

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
	// check if the template zip exists
	err := ZipExists("artifacts/templates.zip")
	if err != nil {
		panic("Template zip not found. Please run the build command to generate it.")
	}
	// extract the templates zip to the ~/.gallium/templates directory
	err = compressor.Unzip("artifacts/templates.zip", "~/gallium/templates")
	if err != nil {
		panic("Failed to extract templates: " + err.Error())
	}
	cmd.Execute()
}
