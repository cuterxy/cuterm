// Package server exposes the terminal manager over HTTP (REST + WebSocket)
// and serves the embedded web UI.
package server

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/cuterxy/cuterm/internal/autostart"
	"github.com/cuterxy/cuterm/internal/terminal"
)

// Server wires the manager to HTTP handlers.
type Server struct {
	mgr      *terminal.Manager
	mux      *http.ServeMux
	upgrader websocket.Upgrader

	// OnPortChange is called by POST /api/port to re-listen on a new port.
	// It is set by main, which owns the listeners.
	OnPortChange func(port int) error

	// OnShellChange is called by POST /api/shell to set the shell for new
	// terminals. When nil, the manager is updated directly.
	OnShellChange func(shell string) error

	// OnAppearanceChange is called by POST /api/appearance after the in-memory
	// settings are updated, so the caller can persist them.
	OnAppearanceChange func(a Appearance) error

	// OnLanguageChange is called by POST /api/language after the in-memory
	// value is updated, so the caller can persist it and switch the tray
	// menu language at runtime.
	OnLanguageChange func(lang string) error

	// OnHubChange is called by POST /api/hub with the validated hub address
	// ("" means disconnect), so the caller can persist it and start or stop
	// the reverse-tunnel client at runtime.
	OnHubChange func(addr string) error

	// HubStatus backs GET /api/hub with the configured hub address and
	// whether the tunnel is currently connected. Nil reports empty/offline.
	HubStatus func() (addr string, connected bool)

	appearanceMu sync.RWMutex
	appearance   Appearance

	languageMu sync.RWMutex
	language   string

	// version is the build version reported by GET /api/version.
	version string
}

// Appearance holds the terminal display settings shared by all web clients.
// Empty fields mean the client-side defaults apply.
type Appearance struct {
	FontFamily string `json:"fontFamily,omitempty"`
	FontSize   int    `json:"fontSize,omitempty"`
	Theme      string `json:"theme,omitempty"`
	Scrollback int    `json:"scrollback,omitempty"`
}

// SetAppearance sets the initial display settings served by GET
// /api/appearance, e.g. values loaded from a persisted config at startup.
func (s *Server) SetAppearance(a Appearance) {
	s.appearanceMu.Lock()
	s.appearance = a
	s.appearanceMu.Unlock()
}

// SetLanguage sets the initial UI language served by GET /api/language,
// e.g. the value loaded from a persisted config at startup. "" means the
// clients follow the browser / system language.
func (s *Server) SetLanguage(lang string) {
	s.languageMu.Lock()
	s.language = lang
	s.languageMu.Unlock()
}

// SetVersion sets the build version reported by GET /api/version.
func (s *Server) SetVersion(v string) { s.version = v }

// New builds a Server serving the web UI from staticFS (the embedded web/
// directory) and terminals from mgr.
func New(staticFS fs.FS, mgr *terminal.Manager) *Server {
	s := &Server{
		mgr: mgr,
		mux: http.NewServeMux(),
		upgrader: websocket.Upgrader{
			// Connections come from browsers on the same origin; nothing
			// secret is at stake beyond shell access itself.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	s.mux.Handle("GET /api/terminals", http.HandlerFunc(s.handleList))
	s.mux.Handle("POST /api/terminals", http.HandlerFunc(s.handleCreate))
	s.mux.Handle("PATCH /api/terminals/{id}", http.HandlerFunc(s.handleRename))
	s.mux.Handle("DELETE /api/terminals/{id}", http.HandlerFunc(s.handleClose))
	s.mux.Handle("POST /api/port", http.HandlerFunc(s.handlePort))
	s.mux.Handle("GET /api/shells", http.HandlerFunc(s.handleShells))
	s.mux.Handle("POST /api/shell", http.HandlerFunc(s.handleSetShell))
	s.mux.Handle("GET /api/appearance", http.HandlerFunc(s.handleGetAppearance))
	s.mux.Handle("POST /api/appearance", http.HandlerFunc(s.handleSetAppearance))
	s.mux.Handle("GET /api/language", http.HandlerFunc(s.handleGetLanguage))
	s.mux.Handle("POST /api/language", http.HandlerFunc(s.handleSetLanguage))
	s.mux.Handle("GET /api/version", http.HandlerFunc(s.handleVersion))
	s.mux.Handle("GET /api/autostart", http.HandlerFunc(s.handleGetAutostart))
	s.mux.Handle("POST /api/autostart", http.HandlerFunc(s.handleSetAutostart))
	s.mux.Handle("GET /api/hub", http.HandlerFunc(s.handleGetHub))
	s.mux.Handle("POST /api/hub", http.HandlerFunc(s.handleSetHub))
	s.mux.Handle("GET /ws/terminals/{id}", http.HandlerFunc(s.handleWS))
	s.mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

type createRequest struct {
	Name string `json:"name"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req) // empty body is fine
	}
	t, err := s.mgr.Create(req.Name, req.Cols, req.Rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, t.Info())
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.Close(r.PathValue("id")); err != nil {
		if errors.Is(err, terminal.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type renameRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name must not be empty"))
		return
	}
	if err := s.mgr.Rename(r.PathValue("id"), req.Name); err != nil {
		if errors.Is(err, terminal.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type portRequest struct {
	Port int `json:"port"`
}

func (s *Server) handlePort(w http.ResponseWriter, r *http.Request) {
	var req portRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Port < 1 || req.Port > 65535 {
		writeError(w, http.StatusBadRequest, errors.New("port must be between 1 and 65535"))
		return
	}
	if s.OnPortChange == nil {
		writeError(w, http.StatusNotImplemented, errors.New("port change not supported"))
		return
	}
	if err := s.OnPortChange(req.Port); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type shellsResponse struct {
	Current   string   `json:"current"`
	Available []string `json:"available"`
}

func (s *Server) handleShells(w http.ResponseWriter, r *http.Request) {
	current := s.mgr.Shell()
	if current == "" {
		current = terminal.DefaultShell()
	}
	writeJSON(w, http.StatusOK, shellsResponse{
		Current:   current,
		Available: terminal.AvailableShells(),
	})
}

type shellRequest struct {
	Shell string `json:"shell"`
}

func (s *Server) handleSetShell(w http.ResponseWriter, r *http.Request) {
	var req shellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	set := s.OnShellChange
	if set == nil {
		set = s.mgr.SetShell
	}
	if err := set(req.Shell); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetAppearance(w http.ResponseWriter, r *http.Request) {
	s.appearanceMu.RLock()
	a := s.appearance
	s.appearanceMu.RUnlock()
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleSetAppearance(w http.ResponseWriter, r *http.Request) {
	var a Appearance
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.FontFamily = strings.TrimSpace(a.FontFamily)
	a.Theme = strings.TrimSpace(a.Theme)
	if a.FontSize < 6 || a.FontSize > 72 {
		writeError(w, http.StatusBadRequest, errors.New("fontSize must be between 6 and 72"))
		return
	}
	if a.FontFamily == "" || len(a.FontFamily) > 200 {
		writeError(w, http.StatusBadRequest, errors.New("fontFamily must be 1-200 characters"))
		return
	}
	if a.Theme == "" || len(a.Theme) > 100 {
		writeError(w, http.StatusBadRequest, errors.New("theme must be 1-100 characters"))
		return
	}
	if a.Scrollback < 0 || a.Scrollback > 100000 {
		writeError(w, http.StatusBadRequest, errors.New("scrollback must be between 0 and 100000"))
		return
	}

	s.appearanceMu.Lock()
	s.appearance = a
	s.appearanceMu.Unlock()

	if s.OnAppearanceChange != nil {
		if err := s.OnAppearanceChange(a); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetLanguage(w http.ResponseWriter, r *http.Request) {
	s.languageMu.RLock()
	lang := s.language
	s.languageMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]string{"language": lang})
}

type languageRequest struct {
	Language string `json:"language"`
}

func (s *Server) handleSetLanguage(w http.ResponseWriter, r *http.Request) {
	var req languageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	switch req.Language {
	case "", "en", "zh-CN":
	default:
		writeError(w, http.StatusBadRequest, errors.New(`language must be "", "en" or "zh-CN"`))
		return
	}

	s.languageMu.Lock()
	s.language = req.Language
	s.languageMu.Unlock()

	if s.OnLanguageChange != nil {
		if err := s.OnLanguageChange(req.Language); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// historyChunk caps how much scroll-back is sent per WebSocket message.
const historyChunk = 32 * 1024

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
}

func (s *Server) handleGetAutostart(w http.ResponseWriter, r *http.Request) {
	enabled, err := autostart.Enabled()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"supported": autostart.Supported(),
		"enabled":   enabled,
	})
}

type autostartRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSetAutostart(w http.ResponseWriter, r *http.Request) {
	if !autostart.Supported() {
		writeError(w, http.StatusNotImplemented, errors.New("autostart not supported on this platform"))
		return
	}
	var req autostartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := autostart.Set(req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/* ---------- cuterm-hub reverse connection ---------- */

func (s *Server) handleGetHub(w http.ResponseWriter, r *http.Request) {
	var addr string
	var connected bool
	if s.HubStatus != nil {
		addr, connected = s.HubStatus()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"addr":      addr,
		"connected": connected,
	})
}

type hubRequest struct {
	Addr string `json:"addr"`
}

// normalizeHubAddr validates user input like "https://host/", "host" or
// "host:7682/" and returns a bare "host:port". A missing port defaults to
// cuterm-hub's 7682. "" means disconnect and passes through.
func normalizeHubAddr(raw string) (string, error) {
	addr := strings.TrimSpace(raw)
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimRight(addr, "/")
	if addr == "" {
		return "", nil
	}
	if strings.ContainsAny(addr, "/?#") {
		return "", fmt.Errorf("invalid hub address: %s", raw)
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		if !strings.Contains(err.Error(), "missing port") {
			return "", fmt.Errorf("invalid hub address: %s", raw)
		}
		addr = net.JoinHostPort(addr, "7682")
	}
	return addr, nil
}

func (s *Server) handleSetHub(w http.ResponseWriter, r *http.Request) {
	var req hubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	addr, err := normalizeHubAddr(req.Addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if s.OnHubChange == nil {
		writeError(w, http.StatusNotImplemented, errors.New("hub connection not supported"))
		return
	}
	if err := s.OnHubChange(addr); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	t, ok := s.mgr.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, terminal.ErrNotFound)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	history, queue, alive := t.Subscribe()
	defer t.Unsubscribe(queue)

	// Writer: replay history, then forward live frames until the queue is
	// closed (terminal exited or client unsubscribed).
	go func() {
		defer conn.Close()
		for len(history) > 0 {
			n := min(len(history), historyChunk)
			if !writeFrame(conn, terminal.FrameOutput, history[:n]) {
				return
			}
			history = history[n:]
		}
		if !alive {
			writeFrame(conn, terminal.FrameClosed, []byte("process exited"))
			return
		}
		for frame := range queue {
			if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
				return
			}
		}
	}()

	// Reader: keystrokes and resizes until the client goes away.
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.BinaryMessage || len(data) == 0 {
			continue
		}
		switch data[0] {
		case terminal.FrameInput:
			t.WriteInput(data[1:])
		case terminal.FrameResize:
			if len(data) >= 5 {
				t.Resize(binary.BigEndian.Uint16(data[1:3]), binary.BigEndian.Uint16(data[3:5]))
			}
		}
	}
}

func writeFrame(conn *websocket.Conn, frameType byte, payload []byte) bool {
	frame := make([]byte, 1+len(payload))
	frame[0] = frameType
	copy(frame[1:], payload)
	return conn.WriteMessage(websocket.BinaryMessage, frame) == nil
}
