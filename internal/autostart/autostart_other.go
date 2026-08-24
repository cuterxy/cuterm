//go:build !darwin && !linux && !windows

package autostart

import "errors"

var errUnsupported = errors.New("autostart is not supported on this platform")

// Supported reports whether login launch can be configured here.
func Supported() bool { return false }

// Enabled always reports false on unsupported platforms.
func Enabled() (bool, error) { return false, nil }

// Set always fails on unsupported platforms.
func Set(bool) error { return errUnsupported }
