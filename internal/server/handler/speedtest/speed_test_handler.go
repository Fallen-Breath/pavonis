package speedtest

import (
	"bytes"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"io"
	"net/http"
	"strconv"
	"sync"
)

type speedTestHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.SpeedTestSettings
}

var _ handler.HttpHandler = &speedTestHandler{}

func NewSpeedTestHandler(info *handler.Info, helper *common.RequestHelper, settings *config.SpeedTestSettings) (handler.HttpHandler, error) {
	return &speedTestHandler{
		info:     info,
		helper:   helper,
		settings: settings,
	}, nil
}

func (h *speedTestHandler) Info() *handler.Info {
	return h.info
}

func (h *speedTestHandler) Shutdown() {
}

func (h *speedTestHandler) ServeHttp(_ *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 0 {
		if *h.settings.MaxUploadBytes < 0 {
			http.Error(w, "Upload test is disabled", http.StatusBadRequest)
			return
		}
		if r.ContentLength > *h.settings.MaxUploadBytes {
			http.Error(w, fmt.Sprintf("Upload size too much (%d > %d)", r.ContentLength, *h.settings.MaxUploadBytes), http.StatusBadRequest)
			return
		}
		if err := h.drainReader(r.Body); err != nil {
			http.Error(w, "read request failed", http.StatusBadRequest)
			return
		}
	}

	downloadSizeStr := r.URL.Query().Get("bytes")
	if downloadSizeStr == "" {
		// no download, ok
		return
	}

	if *h.settings.MaxDownloadBytes < 0 {
		http.Error(w, "Download test is disabled", http.StatusBadRequest)
		return
	}
	downloadSize, err := strconv.ParseInt(downloadSizeStr, 10, 64)
	if err != nil || downloadSize < 0 {
		http.Error(w, "`bytes` must be a non-negative integer", http.StatusBadRequest)
		return
	}
	if downloadSize > *h.settings.MaxDownloadBytes {
		http.Error(w, fmt.Sprintf("Download size too much (%d > %d)", downloadSize, *h.settings.MaxDownloadBytes), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(downloadSize, 10))
	w.Header().Set("Content-Encoding", "identity")

	if err := h.fillWriter(w, downloadSize); err != nil {
		http.Error(w, "Write response failed", http.StatusBadRequest)
		return
	}
}

var drainReaderBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 16384)
	},
}

func (h *speedTestHandler) drainReader(body io.ReadCloser) error {
	defer func() {
		_ = body.Close()
	}()

	buf := drainReaderBufPool.Get().([]byte)
	defer drainReaderBufPool.Put(buf)

	for {
		_, err := body.Read(buf)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}
}

var zeroChunk = bytes.Repeat([]byte{'0'}, 16384)

func (h *speedTestHandler) fillWriter(w io.Writer, size int64) error {
	remaining := size
	for remaining > 0 {
		chunkSize := int64(len(zeroChunk))
		if remaining < chunkSize {
			chunkSize = remaining
		}
		writeNum, err := w.Write(zeroChunk[:chunkSize])
		if err != nil {
			return err
		}
		remaining -= int64(writeNum)
	}
	return nil
}
