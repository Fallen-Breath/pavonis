package httpproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestNewBodyRewritesDropsRuleWithoutEffect(t *testing.T) {
	rewrites := newBodyRewrites([]*config.HttpSiteBodyRewrite{
		{Replacements: []config.HttpSiteBodyReplacement{{Search: "", Replace: "x"}, {Search: "y", Replace: "y"}}},
	})
	require.Empty(t, rewrites)
}

func TestNewBodyRewritesMatchesContentTypeCaseInsensitively(t *testing.T) {
	rewrites := newBodyRewrites([]*config.HttpSiteBodyRewrite{{
		ContentTypes: []string{"Text/HTML"},
		Replacements: []config.HttpSiteBodyReplacement{{Search: "a", Replace: "b"}},
	}})
	require.True(t, rewrites[0].matchContentType("text/html"))
}

func TestBodyRewriteMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		reqPath string
		want    bool
	}{
		{"no paths matches all", nil, "/any/thing", true},
		{"any listed prefix matches", []string{"/a", "/static"}, "/static/app.js", true},
		{"no listed prefix matches", []string{"/static"}, "/other", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, (&bodyRewrite{paths: tc.paths}).matchPath(tc.reqPath))
		})
	}
}

func TestBodyRewriteMatchContentType(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		mediaType string
		want      bool
	}{
		{"exact", []string{"application/json"}, "application/json", true},
		{"subtype wildcard", []string{"text/*"}, "text/css", true},
		{"all types", []string{"*/*"}, "image/png", true},
		{"no match", []string{"text/*", "application/json"}, "image/png", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, (&bodyRewrite{contentTypes: tc.patterns}).matchContentType(tc.mediaType))
		})
	}
}

func TestRewriteResponse(t *testing.T) {
	rewrites := []*bodyRewrite{
		{contentTypes: []string{"text/*"}, replacements: []common.BodyReplacement{{Search: "up", Replace: "down"}}},
	}

	tests := []struct {
		name   string
		method string
		status int
		header map[string]string // merged into the base headers, "" deletes the header
		body   string

		wantBody     string
		wantETag     string
		wantAdjusted bool // Content-Length, Accept-Ranges and Repr-Digest removed
	}{
		{"2xx body rewritten", http.MethodGet, http.StatusOK, nil, "up", "down", `W/"e"`, true},
		{"4xx body rewritten", http.MethodGet, http.StatusNotFound, nil, "up", "down", `W/"e"`, true},
		{"already weak etag kept", http.MethodGet, http.StatusOK, map[string]string{"ETag": `W/"e"`}, "up", "down", `W/"e"`, true},
		{"head adjusts headers only", http.MethodHead, http.StatusOK, map[string]string{"Content-Length": "2", "Content-Encoding": "gzip"}, "", "", `W/"e"`, true},
		{"untyped 304 adjusts headers only", http.MethodGet, http.StatusNotModified, map[string]string{"Content-Type": ""}, "", "", `W/"e"`, true},
		{"304 with unmatched content type untouched", http.MethodGet, http.StatusNotModified, map[string]string{"Content-Type": "image/png"}, "", "", `"e"`, false},
		{"206 untouched", http.MethodGet, http.StatusPartialContent, nil, "up", "up", `"e"`, false},
		{"3xx untouched", http.MethodGet, http.StatusFound, nil, "up", "up", `"e"`, false},
		{"empty body untouched", http.MethodGet, http.StatusOK, nil, "", "", `"e"`, false},
		{"unmatched content type untouched", http.MethodGet, http.StatusOK, map[string]string{"Content-Type": "image/png"}, "up", "up", `"e"`, false},
		{"unsupported encoding untouched", http.MethodGet, http.StatusOK, map[string]string{"Content-Encoding": "compress"}, "up", "up", `"e"`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{
				"Content-Type":   {"text/plain; charset=utf-8"},
				"Content-Length": {strconv.Itoa(len(tc.body))},
				"Accept-Ranges":  {"bytes"},
				"Etag":           {`"e"`},
				"Repr-Digest":    {"sha-256=:AAAA:"},
			}
			for key, value := range tc.header {
				if value == "" {
					header.Del(key)
				} else {
					header.Set(key, value)
				}
			}
			contentLength, err := strconv.ParseInt(header.Get("Content-Length"), 10, 64)
			require.NoError(t, err)
			resp := &http.Response{
				StatusCode:    tc.status,
				Header:        header,
				Body:          io.NopCloser(strings.NewReader(tc.body)),
				ContentLength: contentLength,
				Request:       httptest.NewRequest(tc.method, "/", nil),
			}

			require.NoError(t, rewriteResponse(testutils.NewRequestContext(), resp, rewrites))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.wantBody, string(body))
			require.Equal(t, tc.wantETag, resp.Header.Get("ETag"))
			if tc.wantAdjusted {
				require.Empty(t, resp.Header.Get("Content-Length"))
				require.Empty(t, resp.Header.Get("Accept-Ranges"))
				require.Empty(t, resp.Header.Get("Repr-Digest"))
			} else {
				require.Equal(t, strconv.FormatInt(contentLength, 10), resp.Header.Get("Content-Length"))
				require.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
				require.NotEmpty(t, resp.Header.Get("Repr-Digest"))
			}
		})
	}
}
