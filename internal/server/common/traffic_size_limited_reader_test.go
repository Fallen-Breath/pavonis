package common

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

type errorReader struct{}

var _ io.ReadCloser = &errorReader{}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func (e *errorReader) Close() error {
	return nil
}

func readAllWithBuffer(r io.Reader, bufferSize int) ([]byte, error) {
	result := make([]byte, 0)
	buf := make([]byte, bufferSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return result, nil
			}
			return result, err
		}
	}
}

func TestTrafficSizeLimitedReadCloserExceeded(t *testing.T) {
	for _, bufferSize := range []int{1, 3, 5, 6, 100} {
		limitedReader := NewTrafficSizeLimitedReadCloser(io.NopCloser(bytes.NewReader([]byte("abcdefghij"))), 5)
		totalRead, err := readAllWithBuffer(limitedReader, bufferSize)
		assert.Error(t, err, "bufferSize=%d", bufferSize)
		assert.LessOrEqual(t, len(totalRead), 5, "bufferSize=%d", bufferSize)
		assert.True(t, limitedReader.Exceeded(), "bufferSize=%d", bufferSize)
	}
}

func TestTrafficSizeLimitedReadCloser(t *testing.T) {
	tt := []struct {
		name              string
		readerFunc        func() io.ReadCloser
		maxSize           int64
		bufferSize        int
		expectedTotalRead []byte
		expectedError     error
	}{
		{
			name:              "NormalRead",
			readerFunc:        func() io.ReadCloser { return io.NopCloser(bytes.NewReader([]byte("hello"))) },
			maxSize:           10,
			bufferSize:        3,
			expectedTotalRead: []byte("hello"),
			expectedError:     nil,
		},
		{
			name:              "ExactLimit",
			readerFunc:        func() io.ReadCloser { return io.NopCloser(bytes.NewReader([]byte("abcde"))) },
			maxSize:           5,
			bufferSize:        2,
			expectedTotalRead: []byte("abcde"),
			expectedError:     nil,
		},
		{
			name:              "EmptyData",
			readerFunc:        func() io.ReadCloser { return io.NopCloser(bytes.NewReader([]byte{})) },
			maxSize:           0,
			bufferSize:        1,
			expectedTotalRead: []byte{},
			expectedError:     nil,
		},
		{
			name:              "ReadError",
			readerFunc:        func() io.ReadCloser { return &errorReader{} },
			maxSize:           100,
			bufferSize:        1,
			expectedTotalRead: []byte{},
			expectedError:     fmt.Errorf("read error"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			reader := tc.readerFunc()
			limitedReader := NewTrafficSizeLimitedReadCloser(reader, tc.maxSize)
			totalRead, err := readAllWithBuffer(limitedReader, tc.bufferSize)
			assert.Equal(t, tc.expectedTotalRead, totalRead, "Read bytes mismatch")
			assert.False(t, limitedReader.Exceeded())
			if tc.expectedError == nil {
				assert.NoError(t, err, "Expected no error")
			} else {
				assert.Equal(t, tc.expectedError, err, "Error mismatch")
			}
			assert.NoError(t, limitedReader.Close(), "Close should not fail")
		})
	}
}
