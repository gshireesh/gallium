package main

import "shireesh.com/gallium/internal/compressor"

func main() {
	err := compressor.ZipDir("templates", "./artifacts/templates.zip")
	if err != nil {
		panic(err)
	}
}
