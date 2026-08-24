//go:build windows

package terminal

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/UserExistsError/conpty"
)

// winPTY wraps a Windows ConPTY session.
type winPTY struct {
	c *conpty.ConPty
}

// DefaultShell returns the shell used when none is configured: PowerShell,
// falling back to cmd.exe. "" when nothing is found.
func DefaultShell() string {
	shells := AvailableShells()
	if len(shells) == 0 {
		return ""
	}
	return shells[0]
}

// AvailableShells returns the shells offered on the config page, in
// preference order.
func AvailableShells() []string {
	var out []string
	for _, s := range []string{"powershell.exe", "cmd.exe"} {
		if _, err := exec.LookPath(s); err == nil {
			out = append(out, s)
		}
	}
	return out
}

// spawn starts the given shell (or DefaultShell() when empty) inside a
// ConPTY pseudo-console. ConPTY requires Windows 10 1809 or later.
func spawn(shell string, cols, rows uint16) (ptyHandle, string, error) {
	if !conpty.IsConPtyAvailable() {
		return nil, "", errors.New("ConPTY is not available on this Windows version (requires Windows 10 1809+)")
	}
	if shell == "" {
		shell = DefaultShell()
	}
	if shell == "" {
		return nil, "", errors.New("no shell found")
	}
	opts := []conpty.ConPtyOption{conpty.ConPtyDimensions(int(cols), int(rows))}
	if home, err := os.UserHomeDir(); err == nil {
		opts = append(opts, conpty.ConPtyWorkDir(home))
	}
	c, err := conpty.Start(shell, opts...)
	if err != nil {
		return nil, "", err
	}
	return &winPTY{c: c}, shell, nil
}

func (p *winPTY) Read(b []byte) (int, error)  { return p.c.Read(b) }
func (p *winPTY) Write(b []byte) (int, error) { return p.c.Write(b) }

func (p *winPTY) Resize(cols, rows uint16) error {
	return p.c.Resize(int(cols), int(rows))
}

func (p *winPTY) Kill() error {
	// Closing the pseudo console terminates the attached process.
	return p.c.Close()
}

func (p *winPTY) Wait() error {
	_, err := p.c.Wait(context.Background())
	return err
}
