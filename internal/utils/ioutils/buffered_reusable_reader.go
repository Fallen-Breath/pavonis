package ioutils

import (
	"bytes"
	"io"
	"sync"
)

type BufferedReusableReader struct {
	mu            sync.Mutex
	reader        io.ReadCloser
	maxBufferSize int
	contentLength int64
	buffer        *bytes.Buffer
	fullyConsumed bool
}

func NewBufferedReusableReader(reader io.ReadCloser, maxBufferSize int) *BufferedReusableReader {
	return NewBufferedReusableReaderWithContentLength(reader, maxBufferSize, -1)
}

// NewBufferedReusableReaderWithContentLength also recognizes a complete body
// when the declared number of bytes has been read. Use -1 for unknown lengths.
func NewBufferedReusableReaderWithContentLength(reader io.ReadCloser, maxBufferSize int, contentLength int64) *BufferedReusableReader {
	return &BufferedReusableReader{
		reader:        reader,
		maxBufferSize: maxBufferSize,
		contentLength: contentLength,
		buffer:        bytes.NewBuffer([]byte{}),
		fullyConsumed: contentLength == 0,
	}
}

func (b *BufferedReusableReader) Read(buf []byte) (n int, err error) {
	n, err = b.reader.Read(buf)

	// Do not hold the mutex while reading: an upstream may redirect before
	// the client finishes uploading, and replay must not wait for that upload.
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buffer == nil {
		return n, err
	}
	bufferSize := b.buffer.Len() + n
	if bufferSize > b.maxBufferSize ||
		(b.contentLength >= 0 && int64(bufferSize) > b.contentLength) ||
		(err != nil && err != io.EOF) {
		b.fullyConsumed = false
		b.buffer = nil // An incomplete or oversized body cannot be replayed.
		return n, err
	}
	b.buffer.Write(buf[:n])
	if b.contentLength >= 0 {
		b.fullyConsumed = int64(bufferSize) == b.contentLength
	} else if err == io.EOF {
		b.fullyConsumed = true
	}

	return n, err
}

func (b *BufferedReusableReader) Close() error {
	return b.reader.Close()
}

func (b *BufferedReusableReader) GetNextReader() (io.ReadCloser, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fullyConsumed {
		// The transport may still be reading or closing the original body.
		// Give each replay its own stable snapshot.
		return io.NopCloser(bytes.NewReader(bytes.Clone(b.buffer.Bytes()))), true
	} else {
		return io.NopCloser(bytes.NewReader([]byte(""))), false
	}
}
