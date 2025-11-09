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
	subnetBytes := make([]byte, 16)
	copy(subnetBytes[:8], ipBytes[:8])
	subnetIP := net.IP(subnetBytes)
	return "ipv6$" + subnetIP.String()
}

func GetBucketForIpString(ipStr string) string {
	return GetBucketForIp(net.ParseIP(ipStr))
}
