#!/bin/bash
# Build cuterm and cuterm-hub release binaries for the current platform into
# dist/. The system tray requires CGO, so cross-compilation is not supported;
# run this script on each target OS instead.
# Usage: ./build.sh [version]   (default version: dev)
set -euo pipefail

cd "$(dirname "$0")"

VERSION="${1:-dev}"
OUT=dist

goos="$(go env GOOS)"
goarch="$(go env GOARCH)"

ldflags="-s -w -X main.version=$VERSION"
if [ "$goos" = windows ]; then
  ldflags="-H windowsgui $ldflags"
fi

rm -rf "$OUT"
mkdir -p "$OUT"

if [ "$goos" = windows ]; then
  # Embed the tray icon as each .exe's application icon (shown in Explorer,
  # the taskbar, Start Menu shortcuts and "Apps & Features").
  go run github.com/akavel/rsrc@latest -ico assets/icon-tray.ico -o rsrc.syso
  go run github.com/akavel/rsrc@latest -ico cmd/cuterm-hub/assets/icon-tray.ico -o cmd/cuterm-hub/rsrc.syso
  trap 'rm -f rsrc.syso cmd/cuterm-hub/rsrc.syso' EXIT
fi

# build <name> <package>: native build with CGO (the tray needs it).
build() {
  local name="$1" pkg="$2"
  local bin="$OUT/${name}-${VERSION}-${goos}-${goarch}"
  if [ "$goos" = windows ]; then
    bin="$bin.exe"
  fi
  echo "-> $goos/$goarch $name (native, CGO enabled)"
  CGO_ENABLED=1 go build -trimpath -ldflags "$ldflags" -o "$bin" "$pkg"
  # Older Go toolchains (e.g. 1.22) with a newer Xcode linker produce binaries
  # that macOS kills on launch; re-signing ad-hoc fixes that.
  if [ "$goos" = darwin ]; then
    codesign --sign - --force "$bin"
  fi
}

build cuterm .
build cuterm-hub ./cmd/cuterm-hub

echo
ls -lh "$OUT"
