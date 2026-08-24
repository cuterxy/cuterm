#!/bin/sh
# Build a macOS .pkg installer for an already-built cuterm binary in dist/.
# Usage: packaging/build-pkg.sh <tag> <arch>   e.g. packaging/build-pkg.sh v0.2.4 arm64
# Output: dist/cuterm-<tag>-darwin-<arch>.pkg, which installs:
#   /Applications/cuterm.app  (app bundle with icon, double-click starts the daemon)
#   /usr/local/bin/cuterm     (CLI, same binary)
set -eu

tag="${1:?usage: build-pkg.sh <tag> <arch>}"
arch="${2:?usage: build-pkg.sh <tag> <arch>}"
version="${tag#v}"

bin="dist/cuterm-${tag}-darwin-${arch}"
[ -f "$bin" ] || { echo "error: $bin not found (run build.sh first)" >&2; exit 1; }

repo="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

app="$work/stage/Applications/cuterm.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" "$work/stage/usr/local/bin"
install -m 755 "$bin" "$app/Contents/MacOS/cuterm"
install -m 755 "$bin" "$work/stage/usr/local/bin/cuterm"

# App icon: render every required size from the 1024px source, build .icns.
iconset="$work/cuterm.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$repo/assets/icon-1024.png" --out "$iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$repo/assets/icon-1024.png" --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/cuterm.icns"

cat > "$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>cuterm</string>
	<key>CFBundleDisplayName</key>
	<string>cuterm</string>
	<key>CFBundleIdentifier</key>
	<string>com.cuterxy.cuterm</string>
	<key>CFBundleVersion</key>
	<string>${version}</string>
	<key>CFBundleShortVersionString</key>
	<string>${version}</string>
	<key>CFBundleExecutable</key>
	<string>cuterm</string>
	<key>CFBundleIconFile</key>
	<string>cuterm</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
EOF

# Ad-hoc sign the bundle so LaunchServices treats it consistently.
codesign --sign - --force --deep "$app"

# NOTE: on macOS 13+ a build process that itself carries the (undeletable)
# com.apple.provenance xattr taints every file it creates, and pkgbuild then
# archives that metadata as AppleDouble (._*) files in the payload. That does
# not happen on clean CI runners; either way the bundled postinstall script
# removes the junk from the installed system.
scripts="$(cd "$(dirname "$0")" && pwd)/macos-scripts"

out="dist/cuterm-${tag}-darwin-${arch}.pkg"
pkgbuild \
  --root "$work/stage" \
  --scripts "$scripts" \
  --identifier com.cuterxy.cuterm \
  --version "$version" \
  --install-location / \
  "$out"

echo "-> $out"
