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

func GetRequestClientIpFromProxyHeader(r *http.Request, headers []string) (string, bool) {
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

var sensitiveHeaders = map[string]bool{
	"authorization":  true,
	"cookie":         true,
	"set-cookie":     true,
	"x-api-key":      true,
	"token":          true,
	"x-access-token": true,
}

func MaskSensitiveHeaders(header http.Header) {
	for key, values := range header {
		if sensitiveHeaders[strings.ToLower(key)] {
			for i := range values {
				if values[i] != "" {
					values[i] = "***"
				}
			}
		}
	}
}

func MaskRequestForLogging(req *http.Request) *http.Request {
	newReq := req.Clone(req.Context())
	MaskSensitiveHeaders(newReq.Header)
	return newReq
}

func MaskResponseForLogging(rsp *http.Response) *http.Response {
	newRsp := *rsp
	newRsp.Header = make(http.Header)
	for key, values := range rsp.Header {
		newRsp.Header[key] = append([]string{}, values...)
	}
	MaskSensitiveHeaders(rsp.Header)
	return &newRsp
}
