//go:build windows

package main

import "syscall"

// sysUILang returns "zh-CN" when the Windows display language is Chinese,
// else "en".
func sysUILang() string {
	// GetUserDefaultUILanguage returns a LANGID; its low 10 bits are the
	// primary language identifier (LANG_CHINESE = 0x04).
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	langID, _, _ := proc.Call()
	if uint16(langID)&0x3ff == 0x04 {
		return "zh-CN"
	}
	return "en"
}
