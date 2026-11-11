package ghproxy

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func matchTargetUrl(t *testing.T, rawUrl string) (*targetMatch, bool) {
	u, err := url.Parse(rawUrl)
	require.NoError(t, err)
	return matchTarget(u.Host, u.Path, u.Query())
}

func TestMatchTargetServedUrls(t *testing.T) {
	for _, tc := range []struct {
		url      string
		expected targetMatch
	}{
		{"https://github.com/cli/cli/releases/download/v2.0.0/gh_2.0.0_checksums.txt", targetMatch{Owner: "cli", Repo: "cli"}},
		{"https://github.com/cli/cli/releases/latest/download/checksums.txt", targetMatch{Owner: "cli", Repo: "cli"}},
		{"https://github.com/cli/cli/archive/refs/tags/v2.0.0.zip", targetMatch{Owner: "cli", Repo: "cli"}},
		{"https://github.com/cli/cli/archive/v2.0.0.tar.gz", targetMatch{Owner: "cli", Repo: "cli"}},
		{"https://github.com/cli/cli/raw/trunk/README.md", targetMatch{Owner: "cli", Repo: "cli"}},
		{"https://github.com/cli/cli/blob/trunk/docs/README.md", targetMatch{Owner: "cli", Repo: "cli", RewrittenPath: "/cli/cli/raw/trunk/docs/README.md"}},
		{"https://github.com/cli/cli.git/info/refs?service=git-upload-pack", targetMatch{Owner: "cli", Repo: "cli.git"}},
		{"https://github.com/cli/cli.git/git-upload-pack", targetMatch{Owner: "cli", Repo: "cli.git"}},
		{"https://github.com/cli/cli.git/info/lfs/objects/batch", targetMatch{Owner: "cli", Repo: "cli.git"}},
		{"https://github.com/cli/cli.git/info/refs?service=git-receive-pack", targetMatch{Owner: "cli", Repo: "cli.git", Push: true}},
		{"https://github.com/cli/cli.git/git-receive-pack", targetMatch{Owner: "cli", Repo: "cli.git", Push: true}},
		{"https://github.com/cli/cli.git/info/lfs/locks", targetMatch{Owner: "cli", Repo: "cli.git", Push: true}},
		{"https://github.com/cli/cli.git/info/lfs/locks/verify", targetMatch{Owner: "cli", Repo: "cli.git", Push: true}},

		{"https://raw.githubusercontent.com/cli/cli/trunk/README.md", targetMatch{Owner: "cli", Repo: "cli"}},

		{"https://gist.github.com/schacon/1/raw", targetMatch{Owner: "schacon"}},
		{"https://gist.github.com/schacon/1/raw/abc/file.txt", targetMatch{Owner: "schacon"}},
		{"https://gist.github.com/schacon/1/archive/master.zip", targetMatch{Owner: "schacon"}},
		{"https://gist.github.com/schacon/1/download", targetMatch{Owner: "schacon"}},
		{"https://gist.github.com/schacon/1.git/info/refs?service=git-upload-pack", targetMatch{Owner: "schacon"}},
		{"https://gist.github.com/schacon/1/git-upload-pack", targetMatch{Owner: "schacon"}},
		{"https://gist.github.com/schacon/1.git/info/refs?service=git-receive-pack", targetMatch{Owner: "schacon", Push: true}},
		{"https://gist.github.com/schacon/1.git/git-receive-pack", targetMatch{Owner: "schacon", Push: true}},
		{"https://gist.github.com/1.git/info/refs?service=git-upload-pack", targetMatch{}},
		{"https://gist.github.com/aa5a315d61ae9438b18d/git-upload-pack", targetMatch{}},
		{"https://gist.github.com/1.git/git-receive-pack", targetMatch{Push: true}},

		{"https://gist.githubusercontent.com/schacon/1/raw", targetMatch{Owner: "schacon"}},
		{"https://gist.githubusercontent.com/schacon/1/raw/abc/file.txt", targetMatch{Owner: "schacon"}},
	} {
		t.Run(tc.url, func(t *testing.T) {
			tm, hostOk := matchTargetUrl(t, tc.url)
			require.True(t, hostOk)
			require.NotNil(t, tm)
			require.Equal(t, tc.expected, *tm)
		})
	}
}

func TestMatchTargetUnsupportedHost(t *testing.T) {
	for _, u := range []string{
		"https://example.com/cli/cli/raw/trunk/README.md",
		"https://codeload.github.com/cli/cli/zip/refs/heads/trunk",
	} {
		t.Run(u, func(t *testing.T) {
			tm, hostOk := matchTargetUrl(t, u)
			require.False(t, hostOk)
			require.Nil(t, tm)
		})
	}
}

func TestMatchTargetUnsupportedPath(t *testing.T) {
	for _, u := range []string{
		"https://github.com/cli/cli",
		"https://github.com/cli/cli/releases",
		"https://github.com/cli/cli/releases/tag/v2.0.0",
		"https://github.com/cli/cli/issues/1",
		"https://github.com/cli/cli.git/info/refs",
		"https://github.com/cli/cli.git/info/refs?service=other",
		"https://github.com/cli/cli/raw/../../../settings",
		"https://github.com/cli/cli/raw/trunk/./README.md",
		"https://raw.githubusercontent.com/cli/cli/trunk",
		"https://gist.github.com/schacon/1",
		"https://gist.github.com/schacon/1/revisions",
		"https://gist.githubusercontent.com/schacon/1/info/refs?service=git-upload-pack",
	} {
		t.Run(u, func(t *testing.T) {
			tm, hostOk := matchTargetUrl(t, u)
			require.True(t, hostOk)
			require.Nil(t, tm)
		})
	}
}
