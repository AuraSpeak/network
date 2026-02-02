// Package mock provides a mock net.Conn for testing network/router, network/readloop, network/client, and network/server.
package mock

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Conn is a mock implementation of net.Conn for testing.
type Conn struct {
	readData    []byte
	readErr     error
	writeData   []byte
	writeErr    error
	closeErr    error
	localAddr   net.Addr
	remoteAddr  net.Addr
	closed      bool
	mu          sync.RWMutex
	readCalled  bool
	writeCalled bool
	closeCalled bool
}

// NewConn creates a new mock connection.
func NewConn(localAddr, remoteAddr net.Addr) *Conn {
	return &Conn{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}

// Read implements net.Conn.
func (m *Conn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalled = true

	if m.closed {
		return 0, io.EOF
	}

	if m.readErr != nil {
		return 0, m.readErr
	}

	if len(m.readData) == 0 {
		return 0, io.EOF
	}

	n = copy(b, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

// SetReadDataLoop sets data that will be returned repeatedly.
func (m *Conn) SetReadDataLoop(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readData = data
}

// Write implements net.Conn.
func (m *Conn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalled = true

	if m.closed {
		return 0, errors.New("connection closed")
	}

	if m.writeErr != nil {
		return 0, m.writeErr
	}

	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

// Close implements net.Conn.
func (m *Conn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	m.closed = true
	return m.closeErr
}

// LocalAddr implements net.Conn.
func (m *Conn) LocalAddr() net.Addr {
	return m.localAddr
}

// RemoteAddr implements net.Conn.
func (m *Conn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

// SetDeadline implements net.Conn.
func (m *Conn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn.
func (m *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn.
func (m *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

// SetReadData sets the data to be returned by Read.
func (m *Conn) SetReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readData = data
}

// SetReadError sets the error to be returned by Read.
func (m *Conn) SetReadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readErr = err
}

// SetWriteError sets the error to be returned by Write.
func (m *Conn) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

// GetWriteData returns the data written to the connection.
func (m *Conn) GetWriteData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]byte, len(m.writeData))
	copy(result, m.writeData)
	return result
}

// IsClosed returns whether the connection is closed.
func (m *Conn) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// WasReadCalled returns whether Read was called.
func (m *Conn) WasReadCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readCalled
}

// WasWriteCalled returns whether Write was called.
func (m *Conn) WasWriteCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writeCalled
}

// WasCloseCalled returns whether Close was called.
func (m *Conn) WasCloseCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closeCalled
}
