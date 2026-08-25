// Package wsconn adapts a *websocket.Conn to net.Conn, so HTTP requests and
// WebSocket handshakes can ride over a single WebSocket tunnel. Frames are
// binary; message boundaries are invisible to the byte stream.
package wsconn

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Conn is a net.Conn backed by a WebSocket. A single reader and a single
// writer may use it concurrently (the same constraint as a TCP conn).
type Conn struct {
	ws *websocket.Conn
	r  io.Reader // reader of the message currently being drained

	wmu sync.Mutex // serializes WriteMessage
}

// New wraps ws in a net.Conn.
func New(ws *websocket.Conn) *Conn {
	return &Conn{ws: ws}
}

func (c *Conn) Read(p []byte) (int, error) {
	for {
		if c.r == nil {
			_, r, err := c.ws.NextReader()
			if err != nil {
				return 0, err
			}
			c.r = r
		}
		n, err := c.r.Read(p)
		if err == io.EOF {
			// Message exhausted; move on to the next one.
			c.r = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *Conn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := c.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *Conn) Close() error { return c.ws.Close() }

func (c *Conn) LocalAddr() net.Addr { return c.ws.LocalAddr() }

func (c *Conn) RemoteAddr() net.Addr { return c.ws.RemoteAddr() }

func (c *Conn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error { return c.ws.SetReadDeadline(t) }

func (c *Conn) SetWriteDeadline(t time.Time) error { return c.ws.SetWriteDeadline(t) }
