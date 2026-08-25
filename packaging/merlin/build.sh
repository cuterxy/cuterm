#!/bin/bash
# Build cuterm-hub offline plugin packages for the koolshare software center
# (Asuswrt-Merlin based router firmwares). Produces two packages:
#   dist/merlin/arm/cutermhub.tar.gz  - 梅林改 384/386 (armv7l, kernel 2.6)
#   dist/merlin/hnd/cutermhub.tar.gz  - 梅林改/官改 hnd/axhnd/axhnd.675x (kernel 4.1+)
# Both carry the same statically linked 32-bit ARM binary, which runs on every
# supported model (armv8 models run 32-bit userspace); they differ only in the
# .valid marker each software center generation checks for.
# Usage: packaging/merlin/build.sh [version]   (default version: dev)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION="${1:-dev}"
MODULE=cutermhub
TEMPLATE="$ROOT/packaging/merlin/$MODULE"
OUT="$ROOT/dist/merlin"

cd "$ROOT"

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

# Static (CGO off), no system tray: runs on the router's busybox userland.
echo "-> linux/arm (GOARM=7) cuterm-hub $VERSION (headless, static)"
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -tags headless -trimpath \
  -ldflags "-s -w -X main.version=$VERSION" -o "$STAGE/cuterm-hub" ./cmd/cuterm-hub

# valid marker expected by each software center generation.
for platform in arm384 hnd; do
  pkg="$STAGE/$platform"
  mkdir -p "$pkg/$MODULE/bin"
  cp -R "$TEMPLATE/" "$pkg/$MODULE/"
  cp "$STAGE/cuterm-hub" "$pkg/$MODULE/bin/cuterm-hub"
  echo "$VERSION" > "$pkg/$MODULE/version"
  echo "$platform" > "$pkg/$MODULE/.valid"
  outdir="$OUT/$([ "$platform" = arm384 ] && echo arm || echo hnd)"
  mkdir -p "$outdir"
  tar -czf "$outdir/$MODULE.tar.gz" -C "$pkg" "$MODULE"
  echo "-> $outdir/$MODULE.tar.gz"
done

echo
ls -lh "$OUT"/*/*.tar.gz
