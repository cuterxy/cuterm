//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// desktopPath returns the XDG autostart entry location
// ($XDG_CONFIG_HOME/autostart/cuterm.desktop, default ~/.config).
func desktopPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "autostart", "cuterm.desktop"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "cuterm.desktop"), nil
}

// Supported reports whether login launch can be configured here.
func Supported() bool { return true }

// Enabled reports whether the XDG autostart entry exists.
func Enabled() (bool, error) {
	p, err := desktopPath()
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

// Set writes or removes the XDG autostart .desktop entry pointing at the
// current executable.
func Set(enable bool) error {
	p, err := desktopPath()
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
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=cuterm
Comment=Shared terminal server with web UI
Exec=%s
Icon=cuterm
Terminal=false
X-GNOME-Autostart-enabled=true
`, exe)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(entry), 0o644)
}
