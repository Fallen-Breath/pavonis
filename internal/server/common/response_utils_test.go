package common

import (
	"bytes"
	"compress/gzip"
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

		require.NoError(t, ModifyResponseBody(ctx, resp, BodyReplacement{Search: "before", Replace: "after"}))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "after after", string(body))
		require.Empty(t, resp.Header.Get("Content-Length"))
		require.Equal(t, "chunked", resp.Header.Get("Transfer-Encoding"))
	})

	t.Run("rejects unsupported encoding", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{"Content-Encoding": []string{"unknown"}}, Body: io.NopCloser(strings.NewReader("x"))}
		err := ModifyResponseBody(ctx, resp, BodyReplacement{Search: "x", Replace: "y"})
		require.Error(t, err)
	})
}

func TestModifyResponseBodyNoopRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules []BodyReplacement
	}{
		{"no rules", nil},
		{"empty search", []BodyReplacement{{Replace: "x"}}},
		{"identical literal", []BodyReplacement{{Search: "x", Replace: "x"}}},
		{"multiple noop rules", []BodyReplacement{{Search: "x", Replace: "x"}, {Replace: "y"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := io.NopCloser(strings.NewReader("x"))
			resp := &http.Response{
				Header: http.Header{"Content-Encoding": []string{"unknown"}, "Content-Length": []string{"1"}},
				Body:   original,
			}
			require.NoError(t, ModifyResponseBody(appctx.NewRequestContext("test", "127.0.0.1"), resp, tc.rules...))
			require.Equal(t, original, resp.Body)
			require.Equal(t, "1", resp.Header.Get("Content-Length"))
			require.Empty(t, resp.Header.Get("Transfer-Encoding"))
		})
	}
	for _, tc := range []struct {
		name, input, expected string
		rules                 []BodyReplacement
	}{
		{"noop then active", "x", "y", []BodyReplacement{{Search: "x", Replace: "x"}, {Search: "x", Replace: "y"}}},
		{"deletion is active", "x", "", []BodyReplacement{{Search: "x"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.input))}
			require.NoError(t, ModifyResponseBody(appctx.NewRequestContext("test", "127.0.0.1"), resp, tc.rules...))
			output, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.expected, string(output))
		})
	}
}

func TestModifyResponseBodyWithMultipleRulesAndCompression(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(`old value="abc" spaced: "def"`))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(&compressed),
	}
	require.NoError(t, ModifyResponseBody(appctx.NewRequestContext("test", "127.0.0.1"), resp,
		BodyReplacement{Search: "old", Replace: "new"},
		BodyReplacement{Search: `value="abc"`, Replace: `value="ABC"`},
		BodyReplacement{Search: `spaced: "def"`, Replace: `spaced: "DEF"`},
	))
	reader, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, `new value="ABC" spaced: "DEF"`, string(output))
	require.NoError(t, reader.Close())
	require.NoError(t, resp.Body.Close())
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
