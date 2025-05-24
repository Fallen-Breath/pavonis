package hfproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.HuggingFaceProxySettings

	selfUrl *url.URL
}

var _ handler.HttpHandler = &proxyHandler{}

func NewHuggingFaceProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.HuggingFaceProxySettings) (handler.HttpHandler, error) {
	var err error
	var selfUrl *url.URL
	if selfUrl, err = url.Parse(info.SelfUrl); err != nil {
		return nil, fmt.Errorf("invalid SelfUrl %v: %v", info.SelfUrl, err)
	}

	h := &proxyHandler{
		info:     info,
		helper:   helper,
		settings: settings,

		selfUrl: selfUrl,
	}

	return h, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
}

// A Hugging Face model/dataset download needs to access these paths
//
//	"/api/models/HuggingFaceH4/zephyr-7b-beta/revision/main" (model)
//	"/api/datasets/HuggingFaceH4/ultrachat_200k/revision/main" (dataset)
//	"/HuggingFaceH4/zephyr-7b-beta/resolve/892b3d7a7b1cf10c7a701c60881cd93df615734c/foo" (model)
//	"/datasets/HuggingFaceH4/ultrachat_200k/resolve/8049631c405ae6576f93f445c6b8166f76f5505a/bar" (dataset)
//
// See: https://github.com/huggingface/huggingface_hub/blob/cadb7a9e2d425c9ee5e893968ec311dfe0742683/src/huggingface_hub/constants.py#L68
// i.e. `HUGGINGFACE_CO_URL_TEMPLATE = ENDPOINT + "/{repo_id}/resolve/{revision}/{filename}"`
var hfPathWhitelist = []*regexp.Regexp{
	regexp.MustCompile(`^/api/(models|datasets)/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+/.*$`),
	regexp.MustCompile(`^(/datasets)?/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+/resolve/[0-9a-f]+(/.*)?$`),
}

func isValidHfPath(path string) bool {
	for _, pattern := range hfPathWhitelist {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Link
var linkUrlPattern = regexp.MustCompile(`<([^>]+)>`)

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.info.PathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}
	reqPath := r.URL.Path[len(h.info.PathPrefix):]

	// it's currently just a download proxy
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var targetUrl *url.URL
	var remainingPath string
	var opts []common.ReverseProxyOption
	for _, pm := range pathMappings {
		if pm.PathPrefix != "" && strings.HasPrefix(reqPath, pm.PathPrefix) {
			targetUrl = pm.Destination
			remainingPath = reqPath[len(pm.PathPrefix):]
		}
	}
	if targetUrl == nil {
		if !isValidHfPath(reqPath) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		targetUrl = hfUrl
		remainingPath = reqPath

		// rewrite those self-redirect response (e.g. for repos rename redirection)
		// do nothing for other redirects
		opts = append(opts, common.WithRedirectRewriteOnly(hfUrl, func(location *url.URL) bool {
			return location.Scheme == h.selfUrl.Scheme && location.Host == h.selfUrl.Host && isValidHfPath(location.Path)
		}))

		// rewrite redirect location of those download requests
		// we cannot follow the redirect here, since the hf client might rely on header values in the redirect response header (e.g. x-repo-commit)
		opts = append(opts, common.WithResponseModifier(func(resp *http.Response) error {
			redirected := false
			if resp.StatusCode == http.StatusFound {
				if location, err := resp.Location(); err == nil && location != nil {
					if newLocation := h.tryRewriteUrlToSelf(location); newLocation != nil {
						log.Debugf("%sRewriting %s Location: %+q to %+q", ctx.LogPrefix, resp.Status, location.String(), newLocation.String())
						resp.Header.Set("Location", newLocation.String())
						redirected = true
					}
				}
			}
			if common.IsStatusCodeRedirect(resp.StatusCode) && !redirected {
				log.Warnf("%sGot %s with unknown Location %+q, return as-is", ctx.LogPrefix, resp.Status, resp.Header.Get("Location"))
			}

			// hugging face client might utilize urls in the Link header
			// e.g. the "xet-auth" link: https://github.com/huggingface/huggingface_hub/blob/cadb7a9e2d425c9ee5e893968ec311dfe0742683/src/huggingface_hub/utils/_xet.py#L50C54-L50C90
			if linkHeader := resp.Header.Get("Link"); linkHeader != "" {
				newLinkHeader := linkUrlPattern.ReplaceAllStringFunc(linkHeader, func(match string) string {
					submatches := linkUrlPattern.FindStringSubmatch(match)
					if len(submatches) < 2 {
						return match
					}

					var newUrl *url.URL
					if oldUrl, err := url.Parse(submatches[1]); err == nil && oldUrl != nil {
						newUrl = h.tryRewriteUrlToSelf(oldUrl)
					}

					if newUrl != nil {
						return fmt.Sprintf("<%s>", newUrl.String())
					} else {
						log.Warnf("%sSkipping unknown url match %+q in Link header", ctx.LogPrefix, match)
					}
					return match
				})
				resp.Header.Set("Link", newLinkHeader)
			}

			// this header exists in links like "/api/models/HuggingFaceH4/zephyr-7b-beta/xet-read-token/892b3d7a7b1cf10c7a701c60881cd93df615734c"
			// see https://github.com/huggingface/huggingface_hub/blob/cadb7a9e2d425c9ee5e893968ec311dfe0742683/src/huggingface_hub/utils/_xet.py#L62
			// TODO: should we rewrite the "casUrl" field in the response json body as well?
			if xetCasUrlStr := resp.Header.Get("X-Xet-Cas-Url"); xetCasUrlStr != "" {
				if xetCasUrl, err := url.Parse(xetCasUrlStr); err == nil && xetCasUrl != nil {
					if newUrl := h.tryRewriteUrlToSelf(xetCasUrl); newUrl != nil {
						resp.Header.Set("X-Xet-Cas-Url", newUrl.String())
					}
				}
			}

			return nil
		}))
	}
	if targetUrl == pmCasServer.Destination {
		// modify the reconstruction json result
		// e.g. for path "/.cas-server.xethub/reconstruction/21938ae6f4b5ccb1b8ef2e633a81d6cf4382fea439ef18a579013f9d5399b8dd"
		opts = append(opts, common.WithResponseModifier(func(resp *http.Response) error {
			if regexp.MustCompile(`^/reconstruction/[0-9a-f]+$`).MatchString(remainingPath) {
				return common.ModifyResponseBody(
					ctx, resp,
					fmt.Sprintf(`"url":"%s/`, pmTransfer.Destination.String()),
					fmt.Sprintf(`"url":"%s%s/`, h.selfUrl.String(), pmTransfer.PathPrefix),
				)
			}
			return nil
		}))
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetUrl.Scheme
	downstreamUrl.Host = targetUrl.Host
	downstreamUrl.Path = targetUrl.Path + remainingPath

	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, opts...)
}

func (h *proxyHandler) tryRewriteUrlToSelf(oldUrl *url.URL) (newUrl *url.URL) {
	for _, pm := range pathMappings {
		if oldUrl.Scheme == pm.Destination.Scheme && oldUrl.Host == pm.Destination.Host {
			oldUrlCopy := *oldUrl
			newUrl = &oldUrlCopy
			newUrl.Scheme = h.selfUrl.Scheme
			newUrl.Host = h.selfUrl.Host
			newUrl.Path = h.info.PathPrefix + pm.PathPrefix + oldUrl.Path
			break
		}
	}
	return
}
