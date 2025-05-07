package proxy

import (
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"net/http"
	"net/url"
	"strings"
)

type GithubProxyHandler struct {
	helper   *common.RequestHelper
	settings *config.GithubDownloadProxySettings
}

var _ http.Handler = &GithubProxyHandler{}

var allowedHosts = map[string]bool{
	"github.com":                 true,
	"raw.githubusercontent.com":  true,
	"gist.github.com":            true,
	"gist.githubusercontent.com": true,
}

func NewGithubProxyHandler(helper *common.RequestHelper, settings *config.GithubDownloadProxySettings) *GithubProxyHandler {
	return &GithubProxyHandler{
		helper:   helper,
		settings: settings,
	}
}

func (h *GithubProxyHandler) parseTargetUrl(w http.ResponseWriter, r *http.Request) (*url.URL, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return nil, false
	}
	targetURLStr := path[1:] // Remove leading "/"

	targetURL, err := url.Parse(targetURLStr)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return nil, false
	}
	if targetURL.Scheme == "" {
		targetURL.Scheme = "https"
	}
	if targetURL.Scheme != "https" {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return nil, false
	}

	return targetURL, true
}

func (h *GithubProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetUrl, ok := h.parseTargetUrl(w, r)
	if !ok {
		return
	}

	if !allowedHosts[targetUrl.Host] {
		http.Error(w, "Forbidden host", http.StatusForbidden)
		return
	}

	h.helper.RunReverseProxy(w, r, targetUrl, nil)
}
