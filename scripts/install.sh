#!/bin/sh
# openByte installer — detects OS/arch, downloads latest release from GitHub.
# Usage: curl -fsSL https://raw.githubusercontent.com/saveenergy/openbyte/main/scripts/install.sh | sh
set -e

REPO="saveenergy/openbyte"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="openbyte"

if [ -z "${INSTALL_DIR}" ]; then
    echo "Error: INSTALL_DIR must be non-empty"
    exit 1
fi

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
release_json="$(curl -fsSL -H "Accept: application/vnd.github+json" "https://api.github.com/repos/${REPO}/releases/latest")"
if command -v jq >/dev/null 2>&1; then
    LATEST="$(printf '%s' "$release_json" | jq -r '.tag_name // empty')"
else
    LATEST="$(printf '%s' "$release_json" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | cut -d '"' -f4)"
fi
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release"
    exit 1
fi
if ! echo "$LATEST" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$'; then
    echo "Error: unexpected release tag format: ${LATEST}"
    exit 1
fi
echo "Latest version: ${LATEST}"

# Build download URL
VERSION="${LATEST#v}"
ARCHIVE="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${LATEST}/checksums.txt"

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL --connect-timeout 10 --max-time 60 "$URL" -o "${TMPDIR}/${ARCHIVE}"
echo "Downloading checksums..."
curl -fsSL --connect-timeout 10 --max-time 60 "$CHECKSUMS_URL" -o "${TMPDIR}/checksums.txt"

if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    echo "Error: sha256sum or shasum required for checksum verification"
    exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL_SUM="$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print tolower($1)}')"
else
    ACTUAL_SUM="$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print tolower($1)}')"
fi
EXPECTED_SUM="$(
    awk -v f="$ARCHIVE" '
        NF >= 2 {
            sum = tolower($1)
            $1 = ""
            sub(/^[[:space:]]+/, "", $0)
            if ($0 == f && sum ~ /^[0-9a-f]{64}$/) {
                print sum
                found = 1
                exit
            }
        }
        END {
            if (!found) exit 1
        }
    ' "${TMPDIR}/checksums.txt" 2>/dev/null || true
)"
if [ -z "$EXPECTED_SUM" ]; then
    echo "Error: checksum entry missing for ${ARCHIVE}"
    exit 1
fi
if [ "$ACTUAL_SUM" != "$EXPECTED_SUM" ]; then
    echo "Error: checksum mismatch for ${ARCHIVE}"
    exit 1
fi

echo "Extracting..."
tar xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

# Install
if [ ! -d "$INSTALL_DIR" ]; then
    if [ -w "$(dirname "$INSTALL_DIR")" ]; then
        mkdir -p "$INSTALL_DIR"
    else
        echo "Creating ${INSTALL_DIR} (requires sudo)..."
        sudo mkdir -p "$INSTALL_DIR"
    fi
fi

if [ ! -f "${TMPDIR}/${BINARY_NAME}" ]; then
    echo "Error: extracted archive did not contain ${BINARY_NAME}"
    exit 1
fi

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
echo "  openbyte check --json https://speed.example.com  # Quick connectivity check"
echo "  openbyte client -p http -d download      # Full speed test"
echo "  openbyte mcp                             # MCP server for AI agents"
