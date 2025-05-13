package pypiproxy

import (
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
)

var unsupportedEncodingError = errors.New("unsupported encoding")

func newDecompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
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
		return nil, unsupportedEncodingError
	}
}

func newCompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
	return newCompressReaderWithBufSize(reader, encoding, 4096)
}
