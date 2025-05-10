package pypiproxy

import (
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
)

var unsupportedEncodingError = errors.New("unsupported encoding")

func decompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
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

func compressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
	if encoding == "" {
		return reader, nil
	}

	pr, pw := io.Pipe()
	var writer io.WriteCloser
	var err error

	switch encoding {
	case "gzip":
		writer = gzip.NewWriter(pw)
	case "deflate":
		writer, err = flate.NewWriter(pw, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
	default:
		return nil, unsupportedEncodingError
	}

	go func() {
		_, copyErr := io.Copy(writer, reader)
		_ = writer.Close()
		_ = pw.CloseWithError(copyErr)
	}()

	return pr, nil
}
