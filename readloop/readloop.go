// Package readloop provides a shared read loop that reads packets from a connection,
// decodes them with the protocol, and dispatches to the router.
package readloop

import (
	"io"
	"net"

	"github.com/auraspeak/network/router"
	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

const defaultBufSize = 8192

// TraceFunc is called for each packet received when non-nil. dir is "in", local and remote are address strings.
type TraceFunc func(local, remote, dir string, payload []byte)

// RunReadLoop reads packets from conn, decodes with protocol.Decode, and dispatches via router.
// peer is passed to handlers (e.g. conn.RemoteAddr().String() on server, "" on client).
// If traceFunc is non-nil, it is called after decode for each packet (dir "in").
// On read error it returns; caller is responsible for closing conn.
func RunReadLoop(conn net.Conn, r *router.Router, peer string, bufSize int, traceFunc TraceFunc) {
	if bufSize <= 0 {
		bufSize = defaultBufSize
	}
	buf := make([]byte, bufSize)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.WithError(err).WithField("peer", peer).Error("read loop error")
			}
			return
		}
		if n == 0 {
			continue
		}
		b := make([]byte, n)
		copy(b, buf[:n])
		packet, err := protocol.Decode(b)
		if err != nil {
			log.WithError(err).WithField("peer", peer).Error("decode packet")
			continue
		}
		if traceFunc != nil {
			local := ""
			if conn.LocalAddr() != nil {
				local = conn.LocalAddr().String()
			}
			traceFunc(local, peer, "in", packet.Payload)
		}
		if err := r.HandlePacket(packet, peer); err != nil {
			log.WithError(err).WithField("peer", peer).Error("handle packet")
		}
	}
}
