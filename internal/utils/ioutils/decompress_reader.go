package ioutils

import (
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func NewDecompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
	if encoding == "" || encoding == "identity" {
		return reader, nil
	}

	input := &closeOnceReadCloser{ReadCloser: reader} // in case the decoder close it before the Close() call
	var decoder io.ReadCloser
	var err error
	switch encoding {
	case "gzip":
		decoder, err = gzip.NewReader(input)
	case "deflate":
		decoder, err = zlib.NewReader(input)
	case "br":
		decoder = io.NopCloser(brotli.NewReader(input))
	case "zstd":
		zr, err := zstd.NewReader(input)
		if err != nil {
			return nil, err
		}
		decoder = &zstdReadCloser{Decoder: zr}
	default:
		return nil, UnsupportedEncodingError
	}
	if err != nil {
		return nil, err
	}
	return &decompressingReader{ReadCloser: decoder, input: input}, nil
}

type closeOnceReadCloser struct {
	io.ReadCloser
	closed bool
}

var _ io.ReadCloser = &closeOnceReadCloser{}

func (r *closeOnceReadCloser) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.ReadCloser.Close()
}

type decompressingReader struct {
	io.ReadCloser
	input io.Closer
}

var _ io.ReadCloser = &decompressingReader{}

func (r *decompressingReader) Close() error {
	// Close the input first so a decoder can stop any blocked background reads.
	inputErr := r.input.Close()
	return errors.Join(inputErr, r.ReadCloser.Close())
}

type zstdReadCloser struct {
	*zstd.Decoder
}

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

var _ io.ReadCloser = &zstdReadCloser{}
