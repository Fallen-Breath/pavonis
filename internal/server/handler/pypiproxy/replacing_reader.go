package pypiproxy

import (
	"bytes"
	"io"
)

// ReplacingReader wraps an io.ReadCloser and replaces occurrences of a search byte slice with a replace byte slice.
type ReplacingReader struct {
	reader     io.ReadCloser
	search     []byte
	replace    []byte
	buf        []byte // Buffer for reading data
	pending    []byte // Pending data not yet written to output
	searchLen  int
	replaceLen int
}

var _ io.ReadCloser = &ReplacingReader{}

// NewReplacingReader creates a new ReplacingReader.
func NewReplacingReader(r io.ReadCloser, search []byte, replace []byte) *ReplacingReader {
	if len(search) == 0 {
		panic("search byte slice cannot be empty")
	}
	return &ReplacingReader{
		reader:     r,
		search:     search,
		replace:    replace,
		buf:        make([]byte, 4096), // Adjust buffer size as needed
		pending:    make([]byte, 0),
		searchLen:  len(search),
		replaceLen: len(replace),
	}
}

// Read reads data from the underlying reader, replacing occurrences of search with replace.
func (r *ReplacingReader) Read(p []byte) (n int, err error) {
	if len(r.pending) > 0 {
		n = copy(p, r.pending)
		r.pending = r.pending[n:]
		if len(r.pending) == 0 {
			r.pending = nil // Reset to avoid keeping large slices
		}
		return n, nil
	}

	for {
		// Read data into buf
		n, err := r.reader.Read(r.buf)
		if n == 0 && err != nil {
			if err == io.EOF && len(r.pending) > 0 {
				// Output remaining pending data
				n = copy(p, r.pending)
				r.pending = r.pending[n:]
				if len(r.pending) == 0 {
					r.pending = nil
				}
				return n, nil
			}
			return 0, err
		}

		data := r.buf[:n]
		start := 0

		for {
			// Find the next occurrence of search in data starting from start
			idx := bytes.Index(data[start:], r.search)
			if idx == -1 {
				// No more matches, append remaining data to pending
				r.pending = append(r.pending, data[start:]...)
				break
			}

			// Append data before the match to pending
			r.pending = append(r.pending, data[start:start+idx]...)

			// Append replace to pending
			r.pending = append(r.pending, r.replace...)

			// Move start past the matched search
			start += idx + r.searchLen
		}

		// If pending has data, try to copy to p
		if len(r.pending) > 0 {
			n = copy(p, r.pending)
			r.pending = r.pending[n:]
			if len(r.pending) == 0 {
				r.pending = nil
			}
			return n, nil
		}

		// If no data was copied and we reached EOF, return
		if err == io.EOF {
			return 0, io.EOF
		}
	}
}

// Close closes the underlying reader.
func (r *ReplacingReader) Close() error {
	return r.reader.Close()
}
