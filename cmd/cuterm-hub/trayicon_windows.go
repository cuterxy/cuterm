//go:build windows

package main

import _ "embed"

// The Windows systray loads the icon with LoadImage(IMAGE_ICON), which only
// accepts .ico files; feeding it the PNG used on Unix leaves the tray icon
// blank.
//
//go:embed assets/icon-tray.ico
var trayIcon []byte
