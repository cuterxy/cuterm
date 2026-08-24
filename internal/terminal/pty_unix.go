//go:build !windows

package terminal

import (
	"errors"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// unixPTY wraps a creack/pty file and its process.
type unixPTY struct {
	f   *os.File
	cmd *exec.Cmd
}

// DefaultShell returns the shell used when none is configured: $SHELL,
// falling back to /bin/bash then /bin/sh. "" when nothing is found.
func DefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	for _, s := range []string{"/bin/bash", "/bin/sh"} {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	return ""
}

// AvailableShells returns the shells offered on the config page, in
// preference order.
func AvailableShells() []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range []string{os.Getenv("SHELL"), "/bin/zsh", "/bin/bash", "/bin/sh"} {
		if s == "" || seen[s] {
			continue
		}
		if fi, err := os.Stat(s); err == nil && !fi.IsDir() {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// spawn starts the given shell (or DefaultShell() when empty) attached to a
// new pseudo-terminal.
func spawn(shell string, cols, rows uint16) (ptyHandle, string, error) {
	if shell == "" {
		shell = DefaultShell()
	}
	if shell == "" {
		return nil, "", errors.New("no shell found")
	}

	cmd := exec.Command(shell)
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, "", err
	}
	return &unixPTY{f: f, cmd: cmd}, shell, nil
}

func (p *unixPTY) Read(b []byte) (int, error)  { return p.f.Read(b) }
func (p *unixPTY) Write(b []byte) (int, error) { return p.f.Write(b) }

func (p *unixPTY) Resize(cols, rows uint16) error {
	return pty.Setsize(p.f, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *unixPTY) Kill() error {
	_ = p.f.Close()
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

func (p *unixPTY) Wait() error {
	err := p.cmd.Wait()
	_ = p.f.Close()
	return err
}
