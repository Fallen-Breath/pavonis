package ioutils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"golang.org/x/exp/rand"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReadCloser is a mock implementation of io.ReadCloser for testing.
type mockReadCloser struct {
	ReadFunc  func(p []byte) (n int, err error)
	CloseFunc func() error
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	return m.ReadFunc(p)
}

func (m *mockReadCloser) Close() error {
	return m.CloseFunc()
}

func closeCloser(closer io.Closer) {
	_ = closer.Close()
}

func decompressAndVerify(t *testing.T, encoding string, expectedData []byte, compressedData *bytes.Buffer) {
	var decompressedData bytes.Buffer
	switch encoding {
	case "":
		decompressedData.Write(compressedData.Bytes())
	case "gzip":
		gzReader, err := gzip.NewReader(compressedData)
		require.NoError(t, err)
		defer closeCloser(gzReader)
		_, err = io.Copy(&decompressedData, gzReader)
		require.NoError(t, err)
	case "deflate":
		flateReader := flate.NewReader(compressedData)
		defer closeCloser(flateReader)
		_, err := io.Copy(&decompressedData, flateReader)
		require.NoError(t, err)
	default:
		t.Fatalf("unsupported encoding: %s", encoding)
	}

	assert.Equal(t, expectedData, decompressedData.Bytes())
}

func TestCompressingReader_Read(t *testing.T) {
	tests := []struct {
		name     string
		input    io.ReadCloser
		encoding string
		bufSize  int
		wantErr  bool
		wantData string
	}{
		{
			name:     "no compression",
			input:    io.NopCloser(strings.NewReader("hello world")),
			encoding: "",
			bufSize:  4096,
			wantErr:  false,
			wantData: "hello world",
		},
		{
			name:     "gzip normal",
			input:    io.NopCloser(strings.NewReader("hello world")),
			encoding: "gzip",
			bufSize:  4096,
			wantErr:  false,
			wantData: "hello world",
		},
		{
			name:     "deflate normal",
			input:    io.NopCloser(strings.NewReader("hello world")),
			encoding: "deflate",
			bufSize:  4096,
			wantErr:  false,
			wantData: "hello world",
		},
		{
			name:     "empty encoding",
			input:    io.NopCloser(strings.NewReader("hello world")),
			encoding: "",
			bufSize:  4096,
			wantErr:  false,
			wantData: "hello world",
		},
		{
			name:     "empty input",
			input:    io.NopCloser(strings.NewReader("")),
			encoding: "gzip",
			bufSize:  4096,
			wantErr:  false,
			wantData: "",
		},
		{
			name: "reader error",
			input: &mockReadCloser{
				ReadFunc: func(p []byte) (n int, err error) {
					return 0, errors.New("mock read error")
				},
				CloseFunc: func() error { return nil },
			},
			encoding: "gzip",
			bufSize:  4096,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr, err := newCompressReaderWithBufSize(tt.input, tt.encoding, tt.bufSize)
			if tt.encoding == "" {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, cr)
				// Read directly from the original reader
				data, err := io.ReadAll(cr)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.wantData, string(data))
				}
				return
			}
			require.NoError(t, err)

			var compressedData bytes.Buffer
			_, err = io.Copy(&compressedData, cr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			decompressAndVerify(t, tt.encoding, []byte(tt.wantData), &compressedData)
		})
	}
}

func TestCompressingReader_Fuzzy(t *testing.T) {
	seed := uint64(0)
	iterations := 1000

	rand.Seed(seed)

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("FuzzyTest_%d", i), func(t *testing.T) {
			dataSize := rand.Intn(rand.Intn(1 * 1024 * 100)) // [0, 100K]
			data := make([]byte, dataSize)
			_, err := rand.Read(data)
			require.NoError(t, err, "Failed to generate random data")

			// Randomly select encoding: gzip or deflate
			encodings := []string{"gzip", "deflate"}
			encoding := encodings[rand.Intn(len(encodings))]

			// Create compressingReader with random data, encoding, and buffer size
			reader := io.NopCloser(bytes.NewReader(data))
			bufSize := 1 + rand.Intn(4096) // [1, 4K]
			cr, err := newCompressReaderWithBufSize(reader, encoding, bufSize)
			require.NoError(t, err, "Failed to create compressingReader")

			// Read all compressed data
			var compressedBuf bytes.Buffer
			_, err = io.Copy(&compressedBuf, cr)
			require.NoError(t, err, "Failed to read compressed data")

			// Close the compressingReader (though underlying reader is NopCloser)
			err = cr.Close()
			require.NoError(t, err, "Failed to close compressingReader")

			decompressAndVerify(t, encoding, data, &compressedBuf)
		})
	}
}
