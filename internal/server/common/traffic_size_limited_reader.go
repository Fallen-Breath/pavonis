package common

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

type TrafficSizeLimitedReadCloser struct {
	reader   io.ReadCloser
	maxSize  int64
	readSize int64
	exceeded atomic.Bool
}

var _ io.ReadCloser = &TrafficSizeLimitedReadCloser{}

func NewTrafficSizeLimitedReadCloser(reader io.ReadCloser, maxSize int64) *TrafficSizeLimitedReadCloser {
	if reader == nil {
		panic(fmt.Errorf("reader must not be nil"))
	}
	return &TrafficSizeLimitedReadCloser{
		reader:   reader,
		maxSize:  maxSize,
		readSize: 0,
	}
}

func (r *TrafficSizeLimitedReadCloser) Read(p []byte) (n int, err error) {
	if r.exceeded.Load() {
		return 0, NewHttpError(http.StatusBadGateway, "Response body too large")
	}
	n, err = r.reader.Read(p)
	r.readSize += int64(n)
	// checked after the read, since the reader may return the last chunk together with io.EOF
	if r.readSize > r.maxSize {
		r.exceeded.Store(true)
		n -= int(r.readSize - r.maxSize) // never pass through bytes beyond the limit
		r.readSize = r.maxSize
		return n, NewHttpError(http.StatusBadGateway, "Response body too large")
	}
	return n, err
}

// Exceeded is safe to be called from other goroutines
func (r *TrafficSizeLimitedReadCloser) Exceeded() bool {
	return r.exceeded.Load()
}

func (r *TrafficSizeLimitedReadCloser) Close() error {
	return r.reader.Close()
}
