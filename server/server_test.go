package server

import (
	"net"
	"testing"
	"time"

	"github.com/auraspeak/network/internal/mock"
	"github.com/auraspeak/protocol"
	"github.com/pion/dtls/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalDTLSConfig returns a minimal DTLS config for tests (e.g. Run will still need a real listener).
func minimalDTLSConfig() *dtls.Config {
	return &dtls.Config{
		InsecureSkipVerify: true,
	}
}

func TestNewServer(t *testing.T) {
	cfg := ServerConfig{
		Port:        8080,
		DTLSConfig:  minimalDTLSConfig(),
		ConnBufSize: 8192,
	}
	s := NewServer(cfg)

	require.NotNil(t, s)
	assert.Equal(t, 8080, s.cfg.Port)
	assert.NotNil(t, s.router)
	assert.NotNil(t, s.ctx)
}

func TestOnPacket(t *testing.T) {
	cfg := ServerConfig{Port: 0, DTLSConfig: minimalDTLSConfig()}
	s := NewServer(cfg)

	handlerCalled := false
	handler := func(packet *protocol.Packet, peer string) error {
		handlerCalled = true
		assert.Equal(t, "127.0.0.1:8080", peer)
		return nil
	}

	s.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	err := s.router.HandlePacket(packet, "127.0.0.1:8080")
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestBroadcast_NoConnections(t *testing.T) {
	cfg := ServerConfig{Port: 0, DTLSConfig: minimalDTLSConfig()}
	s := NewServer(cfg)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	s.Broadcast(packet)
}

func TestStop_NoListener(t *testing.T) {
	cfg := ServerConfig{Port: 0, DTLSConfig: minimalDTLSConfig()}
	s := NewServer(cfg)

	s.Stop()

	assert.Equal(t, int32(1), s.shouldStop)
}

func TestBroadcast_OneConnection(t *testing.T) {
	cfg := ServerConfig{Port: 0, DTLSConfig: minimalDTLSConfig(), ConnBufSize: 1024}
	s := NewServer(cfg)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	s.conns.Store(remoteAddr.String(), conn)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	s.Broadcast(packet)

	time.Sleep(50 * time.Millisecond)

	written := conn.GetWriteData()
	assert.NotEmpty(t, written)
	assert.Equal(t, packet.Encode(), written)
}

func TestStop_SendsDisconnectAndClosesConns(t *testing.T) {
	cfg := ServerConfig{Port: 0, DTLSConfig: minimalDTLSConfig()}
	s := NewServer(cfg)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	s.conns.Store(remoteAddr.String(), conn)

	s.Stop()

	time.Sleep(50 * time.Millisecond)

	written := conn.GetWriteData()
	assert.NotEmpty(t, written)
	decoded, err := protocol.Decode(written)
	require.NoError(t, err)
	assert.Equal(t, protocol.PacketTypeClientNeedsDisconnect, decoded.PacketHeader.PacketType)
	assert.True(t, conn.WasCloseCalled())
}
