#!/bin/sh
# openByte installer — detects OS/arch, downloads latest release from GitHub.
# Usage: curl -fsSL https://raw.githubusercontent.com/saveenergy/openbyte/main/scripts/install.sh | sh
set -e

REPO="saveenergy/openbyte"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="openbyte"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)
        echo "Error: unsupported OS: $OS"
        echo "Supported: linux, darwin (macOS)"
        exit 1
        ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)   ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH"
        echo "Supported: amd64, arm64"
        exit 1
        ;;
esac

echo "Detected: ${OS}/${ARCH}"

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release"
    exit 1
fi
echo "Latest version: ${LATEST}"

# Build download URL
VERSION="${LATEST#v}"
ARCHIVE="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
if [ "$OS" = "windows" ]; then
    ARCHIVE="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.zip"
fi
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ARCHIVE}"

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

echo "Extracting..."
if echo "$ARCHIVE" | grep -q '\.zip$'; then
    unzip -q "${TMPDIR}/${ARCHIVE}" -d "$TMPDIR"
else
    tar xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
fi

# Install
if [ -w "$INSTALL_DIR" ]; then
    cp "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo cp "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo ""
echo "openbyte ${LATEST} installed to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "Quick start:"
echo "  openbyte server                          # Start server"
echo "  openbyte check --json <server-url>       # Quick connectivity check"
echo "  openbyte client -p http -d download      # Full speed test"
echo "  openbyte mcp                             # MCP server for AI agents"
