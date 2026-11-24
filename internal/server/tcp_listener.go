package server

import (
	"io"
	"net"
	"sync"
)

type limitedTCPListener struct {
	*net.TCPListener
	slots     chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

var _ net.Listener = (*limitedTCPListener)(nil)

func newLimitedTCPListener(listener *net.TCPListener, maxConnections int) *limitedTCPListener {
	return &limitedTCPListener{
		TCPListener: listener,
		slots:       make(chan struct{}, maxConnections),
		closed:      make(chan struct{}),
	}
}

func (l *limitedTCPListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	conn, err := l.AcceptTCP()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &limitedTCPConn{TCPConn: conn, slots: l.slots}, nil
}

func (l *limitedTCPListener) Close() error {
	err := net.ErrClosed
	l.closeOnce.Do(func() {
		err = l.TCPListener.Close()
		close(l.closed)
	})
	return err
}

// Embedding TCPConn preserves ReadFrom for file transfers and CloseWrite for HTTP/1 shutdown.
type limitedTCPConn struct {
	*net.TCPConn
	slots     <-chan struct{}
	closeOnce sync.Once
}

var _ net.Conn = (*limitedTCPConn)(nil)
var _ io.ReaderFrom = (*limitedTCPConn)(nil)
var _ interface{ CloseWrite() error } = (*limitedTCPConn)(nil)

func (c *limitedTCPConn) Close() error {
	err := net.ErrClosed
	c.closeOnce.Do(func() {
		err = c.TCPConn.Close()
		<-c.slots
	})
	return err
}
