package ghproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReposList(t *testing.T) {
	for _, tc := range []struct {
		name        string
		list        []string
		owner, repo string
		expected    bool
	}{
		{"exact", []string{"cli/cli"}, "cli", "cli", true},
		{"other repo", []string{"cli/cli"}, "cli", "other", false},
		{"other owner", []string{"cli/cli"}, "other", "cli", false},
		{"owner wildcard", []string{"cli/*"}, "cli", "other", true},
		{"repo wildcard", []string{"*/cli"}, "someone", "cli", true},
		{"any entry", []string{"a/b", "cli/cli"}, "cli", "cli", true},
		{"case insensitive", []string{"CLI/Cli"}, "cli", "cLI", true},
		{".git suffix in input", []string{"cli/cli"}, "cli", "cli.git", true},
		{".git suffix in list", []string{"cli/cli.git"}, "cli", "cli", true},
		{"gist", []string{"schacon/*"}, "schacon", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, newReposList(tc.list).Check(tc.owner, tc.repo))
		})
	}
}
