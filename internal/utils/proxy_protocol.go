package utils

import (
	"net"
	"net/netip"

	proxyproto "github.com/pires/go-proxyproto"
)

func NewProxyProtocolHeader(version byte, source, destination netip.AddrPort) *proxyproto.Header {
	source = netip.AddrPortFrom(source.Addr().Unmap().WithZone(""), source.Port())
	if destination.IsValid() {
		destination = netip.AddrPortFrom(destination.Addr().Unmap().WithZone(""), destination.Port())
	}
	if !destination.IsValid() || source.Addr().Is4() != destination.Addr().Is4() {
		addr := netip.IPv6Unspecified()
		if source.Addr().Is4() {
			addr = netip.IPv4Unspecified()
		}
		destination = netip.AddrPortFrom(addr, 0)
	}
	return proxyproto.HeaderProxyFromAddrs(version, net.TCPAddrFromAddrPort(source), net.TCPAddrFromAddrPort(destination))
}
