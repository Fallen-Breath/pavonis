package ioutils

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
	"io"
	"testing"
)

func TestReplacingReader(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		search   string
		replace  string
		bufSize  int
		expected string
	}{
		{
			name:     "Basic replacement",
			data:     "123abc456abc789",
			search:   "abc",
			replace:  "def",
			bufSize:  4096,
			expected: "123def456def789",
		},
		{
			name:     "Cross boundary replacement 1",
			data:     "##abc##abc##",
			search:   "abc",
			replace:  "def",
			bufSize:  1,
			expected: "##def##def##",
		},
		{
			name:     "Cross boundary replacement 2",
			data:     "##abc##abc##",
			search:   "abc",
			replace:  "def",
			bufSize:  2,
			expected: "##def##def##",
		},
		{
			name:     "Cross boundary replacement 3",
			data:     "##abc##abc##",
			search:   "abc",
			replace:  "def",
			bufSize:  3,
			expected: "##def##def##",
		},
		{
			name:     "Cross boundary replacement 4",
			data:     "##abc##abc##",
			search:   "abc",
			replace:  "def",
			bufSize:  4,
			expected: "##def##def##",
		},
		{
			name:     "Empty replace",
			data:     "123abc456",
			search:   "abc",
			replace:  "",
			bufSize:  4096,
			expected: "123456",
		},
		{
			name:     "No match",
			data:     "123456",
			search:   "xyz",
			replace:  "def",
			bufSize:  4096,
			expected: "123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(tt.data))
			rr := NewReplacingReaderWithBufSize(io.NopCloser(reader), []byte(tt.search), []byte(tt.replace), tt.bufSize)
			var buf bytes.Buffer
			_, err := io.Copy(&buf, rr)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestReplacingReaderFuzzy(t *testing.T) {
	seed := uint64(0)
	iterations := 1000

	rand.Seed(seed)

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("FuzzyTest_%d", i), func(t *testing.T) {
			// Generate random data (1k-10k lowercase letters)
			dataLen := rand.Intn(9001) + 1000 // 1000 to 10000
			data := make([]byte, dataLen)
			for j := range data {
				data[j] = byte('a' + rand.Intn(26))
			}

			// Generate random search and replace strings
			searchLen := rand.Intn(20) + 1  // (1-20 chars)
			replaceLen := rand.Intn(30) + 1 // (1-30 chars)
			search := make([]byte, searchLen)
			replace := make([]byte, replaceLen)
			for j := range search {
				search[j] = byte('a' + rand.Intn(26))
			}
			for j := range replace {
				replace[j] = byte('a' + rand.Intn(26))
			}

			// Insert search string at random positions (0 to 10 occurrences)
			numInsertions := rand.Intn(30)
			for j := 0; j < numInsertions; j++ {
				insertPos := rand.Intn(len(data))
				copy(data[:insertPos], search)
			}

			// Test with various buffer sizes
			for _, bufSize := range []int{1, 2, 3, 4, 5, 7, 9, 10, 15, 20, 49, 512, 4096} {
				reader := bytes.NewReader(data)
				rr := NewReplacingReaderWithBufSize(io.NopCloser(reader), search, replace, bufSize)
				var buf bytes.Buffer
				_, err := io.Copy(&buf, rr)
				require.NoError(t, err)

				expected := bytes.ReplaceAll(data, search, replace)
				assert.Equal(t, expected, buf.Bytes(), "With bufSize %d: ReplacingReader result differs from bytes.ReplaceAll", bufSize)
			}
		})
	}
}
