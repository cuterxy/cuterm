package main

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

// trayIcon holds the tray icon image, embedded per platform:
// icon-tray.png on Unix, icon-tray.ico on Windows (see trayicon_*.go).

// trayText holds the tray menu strings in one UI language.
type trayText struct {
	openApp, openAppTip       string
	openConfig, openConfigTip string
	quit, quitTip             string
}

// trayStringsFor returns the menu strings for lang ("en" / "zh-CN"); an
// empty lang falls back to the system UI language.
func trayStringsFor(lang string) trayText {
	if lang == "" {
		lang = sysUILang()
	}
	if lang == "zh-CN" {
		return trayText{
			openApp: "打开应用页面", openAppTip: "在浏览器中打开 cuterm-hub 应用页面",
			openConfig: "打开配置页面", openConfigTip: "在浏览器中打开 cuterm-hub 配置页面",
			quit: "退出", quitTip: "停止 cuterm-hub 服务并退出",
		}
	}
	return trayText{
		openApp: "Open App", openAppTip: "Open the cuterm-hub app page in your browser",
		openConfig: "Open Settings", openConfigTip: "Open the cuterm-hub settings page in your browser",
		quit: "Quit", quitTip: "Stop the cuterm-hub service and quit",
	}
}

// trayLangCh carries language switch requests to the tray event loop.
var trayLangCh = make(chan string, 1)

// traySetLanguage switches the tray menu language at runtime. It is a no-op
// when the tray is not ready yet or a switch is already pending.
func traySetLanguage(lang string) {
	select {
	case trayLangCh <- lang:
	default:
	}
}

// runTray shows the system tray icon and blocks on the main goroutine until
// the user quits from the tray menu. appURL and configURL return the current
// page addresses (the port can change at runtime); lang is the initial menu
// language ("" follows the system); onQuit is invoked before the process
// exits.
func runTray(appURL, configURL func() string, lang string, onQuit func()) {
	systray.Run(func() {
		// The cuterm-hub logo: dark rounded square with a blue CU lettermark.
		systray.SetIcon(trayIcon)
		systray.SetTitle("")
		systray.SetTooltip("cuterm-hub")

		txt := trayStringsFor(lang)
		mApp := systray.AddMenuItem(txt.openApp, txt.openAppTip)
		mConfig := systray.AddMenuItem(txt.openConfig, txt.openConfigTip)
		mQuit := systray.AddMenuItem(txt.quit, txt.quitTip)

		applyLang := func(lang string) {
			txt := trayStringsFor(lang)
			mApp.SetTitle(txt.openApp)
			mApp.SetTooltip(txt.openAppTip)
			mConfig.SetTitle(txt.openConfig)
			mConfig.SetTooltip(txt.openConfigTip)
			mQuit.SetTitle(txt.quit)
			mQuit.SetTooltip(txt.quitTip)
		}

		go func() {
			for {
				select {
				case <-mApp.ClickedCh:
					openBrowser(appURL())
				case <-mConfig.ClickedCh:
					openBrowser(configURL())
				case lang := <-trayLangCh:
					applyLang(lang)
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		if onQuit != nil {
			onQuit()
		}
		os.Exit(0)
	})
}

// openBrowser opens url in the system default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open browser: %v", err)
	}
}
