package ioutils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
)

type flusherType interface {
	Flush() error
}

type compressingReader struct {
	// reader --[buf]-> writer --> outputBuf
	reader    io.ReadCloser
	writer    io.WriteCloser
	outputBuf *bytes.Buffer

	buf []byte
	eof bool
}

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

func (r *compressingReader) Read(readBuf []byte) (n int, err error) {
	for len(readBuf) > 0 {
		// consume all remaining data in outputBuf
		consumeN, _ := r.outputBuf.Read(readBuf)
		n += consumeN
		readBuf = readBuf[consumeN:]

		// read enough, exit
		if len(readBuf) == 0 {
			return n, nil
		}

		// read from reader for more data
		if r.eof {
			return n, io.EOF
		}
		readN, readErr := r.reader.Read(r.buf)
		if readErr != nil && readErr != io.EOF {
			return n, readErr
		}

		// reset the output buf, prepare for write
		if r.outputBuf.Len() > 0 {
			return n, fmt.Errorf("should not happen")
		}
		r.outputBuf.Reset()

		// write new data to writer for compressing
		writeN, writeErr := r.writer.Write(r.buf[:readN]) // this will fill the outputBuf
		if writeErr != nil {
			return n, fmt.Errorf("compressor write error: %v", writeErr)
		}
		if writeN != readN {
			return n, fmt.Errorf("compressor write returns bad size, readN %d, writeN %d", readN, writeN)
		}
		if flusher, ok := r.writer.(flusherType); ok {
			if flushErr := flusher.Flush(); flushErr != nil {
				return n, flushErr
			}
		}
		if readErr == io.EOF {
			r.eof = true
			// finalizing the Compression Stream
			if closeErr := r.writer.Close(); closeErr != nil {
				return n, closeErr
			}
		}
	}
	return n, nil
}

func (r *compressingReader) Close() error {
	var err2 error
	err1 := r.reader.Close()
	if !r.eof { // early Close?
		err2 = r.writer.Close()
	}
	if err1 != nil {
		return err1
	} else {
		return err2
	}
}

func newCompressReaderWithBufSize(reader io.ReadCloser, encoding string, bufSize int) (io.ReadCloser, error) {
	if encoding == "" {
		return reader, nil
	}

	compressBuf := bytes.NewBuffer(make([]byte, 0, bufSize))
	var compressWriter io.WriteCloser
	var err error

	switch encoding {
	case "gzip":
		compressWriter = gzip.NewWriter(compressBuf)
	case "deflate":
		compressWriter, err = flate.NewWriter(compressBuf, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
	}

	return &compressingReader{
		reader:    reader,
		writer:    compressWriter,
		outputBuf: compressBuf,
		buf:       make([]byte, bufSize),
		eof:       false,
	}, nil
}
