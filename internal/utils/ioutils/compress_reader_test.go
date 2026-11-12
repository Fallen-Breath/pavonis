package ioutils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
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
	case "", "identity":
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
	case "br":
		brReader := brotli.NewReader(compressedData)
		_, err := io.Copy(&decompressedData, brReader)
		require.NoError(t, err)
	case "zstd":
		zr, err := zstd.NewReader(compressedData)
		require.NoError(t, err)
		zrCloser := &zstdReadCloser{Decoder: zr}
		defer closeCloser(zrCloser)
		_, err = io.Copy(&decompressedData, zrCloser)
		require.NoError(t, err)
	default:
		t.Fatalf("unsupported encoding: %s", encoding)
	}

	assert.Equal(t, expectedData, decompressedData.Bytes())
}

func TestCompressingReader(t *testing.T) {
	for _, encoding := range []string{"", "identity"} {
		input := io.NopCloser(strings.NewReader("hello world"))
		cr, err := newCompressReaderWithBufSize(input, encoding, 4096)
		require.NoError(t, err)
		require.Equal(t, input, cr, "encoding %q must not wrap the reader", encoding)
	}

	t.Run("empty input", func(t *testing.T) {
		cr, err := newCompressReaderWithBufSize(io.NopCloser(strings.NewReader("")), "gzip", 4096)
		require.NoError(t, err)
		var compressed bytes.Buffer
		_, err = io.Copy(&compressed, cr)
		require.NoError(t, err)
		decompressAndVerify(t, "gzip", []byte{}, &compressed)
	})

	t.Run("reader error", func(t *testing.T) {
		input := &mockReadCloser{
			ReadFunc:  func(p []byte) (n int, err error) { return 0, errors.New("mock read error") },
			CloseFunc: func() error { return nil },
		}
		cr, err := newCompressReaderWithBufSize(input, "gzip", 4096)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, cr)
		require.Error(t, err)
	})
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

			// Randomly select a supported encoding
			encodings := []string{"gzip", "deflate", "br", "zstd"}
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
