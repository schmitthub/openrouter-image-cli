#!/bin/sh
# Install orimage from GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/schmitthub/openrouter-image-cli/main/scripts/install.sh | sh
#
# Environment:
#   ORIMAGE_VERSION      release tag to install (default: latest, e.g. v2026.8.3)
#   ORIMAGE_INSTALL_DIR  target directory (default: /usr/local/bin, falls back
#                        to ~/.local/bin when /usr/local/bin is not writable
#                        and sudo is unavailable)
set -eu

REPO="schmitthub/openrouter-image-cli"
BINARY="orimage"

err() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

need curl
need tar

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
    linux | darwin) ;;
    *) err "unsupported OS: $os (grab a windows zip from https://github.com/$REPO/releases)" ;;
esac

arch=$(uname -m)
case "$arch" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) err "unsupported architecture: $arch" ;;
esac

version="${ORIMAGE_VERSION:-}"
if [ -z "$version" ]; then
    version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
        grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    [ -n "$version" ] || err "could not determine the latest release tag"
fi

archive="${BINARY}_${version#v}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$version"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

printf 'Downloading %s %s (%s/%s)...\n' "$BINARY" "$version" "$os" "$arch"
curl -fsSL -o "$tmpdir/$archive" "$base_url/$archive" ||
    err "download failed: $base_url/$archive"
curl -fsSL -o "$tmpdir/checksums.txt" "$base_url/checksums.txt" ||
    err "download failed: $base_url/checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
    sha=$(sha256sum "$tmpdir/$archive" | cut -d' ' -f1)
else
    need shasum
    sha=$(shasum -a 256 "$tmpdir/$archive" | cut -d' ' -f1)
fi
grep -q "^$sha  $archive\$" "$tmpdir/checksums.txt" ||
    err "checksum verification failed for $archive"

tar -xzf "$tmpdir/$archive" -C "$tmpdir" "$BINARY"

install_dir="${ORIMAGE_INSTALL_DIR:-/usr/local/bin}"
if [ -w "$install_dir" ]; then
    install -m 0755 "$tmpdir/$BINARY" "$install_dir/$BINARY"
elif [ -z "${ORIMAGE_INSTALL_DIR:-}" ] && command -v sudo >/dev/null 2>&1; then
    printf 'Installing to %s (sudo)...\n' "$install_dir"
    sudo install -m 0755 "$tmpdir/$BINARY" "$install_dir/$BINARY"
elif [ -z "${ORIMAGE_INSTALL_DIR:-}" ]; then
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
    install -m 0755 "$tmpdir/$BINARY" "$install_dir/$BINARY"
else
    err "cannot write to $install_dir"
fi

printf 'Installed %s %s to %s/%s\n' "$BINARY" "$version" "$install_dir" "$BINARY"
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) printf 'note: %s is not on your PATH\n' "$install_dir" ;;
esac
