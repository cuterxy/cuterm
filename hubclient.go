package main

import (
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/cuterxy/cuterm/internal/wsconn"
)

// hubClient keeps a persistent reverse tunnel to a cuterm-hub: one control
// WebSocket carrying a hello and dial commands, plus a fresh data WebSocket
// per dial command that is bridged to this node's own HTTP server. The hub
// auto-registers the node and proxies all traffic through the tunnel, so the
// node can sit behind NAT.
type hubClient struct {
	hubAddr   string // hub host:port
	id        string // persistent node ID sent in the hello
	name      string // display name (hostname)
	version   string
	localAddr func() string // current cuterm listen address (127.0.0.1:port)

	mu      sync.Mutex
	stopCh  chan struct{}
	stopped bool
	cur     *websocket.Conn // active control channel, closed by Stop
	online  atomic.Bool
}

func newHubClient(hubAddr, id, name, version string, localAddr func() string) *hubClient {
	return &hubClient{
		hubAddr:   hubAddr,
		id:        id,
		name:      name,
		version:   version,
		localAddr: localAddr,
		stopCh:    make(chan struct{}),
	}
}

// Connected reports whether the control channel is currently up.
func (c *hubClient) Connected() bool { return c.online.Load() }

// Start launches the reconnect loop in the background.
func (c *hubClient) Start() {
	go c.run()
}

// Stop terminates the reconnect loop and the current connection.
func (c *hubClient) Stop() {
	c.mu.Lock()
	if !c.stopped {
		c.stopped = true
		close(c.stopCh)
	}
	cur := c.cur
	c.mu.Unlock()
	if cur != nil {
		cur.Close() // unblocks the read loop
	}
	c.online.Store(false)
}

func (c *hubClient) run() {
	backoff := time.Second
	for {
		established, err := c.connectOnce()
		c.online.Store(false)
		if err != nil {
			log.Printf("hub %s: %v", c.hubAddr, err)
		}
		if established {
			backoff = time.Second
		}
		select {
		case <-c.stopCh:
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

// connectOnce runs one control-channel session until it drops. established
// reports whether the session got as far as the hello, i.e. the failure
// (if any) was a drop rather than a refused connection.
func (c *hubClient) connectOnce() (established bool, err error) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	ws, _, err := dialer.Dial("ws://"+c.hubAddr+"/ws/agent", nil)
	if err != nil {
		return false, err
	}
	defer ws.Close()

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		ws.Close()
		return false, nil
	}
	c.cur = ws
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.cur == ws {
			c.cur = nil
		}
		c.mu.Unlock()
	}()

	if err := ws.WriteJSON(map[string]string{
		"type":    "hello",
		"id":      c.id,
		"name":    c.name,
		"version": c.version,
	}); err != nil {
		return false, err
	}
	c.online.Store(true)
	log.Printf("hub %s: connected", c.hubAddr)

	for {
		var msg struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if err := ws.ReadJSON(&msg); err != nil {
			return true, err
		}
		if msg.Type == "dial" && msg.ID != "" {
			go c.serveDial(msg.ID)
		}
	}
}

// serveDial answers a dial command: open the data channel back to the hub
// and bridge it to the local HTTP server in both directions.
func (c *hubClient) serveDial(id string) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	ws, _, err := dialer.Dial("ws://"+c.hubAddr+"/ws/agent/dial?id="+id, nil)
	if err != nil {
		log.Printf("hub dial %s: %v", id, err)
		return
	}
	tunnel := wsconn.New(ws)
	defer tunnel.Close()

	local, err := net.DialTimeout("tcp", c.localAddr(), 10*time.Second)
	if err != nil {
		log.Printf("hub dial %s: local %s: %v", id, c.localAddr(), err)
		return
	}
	defer local.Close()

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(local, tunnel); done <- struct{}{} }()
	go func() { _, _ = io.Copy(tunnel, local); done <- struct{}{} }()
	<-done
}
