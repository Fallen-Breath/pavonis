package crproxy

import (
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractReposNameFromV1Path(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected *[]string
	}{
		// Single segment repository name
		{
			name:     "Single segment - images",
			path:     "/v1/repositories/foo/images",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Single segment - tags",
			path:     "/v1/repositories/foo/tags",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Single segment - specific tag",
			path:     "/v1/repositories/foo/tags/latest",
			expected: utils.ToPtr([]string{"foo"}),
		},
		// Two segments repository name (namespace/name)
		{
			name:     "Two segments - images",
			path:     "/v1/repositories/someone/somerepos/images",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Two segments - tags",
			path:     "/v1/repositories/someone/somerepos/tags",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Two segments - specific tag",
			path:     "/v1/repositories/someone/somerepos/tags/latest",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		// Invalid paths
		{
			name:     "Invalid path - no v1 prefix",
			path:     "/v2/foo/images",
			expected: nil,
		},
		{
			name:     "Invalid path - empty path",
			path:     "/v1/repositories/",
			expected: nil,
		},
		{
			name:     "Invalid path - no suffix",
			path:     "/v1/repositories/foo",
			expected: nil,
		},
		{
			name:     "Invalid path - tags with no tag name",
			path:     "/v1/repositories/foo/tags/",
			expected: nil,
		},
		{
			name:     "Invalid path - tags with nested path",
			path:     "/v1/repositories/foo/tags/nested/path",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReposNameFromV1Path(tt.path)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestExtractReposNameFromV2Path(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected *[]string
	}{
		{
			name:     "Single segment - tags list",
			path:     "/v2/foo/tags/list",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Single segment - manifests",
			path:     "/v2/foo/manifests/latest",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Single segment - blobs",
			path:     "/v2/foo/blobs/sha256:abc123",
			expected: utils.ToPtr([]string{"foo"}),
		},
		{
			name:     "Two segments - tags list",
			path:     "/v2/someone/somerepos/tags/list",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Two segments - manifests",
			path:     "/v2/someone/somerepos/manifests/latest",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Two segments - blobs",
			path:     "/v2/someone/somerepos/blobs/sha256:abc123",
			expected: utils.ToPtr([]string{"someone", "somerepos"}),
		},
		{
			name:     "Three segments - tags list",
			path:     "/v2/foo/bar/baz/tags/list",
			expected: utils.ToPtr([]string{"foo", "bar", "baz"}),
		},
		{
			name:     "Three segments - manifests",
			path:     "/v2/foo/bar/baz/manifests/latest",
			expected: utils.ToPtr([]string{"foo", "bar", "baz"}),
		},
		{
			name:     "Three segments - blobs",
			path:     "/v2/foo/bar/baz/blobs/sha256:abc123",
			expected: utils.ToPtr([]string{"foo", "bar", "baz"}),
		},
		{
			name:     "Invalid path - no v2 prefix",
			path:     "/v1/foo/bar/baz/tags/list",
			expected: nil,
		},
		{
			name:     "Catalog path",
			path:     "/v2/_catalog",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReposNameFromV2Path(tt.path)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}
