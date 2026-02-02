package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/auraspeak/network/internal/mock"
	"github.com/auraspeak/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	require.NotNil(t, c)
	assert.Equal(t, "localhost", c.cfg.Host)
	assert.Equal(t, 8080, c.cfg.Port)
	assert.NotNil(t, c.sendCh)
	assert.NotNil(t, c.errCh)
	assert.NotNil(t, c.router)
	assert.NotNil(t, c.ctx)
	assert.False(t, c.running.Load())
}

func TestOnPacket(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)
	handlerCalled := false

	handler := func(packet *protocol.Packet, peer string) error {
		handlerCalled = true
		return nil
	}

	c.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	err := c.router.HandlePacket(packet, "")
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestSend_Success(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewConn(localAddr, remoteAddr)
	c.SetConnForTest(mockConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = c.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	msg := []byte("test message")
	err := c.Send(msg)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.Equal(t, msg, writtenData)

	cancel()
	c.Stop()
}

func TestSend_Timeout(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	// Don't start Run, so sendLoop is not running and Send will block then timeout
	msg := []byte("test message")
	err := c.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send timeout")
}

func TestSend_ContextCancelled(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.ctx = ctx
	c.cancel = func() {}

	msg := []byte("test message")
	err := c.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestStop_WithConnection(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewConn(localAddr, remoteAddr)
	c.SetConnForTest(mockConn)

	ctx := context.Background()
	go func() {
		_ = c.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	c.Stop()

	assert.False(t, c.running.Load())
	assert.True(t, mockConn.WasCloseCalled())
}

func TestStop_WithoutConnection(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	c.running.Store(true)
	c.Stop()

	assert.False(t, c.running.Load())
}

func TestStop_MultipleCalls(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewConn(localAddr, remoteAddr)
	c.SetConnForTest(mockConn)

	go func() {
		_ = c.Run(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)

	c.Stop()
	c.Stop()
	c.Stop()

	assert.False(t, c.running.Load())
}

func TestRun_WithContextCancel(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080}
	c := NewClient(cfg)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewConn(localAddr, remoteAddr)
	c.SetConnForTest(mockConn)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestRun_ReceivesPacket(t *testing.T) {
	cfg := ClientConfig{Host: "localhost", Port: 8080, RecvBufSize: 1024}
	c := NewClient(cfg)

	handlerCalled := make(chan bool, 1)
	c.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet, peer string) error {
		handlerCalled <- true
		assert.Equal(t, []byte("test"), packet.Payload)
		assert.Equal(t, "", peer)
		return nil
	})

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewConn(localAddr, remoteAddr)
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	mockConn.SetReadData(packet.Encode())
	c.SetConnForTest(mockConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = c.Run(ctx)
	}()

	select {
	case <-handlerCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler was not called")
	}

	cancel()
	c.Stop()
}
