// Package autostart registers cuterm to launch when the user logs in.
// The mechanism is platform-specific: a LaunchAgent plist on macOS, an XDG
// autostart entry on Linux, and the Run registry key on Windows.
package autostart

import "os"

// appName identifies the app in login entries (LaunchAgent label, desktop
// file name, registry value). It defaults to cuterm; sibling apps such as
// cuterm-hub override it once at startup via SetAppName.
var appName = "cuterm"

// SetAppName overrides the identity used for the login entry. It must be
// called before any other autostart function.
func SetAppName(name string) { appName = name }

// exePath returns the absolute path of the running executable, which is what
// the login entry points at.
func exePath() (string, error) {
	return os.Executable()
}
