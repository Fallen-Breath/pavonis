package ioutils

import (
	"bytes"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"io"
)

type SearchFunc func(buf []byte, lookBehindBuf []byte, eof bool) (idx int, length int, replacement []byte)

// ReplacingReader wraps an io.ReadCloser and replaces occurrences of a search byte slice with a replace byte slice.
type ReplacingReader struct {
	reader         io.ReadCloser
	readBufSize    int
	maxSearchLen   int
	lookBehindSize int
	lookBehindBuf  []byte
	searchFunc     SearchFunc

	buf        []byte
	paddingLen int
	paddingBuf []byte
	pending    bytes.Buffer // Pending data not yet written to output
	eof        bool
}

var _ io.ReadCloser = &ReplacingReader{}

const defaultReadBufSize = 4096

func NewLiteralReplacingReader(reader io.ReadCloser, search []byte, replace []byte) *ReplacingReader {
	return NewLiteralReplacingReaderWithBufSize(reader, search, replace, defaultReadBufSize)
}

func NewReplacingReader(reader io.ReadCloser, searchFunc SearchFunc, maxSearchLen, lookBehindSize int) *ReplacingReader {
	return NewReplacingReaderWithBufSize(reader, searchFunc, defaultReadBufSize, maxSearchLen, lookBehindSize)
}

func NewLiteralReplacingReaderWithBufSize(reader io.ReadCloser, search []byte, replace []byte, readBufSize int) *ReplacingReader {
	if search == nil || reader == nil {
		panic("search and reader cannot be nil")
	}

	searchFunc := func(buf []byte, _ []byte, _ bool) (int, int, []byte) {
		idx := bytes.Index(buf, search)
		if idx == -1 {
			return -1, -1, nil
		}
		return idx, len(search), replace
	}

	return NewReplacingReaderWithBufSize(reader, searchFunc, readBufSize, len(search), 0)
}

func NewReplacingReaderWithBufSize(reader io.ReadCloser, searchFunc SearchFunc, readBufSize, maxSearchLen, lookBehindSize int) *ReplacingReader {
	if readBufSize <= 0 {
		panic(fmt.Sprintf("readBufSize must be > 0, got %d", readBufSize))
	}

	paddingLen := utils.Max(0, maxSearchLen-1)
	return &ReplacingReader{
		reader:         reader,
		readBufSize:    readBufSize,
		maxSearchLen:   maxSearchLen,
		lookBehindSize: lookBehindSize,
		lookBehindBuf:  make([]byte, 0),
		searchFunc:     searchFunc,

		paddingLen: paddingLen,
		paddingBuf: make([]byte, 0),
		buf:        make([]byte, paddingLen+readBufSize),
	}
}

func (r *ReplacingReader) Read(readBuf []byte) (n int, err error) {
	if r.maxSearchLen == 0 {
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
	r.pending.Reset()
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
		//                [ lookBehindBuf ]
		readN, readErr := r.reader.Read(r.buf[paddingN:])
		if readErr != nil && readErr != io.EOF {
			return n, readErr
		}
		totalBufLen := paddingN + readN

		if readErr == io.EOF {
			r.eof = true
		}
		newData, lastMatchIdx := r.replaceAll(r.buf[:totalBufLen], r.lookBehindBuf)

		var readyData []byte
		if readErr == io.EOF {
			r.paddingBuf = make([]byte, 0)
			readyData = newData
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

		// consume readyData, transfer to the output readBuf
		{
			consumeN := copy(readBuf, readyData)
			n += consumeN
			readBuf = readBuf[consumeN:]
			if consumeN < len(readyData) {
				// readBuf should be empty
				r.pending.Write(readyData[consumeN:])
			}
		}

		// update lookBehindBuf
		r.lookBehindBuf = r.updateLookBehindBuf(r.lookBehindBuf, readyData)

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
func (r *ReplacingReader) replaceAll(s []byte, lookBehindBuf []byte) ([]byte, int) {
	var result []byte
	lastMatchIdx := -1
	start := 0
	for {
		idx, oldLen, newBuf := r.searchFunc(s[start:], lookBehindBuf, r.eof)
		if idx == -1 {
			if lastMatchIdx == -1 {
				return s, lastMatchIdx
			}
			result = append(result, s[start:]...)
			break
		}

		if len(newBuf) < r.lookBehindSize { // otherwise the next updateLookBehindBuf() call will override this
			lookBehindBuf = r.updateLookBehindBuf(lookBehindBuf, s[start:start+idx])
		}
		lookBehindBuf = r.updateLookBehindBuf(lookBehindBuf, newBuf)

		result = append(result, s[start:start+idx]...)
		result = append(result, newBuf...)
		lastMatchIdx = len(result)

		start = start + idx + oldLen
	}

	return result, lastMatchIdx
}

func (r *ReplacingReader) updateLookBehindBuf(oldBuf, newData []byte) (newBuf []byte) {
	// [                   newBuf                ]
	// [          oldBuf      ][   newDeltaBuf   ]
	newDeltaLen := utils.Min(r.lookBehindSize, len(newData))
	var newLookBehindBuf []byte
	if newDeltaLen < len(oldBuf) {
		newLookBehindBuf = append(newLookBehindBuf, oldBuf[len(oldBuf)-newDeltaLen:]...)
	}

	newDeltaBuf := newData[len(newData)-newDeltaLen:]
	newLookBehindBuf = append(newLookBehindBuf, newDeltaBuf...)
	return newLookBehindBuf
}
