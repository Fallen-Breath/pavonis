package staticfile

import (
	"net/http"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

const defaultContentType = "text/plain; charset=utf-8"

// staticFileHandler serves the configured static files of a site, and passes other requests to the site handler
type staticFileHandler struct {
	inner       handler.HttpHandler
	helper      *common.RequestHelper
	staticFiles map[string]*config.StaticFileConfig
}

var _ handler.HttpHandler = &staticFileHandler{}

func WrapHandler(inner handler.HttpHandler, helper *common.RequestHelper, staticFiles []*config.StaticFileConfig) handler.HttpHandler {
	staticFileByPath := make(map[string]*config.StaticFileConfig)
	for _, staticFile := range staticFiles {
		staticFileByPath[staticFile.Path] = staticFile
	}
	return &staticFileHandler{
		inner:       inner,
		helper:      helper,
		staticFiles: staticFileByPath,
	}
}

func (h *staticFileHandler) Info() *handler.Info {
	return h.inner.Info()
}

func (h *staticFileHandler) Shutdown() {
	h.inner.Shutdown()
}

func (h *staticFileHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	path := h.Info().EscapedSitePath(r.URL)
	if path == "" {
		path = "/"
	}
	staticFile, ok := h.staticFiles[path]
	if !ok {
		h.inner.ServeHttp(ctx, w, r)
		return
	}

	if !h.helper.AllowRequest(ctx) {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if staticFile.Content != nil {
		contentType := staticFile.ContentType
		if contentType == "" {
			contentType = defaultContentType
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(*staticFile.Content))
		return
	}
	if err := serveFile(w, r, staticFile); err != nil {
		log.Errorf("%sFailed to serve static file: %v", ctx.LogPrefix, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// serveFile writes nothing to w if an error is returned
func serveFile(w http.ResponseWriter, r *http.Request, staticFile *config.StaticFileConfig) error {
	f, info, err := utils.OpenRegularFile(staticFile.File)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if staticFile.ContentType != "" {
		w.Header().Set("Content-Type", staticFile.ContentType)
	}
	// unlike http.ServeFile, it does not redirect paths ending with "/index.html"
	http.ServeContent(w, r, staticFile.File, info.ModTime(), f)
	return nil
}
