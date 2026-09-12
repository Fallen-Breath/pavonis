package utils

import (
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
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-For", "not-an-ip, 192.0.2.10, 192.0.2.11")
	ip, ok := GetRequestClientIpFromProxyHeader(req, []string{"X-Forwarded-For"})
	require.True(t, ok)
	require.Equal(t, "192.0.2.10", ip)

	req.Header.Set("X-Forwarded-For", "not-an-ip")
	ip, ok = GetRequestClientIpFromProxyHeader(req, []string{"X-Forwarded-For"})
	require.False(t, ok)
	require.Empty(t, ip)
}
