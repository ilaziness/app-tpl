package udp

import (
	"testing"

	"github.com/ilaziness/app-tpl/internal/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type testUDPRouteHandler struct {
	msgType string
	resp    []byte
}

func (h *testUDPRouteHandler) MessageType() string { return h.msgType }

func (h *testUDPRouteHandler) Handle(_ *types.UDPPacket) ([]byte, error) {
	return h.resp, nil
}

func TestNewRouterAutoRegistersHandlers(t *testing.T) {
	logger := zap.NewNop()
	time := &testUDPRouteHandler{msgType: "time", resp: []byte(`{"type":"time"}`)}
	stats := &testUDPRouteHandler{msgType: "stats", resp: []byte(`{"type":"stats"}`)}

	router := NewRouter(logger, time, stats)

	resp, err := router.Handle(&types.UDPPacket{Data: []byte(`{"type":"time"}`)})
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"time"}`, string(resp))

	resp, err = router.Handle(&types.UDPPacket{Data: []byte(`{"type":"unknown"}`)})
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func TestRouterDefaultHandler(t *testing.T) {
	logger := zap.NewNop()
	handler := &testUDPRouteHandler{msgType: "echo", resp: []byte("echoed")}
	router := NewRouter(logger).Default(handler)

	resp, err := router.Handle(&types.UDPPacket{Data: []byte(`{"type":"anything"}`)})
	assert.NoError(t, err)
	assert.Equal(t, "echoed", string(resp))
}

func TestRouterRegisterType(t *testing.T) {
	logger := zap.NewNop()
	handler := &testUDPRouteHandler{msgType: "custom", resp: []byte("ok")}
	router := NewRouter(logger).RegisterType("custom", handler)

	resp, err := router.Handle(&types.UDPPacket{Data: []byte(`{"type":"custom"}`)})
	assert.NoError(t, err)
	assert.Equal(t, "ok", string(resp))
}
