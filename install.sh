#!/bin/sh
# Install cuterm on macOS / Linux.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cuterxy/cuterm/main/install.sh | sh
#   sh install.sh v1.0.0          # install a specific version
set -eu

REPO="cuterxy/cuterm"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin|linux) ;;
  *) echo "error: unsupported OS: $os (use install.ps1 on Windows)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

version="${1:-}"
if [ -z "$version" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$version" ] || { echo "error: failed to resolve latest release version" >&2; exit 1; }
fi

name="cuterm-${version}-${os}-${arch}"
url="https://github.com/$REPO/releases/download/${version}/${name}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "-> downloading $url"
curl -fsSL "$url" -o "$tmp/cuterm.tar.gz"
tar xzf "$tmp/cuterm.tar.gz" -C "$tmp"

if [ -w /usr/local/bin ]; then
  dest=/usr/local/bin
else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
fi

mv "$tmp/$name" "$dest/cuterm"
chmod +x "$dest/cuterm"

echo "-> installed: $dest/cuterm ($("$dest/cuterm" -version))"

case ":$PATH:" in
  *":$dest:"*) ;;
  *) echo "note: $dest is not in your PATH" ;;
esac

if [ "$os" = linux ]; then
  echo "note: the system tray needs libayatana-appindicator3 at runtime"
  echo "      Debian/Ubuntu: sudo apt install libayatana-appindicator3-1"
fi

echo "run 'cuterm' to start, then open http://localhost:7681"
