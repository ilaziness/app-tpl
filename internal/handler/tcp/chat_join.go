// Package tcp provides TCP request handlers.
package tcp

import (
	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// JoinHandler handles "join" chat messages.
type JoinHandler struct {
	chat *ChatHandler
}

// MessageType returns the message type this handler processes.
func (h *JoinHandler) MessageType() string {
	return "join"
}

// Handle processes a join event: registers the connection in the clients map and broadcasts the message.
func (h *JoinHandler) Handle(conn *types.Connection, msg *ChatMessage) error {
	h.chat.clientsMux.Lock()
	h.chat.clients[conn.ID] = conn
	total := len(h.chat.clients)
	h.chat.clientsMux.Unlock()

	h.chat.logger.Info("Client joined chat",
		zap.String("conn_id", conn.ID),
		zap.Int("total_clients", total))

	select {
	case h.chat.broadcast <- msg:
	case <-h.chat.shutdown:
		// Roll back the registration so no zombie entry is left after shutdown.
		h.chat.clientsMux.Lock()
		delete(h.chat.clients, conn.ID)
		h.chat.clientsMux.Unlock()
	}
	return nil
}
