package server

import (
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func acceptAtConnectionLimit(t *testing.T) (*limitedTCPListener, net.Conn, <-chan error) {
	t.Helper()
	tcp, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	listener := newLimitedTCPListener(tcp, 1)
	t.Cleanup(func() { listener.Close() })

	// Queue both clients so the second Accept can only be waiting for capacity.
	for range 2 {
		client, err := net.Dial("tcp", listener.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })
	}
	first, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { first.Close() })

	result := make(chan error, 1)
	go func() {
		second, err := listener.Accept()
		if err == nil {
			second.Close()
		}
		result <- err
	}()
	synctest.Wait()
	select {
	case err := <-result:
		t.Fatalf("second Accept returned while first connection was open: %v", err)
	default:
	}
	return listener, first, result
}

func TestLimitedTCPListenerAcceptsAfterConnectionCloses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, first, result := acceptAtConnectionLimit(t)
		require.NoError(t, first.Close())
		select {
		case err := <-result:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("closing the connection did not unblock Accept")
		}
	})
}

func TestLimitedTCPListenerCloseWakesAcceptAtCapacity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener, _, result := acceptAtConnectionLimit(t)
		require.NoError(t, listener.Close())
		select {
		case err := <-result:
			require.ErrorIs(t, err, net.ErrClosed)
		case <-time.After(time.Second):
			t.Fatal("closing the listener did not unblock Accept")
		}
	})
}
