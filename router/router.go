// Package router provides packet routing functionality for client and server.
// PacketHandler uses peer as the second argument: empty string for client, client address for server.
package router

import (
	"errors"
	"fmt"
	"sync"

	"github.com/auraspeak/protocol"
)

// PacketHandler is a function type that handles incoming packets.
// peer is the remote peer identifier (empty for client, client address for server).
type PacketHandler func(packet *protocol.Packet, peer string) error

// Router routes incoming packets to their registered handlers based on packet type.
type Router struct {
	handlers sync.Map // packetType -> PacketHandler
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		handlers: sync.Map{},
	}
}

// OnPacket registers a new PacketHandler for a specific packet type.
func (r *Router) OnPacket(packetType protocol.PacketType, handler PacketHandler) {
	r.handlers.Store(packetType, handler)
}

// HandlePacket routes a packet to the appropriate handler.
func (r *Router) HandlePacket(packet *protocol.Packet, peer string) error {
	if !protocol.IsValidPacketType(packet.PacketHeader.PacketType) {
		return errors.New("invalid packet type")
	}
	handler, ok := r.handlers.Load(packet.PacketHeader.PacketType)
	if !ok {
		strPacketType, exists := protocol.PacketTypeMapType[packet.PacketHeader.PacketType]
		if !exists {
			strPacketType = fmt.Sprintf("Unknown(0x%02X)", packet.PacketHeader.PacketType)
		}
		return fmt.Errorf("no handler found for packet type: %s", strPacketType)
	}
	handlerFunc := handler.(PacketHandler)
	return handlerFunc(packet, peer)
}

// ListRoutes prints all registered packet type routes to stdout.
func (r *Router) ListRoutes() {
	r.handlers.Range(func(key, value any) bool {
		strPacketType, exists := protocol.PacketTypeMapType[key.(protocol.PacketType)]
		if !exists {
			strPacketType = fmt.Sprintf("Unknown(0x%02X)", key.(protocol.PacketType))
		}
		fmt.Printf("Packet type: %s\n", strPacketType)
		return true
	})
}
