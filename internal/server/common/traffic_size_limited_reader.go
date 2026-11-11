package common

import (
	"fmt"
	"io"
	"net/http"
)

type TrafficSizeLimitedReadCloser struct {
	reader   io.ReadCloser
	maxSize  int64
	readSize int64
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
	if r.readSize > r.maxSize {
		return 0, NewHttpError(http.StatusBadGateway, "Response body too large")
	}
	n, err = r.reader.Read(p)
	r.readSize += int64(n)
	return n, err
}

func (r *TrafficSizeLimitedReadCloser) Close() error {
	return r.reader.Close()
}

func LimitResponseSize(resp *http.Response, limit int64) error {
	if resp.ContentLength > limit {
		return NewHttpError(http.StatusBadGateway, "Response ContentLength too large")
	}
	if resp.ContentLength < 0 {
		resp.Body = NewTrafficSizeLimitedReadCloser(resp.Body, limit)
	}
	return nil
}
