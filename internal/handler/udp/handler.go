// Package udp provides UDP request handlers.
package udp

import (
	"github.com/example/app-tpl/internal/types"
)

// SubPacketHandler handles a specific application-level message type within a Dispatcher.
type SubPacketHandler interface {
	MessageType() string
	Handle(packet *types.UDPPacket) ([]byte, error)
}
