package ghproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strings"
)

type proxyHandler struct {
	info      *handler.Info
	helper    *common.RequestHelper
	settings  *config.GithubDownloadProxySettings
	whitelist *reposList
	blacklist *reposList
}

var _ handler.HttpHandler = &proxyHandler{}

func NewGithubProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.GithubDownloadProxySettings) (handler.HttpHandler, error) {
	return &proxyHandler{
		info:      info,
		helper:    helper,
		settings:  settings,
		whitelist: newReposList(settings.ReposWhitelist),
		blacklist: newReposList(settings.ReposBlacklist),
	}, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
}

func (h *proxyHandler) parseTargetUrl(w http.ResponseWriter, reqPath string) (*url.URL, bool) {
	if !strings.HasPrefix(reqPath, "/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return nil, false
	}
	targetUrlStr := reqPath[1:] // Remove leading "/"

	targetUrl, err := url.Parse(targetUrlStr)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return nil, false
	}
	if targetUrl.Scheme == "" {
		targetUrl.Scheme = "https"
	}
	if targetUrl.Scheme != "https" {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return nil, false
	}

	return targetUrl, true
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}
	reqPath := r.URL.Path[len(h.info.PathPrefix):]

	targetUrl, ok := h.parseTargetUrl(w, reqPath)
	if !ok {
		return
	}

	hd, ok := allowedHosts[targetUrl.Host]
	if !ok {
		http.Error(w, "Forbidden host", http.StatusNotFound)
		return
	}

	// whitelist && blacklist check
	if len(*h.whitelist) > 0 || len(*h.blacklist) > 0 {
		author, repos, ok := hd.Parse(targetUrl)
		if !ok {
			http.Error(w, "Forbidden url", http.StatusNotFound)
			return
		}
		log.Debugf("%sExtracted author + repos from reqPath %+q: %+q / %+q", ctx.LogPrefix, reqPath, author, repos)
		if !h.checkAndApplyWhitelists(w, author, repos) {
			return
		}
	}

	targetUrl.User = r.URL.User
	targetUrl.RawQuery = r.URL.RawQuery
	targetUrl.RawFragment = r.URL.RawFragment

	responseModifier := func(resp *http.Response) error {
		if h.settings.SizeLimit > 0 {
			if resp.ContentLength > h.settings.SizeLimit {
				return common.NewHttpError(http.StatusBadGateway, "Response ContentLength too large")
			}
			if isChunkedEncoding(resp.TransferEncoding) {
				resp.Body = NewTrafficSizeLimitedReadCloser(resp.Body, h.settings.SizeLimit)
			}
		}
		return nil
	}

	h.helper.RunReverseProxy(ctx, w, r, targetUrl, common.WithResponseModifier(responseModifier))
}

func (h *proxyHandler) checkAndApplyWhitelists(w http.ResponseWriter, author string, repos string) bool {
	if len(*h.whitelist) > 0 && !h.whitelist.Check(author, repos) {
		http.Error(w, fmt.Sprintf("Repository %s/%s is not whitelisted", author, repos), http.StatusForbidden)
		return false
	}
	if len(*h.blacklist) > 0 && h.blacklist.Check(author, repos) {
		http.Error(w, fmt.Sprintf("Repository %s/%s is blacklisted", author, repos), http.StatusForbidden)
		return false
	}
	return true
}

func isChunkedEncoding(te []string) bool {
	// golang stdlib, net/http transfer.go:603
	return len(te) > 0 && strings.ToLower(te[0]) == "chunked"
}
