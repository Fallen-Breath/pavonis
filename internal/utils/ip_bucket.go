package utils

import "net"

func GetBucketForIp(ip net.IP) string {
	if ip == nil {
		return "unknown$"
	}
	if ip.To4() != nil {
		return "ipv4$" + ip.String()
	}
	ipBytes := ip.To16()
	if ipBytes == nil {
		return "unknown$"
	}
	// by its /64 address
	subnetIP := ip.Mask(net.CIDRMask(64, 128))
	if subnetIP == nil {
		return "unknown$"
	}
	return "ipv6$" + subnetIP.String()
}

func GetBucketForIpString(ipStr string) string {
	return GetBucketForIp(net.ParseIP(ipStr))
}
