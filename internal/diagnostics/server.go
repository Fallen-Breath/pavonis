package diagnostics

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/constants"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"net/http/pprof"
	"strings"
)

type Server struct {
	cfg *config.DiagnosticsConfig
}

func NewServer(cfg *config.DiagnosticsConfig) *Server {
	return &Server{
		cfg: cfg,
	}
}

func (s *Server) CreateHttpServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", s.createRootHandler())
	mux.HandleFunc("/metrics", s.createMetricsHandler())
	mux.HandleFunc("/debug/pprof/", pprof.Index)

	return &http.Server{
		Addr:    *s.cfg.Listen,
		Handler: mux,
	}
}

func (s *Server) createRootHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf("Pavonis v%s", constants.Version)))
	}
}

func (s *Server) createMetricsHandler() func(http.ResponseWriter, *http.Request) {
	promHandler := promhttp.Handler()
	return promHandler.ServeHTTP
}

func (s *Server) createDebugPprofHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if pprofName, found := strings.CutPrefix(r.URL.Path, "/debug/pprof/"); found {
			// ref: net/http/pprof/pprof.go, init()
			switch pprofName {
			case "":
				pprof.Index(w, r)
			case "cmdline":
				pprof.Cmdline(w, r)
			case "profile":
				pprof.Profile(w, r)
			case "symbol":
				pprof.Symbol(w, r)
			case "trace":
				pprof.Trace(w, r)
			default:
				pprof.Handler(pprofName).ServeHTTP(w, r)
			}
		} else {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		}
	}
}
