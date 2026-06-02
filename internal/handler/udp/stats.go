// Package udp provides UDP request handlers.
package udp

import (
	"encoding/json"
	"sync/atomic"

	"github.com/example/app-tpl/internal/types"
	"go.uber.org/zap"
)

// StatsRequest represents a stats query request.
type StatsRequest struct {
	Type string `json:"type"` // "stats"
}

// StatsResponse represents a stats query response.
type StatsResponse struct {
	Type            string `json:"type"`
	PacketsReceived uint64 `json:"packets_received"`
	PacketsSent     uint64 `json:"packets_sent"`
	BytesReceived   uint64 `json:"bytes_received"`
	BytesSent       uint64 `json:"bytes_sent"`
	Sessions        int    `json:"sessions"`
}

// StatsHandler provides statistics service.
type StatsHandler struct {
	logger          *zap.Logger
	packetsReceived atomic.Uint64
	packetsSent     atomic.Uint64
	bytesReceived   atomic.Uint64
	bytesSent       atomic.Uint64
	getSessionCount func() int
}

// MessageType returns the application-level message type this handler processes.
func (h *StatsHandler) MessageType() string {
	return "stats"
}

// NewStatsHandler creates a new stats handler.
func NewStatsHandler(logger *zap.Logger) *StatsHandler {
	return &StatsHandler{
		logger:          logger,
		getSessionCount: func() int { return 0 },
	}
}

// SetSessionCountFunc sets the callback used to retrieve the current session count.
func (h *StatsHandler) SetSessionCountFunc(fn func() int) {
	h.getSessionCount = fn
}

// Handle implements the Handler interface.
func (h *StatsHandler) Handle(packet *types.UDPPacket) ([]byte, error) {
	var req StatsRequest
	if err := json.Unmarshal(packet.Data, &req); err != nil {
		h.logger.Error("Failed to unmarshal stats request",
			zap.String("remote_addr", packet.RemoteAddr.String()),
			zap.Error(err))
		return nil, err
	}

	// Update stats
	h.packetsReceived.Add(1)
	h.bytesReceived.Add(uint64(len(packet.Data)))

	resp := StatsResponse{
		Type:            "stats",
		PacketsReceived: h.packetsReceived.Load(),
		PacketsSent:     h.packetsSent.Load(),
		BytesReceived:   h.bytesReceived.Load(),
		BytesSent:       h.bytesSent.Load(),
		Sessions:        h.getSessionCount(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("Failed to marshal stats response", zap.Error(err))
		return nil, err
	}

	h.logger.Debug("Stats query handled",
		zap.String("remote_addr", packet.RemoteAddr.String()))

	return data, nil
}

// IncrementPacketsSent increments the packets sent counter.
func (h *StatsHandler) IncrementPacketsSent(bytes int) {
	h.packetsSent.Add(1)
	h.bytesSent.Add(uint64(bytes)) //nolint:gosec // packet byte count, always non-negative
}
