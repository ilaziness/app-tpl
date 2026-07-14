// Package tcp provides TCP request handlers.
package tcp

import (
	"github.com/ilaziness/app-tpl/internal/types"
)

// ChatSubHandler handles a specific chat message type within ChatHandler.
type ChatSubHandler interface {
	MessageType() string
	Handle(conn *types.Connection, msg *ChatMessage) error
}
