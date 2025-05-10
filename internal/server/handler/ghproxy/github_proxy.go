package ghproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"net/http"
	"net/url"
	"strings"
)

type proxyHandler struct {
	name       string
	helper     *common.RequestHelper
	settings   *config.GithubDownloadProxySettings
	whitelist  *reposList
	blacklist  *reposList
	bypassList *reposList
}

var _ handler.HttpHandler = &proxyHandler{}

func NewGithubProxyHandler(name string, helper *common.RequestHelper, settings *config.GithubDownloadProxySettings) (handler.HttpHandler, error) {
	return &proxyHandler{
		name:       name,
		helper:     helper,
		settings:   settings,
		whitelist:  newReposList(settings.ReposWhitelist),
		blacklist:  newReposList(settings.ReposBlacklist),
		bypassList: newReposList(settings.ReposBypass),
	}, nil
}

func (h *proxyHandler) Name() string {
	return h.name
}

func (h *proxyHandler) parseTargetUrl(w http.ResponseWriter, r *http.Request) (*url.URL, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return nil, false
	}
	targetUrlStr := path[1:] // Remove leading "/"

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
	targetUrl, ok := h.parseTargetUrl(w, r)
	if !ok {
		return
	}

	hd, ok := allowedHosts[targetUrl.Host]
	if !ok {
		http.Error(w, "Forbidden host", http.StatusNotFound)
		return
	}

	author, repos, ok := hd.Parse(targetUrl)
	if !ok {
		http.Error(w, "Forbidden url", http.StatusNotFound)
		return
	}
	if !h.checkAndApplyWhitelists(w, r, targetUrl, author, repos) {
		return
	}

	targetUrl.User = r.URL.User
	targetUrl.RawQuery = r.URL.RawQuery
	targetUrl.RawFragment = r.URL.RawFragment

	h.helper.RunReverseProxy(ctx, w, r, targetUrl, func(resp *http.Response) error {
		if h.settings.SizeLimit > 0 && resp.ContentLength > h.settings.SizeLimit {
			return common.NewHttpError(http.StatusBadRequest, "Response body too large")
		}
		return nil
	})
}

func (h *proxyHandler) checkAndApplyWhitelists(w http.ResponseWriter, r *http.Request, targetUrl *url.URL, author string, repos string) bool {
	if len(*h.whitelist) > 0 && !h.whitelist.Check(author, repos) {
		http.Error(w, fmt.Sprintf("Repository %s/%s not in whitelist", author, repos), http.StatusForbidden)
		return false
	}
	if len(*h.blacklist) > 0 && h.blacklist.Check(author, repos) {
		http.Error(w, fmt.Sprintf("Repository %s/%s is in blacklist", author, repos), http.StatusForbidden)
		return false
	}
	if len(*h.bypassList) > 0 && h.bypassList.Check(author, repos) {
		http.Redirect(w, r, targetUrl.String(), http.StatusTemporaryRedirect)
		return false
	}
	return true
}
