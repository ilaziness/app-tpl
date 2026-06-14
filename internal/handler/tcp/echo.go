// Package tcp provides TCP request handlers.
package tcp

import (
	"github.com/ilaziness/app-tpl/internal/types"
	"go.uber.org/zap"
)

// EchoHandler echoes back the received data.
// 这是一个简单的回显服务示例，可以直接测试 TCP 连接。
type EchoHandler struct {
	logger *zap.Logger
}

// NewEchoHandler creates a new echo handler.
func NewEchoHandler(logger *zap.Logger) *EchoHandler {
	return &EchoHandler{
		logger: logger,
	}
}

// Handle implements the Handler interface.
// 接收任何数据并原样返回，用于测试 TCP 连接是否正常。
func (h *EchoHandler) Handle(conn *types.Connection, data []byte) ([]byte, error) {
	h.logger.Debug("Echo handler",
		zap.String("conn_id", conn.ID),
		zap.Int("bytes", len(data)))
	return data, nil
}
