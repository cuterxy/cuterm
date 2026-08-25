//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// label returns the LaunchAgent label, derived from the app name
// (com.cuterxy.cuterm, com.cuterxy.cuterm-hub, ...).
func label() string { return "com.cuterxy." + appName }

// plistPath returns the per-user LaunchAgent location
// (~/Library/LaunchAgents/<label>.plist).
func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label()+".plist"), nil
}

// Supported reports whether login launch can be configured here.
func Supported() bool { return true }

// Enabled reports whether the LaunchAgent plist exists.
func Enabled() (bool, error) {
	p, err := plistPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(p); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// Set writes or removes the LaunchAgent plist. The agent runs the current
// executable once at login (it daemonizes itself; no KeepAlive).
func Set(enable bool) error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	if !enable {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := exePath()
	if err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, label(), exe)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(plist), 0o644)
}
