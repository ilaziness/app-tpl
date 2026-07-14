// Package udp provides UDP request handlers.
package udp

import (
	"github.com/ilaziness/app-tpl/internal/types"
)

// SubPacketHandler handles a specific application-level message type within the Router.
// It satisfies types.UDPHandler and can be registered automatically with NewRouter.
type SubPacketHandler interface {
	MessageType() string
	Handle(packet *types.UDPPacket) ([]byte, error)
}
