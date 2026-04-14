#!/usr/bin/env sh

set -eu

REPO="gshireesh/gallium"
BINARY_NAME="gallium"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
	darwin|linux)
		:
		;;
	*)
		echo "unsupported operating system: $os" >&2
		exit 1
		;;
esac

case "$arch" in
	x86_64|amd64)
		arch="amd64"
		;;
	aarch64|arm64)
		arch="arm64"
		;;
	*)
		echo "unsupported architecture: $arch" >&2
		exit 1
		;;
esac

download_url="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}_${os}_${arch}"
tmp_file="$(mktemp)"

cleanup() {
	rm -f "$tmp_file"
}

trap cleanup EXIT INT TERM

echo "Downloading ${download_url}"
curl -fsSL "$download_url" -o "$tmp_file"
chmod +x "$tmp_file"

if [ ! -d "$INSTALL_DIR" ]; then
	parent_dir="$(dirname "$INSTALL_DIR")"
	if [ -w "$parent_dir" ]; then
		mkdir -p "$INSTALL_DIR"
	else
		sudo mkdir -p "$INSTALL_DIR"
	fi
fi

if [ -w "$INSTALL_DIR" ]; then
	install -m 0755 "$tmp_file" "$INSTALL_DIR/$BINARY_NAME"
else
	sudo install -m 0755 "$tmp_file" "$INSTALL_DIR/$BINARY_NAME"
fi

echo "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"