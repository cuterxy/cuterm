//go:build headless

package main

import "sync"

// Headless builds (routers, servers without a desktop) ship no system tray:
// runTray just blocks until the signal handler calls requestQuit. The systray
// dependency (and its CGO requirement) is compiled out in this mode.

var (
	trayQuitCh   = make(chan struct{})
	trayQuitOnce sync.Once
)

// traySetLanguage is a no-op in headless builds (no tray menu to translate).
func traySetLanguage(string) {}

// requestQuit unblocks runTray; called from the SIGINT / SIGTERM handler in
// main.
func requestQuit() { trayQuitOnce.Do(func() { close(trayQuitCh) }) }

// runTray blocks until requestQuit is called, then runs onQuit and returns,
// letting main return and the process exit.
func runTray(_, _ func() string, _ string, onQuit func()) {
	<-trayQuitCh
	if onQuit != nil {
		onQuit()
	}
}
