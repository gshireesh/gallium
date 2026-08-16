run:
	go run .

build:
	go build -o .out/gallium .

install:
	go install .

VERSION ?= dev

release-assets:
	./scripts/build-release-assets.sh $(VERSION)