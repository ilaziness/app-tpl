package app

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShutdown_publishesAppStoppedWhenStarted(t *testing.T) {
	cfg := testSQLiteConfig(t)
	app, err := New(cfg)
	require.NoError(t, err)

	app.appStarted = true
	app.startTime = time.Now().Add(-time.Second)

	require.NoError(t, app.shutdown())
}

func TestStartServers_stopsTCPWhenUDPStartFails(t *testing.T) {
	port := freeTCPPort(t)

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	udpConn, err := net.ListenUDP("udp", udpAddr)
	require.NoError(t, err)
	defer udpConn.Close()

	cfg := testSQLiteConfig(t)
	cfg.TCP.Enabled = true
	cfg.TCP.Host = "127.0.0.1"
	cfg.TCP.Port = port
	cfg.TCP.ShutdownTimeout = 5
	cfg.UDP.Enabled = true
	cfg.UDP.Host = "127.0.0.1"
	cfg.UDP.Port = port

	app, err := New(cfg)
	require.NoError(t, err)

	err = app.startServers(context.Background(), func() {})
	require.Error(t, err)
	require.Contains(t, err.Error(), "UDP")

	_, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	require.Error(t, dialErr)

	require.NoError(t, app.shutdown())
}

func TestStartServers_failsWhenTCPPortUnavailable(t *testing.T) {
	port := freeTCPPort(t)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer ln.Close()

	cfg := testSQLiteConfig(t)
	cfg.TCP.Enabled = true
	cfg.TCP.Host = "127.0.0.1"
	cfg.TCP.Port = port

	app, err := New(cfg)
	require.NoError(t, err)

	err = app.startServers(context.Background(), func() {})
	require.Error(t, err)
	require.Contains(t, err.Error(), "TCP")

	require.NoError(t, app.shutdown())
}

func TestAddrOf_formatsHostPort(t *testing.T) {
	require.Equal(t, "0.0.0.0:8080", addrOf("0.0.0.0", 8080))
}

func freeTCPPort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
