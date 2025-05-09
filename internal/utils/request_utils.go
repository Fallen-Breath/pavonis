package utils

import (
	"net"
	"net/http"
	"strings"
)

func GetIpFromHostPort(hostPort string) (net.IP, string) {
	if host, _, err := net.SplitHostPort(hostPort); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip, host
		}
	}
	return nil, hostPort
}

func GetRequestClientIpFromProxyHeader(r *http.Request) (string, bool) {
	headers := []string{
		"CF-Connecting-IP", // Cloudflare (including cloudflared)
		"X-Forwarded-For",  // Standard proxy header
		"X-Real-IP",        // Common alternative
	}

	// Try each header in order
	for _, header := range headers {
		if value := r.Header.Get(header); value != "" {
			// Parse as X-Forwarded-For: take first valid IP in comma-separated list
			parts := strings.Split(strings.TrimSpace(value), ",")
			for _, part := range parts {
				ip := strings.TrimSpace(part)
				if net.ParseIP(ip) != nil {
					return ip, true
				}
			}
		}
	}

	return "", false
}
