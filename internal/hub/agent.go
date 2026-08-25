// Reverse-connected cuterm nodes ("agents"): a node behind NAT keeps a
// persistent control WebSocket to the hub, which auto-registers it. Every
// proxied request rides a per-request data WebSocket the node dials back on
// demand, so the hub never needs inbound access to the node.
package hub

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/cuterxy/cuterm/internal/wsconn"
)

// agentHello is the first (and only) message a node sends on the control
// channel. ID is a random token the node generates once and persists, so
// reconnects update the same registry entry instead of duplicating it.
type agentHello struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// agentSession is one connected node's control channel.
type agentSession struct {
	ctrl    *websocket.Conn
	name    string
	version string

	wmu sync.Mutex // serializes writes to ctrl
}

// handleAgent serves GET /ws/agent: the persistent control channel of a
// reverse-connected node. The node is registered (or refreshed) on connect
// and goes back to offline when the channel drops.
func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("agent upgrade: %v", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var hello agentHello
	if err := conn.ReadJSON(&hello); err != nil {
		conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	if hello.Type != "hello" || hello.ID == "" || len(hello.ID) > 64 || len(hello.Name) > 100 {
		conn.Close()
		return
	}

	sess := &agentSession{ctrl: conn, name: hello.Name, version: hello.Version}
	s.registerAgent(hello.ID, sess)
	defer s.unregisterAgent(hello.ID, sess)

	// The node sends nothing after the hello; read until the channel drops.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// registerAgent adds the node to the registry on first connect and makes
// sess the node's current session, replacing (and closing) any previous one.
func (s *Server) registerAgent(id string, sess *agentSession) {
	s.sessionsMu.Lock()
	if old, ok := s.sessions[id]; ok && old != sess {
		old.ctrl.Close()
	}
	s.sessions[id] = sess
	s.sessionsMu.Unlock()

	s.nodesMu.Lock()
	found := false
	for i := range s.nodes {
		if s.nodes[i].ID == id {
			found = true
			break
		}
	}
	if !found {
		name := sess.name
		if name == "" {
			name = id
		}
		s.nodes = append(s.nodes, Node{ID: id, Name: name, Reverse: true})
	}
	nodes := append([]Node(nil), s.nodes...)
	s.nodesMu.Unlock()

	// Persist only when the registry actually changed (first connect).
	if !found && s.OnNodesChange != nil {
		if err := s.OnNodesChange(nodes); err != nil {
			log.Printf("persist auto-registered node: %v", err)
		}
	}
}

// unregisterAgent drops the session unless a newer one already replaced it.
func (s *Server) unregisterAgent(id string, sess *agentSession) {
	s.sessionsMu.Lock()
	if cur, ok := s.sessions[id]; ok && cur == sess {
		delete(s.sessions, id)
	}
	s.sessionsMu.Unlock()
	sess.ctrl.Close()
}

// agentSessionFor returns the live session of a reverse node, if connected.
func (s *Server) agentSessionFor(nodeID string) (*agentSession, bool) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	sess, ok := s.sessions[nodeID]
	return sess, ok
}

// handleAgentDial serves GET /ws/agent/dial?id=<token>: the data channel a
// node opens in answer to a dial command. The connection's ownership passes
// to the goroutine waiting in dialNode.
func (s *Server) handleAgentDial(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s.dialsMu.Lock()
	ch, ok := s.dials[id]
	s.dialsMu.Unlock()
	if !ok {
		http.Error(w, "unknown dial id", http.StatusBadRequest)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("agent dial upgrade: %v", err)
		return
	}
	ch <- conn // buffered; dialNode drains and closes on late arrival
}

// dialNode asks the node to open a data channel and returns it as a net.Conn
// bridged to the node's local HTTP server.
func (s *Server) dialNode(ctx context.Context, nodeID string) (net.Conn, error) {
	sess, ok := s.agentSessionFor(nodeID)
	if !ok {
		return nil, errors.New("node is not connected")
	}

	id := newNodeID() + newNodeID()
	ch := make(chan *websocket.Conn, 1)
	s.dialsMu.Lock()
	s.dials[id] = ch
	s.dialsMu.Unlock()
	defer func() {
		s.dialsMu.Lock()
		delete(s.dials, id)
		s.dialsMu.Unlock()
	}()

	sess.wmu.Lock()
	err := sess.ctrl.WriteJSON(map[string]string{"type": "dial", "id": id})
	sess.wmu.Unlock()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case conn := <-ch:
		return wsconn.New(conn), nil
	case <-ctx.Done():
		// A late data channel has no consumer left; close it.
		select {
		case conn := <-ch:
			conn.Close()
		default:
		}
		return nil, ctx.Err()
	}
}
