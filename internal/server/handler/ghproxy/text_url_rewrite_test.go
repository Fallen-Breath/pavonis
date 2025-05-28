package ghproxy

import (
	"bytes"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestHttpsUrlPrefixSearchFunc(t *testing.T) {
	tt := []struct {
		name     string
		src      string
		dst      string
		data     string
		expected string
	}{
		{
			name:     "Single1",
			src:      "foo",
			dst:      "bar",
			data:     "foo",
			expected: "bar",
		},
		{
			name:     "Single2",
			src:      "foo",
			dst:      "bar",
			data:     "'foo",
			expected: "'bar",
		},
		{
			name:     "Single3",
			src:      "foo",
			dst:      "bar",
			data:     "xfoo",
			expected: "xfoo",
		},
		{
			name:     "Multi1",
			src:      "foo",
			dst:      "bar",
			data:     "xfoo Xfoo 1foo }foo -foo +foo /foo",
			expected: "xfoo Xfoo 1foo }foo -foo +foo /foo",
		},
		{
			name:     "Multi2",
			src:      "foo",
			dst:      "bar",
			data:     "foo#foo@foo'foo\"foo\nfoo\tfoo foo",
			expected: "bar#bar@bar'bar\"bar\nbar\tbar bar",
		},
		{
			name:     "Multi3",
			src:      "foo",
			dst:      "bar",
			data:     "foofoo#foo##foooofoo#",
			expected: "barfoo#bar##baroofoo#",
		},
		{
			name:     "Multi4",
			src:      "foo",
			dst:      "barzzz",
			data:     "foofoo#foo##foooofoo#",
			expected: "barzzzfoo#barzzz##barzzzoofoo#",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			searchFunc := createHttpsUrlPrefixSearchFunc(tc.src, tc.dst)
			for readBufSize := 1; readBufSize < len(tc.data)+2; readBufSize++ {
				dataReader := io.NopCloser(bytes.NewReader([]byte(tc.data)))

				reader := ioutils.NewReplacingReaderWithBufSize(dataReader, searchFunc, readBufSize, len(tc.src)+1, 1)
				newDataBuf, err := io.ReadAll(reader)
				require.NoError(t, err)

				newDataStr := string(newDataBuf)
				assert.Equal(t, tc.expected, newDataStr, "readBufSize=%d", readBufSize)
			}
		})
	}
}
