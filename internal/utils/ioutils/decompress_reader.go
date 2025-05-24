package ioutils

import (
	"compress/flate"
	"compress/gzip"
	"io"
)

func NewDecompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
	if encoding == "" {
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
	default:
		return nil, UnsupportedEncodingError
	}
}
