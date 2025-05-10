package pypiproxy

import (
	"bytes"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"io"
)

// ReplacingReader wraps an io.ReadCloser and replaces occurrences of a search byte slice with a replace byte slice.
type ReplacingReader struct {
	reader      io.ReadCloser
	search      []byte
	replace     []byte
	readBufSize int

	buf        []byte
	paddingLen int
	paddingBuf []byte
	pending    bytes.Buffer // Pending data not yet written to output
	eof        bool
}

var _ io.ReadCloser = &ReplacingReader{}

// NewReplacingReader creates a new ReplacingReader with a default buffer size.
func NewReplacingReader(reader io.ReadCloser, search []byte, replace []byte) *ReplacingReader {
	return NewReplacingReaderWithBufSize(reader, search, replace, 4096)
}

// NewReplacingReaderWithBufSize creates a new ReplacingReader with a specified buffer size.
func NewReplacingReaderWithBufSize(reader io.ReadCloser, search []byte, replace []byte, readBufSize int) *ReplacingReader {
	if search == nil || reader == nil {
		panic("search and reader cannot be nil")
	}

	paddingLen := utils.Max(0, len(search)-1)

	return &ReplacingReader{
		reader:      reader,
		search:      search,
		replace:     replace,
		readBufSize: readBufSize,

		paddingLen: paddingLen,
		paddingBuf: make([]byte, 0),
		buf:        make([]byte, paddingLen+readBufSize),
	}
}

func (r *ReplacingReader) Read(readBuf []byte) (n int, err error) {
	if len(r.search) == 0 {
		return r.reader.Read(readBuf)
	}

	n = 0
	if r.pending.Len() > 0 {
		copiedN, _ := r.pending.Read(readBuf)
		n += copiedN
		readBuf = readBuf[copiedN:]
		if len(readBuf) == 0 {
			return n, nil
		}
	}
	if r.pending.Len() > 0 {
		return n, fmt.Errorf("should not happen")
	}
	if r.eof {
		return n, io.EOF
	}

	for len(readBuf) > 0 {
		// move the padding buf to the head
		paddingN := copy(r.buf, r.paddingBuf)

		// [      r.buf (totalBufLen)     ]
		// [ oldPadding ][    read data   ]
		//
		// [                     newData                ]
		// [          ready data          ][ newPadding ]
		readN, readErr := r.reader.Read(r.buf[paddingN:])
		if readErr != nil && readErr != io.EOF {
			return n, readErr
		}
		totalBufLen := paddingN + readN

		newData, lastMatchIdx := replaceAll(r.buf[:totalBufLen], r.search, r.replace)

		var readyData []byte
		if readErr == io.EOF {
			r.paddingBuf = make([]byte, 0)
			readyData = newData
			r.eof = true
		} else {
			newPaddingLen := utils.Min(len(newData), r.paddingLen)
			if lastMatchIdx >= 0 {
				newPaddingLen = utils.Min(newPaddingLen, len(newData)-lastMatchIdx)
			}
			r.paddingBuf = make([]byte, newPaddingLen)
			readyLen := len(newData) - newPaddingLen
			copy(r.paddingBuf, newData[readyLen:])
			readyData = newData[:readyLen]
		}

		consumeN := copy(readBuf, readyData)
		n += consumeN
		readBuf = readBuf[consumeN:]
		if consumeN < len(readyData) {
			// readBuf should be empty
			r.pending.Write(readyData[consumeN:])
		}

		if r.eof {
			break
		}
	}
	return n, nil
}

// Close closes the underlying reader.
func (r *ReplacingReader) Close() error {
	return r.reader.Close()
}

// lastMatchIdx at     v
// result = "#new###new##"
func replaceAll(s, old, new []byte) ([]byte, int) {
	if len(old) == 0 {
		result := make([]byte, len(s))
		copy(result, s)
		return result, -1
	}

	var result []byte
	lastMatchIdx := -1
	start := 0
	for {
		idx := bytes.Index(s[start:], old)
		if idx == -1 {
			if lastMatchIdx == -1 {
				return s, lastMatchIdx
			}
			result = append(result, s[start:]...)
			break
		}

		result = append(result, s[start:start+idx]...)
		result = append(result, new...)
		lastMatchIdx = len(result)

		start = start + idx + len(old)
	}

	return result, lastMatchIdx
}
