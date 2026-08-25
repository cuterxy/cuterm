//go:build !windows

package main

import (
	"os"
	"strings"
)

// sysUILang returns "zh-CN" when the system locale is Chinese, else "en".
// On Unix the daemon inherits the launching user's locale environment.
func sysUILang() string {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return langFromLocale(v)
		}
	}
	return "en"
}

// langFromLocale maps a locale string like "zh_CN.UTF-8" to a UI language.
func langFromLocale(locale string) string {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return "zh-CN"
	}
	return "en"
}
