package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type failedFlushWriter struct{ *httptest.ResponseRecorder }

func (failedFlushWriter) FlushError() error { return os.ErrDeadlineExceeded }

func TestResponseMetricsPreserveFlushError(t *testing.T) {
	var m responseMetrics
	w := m.wrap(failedFlushWriter{httptest.NewRecorder()})
	require.ErrorIs(t, http.NewResponseController(w).Flush(), os.ErrDeadlineExceeded)
}
