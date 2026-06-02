// Package types defines shared data structures and interfaces used by server and handler packages.
package types

import (
	"net"
	"sync"
	"time"
)

// Connection represents a TCP connection with metadata.
type Connection struct {
	ID           string
	Conn         net.Conn
	RemoteAddr   string
	LastActive   time.Time
	CreatedAt    time.Time
	TimeoutCount int
	WriteTimeout time.Duration
	writeMu      sync.Mutex
}

// Write writes data to the underlying connection in a concurrency-safe manner.
// Multiple goroutines (e.g., handleConnection and broadcastLoop) may write
// to the same connection concurrently; this method serializes those writes.
func (c *Connection) Write(data []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.WriteTimeout > 0 {
		if err := c.Conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Write(data)
}

// UDPPacket represents a UDP packet with metadata.
type UDPPacket struct {
	Data       []byte
	RemoteAddr net.Addr
	LocalAddr  net.Addr
	Timestamp  time.Time
}

// Session represents a UDP session (stateless, but can track per-client info).
type Session struct {
	RemoteAddr string
	LastActive time.Time
	CreatedAt  time.Time
}

// TCPHandler defines the interface for TCP message handlers.
type TCPHandler interface {
	Handle(conn *Connection, data []byte) ([]byte, error)
}

// TCPHandlerFunc is a function type that implements the TCPHandler interface.
type TCPHandlerFunc func(conn *Connection, data []byte) ([]byte, error)

// Handle implements the TCPHandler interface for TCPHandlerFunc.
func (f TCPHandlerFunc) Handle(conn *Connection, data []byte) ([]byte, error) {
	return f(conn, data)
}

// UDPHandler defines the interface for UDP message handlers.
type UDPHandler interface {
	Handle(packet *UDPPacket) ([]byte, error)
}

// UDPHandlerFunc is a function type that implements the UDPHandler interface.
type UDPHandlerFunc func(packet *UDPPacket) ([]byte, error)

// Handle implements the UDPHandler interface for UDPHandlerFunc.
func (f UDPHandlerFunc) Handle(packet *UDPPacket) ([]byte, error) {
	return f(packet)
}
