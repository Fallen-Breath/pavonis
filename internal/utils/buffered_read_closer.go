package utils

import (
	"bytes"
	"io"
)

type BufferedReadCloser struct {
	reader        io.ReadCloser
	maxBufferSize int
	buffer        *bytes.Buffer
	fullyConsumed bool
}

func NewBufferedReadCloser(reader io.ReadCloser, maxBufferSize int) *BufferedReadCloser {
	return &BufferedReadCloser{
		reader:        reader,
		maxBufferSize: maxBufferSize,
		buffer:        bytes.NewBuffer([]byte{}),
		fullyConsumed: true,
	}
}

func (b *BufferedReadCloser) Read(buf []byte) (n int, err error) {
	n, err = b.reader.Read(buf)
	if n > 0 && b.fullyConsumed {
		if b.buffer.Len()+n <= b.maxBufferSize {
			b.buffer.Write(buf[:n])
		} else {
			b.fullyConsumed = false
			b.buffer = nil // useless now
		}
	}

	return n, err
}

func (b *BufferedReadCloser) Close() error {
	return b.reader.Close()
}

func (b *BufferedReadCloser) GetNextReadCloser() (io.ReadCloser, bool) {
	if b.fullyConsumed {
		return io.NopCloser(bytes.NewReader(b.buffer.Bytes())), true
	} else {
		return io.NopCloser(bytes.NewReader([]byte(""))), false
	}
}
