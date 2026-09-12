package hfproxy

import (
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/stretchr/testify/require"
)

func TestIsValidHfPath(t *testing.T) {
	for _, path := range []string{
		"/api/models/org/model/revision/main",
		"/api/datasets/org/data/revision/main",
		"/org/model/resolve/abcdef/file.bin",
		"/datasets/org/data/resolve/012345/data.json",
	} {
		require.True(t, isValidHfPath(path), path)
	}
	for _, path := range []string{"", "/", "/org/model/blob/main/file", "/org/model/resolve/not-hex/file"} {
		require.False(t, isValidHfPath(path), path)
	}
}

func TestTryRewriteUrlToSelf(t *testing.T) {
	self := mustUrl("https://proxy.example.test")
	h := &proxyHandler{selfUrl: self, info: &handler.Info{PathPrefix: "/hf"}}
	got := h.tryRewriteUrlToSelf(mustUrl("https://huggingface.co/org/model/resolve/abc/file"))
	require.Equal(t, "https://proxy.example.test/hf/org/model/resolve/abc/file", got.String())
	require.Nil(t, h.tryRewriteUrlToSelf(mustUrl("https://example.test/file")))
}

func mustUrl(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
