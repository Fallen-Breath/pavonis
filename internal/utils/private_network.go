package utils

import (
	"net/netip"
	"strings"
)

var privateNetworkAddrCheckers = []func(netip.Addr) bool{
	netip.Addr.IsLoopback,
	netip.Addr.IsPrivate,
	netip.Addr.IsLinkLocalUnicast, // includes cloud metadata services, e.g. 169.254.169.254
	netip.Addr.IsUnspecified,
	netip.MustParsePrefix("0.0.0.0/8").Contains,     // "this network"
	netip.MustParsePrefix("100.64.0.0/10").Contains, // carrier-grade NAT
}

func IsPrivateNetworkAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, checker := range privateNetworkAddrCheckers {
		if checker(addr) {
			return true
		}
	}
	return false
}

// IsPrivateNetworkHost only checks IP literals and localhost names, no DNS resolution is done
func IsPrivateNetworkHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && IsPrivateNetworkAddr(addr)
}
