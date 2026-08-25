//go:build windows

package autostart

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Supported reports whether login launch can be configured here.
func Supported() bool { return true }

// Enabled reports whether the HKCU Run entry for cuterm exists.
func Enabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(appName); err == nil {
		return true, nil
	} else if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

// Set writes or removes the HKCU Run entry pointing at the current
// executable.
func Set(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if enable {
		exe, err := exePath()
		if err != nil {
			return err
		}
		return k.SetStringValue(appName, `"`+exe+`"`)
	}
	if err := k.DeleteValue(appName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
