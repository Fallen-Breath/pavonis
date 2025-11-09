package utils

import (
	"net"
	"testing"
)

func TestGetBucketForIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       net.IP
		expected string
	}{
		{
			name:     "Valid IPv4",
			ip:       net.ParseIP("192.168.1.1"),
			expected: "ipv4$192.168.1.1",
		},
		{
			name:     "Another IPv4",
			ip:       net.ParseIP("8.8.8.8"),
			expected: "ipv4$8.8.8.8",
		},
		{
			name:     "Valid IPv6 with /64 prefix",
			ip:       net.ParseIP("2001:db8:85a3::8a2e:370:7334"),
			expected: "ipv6$2001:db8:85a3::",
		},
		{
			name:     "IPv6 with short form",
			ip:       net.ParseIP("2001:db8::1"),
			expected: "ipv6$2001:db8::",
		},
		{
			name:     "IPv6 all zeros",
			ip:       net.ParseIP("::"),
			expected: "ipv6$::",
		},
		{
			name:     "Nil IP",
			ip:       nil,
			expected: "unknown$",
		},
		{
			name:     "Invalid IP (nil from ParseIP)",
			ip:       net.ParseIP("invalid"),
			expected: "unknown$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBucketForIp(tt.ip)
			if result != tt.expected {
				t.Errorf("GetBucketForIp(%v) = %q; want %q", tt.ip, result, tt.expected)
			}
		})
	}
}
