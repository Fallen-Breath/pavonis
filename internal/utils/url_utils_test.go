package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHttpUrlWithDefaultScheme(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		scheme string
		host   string
		path   string
	}{
		{
			name:   "scheme omitted",
			rawURL: "example.com/path",
			scheme: "https",
			host:   "example.com",
			path:   "/path",
		},
		{
			name:   "explicit scheme",
			rawURL: "http://example.com/path",
			scheme: "http",
			host:   "example.com",
			path:   "/path",
		},
		{
			name:   "uppercase scheme",
			rawURL: "HTTPS://example.com/path",
			scheme: "https",
			host:   "example.com",
			path:   "/path",
		},
		{
			name:   "host with port",
			rawURL: "example.com:8443/path",
			scheme: "https",
			host:   "example.com:8443",
			path:   "/path",
		},
		{
			name:   "scheme delimiter in path",
			rawURL: "example.com/path/file://name",
			scheme: "https",
			host:   "example.com",
			path:   "/path/file://name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseHttpUrlWithDefaultScheme(test.rawURL, "https")
			require.NoError(t, err)
			require.Equal(t, test.scheme, parsed.Scheme)
			require.Equal(t, test.host, parsed.Host)
			require.Equal(t, test.path, parsed.Path)
		})
	}
}

func TestParseHttpUrlWithDefaultSchemeRejectsInvalidInput(t *testing.T) {
	for _, rawURL := range []string{
		"",
		"/example.com/path",
		"//example.com/path",
		"https:example.com/path",
		"https:/example.com/path",
		"ftp://example.com/path",
		"1http://example.com/path",
		"example.com:0/path",
		"example.com:65536/path",
		" example.com/path",
		"example.com/path ",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := ParseHttpUrlWithDefaultScheme(rawURL, "https")
			require.Error(t, err)
		})
	}
}

func TestParseHttpUrlWithDefaultSchemeRejectsUnsupportedDefault(t *testing.T) {
	_, err := ParseHttpUrlWithDefaultScheme("example.com/path", "ftp")
	require.Error(t, err)
}
