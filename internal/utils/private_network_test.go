package utils

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsPrivateNetworkAddr(t *testing.T) {
	private := []string{
		"127.0.0.1", "::1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "fd00::1",
		"169.254.169.254", "fe80::1", "0.0.0.0", "::", "100.64.0.1", "::ffff:127.0.0.1",
	}
	public := []string{"1.1.1.1", "8.8.8.8", "2606:4700::1111", "100.128.0.1", "::ffff:1.1.1.1"}
	for _, addr := range private {
		require.True(t, IsPrivateNetworkAddr(netip.MustParseAddr(addr)), addr)
	}
	for _, addr := range public {
		require.False(t, IsPrivateNetworkAddr(netip.MustParseAddr(addr)), addr)
	}
}

func TestIsPrivateNetworkHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost", "LocalHost.", "a.localhost"} {
		require.True(t, IsPrivateNetworkHost(host), host)
	}
	for _, host := range []string{"1.1.1.1", "example.com", "localhost.example.com"} {
		require.False(t, IsPrivateNetworkHost(host), host)
	}
}
