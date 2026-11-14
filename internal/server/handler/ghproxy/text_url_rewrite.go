package ghproxy

import (
	"bytes"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	"io"
	"mime"
	"net/url"
	"strings"
)

// shouldRewriteRawText reports whether the response body from the given host is raw text that the url rewrite applies to
func shouldRewriteRawText(host string, contentType string) bool {
	return rawTextUrlRewriteHosts[host] && isUtf8TextType(contentType)
}

// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Type
func isUtf8TextType(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}

	if strings.ToLower(mediaType) != "text/plain" {
		return false
	}

	charset, ok := params["charset"]
	if !ok {
		return false
	}

	switch strings.ToLower(charset) {
	case "utf-8", "utf8", "utf_8":
		return true
	default:
		return false
	}
}

func isBadPrevCharForRewrite(b byte) bool {
	// [0-9a-zA-Z]: bad scheme       : "nothttps://raw.githubusercontent.com/xxx/yyy"
	// '/': part of existing url     : "https://other.ghproxy.com/https://raw.githubusercontent.com/xxx/yyy"
	// '}': has a url prefix variable: "${CDN_PREFIX}https://raw.githubusercontent.com/xxx/yyy"
	// '+', '-': scheme concat       : "magic+https://raw.githubusercontent.com/xxx/yyy"
	return (b >= '0' && b <= '9') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z') ||
		b == '/' ||
		b == '+' || b == '-' ||
		b == '}'
}

// urls longer than this are left unchanged
const maxRewriteUrlLen = 2048

const rewriteUrlScheme = "https://"

// large enough for the longest rewritable url and the char that ends it
const rawTextUrlRewriteMaxSearchLen = maxRewriteUrlLen + 3

func isUrlEndChar(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '"', '\'', '`', '<', '>', '(', ')', '[', ']', '|', ';', '\\':
		return true
	default:
		return false
	}
}

// newRawTextUrlRewriteReader prepends dstPrefix to the https urls in the text that the github mode serves
func newRawTextUrlRewriteReader(reader io.ReadCloser, dstPrefix string) io.ReadCloser {
	return ioutils.NewReplacingReader(reader, createRawTextUrlRewriteSearchFunc(dstPrefix), rawTextUrlRewriteMaxSearchLen, 1)
}

// createRawTextUrlRewriteSearchFunc finds https urls that the github mode serves, and prepends dstPrefix to them
func createRawTextUrlRewriteSearchFunc(dstPrefix string) ioutils.SearchFunc {
	schemeBuf := []byte(rewriteUrlScheme)
	return func(buf []byte, lookBehindBuf []byte, eof bool) (int, int, []byte) {
		start := 0
		for {
			idx := bytes.Index(buf[start:], schemeBuf)
			if idx == -1 {
				return -1, 0, nil
			}
			idx = idx + start // absolute index
			start = idx + len(schemeBuf)

			// look-behind the previous 1 char
			if idx > 0 || len(lookBehindBuf) > 0 {
				var prevChar byte
				if idx == 0 {
					prevChar = lookBehindBuf[len(lookBehindBuf)-1]
				} else {
					prevChar = buf[idx-1]
				}
				if isBadPrevCharForRewrite(prevChar) {
					continue
				}
			}

			end := -1
			limit := min(len(buf), idx+maxRewriteUrlLen+1)
			for i := idx + len(schemeBuf); i < limit; i++ {
				if isUrlEndChar(buf[i]) {
					end = i
					break
				}
			}
			if end == -1 {
				if limit < len(buf) {
					continue // url too long
				}
				if !eof {
					return -1, 0, nil // the url might continue in the upcoming data
				}
				end = len(buf)
			}

			urlBuf := buf[idx:end]
			hostAndPath := string(urlBuf[len(schemeBuf):])
			slashIdx := strings.IndexByte(hostAndPath, '/')
			if slashIdx == -1 {
				continue
			}
			// An explicit HTTPS default port identifies the same allowed host.
			host := strings.TrimSuffix(strings.ToLower(hostAndPath[:slashIdx]), ":443")
			path, _, _ := strings.Cut(hostAndPath[slashIdx:], "#")
			path, rawQuery, _ := strings.Cut(path, "?")
			query, _ := url.ParseQuery(rawQuery)
			if tm, _ := matchTarget(host, path, query); tm == nil {
				continue
			}

			replacement := make([]byte, 0, len(dstPrefix)+len(urlBuf))
			replacement = append(replacement, dstPrefix...)
			replacement = append(replacement, urlBuf...)
			return idx, len(urlBuf), replacement
		}
	}
}
