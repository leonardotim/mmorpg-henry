//go:build !js || !wasm

package network

import (
	"net"
)

// Dial connects to a TCP address.
func Dial(address string) (net.Conn, error) {
	return net.Dial("tcp", address)
}
