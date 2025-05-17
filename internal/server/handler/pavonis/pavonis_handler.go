package pavonis

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/constants"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strings"
)

type pavonisHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.PavonisSiteSettings

	promHandler http.Handler
}

var _ handler.HttpHandler = &pavonisHandler{}

func NewPavonisHandler(info *handler.Info, helper *common.RequestHelper, settings *config.PavonisSiteSettings) (handler.HttpHandler, error) {
	return &pavonisHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		promHandler: promhttp.Handler(),
	}, nil
}

func (h *pavonisHandler) Info() *handler.Info {
	return h.info
}

func (h *pavonisHandler) Shutdown() {
}

func (h *pavonisHandler) ServeHttp(_ *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}
	reqPath := r.URL.Path[len(h.info.PathPrefix):]

	reqPath = strings.TrimSuffix(reqPath, "/")
	switch reqPath {
	case "":
		_, _ = w.Write([]byte(fmt.Sprintf("Pavonis v%s", constants.Version)))
	case "/metrics":
		h.promHandler.ServeHTTP(w, r)
	default:
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}
