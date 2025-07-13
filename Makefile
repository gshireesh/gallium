zip_artifacts:
	go run ./cmd/compress_templates/main.go

run:
	go run main.go

build: zip_artifacts
	go build -o .out/gallium main.go