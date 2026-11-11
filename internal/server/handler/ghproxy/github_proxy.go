package ghproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.GithubDownloadProxySettings

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

func invalidTarget() *common.HttpError {
	return common.NewHttpError(http.StatusBadRequest, "Invalid target URL")
}

// resolveTarget decides where the request should be forwarded to, according to the path rules, allow_push and the repos lists.
// reqPath is the request path with the site path prefix removed, e.g. "/https://github.com/owner/repo/raw/..."
func (h *proxyHandler) resolveTarget(reqPath, rawQuery string) (*url.URL, *common.HttpError) {
	if !strings.HasPrefix(reqPath, "/") {
		return nil, invalidTarget()
	}
	targetUrl, err := utils.ParseHttpUrlWithDefaultScheme(reqPath[1:], "https")
	if err != nil || targetUrl.Scheme != "https" || targetUrl.Port() != "" || targetUrl.User != nil {
		return nil, invalidTarget()
	}
	targetUrl.Host = strings.ToLower(targetUrl.Hostname())

	query, _ := url.ParseQuery(rawQuery)
	tm, hostOk := matchTarget(targetUrl.Host, targetUrl.Path, query)
	if !hostOk {
		return nil, common.NewHttpError(http.StatusNotFound, "Unsupported host")
	}
	if tm == nil {
		return nil, common.NewHttpError(http.StatusNotFound, "Unsupported path")
	}
	if tm.Push && !h.settings.AllowPush {
		return nil, common.NewHttpError(http.StatusForbidden, "Push is not allowed")
	}
	if err := h.checkReposLists(tm); err != nil {
		return nil, err
	}
	if tm.RewrittenPath != "" {
		targetUrl.Path = tm.RewrittenPath
		targetUrl.RawPath = ""
	}
	targetUrl.RawQuery = rawQuery
	return targetUrl, nil
}

func (h *proxyHandler) checkReposLists(tm *targetMatch) *common.HttpError {
	if len(*h.whitelist) == 0 && len(*h.blacklist) == 0 {
		return nil
	}
	if tm.Owner == "" {
		return common.NewHttpError(http.StatusForbidden, "Cannot determine the gist owner, use the clone url with the username")
	}
	if len(*h.whitelist) > 0 && !h.whitelist.Check(tm.Owner, tm.Repo) {
		return common.NewHttpError(http.StatusForbidden, fmt.Sprintf("Repository %s/%s is not whitelisted", tm.Owner, tm.Repo))
	}
	if len(*h.blacklist) > 0 && h.blacklist.Check(tm.Owner, tm.Repo) {
		return common.NewHttpError(http.StatusForbidden, fmt.Sprintf("Repository %s/%s is blacklisted", tm.Owner, tm.Repo))
	}
	return nil
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	reqPath, ok := utils.CutPathPrefix(r.URL.Path, h.info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}

	targetUrl, httpErr := h.resolveTarget(reqPath, r.URL.RawQuery)
	if httpErr != nil {
		http.Error(w, httpErr.Message, httpErr.Status)
		return
	}
	log.Debugf("%sResolved target url from reqPath %+q: %s", ctx.LogPrefix, reqPath, targetUrl)

	responseModifier := func(lastReq *http.Request, resp *http.Response) error {
		if h.settings.RawTextUrlRewrite && shouldRewriteRawText(lastReq.URL.Host, resp.Header.Get("Content-Type")) {
			dstPrefix := h.info.SelfUrl + "/"
			log.Debugf("%sRewriting url inside the raw text content, dst prefix %+q", ctx.LogPrefix, dstPrefix)
			return common.ModifyResponseBodyAdvanced(ctx, resp, func(reader io.ReadCloser) io.ReadCloser {
				return newRawTextUrlRewriteReader(reader, dstPrefix)
			})
		}
		return nil
	}

	h.helper.RunReverseProxy(ctx, w, r, targetUrl,
		common.WithResponseModifier(responseModifier),
		common.WithRequestSizeLimit(h.settings.SizeLimit),
		common.WithResponseSizeLimit(h.settings.SizeLimit),
	)
}
