package speedtest

import (
	"bytes"
	"errors"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
	"sync"
)

type speedTestHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.SpeedTestSiteSettings
}

var _ handler.HttpHandler = &speedTestHandler{}

func NewSpeedTestHandler(info *handler.Info, helper *common.RequestHelper, settings *config.SpeedTestSiteSettings) (handler.HttpHandler, error) {
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

func (h *speedTestHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	logPrefix := ""
	if ctx != nil {
		logPrefix = ctx.LogPrefix
	}
	maxUploadBytes := *h.settings.MaxUploadBytes
	if r.ContentLength > 0 {
		if maxUploadBytes < 0 {
			http.Error(w, "Upload test is disabled", http.StatusBadRequest)
			return
		}
		if r.ContentLength > maxUploadBytes {
			log.Debugf("%sUpload too large: %d > %d bytes", logPrefix, r.ContentLength, maxUploadBytes)
			http.Error(w, "Upload too large", http.StatusRequestEntityTooLarge)
			return
		}
	}

	if err := h.drainReader(http.MaxBytesReader(w, r.Body, max(maxUploadBytes, 0))); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			if maxUploadBytes < 0 {
				http.Error(w, "Upload test is disabled", http.StatusBadRequest)
			} else {
				log.Debugf("%sUpload exceeds size limit of %d bytes", logPrefix, maxUploadBytes)
				http.Error(w, "Upload too large", http.StatusRequestEntityTooLarge)
			}
		} else {
			log.Warnf("%sFailed to read request body: %v", logPrefix, err)
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
		}
		return
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
		log.Debugf("%sDownload size exceeds limit: %d > %d bytes", logPrefix, downloadSize, *h.settings.MaxDownloadBytes)
		http.Error(w, "Download size exceeds limit", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(downloadSize, 10))
	w.Header().Set("Content-Encoding", "identity")

	if err := h.fillWriter(w, downloadSize); err != nil {
		log.Warnf("%sFailed to write speed test response: %v", logPrefix, err)
		panic(http.ErrAbortHandler)
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
