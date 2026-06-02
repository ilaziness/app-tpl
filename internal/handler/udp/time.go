// Package udp provides UDP request handlers.
package udp

import (
	"encoding/json"
	"time"

	"github.com/example/app-tpl/internal/types"
	"go.uber.org/zap"
)

// TimeRequest represents a time query request.
type TimeRequest struct {
	Type string `json:"type"` // "time"
}

// TimeResponse represents a time query response.
type TimeResponse struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Unix      int64  `json:"unix"`
}

// TimeHandler provides time query service.
type TimeHandler struct {
	logger *zap.Logger
}

// NewTimeHandler creates a new time handler.
func NewTimeHandler(logger *zap.Logger) *TimeHandler {
	return &TimeHandler{
		logger: logger,
	}
}

// MessageType returns the application-level message type this handler processes.
func (h *TimeHandler) MessageType() string {
	return "time"
}

// Handle implements the Handler interface.
func (h *TimeHandler) Handle(packet *types.UDPPacket) ([]byte, error) {
	var req TimeRequest
	if err := json.Unmarshal(packet.Data, &req); err != nil {
		h.logger.Error("Failed to unmarshal time request",
			zap.String("remote_addr", packet.RemoteAddr.String()),
			zap.Error(err))
		return nil, err
	}

	now := time.Now()
	resp := TimeResponse{
		Type:      "time",
		Timestamp: now.Format(time.RFC3339),
		Unix:      now.Unix(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("Failed to marshal time response", zap.Error(err))
		return nil, err
	}

	h.logger.Debug("Time query handled",
		zap.String("remote_addr", packet.RemoteAddr.String()))

	return data, nil
}
