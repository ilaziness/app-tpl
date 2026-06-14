// Package tcp provides TCP request handlers.
package tcp

import (
	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// ChatMessageHandler handles "message" chat messages.
type ChatMessageHandler struct {
	chat *ChatHandler
}

// MessageType returns the message type this handler processes.
func (h *ChatMessageHandler) MessageType() string {
	return "message"
}

// Handle processes an incoming chat message and forwards it to the broadcast channel.
func (h *ChatMessageHandler) Handle(conn *types.Connection, msg *ChatMessage) error {
	h.chat.logger.Debug("Chat message received",
		zap.String("conn_id", conn.ID),
		zap.String("content", msg.Content))

	select {
	case h.chat.broadcast <- msg:
	case <-h.chat.shutdown:
	default:
		h.chat.logger.Warn("Broadcast channel full, dropping chat message",
			zap.String("conn_id", conn.ID))
	}
	return nil
}
