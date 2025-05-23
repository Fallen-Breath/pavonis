package ioutils

import (
	"io"
)

func NewCompressReader(reader io.ReadCloser, encoding string) (io.ReadCloser, error) {
	return newCompressReaderWithBufSize(reader, encoding, 4096)
}
