#!/bin/sh
# Build a macOS .pkg installer for an already-built binary in dist/.
# Usage: packaging/build-pkg.sh <tag> <arch> [app]   app: cuterm (default) | cuterm-hub
#   e.g. packaging/build-pkg.sh v0.2.4 arm64 cuterm-hub
# Output: dist/<app>-<tag>-darwin-<arch>.pkg, which installs:
#   /Applications/<app>.app  (app bundle with icon, double-click starts the daemon)
#   /usr/local/bin/<app>     (CLI, same binary)
set -eu

tag="${1:?usage: build-pkg.sh <tag> <arch> [app]}"
arch="${2:?usage: build-pkg.sh <tag> <arch> [app]}"
app="${3:-cuterm}"
version="${tag#v}"

repo="$(cd "$(dirname "$0")/.." && pwd)"

case "$app" in
  cuterm)      icon="$repo/assets/icon-1024.png" ;;
  cuterm-hub)  icon="$repo/cmd/cuterm-hub/assets/icon-1024.png" ;;
  *) echo "error: unknown app: $app" >&2; exit 1 ;;
esac

bin="dist/${app}-${tag}-darwin-${arch}"
[ -f "$bin" ] || { echo "error: $bin not found (run build.sh first)" >&2; exit 1; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

bundle="$work/stage/Applications/${app}.app"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources" "$work/stage/usr/local/bin"
install -m 755 "$bin" "$bundle/Contents/MacOS/$app"
install -m 755 "$bin" "$work/stage/usr/local/bin/$app"

# App icon: render every required size from the 1024px source, build .icns.
iconset="$work/$app.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$icon" --out "$iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$icon" --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$bundle/Contents/Resources/$app.icns"

cat > "$bundle/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>${app}</string>
	<key>CFBundleDisplayName</key>
	<string>${app}</string>
	<key>CFBundleIdentifier</key>
	<string>com.cuterxy.${app}</string>
	<key>CFBundleVersion</key>
	<string>${version}</string>
	<key>CFBundleShortVersionString</key>
	<string>${version}</string>
	<key>CFBundleExecutable</key>
	<string>${app}</string>
	<key>CFBundleIconFile</key>
	<string>${app}</string>
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
codesign --sign - --force --deep "$bundle"

# NOTE: on macOS 13+ a build process that itself carries the (undeletable)
# com.apple.provenance xattr taints every file it creates, and pkgbuild then
# archives that metadata as AppleDouble (._*) files in the payload. That does
# not happen on clean CI runners; either way the generated postinstall script
# removes the junk from the installed system.
mkdir -p "$work/scripts"
cat > "$work/scripts/postinstall" <<EOF
#!/bin/sh
# postinstall for the ${app} .pkg — runs as root after the payload is laid down.
#
# When the pkg is built on a Mac whose processes carry the (undeletable)
# com.apple.provenance xattr, pkgbuild archives that metadata as AppleDouble
# (._*) files in the payload. Remove them if present; a no-op otherwise.
rm -f /usr/._local /usr/local/._bin /usr/local/bin/._${app} /Applications/._${app}.app
if [ -d /Applications/${app}.app ]; then
  find /Applications/${app}.app -name '._*' -delete
fi
exit 0
EOF
chmod +x "$work/scripts/postinstall"

out="dist/${app}-${tag}-darwin-${arch}.pkg"
pkgbuild \
  --root "$work/stage" \
  --scripts "$work/scripts" \
  --identifier "com.cuterxy.${app}" \
  --version "$version" \
  --install-location / \
  "$out"

echo "-> $out"
