package router

import (
	"testing"

	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

func init() {
	// Reduce log output during fuzzing so the fuzzer is not slowed and logs are not flooded.
	log.SetLevel(log.PanicLevel)
}

// FuzzHandlePacket fuzzes HandlePacket with arbitrary byte slices.
// Data is decoded via protocol.Decode; valid packets are routed through a router
// with no-op handlers for all valid packet types. Invalid inputs must not panic.
func FuzzHandlePacket(f *testing.F) {
	// Seed corpus: valid packets, invalid type, empty/short, longer payloads.
	f.Add([]byte{})
	f.Add([]byte{0xFF})
	f.Add([]byte{0x90})
	f.Add([]byte{0x90, 0x00})
	f.Add([]byte{0x01})
	f.Add([]byte{0x91, 0x01, 0x02})
	f.Add(append([]byte{0x90}, make([]byte, 100)...))
	f.Add([]byte{0x91, 0x01, 0x02, 0x03})
	f.Fuzz(func(t *testing.T, data []byte) {
		packet, err := protocol.Decode(data)
		if err != nil {
			return
		}
		r := NewRouter()
		for _, m := range protocol.PacketTypeMap {
			if protocol.IsValidPacketType(m.PacketType) {
				r.OnPacket(m.PacketType, func(*protocol.Packet, string) error { return nil })
			}
		}
		_ = r.HandlePacket(packet, "")
	})
}

// FuzzHandlePacketNoHandler fuzzes HandlePacket with a router that has no handlers.
// Valid decoded packets must cause "no handler found" or "invalid packet type" without panicking.
func FuzzHandlePacketNoHandler(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xFF})
	f.Add([]byte{0x90})
	f.Add([]byte{0x90, 0x00})
	f.Add([]byte{0x01})
	f.Add([]byte{0x91, 0x01, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		packet, err := protocol.Decode(data)
		if err != nil {
			return
		}
		r := NewRouter()
		_ = r.HandlePacket(packet, "")
	})
}
