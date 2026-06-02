// Package tcp provides TCP request handlers.
package tcp

import (
	"github.com/example/app-tpl/internal/types"
)

// SubMessageHandler handles a specific application-level message type within a handler.
type SubMessageHandler interface {
	MessageType() string
	Handle(conn *types.Connection, msg *ChatMessage) error
}
