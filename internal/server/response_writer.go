package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/felixge/httpsnoop"
)

type responseMetrics struct {
	status  int
	written int64
}

func (m *responseMetrics) recordStatus(code int) {
	if m.status == 0 && (code >= 200 || code == http.StatusSwitchingProtocols) {
		m.status = code
	}
}

func (m *responseMetrics) recordWrite(n int, err error) (int, error) {
	if !errors.Is(err, http.ErrHijacked) {
		m.recordStatus(http.StatusOK)
	}
	m.written += int64(n)
	return n, err
}

func (m *responseMetrics) wrap(w http.ResponseWriter) http.ResponseWriter {
	var wrapped http.ResponseWriter
	wrapped = httpsnoop.Wrap(w, httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(code int) {
				next(code)
				m.recordStatus(code)
			}
		},
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(p []byte) (int, error) { return m.recordWrite(next(p)) }
		},
		WriteString: func(next httpsnoop.WriteStringFunc) httpsnoop.WriteStringFunc {
			return func(s string) (int, error) { return m.recordWrite(next(s)) }
		},
		ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
			return func(src io.Reader) (int64, error) {
				if m.status == 0 {
					// A reader may panic after writing bytes, before ReadFrom can report them.
					return io.Copy(struct{ io.Writer }{wrapped}, src)
				}
				n, err := next(src)
				m.written += n
				return n, err
			}
		},
		Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
			return func() {
				next()
				m.recordStatus(http.StatusOK)
			}
		},
		FlushError: func(next httpsnoop.FlushErrorFunc) httpsnoop.FlushErrorFunc {
			return func() error {
				err := next()
				if !errors.Is(err, http.ErrNotSupported) {
					m.recordStatus(http.StatusOK)
				}
				return err
			}
		},
	})
	return wrapped
}
