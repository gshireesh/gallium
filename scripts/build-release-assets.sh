#!/usr/bin/env bash

set -euo pipefail

VERSION="${1:-dev}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
TARGETS=(
	"darwin/amd64"
	"darwin/arm64"
	"linux/amd64"
	"linux/arm64"
)

mkdir -p "${DIST_DIR}"
rm -f "${DIST_DIR}"/gallium_* "${DIST_DIR}"/checksums.txt

for target in "${TARGETS[@]}"; do
	IFS=/ read -r goos goarch <<< "${target}"
	output="${DIST_DIR}/gallium_${goos}_${goarch}"
	CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
		go build -trimpath -ldflags "-s -w -X shireesh.com/gallium/cmd.Version=${VERSION}" -o "${output}" "${ROOT_DIR}"
done

(
	cd "${DIST_DIR}"
	shasum -a 256 gallium_* > checksums.txt
)