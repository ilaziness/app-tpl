// Package tcp provides TCP request handlers.
package tcp

import (
	"encoding/json"

	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// MessageTypeProvider is implemented by handlers that handle a single application-level message type.
type MessageTypeProvider interface {
	MessageType() string
}

// MessageTypesProvider is implemented by handlers that handle multiple application-level message types.
type MessageTypesProvider interface {
	MessageTypes() []string
}

// Router routes application-level TCP messages by the JSON "type" field.
// It satisfies types.TCPHandler so it can be registered directly with the TCP server.
type Router struct {
	logger         *zap.Logger
	handlers       map[string]types.TCPHandler
	defaultHandler types.TCPHandler
}

// NewRouter creates a new Router and registers the provided handlers.
// Handlers may implement MessageTypeProvider or MessageTypesProvider to be registered automatically.
func NewRouter(logger *zap.Logger, handlers ...types.TCPHandler) *Router {
	r := &Router{
		logger:   logger,
		handlers: make(map[string]types.TCPHandler),
	}
	for _, h := range handlers {
		r.Register(h)
	}
	return r
}

// Register adds a handler to the router.
// If the handler implements MessageTypeProvider or MessageTypesProvider, it is registered automatically.
func (r *Router) Register(handler types.TCPHandler) *Router {
	if handler == nil {
		r.logger.Warn("TCP handler registration ignored: nil handler")
		return r
	}

	registered := false
	if p, ok := handler.(MessageTypesProvider); ok {
		for _, t := range p.MessageTypes() {
			r.registerType(t, handler)
			registered = true
		}
	}
	if p, ok := handler.(MessageTypeProvider); ok {
		r.registerType(p.MessageType(), handler)
		registered = true
	}

	if !registered {
		r.logger.Warn("TCP handler registration ignored: handler does not provide message type")
	}
	return r
}

// RegisterType explicitly registers a handler for the given application-level message type.
func (r *Router) RegisterType(msgType string, handler types.TCPHandler) *Router {
	r.registerType(msgType, handler)
	return r
}

func (r *Router) registerType(msgType string, handler types.TCPHandler) {
	if msgType == "" {
		r.logger.Warn("TCP handler registration ignored: empty message type")
		return
	}
	r.handlers[msgType] = handler
	r.logger.Info("TCP route registered", zap.String("message_type", msgType))
}

// Default sets a fallback handler for messages without a matching route.
func (r *Router) Default(handler types.TCPHandler) *Router {
	r.defaultHandler = handler
	return r
}

// Handle implements types.TCPHandler.
// It decodes the application-level message type and dispatches to the matching handler.
func (r *Router) Handle(conn *types.Connection, data []byte) ([]byte, error) {
	if len(data) == 0 {
		if r.defaultHandler != nil {
			return r.defaultHandler.Handle(conn, data)
		}
		return nil, nil
	}

	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		r.logger.Error("Failed to probe TCP message type",
			zap.String("conn_id", conn.ID),
			zap.Error(err))
		if r.defaultHandler != nil {
			return r.defaultHandler.Handle(conn, data)
		}
		return nil, err
	}

	handler, ok := r.handlers[probe.Type]
	if !ok {
		if r.defaultHandler != nil {
			r.logger.Debug("Unknown TCP message type, using default handler",
				zap.String("conn_id", conn.ID),
				zap.String("msg_type", probe.Type))
			return r.defaultHandler.Handle(conn, data)
		}
		r.logger.Warn("Unknown TCP message type",
			zap.String("conn_id", conn.ID),
			zap.String("msg_type", probe.Type))
		return nil, nil
	}

	return handler.Handle(conn, data)
}
