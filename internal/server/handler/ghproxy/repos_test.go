package ghproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReposList(t *testing.T) {
	list := newReposList([]string{"openai/project", "*/wild", "only-author"})
	require.True(t, list.Check("openai", "project"))
	require.True(t, list.Check("someone", "wild"))
	require.True(t, list.Check("only-author", ""))
	require.False(t, list.Check("openai", "other"))
}
