//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// daemonEnv marks the detached child process so it does not daemonize again.
const daemonEnv = "CUTERM_DAEMON"

// daemonize re-launches the current executable as a detached background
// process and exits the parent. When the current process already is the
// daemon child (daemonEnv is set), it just returns the log file path.
func daemonize() string {
	logPath := logFilePath()
	if os.Getenv(daemonEnv) == "1" {
		return logPath
	}

	exe, err := os.Executable()
	if err != nil {
		fatal("locate executable: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fatal("create log dir: %v", err)
	}
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fatal("open log file: %v", err)
	}
	defer logf.Close()

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonEnv+"=1")
	cmd.Stdin = nil
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fatal("start background process: %v", err)
	}
	fmt.Printf("cuterm is running in the background (pid %d)\n", cmd.Process.Pid)
	fmt.Printf("log file: %s\n", logPath)
	os.Exit(0)
	return "" // unreachable
}

// logFilePath returns the daemon log location (~/.cuterm/cuterm.log),
// falling back to the system temp dir when the home dir is unknown.
func logFilePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cuterm", "cuterm.log")
	}
	return filepath.Join(os.TempDir(), "cuterm.log")
}
