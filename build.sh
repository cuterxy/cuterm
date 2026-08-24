#!/bin/bash
# Build a cuterm release binary for the current platform into dist/.
# The system tray requires CGO, so cross-compilation is not supported;
# run this script on each target OS instead.
# Usage: ./build.sh [version]   (default version: dev)
set -euo pipefail

cd "$(dirname "$0")"

VERSION="${1:-dev}"
OUT=dist

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"

name="cuterm-${VERSION}-${goos}-${goarch}"
bin="$OUT/$name"
ldflags="-s -w -X main.version=$VERSION"
if [ "$goos" = windows ]; then
  bin="$bin.exe"
  ldflags="-H windowsgui $ldflags"
fi

rm -rf "$OUT"
mkdir -p "$OUT"

if [ "$goos" = windows ]; then
  # Embed the tray icon as the .exe's application icon (shown in Explorer,
  # the taskbar, Start Menu shortcuts and "Apps & Features").
  go run github.com/akavel/rsrc@latest -ico assets/icon-tray.ico -o rsrc.syso
  trap 'rm -f rsrc.syso' EXIT
fi

echo "-> $goos/$goarch (native, CGO enabled)"
CGO_ENABLED=1 go build -trimpath -ldflags "$ldflags" -o "$bin" .

# Older Go toolchains (e.g. 1.22) with a newer Xcode linker produce binaries
# that macOS kills on launch; re-signing ad-hoc fixes that.
if [ "$goos" = darwin ]; then
  codesign --sign - --force "$bin"
fi

echo
echo "binary: $bin"
ls -lh "$bin"
