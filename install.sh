#!/usr/bin/env sh
# gallium installer: fetches the latest release binary from GitHub Releases.
# Installs to ~/.local/bin as a normal user — no sudo needed, and
# `gallium update` can self-replace without elevated permissions.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/gshireesh/gallium/master/install.sh | sh
#
# Override the target directory with GALLIUM_INSTALL_DIR (or INSTALL_DIR).
set -eu

REPO="gshireesh/gallium"
BINARY_NAME="gallium"
INSTALL_DIR="${GALLIUM_INSTALL_DIR:-${INSTALL_DIR:-$HOME/.local/bin}}"

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

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp_file" "$INSTALL_DIR/$BINARY_NAME"
echo "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"

# Warn if another gallium earlier in PATH would shadow this one.
existing="$(command -v "$BINARY_NAME" 2>/dev/null || true)"
if [ -n "$existing" ] && [ "$existing" != "$INSTALL_DIR/$BINARY_NAME" ]; then
	echo "WARNING: $existing comes first in your PATH and will shadow this install."
	echo "         Remove it (e.g. sudo rm $existing) or reorder your PATH."
fi

# Ensure the install dir is on PATH; persist ~/.local/bin in the shell rc.
case ":$PATH:" in
	*":$INSTALL_DIR:"*)
		:
		;;
	*)
		if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
			line='export PATH="$HOME/.local/bin:$PATH"'
			case "${SHELL:-}" in
				*/zsh) rc="$HOME/.zshrc" ;;
				*/bash) rc="$HOME/.bashrc" ;;
				*) rc="$HOME/.profile" ;;
			esac
			if ! grep -qsF '.local/bin' "$rc"; then
				printf '\n# added by gallium installer\n%s\n' "$line" >>"$rc"
				echo "Added ~/.local/bin to PATH in $rc — restart your shell or run:"
				echo "  $line"
			fi
		else
			echo "NOTE: add $INSTALL_DIR to your PATH"
		fi
		;;
esac
