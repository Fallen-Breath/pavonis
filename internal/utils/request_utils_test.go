package utils

import (
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetIpFromHostPort(t *testing.T) {
	cases := []struct {
		input string
		ip    string
		host  string
	}{
		{"127.0.0.1:8080", "127.0.0.1", "127.0.0.1"},
		{"[::1]:8080", "::1", "::1"},
		{"example.test:8080", "", "example.test:8080"},
		{"127.0.0.1", "", "127.0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			ip, host := GetIpFromHostPort(tc.input)
			if tc.ip == "" {
				require.Nil(t, ip)
			} else {
				require.Equal(t, tc.ip, ip.String())
			}
			require.Equal(t, tc.host, host)
		})
	}
}

func TestGetRequestClientIpFromProxyHeader(t *testing.T) {
	pool, err := NewIpPool([]string{"10.0.0.0/8"})
	require.NoError(t, err)
	trusted := func(ip net.IP) bool { return pool.Contains(ip) }

	tests := []struct {
		name    string
		header  http.Header
		headers []string
		ip      string
		ok      bool
	}{
		{"single value", http.Header{"X-Real-Ip": {"192.0.2.1"}}, []string{"X-Real-IP"}, "192.0.2.1", true},
		{"forged leftmost value", http.Header{"X-Forwarded-For": {"198.51.100.1, 192.0.2.1"}}, []string{"X-Forwarded-For"}, "192.0.2.1", true},
		{"trusted proxies skipped", http.Header{"X-Forwarded-For": {"198.51.100.1, 192.0.2.1, 10.0.0.1, 10.0.0.2"}}, []string{"X-Forwarded-For"}, "192.0.2.1", true},
		{"multiple header lines", http.Header{"X-Forwarded-For": {"198.51.100.1", "192.0.2.1, 10.0.0.1"}}, []string{"X-Forwarded-For"}, "192.0.2.1", true},
		{"all trusted", http.Header{"X-Forwarded-For": {"10.0.0.1, 10.0.0.2"}}, []string{"X-Forwarded-For"}, "10.0.0.1", true},
		{"invalid value", http.Header{"X-Forwarded-For": {"192.0.2.1, not-an-ip, 10.0.0.1"}}, []string{"X-Forwarded-For"}, "", false},
		{"falls back to next header on invalid value", http.Header{"X-Forwarded-For": {"not-an-ip"}, "X-Real-Ip": {"192.0.2.2"}}, []string{"X-Forwarded-For", "X-Real-IP"}, "192.0.2.2", true},
		{"falls back to next header when absent", http.Header{"X-Real-Ip": {"192.0.2.2"}}, []string{"X-Forwarded-For", "X-Real-IP"}, "192.0.2.2", true},
		{"first header wins", http.Header{"X-Forwarded-For": {"192.0.2.1"}, "X-Real-Ip": {"192.0.2.2"}}, []string{"X-Forwarded-For", "X-Real-IP"}, "192.0.2.1", true},
		{"unlisted header ignored", http.Header{"Cf-Connecting-Ip": {"192.0.2.1"}}, []string{"X-Forwarded-For"}, "", false},
		{"no headers configured", http.Header{"X-Forwarded-For": {"192.0.2.1"}}, nil, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{Header: tc.header}
			ip, ok := GetRequestClientIpFromProxyHeader(req, tc.headers, trusted)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.ip, ip)
		})
	}
}
