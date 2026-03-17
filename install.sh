#!/bin/bash
set -e

OS=$(uname -s | tr 'A-Z' 'a-z')

ARCH=$(uname -m)
case "$ARCH" in
	x86_64)
		ARCH=amd64
		;;
	aarch64|arm64)
		ARCH=arm64
		;;
	*)
		echo "Unsupported architecture: $ARCH" >&2
		exit 1
		;;
esac

echo "Select the ffwebp variant to install:"
echo "  1) core"
echo "  2) full (default)"
echo "  3) full-play"
read -p "Enter choice [1-3]: " VARIANT_CHOICE || true

case "$VARIANT_CHOICE" in
	1)
		VARIANT="core"
		;;
	3)
		VARIANT="full-play"
		;;
	*)
		VARIANT="full"
		;;
esac

echo "Resolving latest version..."

VERSION=$(curl -sL https://api.github.com/repos/coalaura/ffwebp/releases/latest | grep -Po '"tag_name": *"\K.*?(?=")')

if ! printf '%s\n' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "Error: '$VERSION' is not in vMAJOR.MINOR.PATCH format" >&2
	exit 1
fi

rm -f /tmp/ffwebp

BIN="ffwebp_${VERSION}_${VARIANT}_${OS}_${ARCH}"
URL="https://github.com/coalaura/ffwebp/releases/download/${VERSION}/${BIN}"

echo "Downloading ${BIN}..."

if ! curl -sL "$URL" -o /tmp/ffwebp; then
	echo "Error: failed to download $URL" >&2
	exit 1
fi

trap 'rm -f /tmp/ffwebp' EXIT

chmod +x /tmp/ffwebp

echo "Installing to /usr/local/bin/ffwebp requires sudo"

if ! sudo install -m755 /tmp/ffwebp /usr/local/bin/ffwebp; then
	echo "Error: install failed" >&2
	exit 1
fi

echo "ffwebp $VERSION ($VARIANT) installed to /usr/local/bin/ffwebp"