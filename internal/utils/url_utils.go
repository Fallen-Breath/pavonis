package utils

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func MustParseUrl(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

// ParseHttpUrlWithDefaultScheme parses an HTTP(S) URL. If rawURL omits its
// scheme, it is parsed as an authority plus path and defaultScheme is used.
func ParseHttpUrlWithDefaultScheme(rawURL string, defaultScheme string) (*url.URL, error) {
	defaultScheme = strings.ToLower(defaultScheme)
	if defaultScheme != "http" && defaultScheme != "https" {
		return nil, fmt.Errorf("unsupported default URL scheme %q", defaultScheme)
	}
	if rawURL == "" {
		return nil, fmt.Errorf("target URL is empty")
	}
	if strings.TrimSpace(rawURL) != rawURL {
		return nil, fmt.Errorf("target URL has leading or trailing whitespace")
	}
	for _, c := range rawURL {
		if c < 0x20 || c == 0x7f {
			return nil, fmt.Errorf("target URL contains a control character")
		}
	}
	if strings.HasPrefix(rawURL, "/") {
		return nil, fmt.Errorf("target URL must not start with a slash")
	}

	lowerRawURL := strings.ToLower(rawURL)
	var (
		parsed *url.URL
		err    error
	)
	switch {
	case strings.HasPrefix(lowerRawURL, "http://"), strings.HasPrefix(lowerRawURL, "https://"):
		parsed, err = url.Parse(rawURL)
	case strings.HasPrefix(lowerRawURL, "http:"), strings.HasPrefix(lowerRawURL, "https:"):
		return nil, fmt.Errorf("malformed HTTP URL scheme")
	case hasExplicitURLScheme(rawURL):
		return nil, fmt.Errorf("unsupported URL scheme %q", rawURL[:strings.Index(rawURL, "://")])
	case hasLeadingSchemeDelimiter(rawURL):
		return nil, fmt.Errorf("malformed URL scheme")
	default:
		parsed, err = url.Parse("//" + rawURL)
		if parsed != nil {
			parsed.Scheme = defaultScheme
		}
	}
	if err != nil {
		return nil, err
	}
	if parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("target URL has no host")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if port := parsed.Port(); port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return nil, fmt.Errorf("invalid URL port %q", port)
		}
	}
	return parsed, nil
}

func hasExplicitURLScheme(rawURL string) bool {
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd <= 0 {
		return false
	}

	for i := 0; i < schemeEnd; i++ {
		c := rawURL[i]
		if i == 0 {
			if !isASCIILetter(c) {
				return false
			}
			continue
		}
		if !isASCIILetter(c) && (c < '0' || c > '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func hasLeadingSchemeDelimiter(rawURL string) bool {
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return false
	}
	pathStart := strings.IndexAny(rawURL, "/?#")
	return pathStart < 0 || schemeEnd < pathStart
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
