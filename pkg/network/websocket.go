package network

import (
	"context"
	"net"
	"net/http"

	"github.com/coder/websocket"
)

// WebSocketConn adapts a websocket.Conn to net.Conn interface
// Removed unused struct.

func NewWebSocketConn(ctx context.Context, c *websocket.Conn) net.Conn {
	return websocket.NetConn(ctx, c, websocket.MessageBinary)
}

// StartWebSocketServer starts a simple HTTP server that upgrades to WebSocket and passes net.Conn to a handler
func StartWebSocketServer(addr string, handler func(net.Conn)) error {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // Allow all origins for prototype
			OriginPatterns:     []string{"*"},
		})
		if err != nil {
			return
		}

		// Adapt to net.Conn
		// Use request context or background?
		// Note: websocket.NetConn closes the connection when the context is cancelled.
		// We should use a context that lives as long as the connection.
		// Actually, NetConn takes a context for the *Dial* usually, but here...
		// Ah, NetConn function creates a wrapper.

		conn := websocket.NetConn(context.Background(), c, websocket.MessageBinary)

		// Hand off to existing handler
		go handler(conn)
	})

	// Also serve static files for the client!
	http.Handle("/", http.FileServer(http.Dir("./static")))

	return http.ListenAndServe(addr, nil)
}
