//go:build !windows

package main

import _ "embed"

//go:embed assets/icon-tray.png
var trayIcon []byte
