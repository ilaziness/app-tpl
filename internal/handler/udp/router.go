// Package udp provides UDP request handlers.
package udp

import (
	"encoding/json"
	"net"

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

// Router routes application-level UDP packets by the JSON "type" field.
// It satisfies types.UDPHandler so it can be registered directly with the UDP server.
type Router struct {
	logger         *zap.Logger
	handlers       map[string]types.UDPHandler
	defaultHandler types.UDPHandler
}

// NewRouter creates a new Router and registers the provided handlers.
// Handlers may implement MessageTypeProvider or MessageTypesProvider to be registered automatically.
func NewRouter(logger *zap.Logger, handlers ...types.UDPHandler) *Router {
	r := &Router{
		logger:   logger,
		handlers: make(map[string]types.UDPHandler),
	}
	for _, h := range handlers {
		r.Register(h)
	}
	return r
}

// Register adds a handler to the router.
// If the handler implements MessageTypeProvider or MessageTypesProvider, it is registered automatically.
func (r *Router) Register(handler types.UDPHandler) *Router {
	if handler == nil {
		r.logger.Warn("UDP handler registration ignored: nil handler")
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
		r.logger.Warn("UDP handler registration ignored: handler does not provide message type")
	}
	return r
}

// RegisterType explicitly registers a handler for the given application-level message type.
func (r *Router) RegisterType(msgType string, handler types.UDPHandler) *Router {
	r.registerType(msgType, handler)
	return r
}

func (r *Router) registerType(msgType string, handler types.UDPHandler) {
	if msgType == "" {
		r.logger.Warn("UDP handler registration ignored: empty message type")
		return
	}
	r.handlers[msgType] = handler
	r.logger.Info("UDP route registered", zap.String("message_type", msgType))
}

// Default sets a fallback handler for packets without a matching route.
func (r *Router) Default(handler types.UDPHandler) *Router {
	r.defaultHandler = handler
	return r
}

func remoteAddrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

// Handle implements types.UDPHandler.
// It decodes the application-level message type and dispatches to the matching handler.
func (r *Router) Handle(packet *types.UDPPacket) ([]byte, error) {
	if len(packet.Data) == 0 {
		if r.defaultHandler != nil {
			return r.defaultHandler.Handle(packet)
		}
		return nil, nil
	}

	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(packet.Data, &probe); err != nil {
		r.logger.Error("Failed to probe UDP packet type",
			zap.String("remote_addr", remoteAddrString(packet.RemoteAddr)),
			zap.Error(err))
		if r.defaultHandler != nil {
			return r.defaultHandler.Handle(packet)
		}
		return nil, err
	}

	handler, ok := r.handlers[probe.Type]
	if !ok {
		if r.defaultHandler != nil {
			r.logger.Debug("Unknown UDP message type, using default handler",
				zap.String("remote_addr", remoteAddrString(packet.RemoteAddr)),
				zap.String("msg_type", probe.Type))
			return r.defaultHandler.Handle(packet)
		}
		r.logger.Warn("Unknown UDP message type",
			zap.String("remote_addr", remoteAddrString(packet.RemoteAddr)),
			zap.String("msg_type", probe.Type))
		return nil, nil
	}

	return handler.Handle(packet)
}
