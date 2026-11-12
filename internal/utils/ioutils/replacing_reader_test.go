package ioutils

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
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
			rr := NewLiteralReplacingReaderWithBufSize(io.NopCloser(reader), []byte(tt.search), []byte(tt.replace), tt.bufSize)
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
				rr := NewLiteralReplacingReaderWithBufSize(io.NopCloser(reader), search, replace, bufSize)
				var buf bytes.Buffer
				_, err := io.Copy(&buf, rr)
				require.NoError(t, err)

				expected := bytes.ReplaceAll(data, search, replace)
				assert.Equal(t, expected, buf.Bytes(), "With bufSize %d: ReplacingReader result differs from bytes.ReplaceAll", bufSize)
			}
		})
	}
}

func TestRuleReplacingReader(t *testing.T) {
	for _, tc := range []struct {
		name, input, expected string
		rules                 []Replacement
	}{
		{"multiple rules", "old/a upstream/b", "new/a local/b", []Replacement{{Search: "old/", Replace: "new/"}, {Search: "upstream/", Replace: "local/"}}},
		{"replacement output is not matched again", "upstream/a local/b", "local/a again/b", []Replacement{{Search: "upstream/", Replace: "local/"}, {Search: "local/", Replace: "again/"}}},
		{"long prefix first", "abcdef abcX", "A BX", []Replacement{{Search: "abcdef", Replace: "A"}, {Search: "abc", Replace: "B"}}},
		{"short prefix first", "abcdef", "Adef", []Replacement{{Search: "abc", Replace: "A"}, {Search: "abcdef", Replace: "B"}}},
		{"earlier incomplete prefix precedes a later complete match", "abcdef", "A", []Replacement{{Search: "abcdef", Replace: "A"}, {Search: "de", Replace: "B"}}},
		{"earliest position takes precedence", "early later", "E L", []Replacement{{Search: "later", Replace: "L"}, {Search: "early", Replace: "E"}}},
		{"unmatched common prefix", `href="other/a" href="old/b"`, `href="other/a" href="new/b"`, []Replacement{{Search: `href="old/`, Replace: `href="new/`}, {Search: `href="upstream/`, Replace: `href="local/`}}},
		{"truncated prefix at EOF", "abc", "abc", []Replacement{{Search: "abcdef", Replace: "A"}, {Search: "abcd", Replace: "B"}}},
		{"empty replacement", "a-b-a", "---", []Replacement{{Search: "a", Replace: ""}, {Search: "b", Replace: "-"}}},
		{"noop rule does not hide active rule", "abc", "Xc", []Replacement{{Search: "abc", Replace: "abc"}, {Search: "ab", Replace: "X"}}},
		{"no active rules", "abc", "abc", []Replacement{{Replace: "x"}, {Search: "abc", Replace: "abc"}}},
		{"no rules", "abc", "abc", nil},
		{"UTF-8 text across reads", "你好 世界 你好", "hello world hello", []Replacement{{Search: "你好", Replace: "hello"}, {Search: "世界", Replace: "world"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, size := range []int{1, 2, 7, 4096} {
				t.Run(fmt.Sprint(size), func(t *testing.T) {
					reader := newRuleReplacingReaderWithBufSize(io.NopCloser(strings.NewReader(tc.input)), size, tc.rules...)
					output, err := io.ReadAll(reader)
					require.NoError(t, err)
					require.Equal(t, tc.expected, string(output))
					require.NoError(t, reader.Close())
				})
			}
		})
	}
}

func FuzzRuleReplacingReader(f *testing.F) {
	families := [][]Replacement{
		{{Search: "ab", Replace: "X"}, {Search: "a", Replace: "ab"}, {Search: "old/", Replace: "new/"}},
		{{Search: `href="old/`, Replace: `href="new/`}, {Search: `href="../../packages/`, Replace: `href="/files/`}},
		{{Search: `"url":"https://files/`, Replace: `"url":"/files/`}, {Search: `"url": "https://files/`, Replace: `"url": "/files/`}},
	}
	var references []*regexp.Regexp
	for _, rules := range families {
		var alternatives []string
		for _, rule := range rules {
			alternatives = append(alternatives, regexp.QuoteMeta(rule.Search))
		}
		references = append(references, regexp.MustCompile(strings.Join(alternatives, "|")))
	}
	for family := range families {
		for _, seed := range []string{
			`ab a old/`,
			`href="../../packages/a.whl" href="old/b.whl"`,
			`{"url":"https://files/a.whl","url": "https://files/b.whl"}`,
			strings.Repeat("a", 4090) + `href="../../packages/a.whl"`,
		} {
			f.Add(seed, uint8(1), uint8(family))
			f.Add(seed, uint8(63), uint8(family))
		}
	}
	f.Fuzz(func(t *testing.T, input string, size, family uint8) {
		if len(input) > 8192 {
			return
		}
		group := int(family) % len(families)
		rules := families[group]
		expected := references[group].ReplaceAllStringFunc(input, func(match string) string {
			for _, rule := range rules {
				if match == rule.Search {
					return rule.Replace
				}
			}
			panic("unexpected reference match")
		})
		reader := newRuleReplacingReaderWithBufSize(io.NopCloser(strings.NewReader(input)), int(size)+1, rules...)
		output, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, expected, string(output))
	})
}

type recordingReadCloser struct {
	io.Reader
	read   int
	closed bool
}

func (r *recordingReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func (r *recordingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestRuleReplacingReaderStreamsAndCloses(t *testing.T) {
	content := strings.Repeat(`value="abc" `, 20_000)
	source := &recordingReadCloser{Reader: io.MultiReader(strings.NewReader(content), iotest.ErrReader(io.ErrUnexpectedEOF))}
	reader := NewRuleReplacingReader(source,
		Replacement{Search: `value="abc"`, Replace: `value="ABC"`},
		Replacement{Search: `value="def"`, Replace: `value="DEF"`},
	)
	first := make([]byte, 120)
	_, err := io.ReadFull(reader, first)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat(`value="ABC" `, 10), string(first))
	require.Less(t, source.read, len(content), "output must be available before the source is exhausted")
	_, err = io.Copy(io.Discard, reader)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.NoError(t, reader.Close())
	require.True(t, source.closed)
}
