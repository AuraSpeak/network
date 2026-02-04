package readloop

import (
	"net"
	"testing"

	"github.com/auraspeak/network/internal/mock"
	"github.com/auraspeak/network/router"
	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

func init() {
	// Reduce log output during fuzzing so the fuzzer is not slowed and logs are not flooded.
	log.SetLevel(log.PanicLevel)
}

// FuzzRunReadLoop fuzzes the full path Read -> Decode -> HandlePacket (and optional TraceFunc)
// using a mock connection. Arbitrary byte slices must not cause a panic.
func FuzzRunReadLoop(f *testing.F) {
	f.Add([]byte{0x90})
	f.Add([]byte{0x90, 0x00})
	f.Add([]byte{0x01})
	f.Add([]byte{0x91, 0x01, 0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
		remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
		conn := mock.NewConn(localAddr, remoteAddr)
		conn.SetReadData(data)

		r := router.NewRouter()
		for _, m := range protocol.PacketTypeMap {
			if protocol.IsValidPacketType(m.PacketType) {
				r.OnPacket(m.PacketType, func(*protocol.Packet, string) error { return nil })
			}
		}

		traceFunc := func(_, _, _ string, _ []byte) {}
		RunReadLoop(conn, r, "peer", 1024, traceFunc)
	})
}
