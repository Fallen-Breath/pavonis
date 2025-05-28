package ghproxy

import (
	"bytes"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	"mime"
	"strings"
)

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

func createHttpsUrlPrefixSearchFunc(src, dst string) ioutils.SearchFunc {
	srcBuf := []byte(src)
	dstBuf := []byte(dst)
	return func(buf []byte, lookBehindBuf []byte, eof bool) (int, int, []byte) {
		start := 0
		for {
			idx := bytes.Index(buf[start:], srcBuf)
			if idx == -1 {
				return -1, 0, nil
			}
			idx = idx + start // absolute index

			if idx == 0 && len(lookBehindBuf) == 0 { // start of the reader
				return 0, len(srcBuf), dstBuf
			}

			// look-behind the previous 1 char
			var prevChar byte
			if idx == 0 {
				prevChar = lookBehindBuf[len(lookBehindBuf)-1]
			} else {
				prevChar = buf[idx-1]
			}

			if !isBadPrevCharForRewrite(prevChar) {
				// if it's not a bad char, accept this
				replacement := []byte{prevChar}
				replacement = append(replacement, dstBuf...)
				return idx - 1, len(srcBuf) + 1, replacement
			}
			start = idx + len(srcBuf)
		}
	}
}
