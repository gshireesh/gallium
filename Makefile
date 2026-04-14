run:
	go run .

build:
	go build -o .out/gallium .

VERSION ?= dev

release-assets:
	./scripts/build-release-assets.sh $(VERSION)