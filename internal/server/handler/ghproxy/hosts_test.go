package ghproxy

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllowedGithubHosts(t *testing.T) {
	cases := []struct {
		host, path, author, repos string
	}{
		{"github.com", "/owner/repo/releases/download/v1/a.zip", "owner", "repo"},
		{"raw.githubusercontent.com", "/owner/repo/main/file.txt", "owner", "repo"},
		{"gist.github.com", "/owner/abcdef/file.txt", "owner", ""},
	}
	for _, tc := range cases {
		u := &url.URL{Host: tc.host, Path: tc.path}
		author, repos, ok := allowedHosts[tc.host].Parse(u)
		require.True(t, ok)
		require.Equal(t, tc.author, author)
		require.Equal(t, tc.repos, repos)
	}
	_, _, ok := allowedHosts["github.com"].Parse(&url.URL{Path: "/not-a-proxy-path"})
	require.False(t, ok)
}
