// Package terminal manages interactive terminal sessions backed by
// pseudo-terminals. A Terminal fans its PTY output out to any number of
// attached subscribers and keeps a bounded scroll-back history so that
// late-joining clients can catch up.
package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"sync"
	"time"
)

// historyLimit is the maximum number of output bytes kept per terminal for
// replaying to newly attached clients.
const historyLimit = 128 * 1024

// subQueue is the per-subscriber buffered frame queue size. When full, frames
// are dropped for that subscriber (it can resync via history on reconnect).
const subQueue = 256

// WebSocket binary frame types. Every frame is one type byte followed by the
// payload.
const (
	FrameOutput byte = 0 // server -> client, raw PTY output bytes
	FrameInput  byte = 1 // client -> server, raw keystroke bytes
	FrameResize byte = 2 // client -> server, 4 bytes: cols, rows as uint16 BE
	FrameClosed byte = 3 // server -> client, text reason; terminal has exited
)

var ErrNotFound = errors.New("terminal not found")

// ptyHandle abstracts a running pseudo-terminal on all supported platforms.
type ptyHandle interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows uint16) error
	// Kill terminates the shell process.
	Kill() error
	// Wait blocks until the shell process exits and returns its exit error
	// (nil on clean exit).
	Wait() error
}

// Info is the JSON-serializable snapshot of a terminal.
type Info struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Shell     string    `json:"shell"`
	Cols      uint16    `json:"cols"`
	Rows      uint16    `json:"rows"`
	Alive     bool      `json:"alive"`
	Clients   int       `json:"clients"`
	CreatedAt time.Time `json:"createdAt"`
}

// Terminal is a single shared terminal session.
type Terminal struct {
	id        string
	name      string
	shell     string
	cols      uint16
	rows      uint16
	createdAt time.Time

	pty ptyHandle

	mu    sync.Mutex
	subs  map[chan []byte]struct{}
	hist  []byte
	alive bool
}

// Info returns a snapshot describing the terminal.
func (t *Terminal) Info() Info {
	t.mu.Lock()
	defer t.mu.Unlock()
	return Info{
		ID:        t.id,
		Name:      t.name,
		Shell:     t.shell,
		Cols:      t.cols,
		Rows:      t.rows,
		Alive:     t.alive,
		Clients:   len(t.subs),
		CreatedAt: t.createdAt,
	}
}

// SetName renames the terminal.
func (t *Terminal) SetName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.name = name
}

// run pumps PTY output to history and subscribers until the process exits.
func (t *Terminal) run() {
	buf := make([]byte, 32*1024)
	for {
		n, err := t.pty.Read(buf)
		if n > 0 {
			t.appendOutput(buf[:n])
		}
		if err != nil {
			break
		}
	}
	_ = t.pty.Wait()
	t.markExited()
}

// appendOutput records the chunk in history and broadcasts it.
func (t *Terminal) appendOutput(chunk []byte) {
	frame := make([]byte, 1+len(chunk))
	frame[0] = FrameOutput
	copy(frame[1:], chunk)

	t.mu.Lock()
	defer t.mu.Unlock()
	t.hist = append(t.hist, chunk...)
	if len(t.hist) > historyLimit {
		t.hist = t.hist[len(t.hist)-historyLimit:]
	}
	t.broadcastLocked(frame)
}

// markExited notifies all subscribers that the process ended and detaches
// them (closing their queues).
func (t *Terminal) markExited() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.alive = false
	frame := []byte{FrameClosed, 'p', 'r', 'o', 'c', 'e', 's', 's', ' ', 'e', 'x', 'i', 't', 'e', 'd'}
	t.broadcastLocked(frame)
	for ch := range t.subs {
		close(ch)
	}
	t.subs = make(map[chan []byte]struct{})
}

// broadcastLocked sends a frame to every subscriber without blocking; slow
// subscribers simply miss frames. Callers must hold t.mu.
func (t *Terminal) broadcastLocked(frame []byte) {
	for ch := range t.subs {
		select {
		case ch <- frame:
		default:
		}
	}
}

// Subscribe registers a new subscriber. It returns the current history, the
// subscriber's frame queue, and whether the terminal is still alive. Dead
// terminals return no live queue (nil) so the caller just replays history and
// reports the exit.
func (t *Terminal) Subscribe() (history []byte, queue chan []byte, alive bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	history = append([]byte(nil), t.hist...)
	if !t.alive {
		return history, nil, false
	}
	queue = make(chan []byte, subQueue)
	t.subs[queue] = struct{}{}
	return history, queue, true
}

// Unsubscribe removes and closes a subscriber queue previously returned by
// Subscribe. It is a no-op for nil queues.
func (t *Terminal) Unsubscribe(queue chan []byte) {
	if queue == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.subs[queue]; ok {
		delete(t.subs, queue)
		close(queue)
	}
}

// WriteInput forwards client keystrokes to the PTY. Input to a dead terminal
// is silently discarded.
func (t *Terminal) WriteInput(data []byte) {
	t.mu.Lock()
	alive := t.alive
	t.mu.Unlock()
	if !alive || len(data) == 0 {
		return
	}
	_, _ = t.pty.Write(data)
}

// Resize changes the PTY window size.
func (t *Terminal) Resize(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		return
	}
	t.mu.Lock()
	t.cols, t.rows = cols, rows
	alive := t.alive
	t.mu.Unlock()
	if alive {
		_ = t.pty.Resize(cols, rows)
	}
}

// Kill terminates the shell process. The read loop notices the exit and runs
// the normal shutdown path (notify subscribers, mark dead).
func (t *Terminal) Kill() {
	_ = t.pty.Kill()
}

// Manager owns the set of live terminals.
type Manager struct {
	mu    sync.RWMutex
	terms map[string]*Terminal
	seq   int
	shell string // shell for new terminals; empty means platform auto-detect
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{terms: make(map[string]*Terminal)}
}

// SetShell sets the shell used by newly created terminals. An empty shell
// restores platform auto-detection. Existing terminals are unaffected.
func (m *Manager) SetShell(shell string) error {
	if shell != "" {
		if _, err := exec.LookPath(shell); err != nil {
			return fmt.Errorf("shell not found or not executable: %s", shell)
		}
	}
	m.mu.Lock()
	m.shell = shell
	m.mu.Unlock()
	return nil
}

// Shell returns the configured shell, or "" when auto-detection is used.
func (m *Manager) Shell() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.shell
}

// Create spawns a new terminal running the configured shell (or the
// platform default when none is configured). An empty name gets a generated
// default. Zero cols/rows fall back to 80x24.
func (m *Manager) Create(name string, cols, rows uint16) (*Terminal, error) {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	m.mu.RLock()
	shell := m.shell
	m.mu.RUnlock()
	p, shell, err := spawn(shell, cols, rows)
	if err != nil {
		return nil, fmt.Errorf("spawn shell: %w", err)
	}

	m.mu.Lock()
	m.seq++
	if name == "" {
		name = fmt.Sprintf("terminal-%d", m.seq)
	}
	t := &Terminal{
		id:        newID(),
		name:      name,
		shell:     shell,
		cols:      cols,
		rows:      rows,
		createdAt: time.Now(),
		pty:       p,
		subs:      make(map[chan []byte]struct{}),
		alive:     true,
	}
	m.terms[t.id] = t
	m.mu.Unlock()

	go t.run()
	return t, nil
}

// List returns snapshots of all terminals, ordered by creation time
// (oldest first).
func (m *Manager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]Info, 0, len(m.terms))
	for _, t := range m.terms {
		infos = append(infos, t.Info())
	}
	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].CreatedAt.Before(infos[j].CreatedAt)
	})
	return infos
}

// Get looks a terminal up by ID.
func (m *Manager) Get(id string) (*Terminal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.terms[id]
	return t, ok
}

// Close kills the terminal's process and removes it from the manager.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	t, ok := m.terms[id]
	if ok {
		delete(m.terms, id)
	}
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	t.Kill()
	return nil
}

// Rename changes a terminal's display name.
func (m *Manager) Rename(id, name string) error {
	m.mu.RLock()
	t, ok := m.terms[id]
	m.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	t.SetName(name)
	return nil
}

// CloseAll kills every terminal (used on server shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	terms := make([]*Terminal, 0, len(m.terms))
	for id, t := range m.terms {
		terms = append(terms, t)
		delete(m.terms, id)
	}
	m.mu.Unlock()
	for _, t := range terms {
		t.Kill()
	}
}

func newID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
