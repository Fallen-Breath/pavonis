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
		settings: &config.SpeedTestSiteSettings{
			MaxUploadBytes: &maxUpload, MaxDownloadBytes: &maxDownload,
		},
	}
}

func TestServeHttpDownload(t *testing.T) {
	h := newTestHandler(0, 100_000)
	r := httptest.NewRequest(http.MethodGet, "/speed?bytes=50000", nil)
	w := httptest.NewRecorder()
	h.ServeHttp(nil, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "50000", w.Header().Get("Content-Length"))
	require.Equal(t, "identity", w.Header().Get("Content-Encoding"))
	require.Len(t, w.Body.Bytes(), 50000)
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

func TestServeHttpUploadLimit(t *testing.T) {
	h := newTestHandler(4, 0)
	r := httptest.NewRequest(http.MethodPost, "/speed", bytes.NewReader([]byte("1234")))
	w := httptest.NewRecorder()
	h.ServeHttp(nil, w, r)
	require.Equal(t, http.StatusOK, w.Code)

	r = httptest.NewRequest(http.MethodPost, "/speed", bytes.NewReader([]byte("12345")))
	w = httptest.NewRecorder()
	h.ServeHttp(nil, w, r)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}
