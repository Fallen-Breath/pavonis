package common

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	appctx "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/stretchr/testify/require"
)

func TestModifyResponseBody(t *testing.T) {
	ctx := appctx.NewRequestContext("test", "127.0.0.1")

	t.Run("replaces text and removes content length", func(t *testing.T) {
		resp := &http.Response{Header: make(http.Header), Body: io.NopCloser(strings.NewReader("before before"))}
		resp.Header.Set("Content-Length", "13")

		require.NoError(t, ModifyResponseBody(ctx, resp, "before", "after"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "after after", string(body))
		require.Empty(t, resp.Header.Get("Content-Length"))
		require.Equal(t, "chunked", resp.Header.Get("Transfer-Encoding"))
	})

	t.Run("skips empty and identical replacements", func(t *testing.T) {
		for _, tc := range []struct{ search, replace string }{{"", "x"}, {"x", "x"}} {
			resp := &http.Response{Header: make(http.Header), Body: io.NopCloser(strings.NewReader("x"))}
			require.NoError(t, ModifyResponseBody(ctx, resp, tc.search, tc.replace))
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "x", string(body))
		}
	})

	t.Run("rejects unsupported encoding", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{"Content-Encoding": []string{"unknown"}}, Body: io.NopCloser(strings.NewReader("x"))}
		err := ModifyResponseBody(ctx, resp, "x", "y")
		require.ErrorContains(t, err, "Unsupported Content-Encoding")
	})
}

func TestRewriteLinkHeaderUrls(t *testing.T) {
	header := http.Header{}
	header.Set("Link", "<https://old.test/a>; rel=next, <not a url>; rel=prev")
	var unknown []string
	RewriteLinkHeaderUrls(&header, func(oldURL *url.URL) *url.URL {
		if oldURL.Host == "old.test" {
			return &url.URL{Scheme: "https", Host: "new.test", Path: oldURL.Path}
		}
		return nil
	}, func(urlStr string) { unknown = append(unknown, urlStr) })
	require.Equal(t, "<https://new.test/a>; rel=next, <not a url>; rel=prev", header.Get("Link"))
	require.Equal(t, []string{"not a url"}, unknown)
}
