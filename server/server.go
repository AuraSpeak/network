// Package server provides a DTLS-based UDP server that accepts connections and routes packets.
package server

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/auraspeak/network/readloop"
	"github.com/auraspeak/network/router"
	"github.com/auraspeak/protocol"
	"github.com/pion/dtls/v3"
	log "github.com/sirupsen/logrus"
)

const defaultConnBufSize = 8192
const handshakeTimeout = 30 * time.Second

// TraceFunc is called for each packet sent or received when set in ServerConfig.
// dir is "in" or "out", local and remote are address strings.
type TraceFunc func(local, remote, dir string, payload []byte)

// ServerConfig configures the server.
type ServerConfig struct {
	Port        int
	DTLSConfig  *dtls.Config
	ConnBufSize int       // default defaultConnBufSize if <= 0
	TraceFunc   TraceFunc // optional
}

// Server is a DTLS-based UDP server that accepts connections and routes packets.
type Server struct {
	cfg ServerConfig

	ln    net.Listener
	conns sync.Map // peer string -> net.Conn

	ctx        context.Context
	shouldStop int32

	router *router.Router
}

// NewServer creates a new server with the given config.
func NewServer(cfg ServerConfig) *Server {
	if cfg.ConnBufSize <= 0 {
		cfg.ConnBufSize = defaultConnBufSize
	}
	return &Server{
		cfg:    cfg,
		router: router.NewRouter(),
		ctx:    context.Background(),
	}
}

// OnPacket registers a handler for a packet type.
func (s *Server) OnPacket(packetType protocol.PacketType, handler router.PacketHandler) {
	s.router.OnPacket(packetType, handler)
}

// Run listens for connections and runs until context is done or Stop is called.
func (s *Server) Run(ctx context.Context) error {
	if ctx != nil {
		s.ctx = ctx
	}
	s.router.ListRoutes()
	addr := &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: s.cfg.Port,
	}
	ln, err := dtls.Listen("udp", addr, s.cfg.DTLSConfig)
	if err != nil {
		return err
	}
	s.ln = ln
	defer func() {
		if err := s.ln.Close(); err != nil {
			log.WithField("caller", "network/server").WithError(err).Warn("close listener")
		}
	}()

	log.WithField("caller", "network/server").Infof("Server started on port %d", s.cfg.Port)

	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}
		if atomic.LoadInt32(&s.shouldStop) == 1 {
			return nil
		}
		conn, err := s.ln.Accept()
		if err != nil {
			log.WithField("caller", "network/server").WithError(err).Error("Accept error")
			continue
		}
		handshakeCtx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
		dtlsConn, ok := conn.(*dtls.Conn)
		if ok {
			err = dtlsConn.HandshakeContext(handshakeCtx)
		}
		cancel()
		if !ok {
			log.WithField("caller", "network/server").Error("Connection is not DTLS")
			if err := conn.Close(); err != nil {
				log.WithField("caller", "network/server").WithError(err).Warn("close connection after handshake failure")
			}
			continue
		}
		if err != nil {
			if err := conn.Close(); err != nil {
				log.WithField("caller", "network/server").WithError(err).Warn("close connection after handshake failure")
			}
			continue
		}
		peer := conn.RemoteAddr().String()
		s.conns.Store(peer, conn)
		go func(c net.Conn, peer string) {
			var tf readloop.TraceFunc
			if s.cfg.TraceFunc != nil {
				tf = func(local, remote, dir string, payload []byte) {
					s.cfg.TraceFunc(local, remote, dir, payload)
				}
			}
			readloop.RunReadLoop(c, s.router, peer, s.cfg.ConnBufSize, tf)
			s.connUnregister(c)
		}(conn, peer)
	}
}

// Stop stops the server: sends ClientNeedsDisconnect to all clients, then closes all connections and the listener.
func (s *Server) Stop() {
	atomic.StoreInt32(&s.shouldStop, 1)
	if s.ln != nil {
		if err := s.ln.Close(); err != nil {
			log.WithField("caller", "network/server").WithError(err).Warn("close listener")
		}
		s.ln = nil
	}
	// Send disconnect packet then close all conns
	hdr := protocol.Header{PacketType: protocol.PacketTypeClientNeedsDisconnect}
	pkg := &protocol.Packet{PacketHeader: hdr, Payload: []byte("Server Stops")}
	s.conns.Range(func(key, value any) bool {
		conn, ok := value.(net.Conn)
		if !ok {
			return true
		}
		_, _ = conn.Write(pkg.Encode())
		if s.cfg.TraceFunc != nil {
			s.cfg.TraceFunc(conn.LocalAddr().String(), conn.RemoteAddr().String(), "out", pkg.Payload)
		}
		if err := conn.Close(); err != nil {
			log.WithField("caller", "network/server").WithField("peer", key).WithError(err).Warn("close connection")
		}
		s.conns.Delete(key)
		return true
	})
	log.WithField("caller", "network/server").Info("Server stopped")
}

// Broadcast sends a packet to all connected clients.
func (s *Server) Broadcast(packet *protocol.Packet) {
	data := packet.Encode()
	s.conns.Range(func(key, value any) bool {
		conn, ok := value.(net.Conn)
		if !ok {
			return true
		}
		if _, err := conn.Write(data); err != nil {
			s.conns.Delete(key)
			return true
		}
		if s.cfg.TraceFunc != nil {
			s.cfg.TraceFunc(conn.LocalAddr().String(), conn.RemoteAddr().String(), "out", packet.Payload)
		}
		return true
	})
}

func (s *Server) connUnregister(conn net.Conn) {
	peer := conn.RemoteAddr().String()
	s.conns.Delete(peer)
	if err := conn.Close(); err != nil {
		log.WithField("caller", "network/server").WithField("peer", peer).WithError(err).Warn("close connection")
	}
}

// HandlePacketForTest invokes the router for the given packet and peer (for tests).
func (s *Server) HandlePacketForTest(packet *protocol.Packet, peer string) error {
	return s.router.HandlePacket(packet, peer)
}
