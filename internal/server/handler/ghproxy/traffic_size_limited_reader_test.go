package ghproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/server/common"
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
			name:              "ExceededLimit",
			readerFunc:        func() io.ReadCloser { return io.NopCloser(bytes.NewReader([]byte("abcdefghij"))) },
			maxSize:           5,
			bufferSize:        3,
			expectedTotalRead: []byte("abcdef"), // Reads 3+3=6 bytes, then errors on next read
			expectedError:     common.NewHttpError(http.StatusBadGateway, "Response body too large"),
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
			if tc.expectedError == nil {
				assert.NoError(t, err, "Expected no error")
			} else {
				assert.Equal(t, tc.expectedError, err, "Error mismatch")
			}
			assert.NoError(t, limitedReader.Close(), "Close should not fail")
		})
	}
}
