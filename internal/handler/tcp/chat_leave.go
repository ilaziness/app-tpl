// Package tcp provides TCP request handlers.
package tcp

import (
	"github.com/example/app-tpl/internal/types"
	"go.uber.org/zap"
)

// LeaveHandler handles "leave" chat messages.
type LeaveHandler struct {
	chat *ChatHandler
}

// MessageType returns the message type this handler processes.
func (h *LeaveHandler) MessageType() string {
	return "leave"
}

// Handle processes a leave event: removes the connection from the clients map and broadcasts the message.
func (h *LeaveHandler) Handle(conn *types.Connection, msg *ChatMessage) error {
	h.chat.clientsMux.Lock()
	delete(h.chat.clients, conn.ID)
	total := len(h.chat.clients)
	h.chat.clientsMux.Unlock()

	h.chat.logger.Info("Client left chat",
		zap.String("conn_id", conn.ID),
		zap.Int("total_clients", total))

	select {
	case h.chat.broadcast <- msg:
	case <-h.chat.shutdown:
	}
	return nil
}
