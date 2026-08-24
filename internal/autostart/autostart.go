// Package autostart registers cuterm to launch when the user logs in.
// The mechanism is platform-specific: a LaunchAgent plist on macOS, an XDG
// autostart entry on Linux, and the Run registry key on Windows.
package autostart

import "os"

// exePath returns the absolute path of the running executable, which is what
// the login entry points at.
func exePath() (string, error) {
	return os.Executable()
}
