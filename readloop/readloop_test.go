package readloop

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/auraspeak/network/internal/mock"
	"github.com/auraspeak/network/router"
	"github.com/auraspeak/protocol"
	"github.com/stretchr/testify/assert"
)

func TestRunReadLoop_NormalPacket(t *testing.T) {
	r := router.NewRouter()
	handlerCalled := make(chan bool, 1)
	handler := func(packet *protocol.Packet, peer string) error {
		handlerCalled <- true
		assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)
		assert.Equal(t, []byte("test"), packet.Payload)
		assert.Equal(t, "127.0.0.1:8081", peer)
		return nil
	}
	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	conn.SetReadData(packet.Encode())

	RunReadLoop(conn, r, "127.0.0.1:8081", 1024, nil)

	select {
	case <-handlerCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler was not called within timeout")
	}
}

func TestRunReadLoop_ReadError(t *testing.T) {
	r := router.NewRouter()
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	conn.SetReadError(io.EOF)

	RunReadLoop(conn, r, "127.0.0.1:8081", 1024, nil)

	// RunReadLoop should return (not crash)
	assert.True(t, conn.WasReadCalled())
}

func TestRunReadLoop_EmptyRead(t *testing.T) {
	r := router.NewRouter()
	handlerCalled := false
	r.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet, peer string) error {
		handlerCalled = true
		return nil
	})

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	conn.SetReadData([]byte{})

	RunReadLoop(conn, r, "", 1024, nil)

	assert.False(t, handlerCalled)
}

func TestRunReadLoop_DecodeError(t *testing.T) {
	r := router.NewRouter()
	handlerCalled := false
	r.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet, peer string) error {
		handlerCalled = true
		return nil
	})

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	// Invalid packet data (too short)
	conn.SetReadData([]byte{0x01})

	RunReadLoop(conn, r, "", 1024, nil)

	assert.False(t, handlerCalled)
}

func TestRunReadLoop_HandlerError(t *testing.T) {
	r := router.NewRouter()
	handlerCalled := make(chan bool, 1)
	r.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet, peer string) error {
		handlerCalled <- true
		return errors.New("handler error")
	})

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	conn.SetReadData(packet.Encode())

	RunReadLoop(conn, r, "", 1024, nil)

	select {
	case <-handlerCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler was not called within timeout")
	}
}

func TestRunReadLoop_EmptyPeer(t *testing.T) {
	r := router.NewRouter()
	peerSeen := make(chan string, 1)
	r.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet, peer string) error {
		peerSeen <- peer
		return nil
	})

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	conn := mock.NewConn(localAddr, remoteAddr)
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	conn.SetReadData(packet.Encode())

	RunReadLoop(conn, r, "", 1024, nil)

	select {
	case p := <-peerSeen:
		assert.Equal(t, "", p)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler was not called within timeout")
	}
}
