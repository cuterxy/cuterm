// Package hub exposes a fleet of cuterm servers ("nodes") over a single HTTP
// API: node management, transparent REST/WebSocket proxying to the nodes'
// terminal APIs, and hub-local display settings shared by all web clients.
package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/cuterxy/cuterm/internal/autostart"
)

// errNodeNotFound is returned when no registered node matches a request ID.
var errNodeNotFound = errors.New("node not found")

// Node is a registered cuterm server. A Reverse node connected to the hub
// itself (see agent.go); its Addr is empty and all traffic rides the
// reverse tunnel instead of a direct connection.
type Node struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Addr    string `json:"addr"` // host:port, no scheme
	Reverse bool   `json:"reverse,omitempty"`
}

// NodeStatus is a Node plus its live reachability, as served by GET
// /api/nodes. An offline node keeps Online=false and no Version.
type NodeStatus struct {
	Node
	Online  bool   `json:"online"`
	Version string `json:"version,omitempty"`
}

// Appearance holds the terminal display settings shared by all web clients.
// Empty fields mean the client-side defaults apply.
type Appearance struct {
	FontFamily string `json:"fontFamily,omitempty"`
	FontSize   int    `json:"fontSize,omitempty"`
	Theme      string `json:"theme,omitempty"`
	Scrollback int    `json:"scrollback,omitempty"`
}

// Server wires the node registry to HTTP handlers.
type Server struct {
	mux      *http.ServeMux
	upgrader websocket.Upgrader
	dialer   websocket.Dialer
	client   *http.Client

	// OnPortChange is called by POST /api/port to re-listen on a new port.
	// It is set by main, which owns the listeners.
	OnPortChange func(port int) error

	// OnAppearanceChange is called by POST /api/appearance after the in-memory
	// settings are updated, so the caller can persist them.
	OnAppearanceChange func(a Appearance) error

	// OnLanguageChange is called by POST /api/language after the in-memory
	// value is updated, so the caller can persist it and switch the tray
	// menu language at runtime.
	OnLanguageChange func(lang string) error

	// OnNodesChange is called after the node registry changes (add / edit /
	// remove), so the caller can persist the new list.
	OnNodesChange func(nodes []Node) error

	nodesMu sync.RWMutex
	nodes   []Node

	// sessions holds the control channels of connected reverse nodes,
	// keyed by node ID; dials maps a dial token to the waiter expecting
	// the node's data channel (see agent.go).
	sessionsMu sync.RWMutex
	sessions   map[string]*agentSession
	dialsMu    sync.Mutex
	dials      map[string]chan *websocket.Conn

	appearanceMu sync.RWMutex
	appearance   Appearance

	languageMu sync.RWMutex
	language   string

	// version is the build version reported by GET /api/version.
	version string
}

// New builds a Server serving the web UI from staticFS (the embedded web/
// directory) and the initial node list (loaded from the persisted config).
func New(staticFS fs.FS, nodes []Node) *Server {
	s := &Server{
		mux: http.NewServeMux(),
		upgrader: websocket.Upgrader{
			// Connections come from browsers on the same origin; nothing
			// secret is at stake beyond shell access itself.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		dialer:   websocket.Dialer{HandshakeTimeout: 10 * time.Second},
		client:   &http.Client{Timeout: 15 * time.Second},
		nodes:    append([]Node(nil), nodes...),
		sessions: make(map[string]*agentSession),
		dials:    make(map[string]chan *websocket.Conn),
	}
	s.mux.Handle("GET /api/nodes", http.HandlerFunc(s.handleListNodes))
	s.mux.Handle("POST /api/nodes", http.HandlerFunc(s.handleAddNode))
	s.mux.Handle("PATCH /api/nodes/{id}", http.HandlerFunc(s.handleEditNode))
	s.mux.Handle("DELETE /api/nodes/{id}", http.HandlerFunc(s.handleRemoveNode))
	s.mux.Handle("GET /api/nodes/{id}/terminals", http.HandlerFunc(s.proxyToNode))
	s.mux.Handle("POST /api/nodes/{id}/terminals", http.HandlerFunc(s.proxyToNode))
	s.mux.Handle("PATCH /api/nodes/{id}/terminals/{tid}", http.HandlerFunc(s.proxyToNode))
	s.mux.Handle("DELETE /api/nodes/{id}/terminals/{tid}", http.HandlerFunc(s.proxyToNode))
	s.mux.Handle("GET /api/nodes/{id}/shells", http.HandlerFunc(s.proxyToNode))
	s.mux.Handle("POST /api/nodes/{id}/shell", http.HandlerFunc(s.proxyToNode))
	s.mux.Handle("POST /api/port", http.HandlerFunc(s.handlePort))
	s.mux.Handle("GET /api/appearance", http.HandlerFunc(s.handleGetAppearance))
	s.mux.Handle("POST /api/appearance", http.HandlerFunc(s.handleSetAppearance))
	s.mux.Handle("GET /api/language", http.HandlerFunc(s.handleGetLanguage))
	s.mux.Handle("POST /api/language", http.HandlerFunc(s.handleSetLanguage))
	s.mux.Handle("GET /api/version", http.HandlerFunc(s.handleVersion))
	s.mux.Handle("GET /api/autostart", http.HandlerFunc(s.handleGetAutostart))
	s.mux.Handle("POST /api/autostart", http.HandlerFunc(s.handleSetAutostart))
	s.mux.Handle("GET /ws/nodes/{id}/terminals/{tid}", http.HandlerFunc(s.handleWS))
	s.mux.Handle("GET /ws/agent", http.HandlerFunc(s.handleAgent))
	s.mux.Handle("GET /ws/agent/dial", http.HandlerFunc(s.handleAgentDial))
	s.mux.Handle("GET /", http.FileServer(http.FS(staticFS)))
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

// Nodes returns a copy of the current node list.
func (s *Server) Nodes() []Node {
	s.nodesMu.RLock()
	defer s.nodesMu.RUnlock()
	return append([]Node(nil), s.nodes...)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

/* ---------- node registry ---------- */

// normalizeAddr turns user input like "https://host/", "host" or
// "host:7681/" into a bare "host:port". A missing port defaults to cuterm's
// 7681.
func normalizeAddr(raw string) (string, error) {
	addr := strings.TrimSpace(raw)
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimRight(addr, "/")
	if addr == "" {
		return "", errors.New("address must not be empty")
	}
	if strings.ContainsAny(addr, "/?#") {
		return "", fmt.Errorf("invalid address: %s", raw)
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		if !strings.Contains(err.Error(), "missing port") {
			return "", fmt.Errorf("invalid address: %s", raw)
		}
		addr = net.JoinHostPort(addr, "7681")
	}
	return addr, nil
}

func (s *Server) node(id string) (Node, bool) {
	s.nodesMu.RLock()
	defer s.nodesMu.RUnlock()
	for _, n := range s.nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// nodeStatus probes a node's /api/version with a short timeout. A reverse
// node needs no probe: it is online while its control channel is up, and
// its version came with the hello.
func (s *Server) nodeStatus(n Node) NodeStatus {
	st := NodeStatus{Node: n}
	if n.Reverse {
		if sess, ok := s.agentSessionFor(n.ID); ok {
			st.Online = true
			st.Version = sess.version
		}
		return st
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+n.Addr+"/api/version", nil)
	if err != nil {
		return st
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return st
	}
	defer resp.Body.Close()
	var v struct {
		Version string `json:"version"`
	}
	if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&v) == nil {
		st.Online = true
		st.Version = v.Version
	}
	return st
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.Nodes()
	statuses := make([]NodeStatus, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func(i int, n Node) {
			defer wg.Done()
			statuses[i] = s.nodeStatus(n)
		}(i, n)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, statuses)
}

type nodeRequest struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
}

func (s *Server) handleAddNode(w http.ResponseWriter, r *http.Request) {
	var req nodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	addr, err := normalizeAddr(req.Addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = addr
	}
	if len(name) > 100 {
		writeError(w, http.StatusBadRequest, errors.New("name must be at most 100 characters"))
		return
	}

	s.nodesMu.Lock()
	for _, n := range s.nodes {
		if n.Addr == addr {
			s.nodesMu.Unlock()
			writeError(w, http.StatusConflict, fmt.Errorf("node already exists: %s", addr))
			return
		}
	}
	node := Node{ID: newNodeID(), Name: name, Addr: addr}
	s.nodes = append(s.nodes, node)
	nodes := append([]Node(nil), s.nodes...)
	s.nodesMu.Unlock()

	if s.OnNodesChange != nil {
		if err := s.OnNodesChange(nodes); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) handleEditNode(w http.ResponseWriter, r *http.Request) {
	var req nodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.nodesMu.Lock()
	idx := -1
	for i, n := range s.nodes {
		if n.ID == r.PathValue("id") {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.nodesMu.Unlock()
		writeError(w, http.StatusNotFound, errNodeNotFound)
		return
	}
	node := s.nodes[idx]
	if req.Addr != "" {
		if node.Reverse {
			s.nodesMu.Unlock()
			writeError(w, http.StatusBadRequest, errors.New("a reverse node's address is managed by the node itself"))
			return
		}
		addr, err := normalizeAddr(req.Addr)
		if err != nil {
			s.nodesMu.Unlock()
			writeError(w, http.StatusBadRequest, err)
			return
		}
		node.Addr = addr
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		if len(name) > 100 {
			s.nodesMu.Unlock()
			writeError(w, http.StatusBadRequest, errors.New("name must be at most 100 characters"))
			return
		}
		node.Name = name
	}
	s.nodes[idx] = node
	nodes := append([]Node(nil), s.nodes...)
	s.nodesMu.Unlock()

	if s.OnNodesChange != nil {
		if err := s.OnNodesChange(nodes); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// A reverse node with a live control channel would just re-register on
	// its next reconnect, so deleting it is rejected; clear the hub address
	// on the node instead. An offline reverse node can be removed normally.
	// The session check runs before nodesMu is taken: registerAgent takes
	// sessionsMu first and nodesMu second, so this keeps the lock order.
	if _, ok := s.agentSessionFor(id); ok {
		writeError(w, http.StatusConflict, errors.New("cannot remove a connected reverse node; clear the hub address on the node instead"))
		return
	}

	s.nodesMu.Lock()
	idx := -1
	for i, n := range s.nodes {
		if n.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.nodesMu.Unlock()
		writeError(w, http.StatusNotFound, errNodeNotFound)
		return
	}
	s.nodes = append(s.nodes[:idx], s.nodes[idx+1:]...)
	nodes := append([]Node(nil), s.nodes...)
	s.nodesMu.Unlock()

	if s.OnNodesChange != nil {
		if err := s.OnNodesChange(nodes); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func newNodeID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

/* ---------- proxying ---------- */

// proxyToNode forwards the request to the node's API unchanged: the hub
// route /api/nodes/{id}/... maps to the node route /api/....
func (s *Server) proxyToNode(w http.ResponseWriter, r *http.Request) {
	node, ok := s.node(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errNodeNotFound)
		return
	}
	prefix := "/api/nodes/" + node.ID
	upstream := "http://" + node.Addr + "/api" + strings.TrimPrefix(r.URL.Path, prefix)
	if node.Reverse {
		upstream = "http://tunnel/api" + strings.TrimPrefix(r.URL.Path, prefix)
	}
	if r.URL.RawQuery != "" {
		upstream += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstream, r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}

	var resp *http.Response
	if node.Reverse {
		// One HTTP round-trip over a fresh tunnel connection.
		conn, err := s.dialNode(r.Context(), node.ID)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("node %s: %v", node.Name, err))
			return
		}
		tr := &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return conn, nil
			},
		}
		resp, err = tr.RoundTrip(req)
		if err != nil {
			conn.Close()
			writeError(w, http.StatusBadGateway, fmt.Errorf("node %s: %v", node.Name, err))
			return
		}
	} else {
		resp, err = s.client.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("node %s unreachable: %v", node.Addr, err))
			return
		}
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleWS bridges a browser WebSocket to the node's terminal WebSocket,
// copying frames verbatim in both directions until either side goes away.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	node, ok := s.node(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errNodeNotFound)
		return
	}

	var upstream *websocket.Conn
	if node.Reverse {
		// Run the WebSocket handshake over a fresh tunnel connection.
		conn, err := s.dialNode(r.Context(), node.ID)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("node %s: %v", node.Name, err))
			return
		}
		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
			NetDialContext: func(context.Context, string, string) (net.Conn, error) {
				return conn, nil
			},
		}
		upstream, _, err = dialer.Dial("ws://tunnel/ws/terminals/"+r.PathValue("tid"), nil)
		if err != nil {
			conn.Close()
			writeError(w, http.StatusBadGateway, fmt.Errorf("node %s: %v", node.Name, err))
			return
		}
	} else {
		var err error
		upstream, _, err = s.dialer.Dial("ws://"+node.Addr+"/ws/terminals/"+r.PathValue("tid"), nil)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Errorf("node %s unreachable: %v", node.Addr, err))
			return
		}
	}
	defer upstream.Close()

	downstream, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer downstream.Close()

	// When one direction ends (client closed, terminal exited, node gone),
	// return and let the deferred closes unblock the other goroutine.
	done := make(chan struct{}, 2)
	go relayWS(upstream, downstream, done) // node -> browser
	go relayWS(downstream, upstream, done) // browser -> node
	<-done
}

func relayWS(dst, src *websocket.Conn, done chan<- struct{}) {
	for {
		mt, data, err := src.ReadMessage()
		if err != nil {
			break
		}
		if err := dst.WriteMessage(mt, data); err != nil {
			break
		}
	}
	done <- struct{}{}
}

/* ---------- hub-local settings ---------- */

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
