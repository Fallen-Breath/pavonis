package ioutils

import (
	"compress/flate"
	"compress/gzip"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"io"
)

func NewDecompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
	if encoding == "" || encoding == "identity" {
		return reader, nil
	}

	switch encoding {
	case "gzip":
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		return gz, nil
	case "deflate":
		return flate.NewReader(reader), nil
	case "br":
		return io.NopCloser(brotli.NewReader(reader)), nil
	case "zstd":
		zr, err := zstd.NewReader(reader)
		if err != nil {
			return nil, err
		}
		return &zstdReadCloser{Decoder: zr}, nil
	default:
		return nil, UnsupportedEncodingError
	}
}

type zstdReadCloser struct {
	*zstd.Decoder
}

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

var _ io.ReadCloser = &zstdReadCloser{}
