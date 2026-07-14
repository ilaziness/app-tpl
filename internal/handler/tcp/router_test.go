package tcp

import (
	"encoding/json"
	"testing"

	"github.com/ilaziness/app-tpl/internal/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type testTCPRouteHandler struct {
	msgType string
	resp    []byte
}

func (h *testTCPRouteHandler) MessageType() string { return h.msgType }

func (h *testTCPRouteHandler) Handle(_ *types.Connection, data []byte) ([]byte, error) {
	if h.resp != nil {
		return h.resp, nil
	}
	return data, nil
}

func TestNewRouterAutoRegistersHandlers(t *testing.T) {
	logger := zap.NewNop()
	echo := &testTCPRouteHandler{msgType: "echo", resp: []byte("echoed")}
	chat := &testMultiTCPRouteHandler{msgTypes: []string{"join", "message", "leave"}, resp: []byte("chat")}

	router := NewRouter(logger, echo, chat)

	conn := &types.Connection{ID: "1"}

	resp, err := router.Handle(conn, []byte(`{"type":"echo"}`))
	assert.NoError(t, err)
	assert.Equal(t, "echoed", string(resp))

	resp, err = router.Handle(conn, []byte(`{"type":"join"}`))
	assert.NoError(t, err)
	assert.Equal(t, "chat", string(resp))

	resp, err = router.Handle(conn, []byte(`{"type":"unknown"}`))
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func TestRouterDefaultHandler(t *testing.T) {
	logger := zap.NewNop()
	echo := &testTCPRouteHandler{msgType: "echo", resp: []byte("echoed")}

	router := NewRouter(logger).Default(echo)

	conn := &types.Connection{ID: "1"}
	resp, err := router.Handle(conn, []byte(`{"type":"anything"}`))
	assert.NoError(t, err)
	assert.Equal(t, "echoed", string(resp))
}

func TestRouterRegisterType(t *testing.T) {
	logger := zap.NewNop()
	handler := &testTCPRouteHandler{msgType: "custom", resp: []byte("ok")}
	router := NewRouter(logger).RegisterType("custom", handler)

	conn := &types.Connection{ID: "1"}
	resp, err := router.Handle(conn, []byte(`{"type":"custom"}`))
	assert.NoError(t, err)
	assert.Equal(t, "ok", string(resp))
}

func TestRouterHandleInvalidJSON(t *testing.T) {
	logger := zap.NewNop()
	router := NewRouter(logger)

	conn := &types.Connection{ID: "1"}
	_, err := router.Handle(conn, []byte(`not json`))
	assert.Error(t, err)
}

type testMultiTCPRouteHandler struct {
	msgTypes []string
	resp     []byte
}

func (h *testMultiTCPRouteHandler) MessageTypes() []string { return h.msgTypes }

func (h *testMultiTCPRouteHandler) Handle(_ *types.Connection, data []byte) ([]byte, error) {
	// Return a stable copy to avoid slice aliasing in tests.
	var probe struct{ Type string }
	_ = json.Unmarshal(data, &probe) //nolint:errcheck // test only
	return h.resp, nil
}
