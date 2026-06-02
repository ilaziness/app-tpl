package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWireHTTP_skipsHTTPServerWhenDisabled(t *testing.T) {
	cfg := testSQLiteConfig(t)
	cfg.HTTP.Enabled = false

	app, err := New(cfg)
	require.NoError(t, err)
	require.Nil(t, app.httpServer)

	require.NoError(t, app.shutdown())
}

func TestWireHTTP_buildsHTTPServerWhenEnabled(t *testing.T) {
	cfg := testSQLiteConfig(t)
	cfg.HTTP.Enabled = true
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = 18080
	cfg.Metrics.Enabled = false
	cfg.Tracing.Enabled = false
	cfg.JWT.Secret = ""

	app, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, app.httpServer)
	require.True(t, app.httpServer.Enabled())
	require.Equal(t, "127.0.0.1:18080", app.httpServer.Addr())

	require.NoError(t, app.shutdown())
}

func TestWireTCP_buildsServerWhenEnabled(t *testing.T) {
	cfg := testSQLiteConfig(t)
	cfg.TCP.Enabled = true
	cfg.TCP.Host = "127.0.0.1"
	cfg.TCP.Port = 19081

	app, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, app.tcpServer)
	require.True(t, app.tcpServer.Enabled())

	require.NoError(t, app.shutdown())
}

func TestWireUDP_buildsServerWhenEnabled(t *testing.T) {
	cfg := testSQLiteConfig(t)
	cfg.UDP.Enabled = true
	cfg.UDP.Host = "127.0.0.1"
	cfg.UDP.Port = 19082

	app, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, app.udpServer)
	require.True(t, app.udpServer.Enabled())

	require.NoError(t, app.shutdown())
}
