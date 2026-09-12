package speedtest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/stretchr/testify/require"
)

func newTestHandler(maxUpload, maxDownload int64) *speedTestHandler {
	return &speedTestHandler{
		info: &handler.Info{Id: "speed", PathPrefix: "/speed"},
		settings: &config.SpeedTestSettings{
			MaxUploadBytes: &maxUpload, MaxDownloadBytes: &maxDownload,
		},
	}
}

func TestServeHttpDownload(t *testing.T) {
	h := newTestHandler(32, 10)
	r := httptest.NewRequest(http.MethodGet, "/speed?bytes=7", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(nil, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "7", w.Header().Get("Content-Length"))
	require.Equal(t, "identity", w.Header().Get("Content-Encoding"))
	require.Len(t, w.Body.Bytes(), 7)
}

func TestServeHttpRejectsInvalidDownload(t *testing.T) {
	tests := []string{"-1", "abc", "11"}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			h := newTestHandler(32, 10)
			r := httptest.NewRequest(http.MethodGet, "/speed?bytes="+value, nil)
			w := httptest.NewRecorder()
			h.ServeHttp(nil, w, r)
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestServeHttpUploadLimitAndDrain(t *testing.T) {
	h := newTestHandler(4, 0)
	r := httptest.NewRequest(http.MethodPost, "/speed", bytes.NewReader([]byte("1234")))
	w := httptest.NewRecorder()
	h.ServeHttp(nil, w, r)
	require.Equal(t, http.StatusOK, w.Code)

	r = httptest.NewRequest(http.MethodPost, "/speed", bytes.NewReader([]byte("12345")))
	w = httptest.NewRecorder()
	h.ServeHttp(nil, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFillWriterStopsAtRequestedSize(t *testing.T) {
	h := newTestHandler(0, 0)
	var out bytes.Buffer
	require.NoError(t, h.fillWriter(&out, 16384+3))
	require.Len(t, out.Bytes(), 16387)
	require.Equal(t, byte('0'), out.Bytes()[0])
}
