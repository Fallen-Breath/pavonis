package utils

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
	"io"
	"testing"
)

func testOneCase(t *testing.T, data []byte, maxBufferSize int, maxReadSize, fixedReadSize *int) {
	brc := NewBufferedReadCloser(io.NopCloser(bytes.NewReader(data)), maxBufferSize)

	readData := make([]byte, 0)

	var bufLen int
	if maxReadSize != nil {
		bufLen = *maxReadSize
	} else if fixedReadSize != nil {
		bufLen = *fixedReadSize
	} else {
		panic("both maxReadSize and fixedReadSize are nil")
	}

	buf := make([]byte, bufLen)
	for {
		var readSize int
		if maxReadSize != nil {
			readSize = rand.Intn(*maxReadSize) + 1
		} else if fixedReadSize != nil {
			readSize = *fixedReadSize
		} else {
			panic("impossible")
		}
		readBuf := buf[:Min(readSize, len(buf))]
		n, err := brc.Read(readBuf)
		require.LessOrEqual(t, n, len(readBuf))
		if n > 0 {
			readData = append(readData, readBuf[:n]...)
		}
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
	}

	assert.Equal(t, data, readData)

	if len(data) <= maxBufferSize {
		assert.True(t, brc.fullyConsumed)
		nextReader, ok := brc.GetNextReadCloser()
		assert.True(t, ok)
		nextData, err := io.ReadAll(nextReader)
		assert.NoError(t, err)
		assert.Equal(t, data, nextData)
	} else {
		assert.False(t, brc.fullyConsumed)
		nextReader, ok := brc.GetNextReadCloser()
		assert.False(t, ok)
		nextData, err := io.ReadAll(nextReader)
		assert.NoError(t, err)
		assert.Empty(t, nextData)
	}
}

func TestBufferedReadCloser(t *testing.T) {
	rand.Seed(0)

	tests := []struct {
		name          string
		data          []byte
		maxBufferSize int
		readSize      int
	}{
		{
			name:          "EmptyData",
			data:          []byte(""),
			maxBufferSize: 0,
			readSize:      1,
		},
		{
			name:          "SingleByteZeroBuffer",
			data:          []byte("a"),
			maxBufferSize: 0,
			readSize:      1,
		},
		{
			name:          "SmallDataLargeBuffer",
			data:          []byte("hello"),
			maxBufferSize: 10,
			readSize:      1,
		},
		{
			name:          "LargeDataSmallBuffer",
			data:          []byte("hello world"),
			maxBufferSize: 5,
			readSize:      10,
		},
		{
			name: "LargeDataTinyBuffer",
			data: func() []byte {
				data := make([]byte, 1024)
				for i := range data {
					data[i] = byte(i % 256) // Deterministic bytes
				}
				return data
			}(),
			maxBufferSize: 1,
			readSize:      1,
		},
		{
			name: "MediumDataLargeBuffer",
			data: func() []byte {
				data := make([]byte, 500)
				for i := range data {
					data[i] = byte(i % 256) // Deterministic bytes
				}
				return data
			}(),
			maxBufferSize: 1000,
			readSize:      1000,
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testOneCase(t, tt.data, tt.maxBufferSize, nil, ToPtr(tt.readSize))
		})
	}
}

func TestBufferedReadCloserFuzzy(t *testing.T) {
	seed := uint64(0)
	iterations := 1000

	rand.Seed(seed)

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("FuzzyTest_%d", i), func(t *testing.T) {
			dataLen := rand.Intn(8192)
			data := make([]byte, dataLen)
			_, _ = rand.Read(data)

			maxBufferSize := rand.Intn(dataLen * 2)
			readSize := rand.Intn(dataLen+1) + 1

			testOneCase(t, data, maxBufferSize, &readSize, nil)
		})
	}
}
