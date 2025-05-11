package crproxy

import (
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractReposNameFromV2Path(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected *[]string
	}{
		// https://distribution.github.io/distribution/spec/api/#detail
		// Possible paths:
		//
		//     "/v2/_catalog"
		//     "/v2/<name>/blobs/<digest>"
		//     "/v2/<name>/blobs/uploads/"
		//     "/v2/<name>/blobs/uploads/<uuid>"
		//     "/v2/<name>/manifests/<reference>"
		//     "/v2/<name>/tags/list"
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
