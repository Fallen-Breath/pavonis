package ghproxy

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestRawTextUrlRewriteReader(t *testing.T) {
	const p = "https://proxy.example/"
	for _, tc := range []struct {
		name, content, expected string
	}{
		{
			"release download",
			"curl -L https://github.com/cli/cli/releases/download/v2.0.0/gh.tgz -o gh.tgz",
			"curl -L " + p + "https://github.com/cli/cli/releases/download/v2.0.0/gh.tgz -o gh.tgz",
		},
		{
			"all served hosts",
			"https://raw.githubusercontent.com/cli/cli/trunk/a.sh\nhttps://gist.github.com/schacon/1/raw\nhttps://gist.githubusercontent.com/schacon/1/raw/x",
			p + "https://raw.githubusercontent.com/cli/cli/trunk/a.sh\n" + p + "https://gist.github.com/schacon/1/raw\n" + p + "https://gist.githubusercontent.com/schacon/1/raw/x",
		},
		{
			"shell variables in path",
			`curl -L "https://github.com/$OWNER/$REPO/releases/download/${VERSION}/x.tgz"`,
			`curl -L "` + p + `https://github.com/$OWNER/$REPO/releases/download/${VERSION}/x.tgz"`,
		},
		{
			"delimiters",
			"(https://github.com/cli/cli/raw/trunk/a) 'https://github.com/cli/cli/raw/trunk/b' <https://github.com/cli/cli/raw/trunk/c>",
			"(" + p + "https://github.com/cli/cli/raw/trunk/a) '" + p + "https://github.com/cli/cli/raw/trunk/b' <" + p + "https://github.com/cli/cli/raw/trunk/c>",
		},
		{
			"query and fragment",
			"https://github.com/cli/cli/raw/trunk/a?x=1#y",
			p + "https://github.com/cli/cli/raw/trunk/a?x=1#y",
		},
		{
			"unsupported paths and hosts",
			"https://github.com/cli/cli https://github.com/cli/cli/issues/1 https://example.com/cli/cli/raw/trunk/a https://github.com/cli/cli/raw/../../settings",
			"https://github.com/cli/cli https://github.com/cli/cli/issues/1 https://example.com/cli/cli/raw/trunk/a https://github.com/cli/cli/raw/../../settings",
		},
		{
			"unsupported url with a supported prefix",
			"https://github.com/cli/cli/raw/trunk/../../../settings",
			"https://github.com/cli/cli/raw/trunk/../../../settings",
		},
		{
			"already proxied",
			"https://other.proxy/https://github.com/cli/cli/raw/trunk/a",
			"https://other.proxy/https://github.com/cli/cli/raw/trunk/a",
		},
		{
			"url prefix variable",
			"${CDN}https://github.com/cli/cli/raw/trunk/a",
			"${CDN}https://github.com/cli/cli/raw/trunk/a",
		},
		{
			"bad scheme",
			"nothttps://github.com/cli/cli/raw/trunk/a magic+https://github.com/cli/cli/raw/trunk/b",
			"nothttps://github.com/cli/cli/raw/trunk/a magic+https://github.com/cli/cli/raw/trunk/b",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reader := range []struct {
				name string
				r    io.Reader
			}{
				{"whole", strings.NewReader(tc.content)},
				{"one byte per read", iotest.OneByteReader(strings.NewReader(tc.content))},
			} {
				t.Run(reader.name, func(t *testing.T) {
					out, err := io.ReadAll(newRawTextUrlRewriteReader(io.NopCloser(reader.r), p))
					require.NoError(t, err)
					require.Equal(t, tc.expected, string(out))
				})
			}
		})
	}
}

func TestShouldRewriteRawText(t *testing.T) {
	for _, tc := range []struct {
		name, host, contentType string
		expected                bool
	}{
		{"raw.githubusercontent.com", "raw.githubusercontent.com", "text/plain; charset=utf-8", true},
		{"gist.githubusercontent.com", "gist.githubusercontent.com", "text/plain; charset=utf-8", true},
		{"charset case", "raw.githubusercontent.com", "text/plain; charset=UTF-8", true},
		{"not a raw host", "github.com", "text/plain; charset=utf-8", false},
		{"binary", "raw.githubusercontent.com", "application/octet-stream", false},
		{"no charset", "raw.githubusercontent.com", "text/plain", false},
		{"non utf-8", "raw.githubusercontent.com", "text/plain; charset=iso-8859-1", false},
		{"no content type", "raw.githubusercontent.com", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, shouldRewriteRawText(tc.host, tc.contentType))
		})
	}
}
