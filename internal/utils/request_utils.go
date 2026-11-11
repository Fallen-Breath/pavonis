package utils

import (
	"golang.org/x/exp/slices"
	"net"
	"net/http"
	"os"
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

// GetRequestClientIpFromProxyHeader returns the client IP from the first usable header,
// taking the rightmost untrusted IP like nginx's real_ip_recursive
func GetRequestClientIpFromProxyHeader(r *http.Request, headers []string, isTrusted func(ip net.IP) bool) (string, bool) {
	for _, header := range headers {
		values := r.Header.Values(header)
		if len(values) == 0 {
			continue
		}
		parts := strings.Split(strings.Join(values, ","), ",")

		var clientIp string
		for i := len(parts) - 1; i >= 0; i-- {
			ipStr := strings.TrimSpace(parts[i])
			ip := net.ParseIP(ipStr)
			if ip == nil {
				clientIp = ""
				break
			}
			clientIp = ipStr
			if !isTrusted(ip) {
				break
			}
		}
		if clientIp != "" {
			return clientIp, true
		}
	}
	return "", false
}

// For debugging only: set PAVONIS_DEBUG_PRESERVE_SENSITIVE_HEADERS=1 to log sensitive headers unmasked
var flagPreserveSensitiveHeaders = slices.Contains([]string{"1", "true"}, os.Getenv("PAVONIS_DEBUG_PRESERVE_SENSITIVE_HEADERS"))

var sensitiveHeaders = map[string]bool{
	"authorization":  true,
	"cookie":         true,
	"set-cookie":     true,
	"x-api-key":      true,
	"token":          true,
	"x-access-token": true,
}

func MaskSensitiveHeaders(header http.Header) {
	if flagPreserveSensitiveHeaders {
		return
	}
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
	MaskSensitiveHeaders(newRsp.Header)
	return &newRsp
}
