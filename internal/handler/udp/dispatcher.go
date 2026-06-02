// Package udp provides UDP request handlers.
package udp

import (
	"encoding/json"

	"github.com/example/app-tpl/internal/types"
	"go.uber.org/zap"
)

// packetTypeProbe is used to peek at the "type" field of an incoming JSON packet.
type packetTypeProbe struct {
	Type string `json:"type"`
}

// Dispatcher is the application-level UDP handler orchestrator.
// It inspects the "type" field of each packet's JSON payload and routes
// it to the registered SubPacketHandler, eliminating switch statements
// when new message types are added.
//
// NOTE: Dispatcher assumes JSON-encoded payloads. It is incompatible with
// the binary (Gob) codec. If udp.codec is set to "binary" in the config,
// a custom dispatcher that handles Gob decoding must be provided instead.
type Dispatcher struct {
	logger   *zap.Logger
	handlers map[string]SubPacketHandler
}

// NewDispatcher creates a new Dispatcher with all sub-handlers registered.
func NewDispatcher(logger *zap.Logger, timeHandler *TimeHandler, statsHandler *StatsHandler) *Dispatcher {
	d := &Dispatcher{
		logger:   logger,
		handlers: make(map[string]SubPacketHandler),
	}
	d.register(timeHandler)
	d.register(statsHandler)
	return d
}

// register adds a SubPacketHandler to the dispatch table.
func (d *Dispatcher) register(sub SubPacketHandler) {
	d.handlers[sub.MessageType()] = sub
}

// Handle implements the types.UDPHandler interface.
// It decodes the message type and dispatches to the appropriate SubPacketHandler.
func (d *Dispatcher) Handle(packet *types.UDPPacket) ([]byte, error) {
	var probe packetTypeProbe
	if err := json.Unmarshal(packet.Data, &probe); err != nil {
		d.logger.Error("Failed to probe UDP packet type",
			zap.String("remote_addr", packet.RemoteAddr.String()),
			zap.Error(err))
		return nil, err
	}

	sub, ok := d.handlers[probe.Type]
	if !ok {
		d.logger.Warn("Unknown UDP message type",
			zap.String("remote_addr", packet.RemoteAddr.String()),
			zap.String("msg_type", probe.Type))
		return nil, nil
	}

	return sub.Handle(packet)
}
