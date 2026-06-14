// Package tcp provides TCP request handlers.
package tcp

import (
	"encoding/json"
	"sync"

	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// ChatMessage represents a chat message.
type ChatMessage struct {
	Type    string `json:"type"`    // "join", "message", "leave"
	From    string `json:"from"`    // sender connection ID
	Content string `json:"content"` // message content
}

// ChatHandler manages a simple chat room.
// It acts as an orchestrator that dispatches messages to registered SubMessageHandlers,
// eliminating the need for a switch statement when new message types are added.
type ChatHandler struct {
	logger       *zap.Logger
	clients      map[string]*types.Connection
	clientsMux   sync.RWMutex
	broadcast    chan *ChatMessage
	shutdown     chan struct{}
	shutdownOnce sync.Once
	handlers     map[string]SubMessageHandler
}

// NewChatHandler creates a new chat handler with all sub-handlers registered.
func NewChatHandler(logger *zap.Logger) *ChatHandler {
	h := &ChatHandler{
		logger:    logger,
		clients:   make(map[string]*types.Connection),
		broadcast: make(chan *ChatMessage, 100),
		shutdown:  make(chan struct{}),
		handlers:  make(map[string]SubMessageHandler),
	}
	h.register(&JoinHandler{chat: h})
	h.register(&ChatMessageHandler{chat: h})
	h.register(&LeaveHandler{chat: h})
	go h.broadcastLoop()
	return h
}

// register adds a SubMessageHandler to the dispatch table.
func (h *ChatHandler) register(sub SubMessageHandler) {
	h.handlers[sub.MessageType()] = sub
}

// Shutdown stops the broadcast loop. Safe to call multiple times.
func (h *ChatHandler) Shutdown() {
	h.shutdownOnce.Do(func() { close(h.shutdown) })
}

// Handle implements the types.TCPHandler interface.
// It decodes the message and dispatches to the appropriate SubMessageHandler.
func (h *ChatHandler) Handle(conn *types.Connection, data []byte) ([]byte, error) {
	var msg ChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.logger.Error("Failed to unmarshal chat message",
			zap.String("conn_id", conn.ID),
			zap.Error(err))
		return nil, err
	}

	msg.From = conn.ID

	sub, ok := h.handlers[msg.Type]
	if !ok {
		h.logger.Warn("Unknown chat message type",
			zap.String("conn_id", conn.ID),
			zap.String("msg_type", msg.Type))
		return nil, nil
	}

	if err := sub.Handle(conn, &msg); err != nil {
		return nil, err
	}

	return nil, nil
}

// broadcastLoop broadcasts messages to all connected clients.
func (h *ChatHandler) broadcastLoop() {
	for {
		select {
		case <-h.shutdown:
			return
		case msg, ok := <-h.broadcast:
			if !ok {
				return
			}

			h.clientsMux.RLock()
			clients := make([]*types.Connection, 0, len(h.clients))
			for _, conn := range h.clients {
				clients = append(clients, conn)
			}
			h.clientsMux.RUnlock()

			data, err := json.Marshal(msg)
			if err != nil {
				h.logger.Error("Failed to marshal chat message", zap.Error(err))
				continue
			}

			for _, conn := range clients {
				if msg.From != conn.ID {
					// Send to all clients except sender
					_, err := conn.Write(data)
					if err != nil {
						h.logger.Error("Failed to send chat message, removing client",
							zap.String("conn_id", conn.ID),
							zap.Error(err))
						// Remove disconnected client
						h.clientsMux.Lock()
						delete(h.clients, conn.ID)
						h.clientsMux.Unlock()
					}
				}
			}
		}
	}
}

// GetClientCount returns the current number of connected clients.
func (h *ChatHandler) GetClientCount() int {
	h.clientsMux.RLock()
	defer h.clientsMux.RUnlock()
	return len(h.clients)
}
