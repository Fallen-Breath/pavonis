package utils

import (
	"net"
	"net/http"
	"strings"
)

func GetRequestClientIp(r *http.Request) string {
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
					return ip
				}
			}
		}
	}

	// Fallback to RemoteAddr
	if r.RemoteAddr != "" {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			if net.ParseIP(host) != nil {
				return host
			}
		}
		return r.RemoteAddr
	}

	return ""
}
