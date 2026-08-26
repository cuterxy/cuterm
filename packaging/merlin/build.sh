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
  # "/." copies the template's contents on both BSD and GNU cp; a bare
  # trailing slash nests the template directory itself under GNU cp.
  cp -R "$TEMPLATE/." "$pkg/$MODULE/"
  cp "$STAGE/cuterm-hub" "$pkg/$MODULE/bin/cuterm-hub"
  # The software center plugin list prepends its own "v" to the version, so
  # the version file carries the bare number ("1.2.5", not "v1.2.5").
  echo "${VERSION#v}" > "$pkg/$MODULE/version"
  echo "$platform" > "$pkg/$MODULE/.valid"
  outdir="$OUT/$([ "$platform" = arm384 ] && echo arm || echo hnd)"
  mkdir -p "$outdir"
  # The software center (armsoft & rogsoft ks_tar_install.sh) extracts the
  # tarball into /tmp, locates the single install.sh (find -maxdepth 2), then
  # requires <module>/webs/Module_<module>.asp and <module>/scripts/ next to
  # it. So the tarball must carry the cutermhub/ directory, not its contents.
  tar -czf "$outdir/$MODULE.tar.gz" -C "$pkg" "$MODULE"
  # Guard the layout: GNU cp has already shipped a nested cutermhub/cutermhub/
  # here once, which put install.sh below the installer's find depth.
  # NOTE: read the listing into a variable first — piping tar straight into
  # grep -q is a SIGPIPE race that pipefail turns into a spurious failure.
  listing="$(tar -tzf "$outdir/$MODULE.tar.gz")"
  for want in "$MODULE/install.sh" "$MODULE/webs/Module_$MODULE.asp"; do
    grep -qx "$want" <<<"$listing" || {
      echo "FATAL: tarball missing $want" >&2
      exit 1
    }
  done
  echo "-> $outdir/$MODULE.tar.gz"
done

echo
ls -lh "$OUT"/*/*.tar.gz
