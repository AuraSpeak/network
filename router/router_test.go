package router

import (
	"errors"
	"sync"
	"testing"

	"github.com/auraspeak/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	r := NewRouter()
	require.NotNil(t, r)
	r.handlers.Store(protocol.PacketTypeDebugHello, func(*protocol.Packet, string) error { return nil })
	_, ok := r.handlers.Load(protocol.PacketTypeDebugHello)
	assert.True(t, ok)
}

func TestOnPacket(t *testing.T) {
	r := NewRouter()
	handler := func(packet *protocol.Packet, peer string) error {
		return nil
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)
	handlerFromMap, ok := r.handlers.Load(protocol.PacketTypeDebugHello)
	require.True(t, ok)
	assert.NotNil(t, handlerFromMap)

	handler2 := func(packet *protocol.Packet, peer string) error {
		return nil
	}
	r.OnPacket(protocol.PacketTypeDebugHello, handler2)
	handlerFromMap2, ok2 := r.handlers.Load(protocol.PacketTypeDebugHello)
	require.True(t, ok2)
	assert.NotNil(t, handlerFromMap2)
}

func TestHandlePacket_Success(t *testing.T) {
	r := NewRouter()
	called := false
	peer := "127.0.0.1:8080"
	handler := func(packet *protocol.Packet, p string) error {
		called = true
		assert.Equal(t, "127.0.0.1:8080", p)
		return nil
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	err := r.HandlePacket(packet, peer)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestHandlePacket_Success_EmptyPeer(t *testing.T) {
	r := NewRouter()
	called := false
	handler := func(packet *protocol.Packet, peer string) error {
		called = true
		assert.Equal(t, "", peer)
		return nil
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	err := r.HandlePacket(packet, "")
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestHandlePacket_HandlerError(t *testing.T) {
	r := NewRouter()
	expectedErr := errors.New("handler error")
	handler := func(packet *protocol.Packet, peer string) error {
		return expectedErr
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	err := r.HandlePacket(packet, "127.0.0.1:8080")
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestHandlePacket_InvalidPacketType(t *testing.T) {
	r := NewRouter()
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeNone},
		Payload:      []byte("test"),
	}

	err := r.HandlePacket(packet, "127.0.0.1:8080")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid packet type")
}

func TestHandlePacket_NoHandler(t *testing.T) {
	r := NewRouter()
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugAny},
		Payload:      []byte("test"),
	}

	err := r.HandlePacket(packet, "127.0.0.1:8080")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler found")
}

func TestHandlePacket_NilPacket(t *testing.T) {
	r := NewRouter()
	handler := func(packet *protocol.Packet, peer string) error {
		return nil
	}
	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	assert.Panics(t, func() {
		_ = r.HandlePacket(nil, "127.0.0.1:8080")
	})
}

func TestListRoutes_Empty(t *testing.T) {
	r := NewRouter()
	r.ListRoutes()
}

func TestListRoutes_Multiple(t *testing.T) {
	r := NewRouter()
	handler := func(packet *protocol.Packet, peer string) error {
		return nil
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)
	r.OnPacket(protocol.PacketTypeDebugAny, handler)

	r.ListRoutes()
}

func TestHandlePacket_ConcurrentRegistration(t *testing.T) {
	r := NewRouter()
	done := make(chan bool, 2)

	go func() {
		handler := func(packet *protocol.Packet, peer string) error {
			return nil
		}
		r.OnPacket(protocol.PacketTypeDebugHello, handler)
		done <- true
	}()

	go func() {
		handler := func(packet *protocol.Packet, peer string) error {
			return nil
		}
		r.OnPacket(protocol.PacketTypeDebugAny, handler)
		done <- true
	}()

	<-done
	<-done

	_, ok1 := r.handlers.Load(protocol.PacketTypeDebugHello)
	_, ok2 := r.handlers.Load(protocol.PacketTypeDebugAny)
	assert.True(t, ok1)
	assert.True(t, ok2)
}

func TestHandlePacket_HandlerPanic(t *testing.T) {
	r := NewRouter()
	handler := func(packet *protocol.Packet, peer string) error {
		panic("handler panic")
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	assert.Panics(t, func() {
		_ = r.HandlePacket(packet, "127.0.0.1:8080")
	})
}

func TestHandlePacket_ConcurrentHandling(t *testing.T) {
	r := NewRouter()
	callCount := 0
	var mu sync.Mutex
	handler := func(packet *protocol.Packet, peer string) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}

	r.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = r.HandlePacket(packet, "")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	mu.Lock()
	assert.Equal(t, 10, callCount)
	mu.Unlock()
}
