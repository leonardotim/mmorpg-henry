//go:build js && wasm

package network

import (
	"context"
	"net"

	"github.com/coder/websocket"
)

// Dial connects to the server. On WASM, we interpret the address as a WebSocket URL.
func Dial(address string) (net.Conn, error) {
	// Address is like "127.0.0.1:8080"
	// For WebSocket we need "ws://127.0.0.1:8081/ws"

	// Hardcoded conversion for now for prototype simplicity
	// We'll ignore the passed address mostly and connect to the HTTP owner + 8081
	// Or just assume address passed is proper WS url or we fix it here.

	wsURL := "ws://localhost:8081/ws"

	ctx := context.Background()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}

	return websocket.NetConn(ctx, c, websocket.MessageBinary), nil
}
