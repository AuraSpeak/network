// Package client provides a DTLS-based UDP client that connects to a server and routes packets.
package client

import (
	"context"
	"errors"
	"fmt"
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

const defaultRecvBufSize = 1024
const sendTimeout = 5 * time.Second

// ClientConfig configures the client.
type ClientConfig struct {
	Host               string
	Port               int
	InsecureSkipVerify bool
	RecvBufSize        int    // default defaultRecvBufSize if <= 0
	OnConnect          func() // optional, called once after connection is established (before read loop)
}

// Client is a DTLS-based UDP client that handles connection and packet routing.
type Client struct {
	cfg ClientConfig

	conn     net.Conn
	testConn net.Conn // set by SetConnForTest for tests

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex // protects ctx, cancel for use from Send and Run
	wg     sync.WaitGroup

	sendCh chan []byte
	errCh  chan error

	router *router.Router

	running atomic.Bool
}

// NewClient creates a new client with the given config.
func NewClient(cfg ClientConfig) *Client {
	if cfg.RecvBufSize <= 0 {
		cfg.RecvBufSize = defaultRecvBufSize
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		cfg:    cfg,
		sendCh: make(chan []byte),
		errCh:  make(chan error),
		router: router.NewRouter(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// OnPacket registers a handler for a packet type.
func (c *Client) OnPacket(packetType protocol.PacketType, handler router.PacketHandler) {
	c.router.OnPacket(packetType, handler)
}

// Run connects to the server and runs send/recv loops until Stop or context is done.
// If SetConnForTest was called, uses that connection instead of dialing.
func (c *Client) Run(ctx context.Context) error {
	if ctx != nil {
		c.mu.Lock()
		c.ctx, c.cancel = context.WithCancel(ctx)
		c.mu.Unlock()
	}
	var err error
	if c.testConn != nil {
		c.conn = c.testConn
		c.testConn = nil
	} else {
		addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
		raddr, errResolve := net.ResolveUDPAddr("udp4", addr)
		if errResolve != nil {
			return errResolve
		}
		c.conn, err = dtls.Dial("udp", raddr, &dtls.Config{InsecureSkipVerify: c.cfg.InsecureSkipVerify})
		if err != nil {
			return err
		}
	}
	defer c.conn.Close()

	c.running.Store(true)
	defer c.running.Store(false)

	if c.cfg.OnConnect != nil {
		c.cfg.OnConnect()
	}

	c.wg.Go(func() {
		readloop.RunReadLoop(c.conn, c.router, "", c.cfg.RecvBufSize, nil)
	})

	c.wg.Go(func() {
		c.sendLoop()
	})

	c.wg.Go(func() {
		c.handleErrors()
	})

	log.WithField("caller", "network/client").Info("Starting client")
	c.wg.Wait()
	log.WithField("caller", "network/client").Info("Client stopped")
	return nil
}

// Stop stops the client and closes the connection.
func (c *Client) Stop() {
	c.running.Store(false)
	c.mu.Lock()
	c.cancel()
	c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.wg.Wait()
	log.WithField("caller", "network/client").Info("Client stopped")
}

// Send sends raw bytes to the server.
func (c *Client) Send(msg []byte) error {
	c.mu.RLock()
	done := c.ctx.Done()
	c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	select {
	case <-done:
		return errors.New("context cancelled")
	case <-ctx.Done():
		return errors.New("send timeout: sendLoop may not be running or is blocked")
	case c.sendCh <- msg:
		return nil
	}
}

// SetConnForTest injects a connection for testing so Run uses it instead of dialing.
// Must be called before Run.
func (c *Client) SetConnForTest(conn net.Conn) {
	c.testConn = conn
}

// HandlePacketForTest invokes the router for the given packet and peer (for tests).
func (c *Client) HandlePacketForTest(packet *protocol.Packet, peer string) error {
	return c.router.HandlePacket(packet, peer)
}

// SetContextForTest sets the context used by Send and the loops (for tests).
func (c *Client) SetContextForTest(ctx context.Context) {
	c.mu.Lock()
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()
}

func (c *Client) sendLoop() {
	for {
		c.mu.RLock()
		done := c.ctx.Done()
		c.mu.RUnlock()
		select {
		case <-done:
			return
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			if c.conn == nil {
				continue
			}
			if _, err := c.conn.Write(msg); err != nil {
				select {
				case c.errCh <- err:
				default:
				}
				continue
			}
		}
	}
}

func (c *Client) handleErrors() {
	for {
		c.mu.RLock()
		done := c.ctx.Done()
		c.mu.RUnlock()
		select {
		case <-done:
			return
		case err := <-c.errCh:
			log.WithField("caller", "network/client").WithError(err).Error("Error in client")
		}
	}
}
