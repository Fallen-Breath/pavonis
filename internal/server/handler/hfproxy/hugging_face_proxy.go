package hfproxy

import (
	"bytes"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
	"io"
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
//	"/HuggingFaceH4/zephyr-7b-beta/resolve/main/foo" (model)
//	"/datasets/HuggingFaceH4/ultrachat_200k/resolve/main/bar" (dataset)
//	"/api/resolve-cache/models/HuggingFaceH4/zephyr-7b-beta/892b3d7a7b1cf10c7a701c60881cd93df615734c/foo" (redirected from "resolve")
//
// Legacy repo ids without the author (e.g. "bert-base-uncased") are still served by Hugging Face
//
// See: https://github.com/huggingface/huggingface_hub/blob/cadb7a9e2d425c9ee5e893968ec311dfe0742683/src/huggingface_hub/constants.py#L68
// i.e. `HUGGINGFACE_CO_URL_TEMPLATE = ENDPOINT + "/{repo_id}/resolve/{revision}/{filename}"`
var hfPathWhitelist = []*regexp.Regexp{
	regexp.MustCompile(`^/api/(models|datasets)/[a-zA-Z0-9._-]+(/.*)?$`),
	regexp.MustCompile(`^/api/resolve-cache/(models|datasets)/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+/[^/]+/.+$`),
	regexp.MustCompile(`^(/datasets)?/[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)?/resolve/[^/]+(/.*)?$`),
}

var xetReconstructionPathPattern = regexp.MustCompile(`^/v[0-9]+/reconstructions/[0-9a-f]+$`)

func isValidHfPath(path string) bool {
	for _, pattern := range hfPathWhitelist {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	reqPath, ok := utils.CutPathPrefix(r.URL.Path, h.info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, h.info.PathPrefix))
	}

	// it's currently just a download proxy
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	upstreamUrl := *r.URL
	var opts []common.ReverseProxyOption
	if storagePath, ok := utils.CutPathPrefix(reqPath, storagePathPrefix); ok {
		host, _, _ := strings.Cut(strings.TrimPrefix(storagePath, "/"), "/")
		if !isStorageHost(host) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		upstreamUrl.Scheme = "https"
		upstreamUrl.Host = host
		upstreamUrl.Path = storagePath[len("/"+host):]

		if xetReconstructionPathPattern.MatchString(upstreamUrl.Path) {
			opts = append(opts, common.WithResponseModifier(func(_ *http.Request, resp *http.Response) error {
				return common.ModifyResponseBodyAdvanced(ctx, resp, h.newReconstructionUrlRewriteReader)
			}))
		}
	} else {
		if !isValidHfPath(reqPath) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		upstreamUrl.Scheme = hfUrl.Scheme
		upstreamUrl.Host = hfUrl.Host
		upstreamUrl.Path = reqPath

		var redirectHeader http.Header
		opts = append(opts, common.WithRedirectHandler(func(resp *http.Response) *common.RedirectResult {
			if location, err := resp.Location(); err == nil && isStorageHost(location.Host) {
				if redirectHeader == nil {
					redirectHeader = resp.Header.Clone()
				}
				return &common.RedirectResult{Decision: common.RedirectDecisionFollow}
			}
			return &common.RedirectResult{Decision: common.RedirectDecisionReturn}
		}), common.WithResponseModifier(func(_ *http.Request, resp *http.Response) error {
			// Newer hub clients follow same-host redirects and read metadata like X-Repo-Commit from the final response,
			// so follow storage redirects here and keep the redirect's headers (tested with huggingface_hub 2.4)
			for key, values := range redirectHeader {
				if _, exists := resp.Header[key]; !exists && key != "Location" {
					resp.Header[key] = values
				}
			}
			h.rewriteHfResponseHeaders(ctx, resp)
			return nil
		}))
	}

	h.helper.RunReverseProxy(ctx, w, r, &upstreamUrl, opts...)
}

func (h *proxyHandler) rewriteHfResponseHeaders(ctx *context.RequestContext, resp *http.Response) {
	if common.IsStatusCodeRedirect(resp.StatusCode) {
		locationStr := resp.Header.Get("Location")
		var newLocation *url.URL
		if location, err := url.Parse(locationStr); err == nil && locationStr != "" {
			if location.Scheme == "" && location.Host == "" && strings.HasPrefix(location.Path, "/") {
				// keep it relative, older hub clients only follow relative redirects (tested with huggingface_hub 0.36)
				location.Path = h.info.PathPrefix + location.Path
				newLocation = location
			} else {
				newLocation = h.tryRewriteUrlToSelf(location)
			}
		}
		if newLocation != nil {
			log.Debugf("%sRewriting %s Location: %+q to %+q", ctx.LogPrefix, resp.Status, locationStr, newLocation.String())
			resp.Header.Set("Location", newLocation.String())
		} else {
			log.Warnf("%sGot %s with unknown Location %+q, return as-is", ctx.LogPrefix, resp.Status, locationStr)
		}
	}

	// hugging face client might utilize urls in the Link header
	// e.g. the "xet-auth" link: https://github.com/huggingface/huggingface_hub/blob/cadb7a9e2d425c9ee5e893968ec311dfe0742683/src/huggingface_hub/utils/_xet.py#L50C54-L50C90
	common.RewriteLinkHeaderUrls(&resp.Header, h.tryRewriteUrlToSelf, func(urlStr string) {
		log.Warnf("%sSkipping unknown url str %+q in Link header", ctx.LogPrefix, urlStr)
	})

	// this header exists in links like "/api/models/HuggingFaceH4/zephyr-7b-beta/xet-read-token/892b3d7a7b1cf10c7a701c60881cd93df615734c"
	// see https://github.com/huggingface/huggingface_hub/blob/cadb7a9e2d425c9ee5e893968ec311dfe0742683/src/huggingface_hub/utils/_xet.py#L62
	if xetCasUrlStr := resp.Header.Get("X-Xet-Cas-Url"); xetCasUrlStr != "" {
		if xetCasUrl, err := url.Parse(xetCasUrlStr); err == nil && xetCasUrl != nil {
			if newUrl := h.tryRewriteUrlToSelf(xetCasUrl); newUrl != nil {
				resp.Header.Set("X-Xet-Cas-Url", newUrl.String())
			}
		}
	}
}

const maxHostLen = 253

// newReconstructionUrlRewriteReader rewrites the storage host urls in xet reconstruction json, e.g. "url":"https://us.aws.cdn.hf.co/..."
func (h *proxyHandler) newReconstructionUrlRewriteReader(reader io.ReadCloser) io.ReadCloser {
	prefix := []byte(`"url":"https://`)
	searchFunc := func(buf []byte, _ []byte, eof bool) (int, int, []byte) {
		start := 0
		for {
			idx := bytes.Index(buf[start:], prefix)
			if idx == -1 {
				return -1, 0, nil
			}
			idx += start
			hostStart := idx + len(prefix)
			start = hostStart

			limit := min(len(buf), hostStart+maxHostLen+1)
			hostLen := bytes.IndexByte(buf[hostStart:limit], '/')
			if hostLen == -1 {
				if limit == len(buf) && !eof {
					return -1, 0, nil // the host might continue in the upcoming data
				}
				continue
			}
			host := string(buf[hostStart : hostStart+hostLen])
			if !isStorageHost(host) {
				continue
			}
			newUrl := h.tryRewriteUrlToSelf(&url.URL{Scheme: "https", Host: host})
			return idx, len(prefix) + hostLen + 1, []byte(fmt.Sprintf(`"url":"%s/`, newUrl.String()))
		}
	}
	return ioutils.NewReplacingReader(reader, searchFunc, len(prefix)+maxHostLen+1, 0)
}

func (h *proxyHandler) tryRewriteUrlToSelf(oldUrl *url.URL) *url.URL {
	var path string
	if oldUrl.Scheme == hfUrl.Scheme && oldUrl.Host == hfUrl.Host {
		path = oldUrl.Path
	} else if oldUrl.Scheme == "https" && isStorageHost(oldUrl.Host) {
		path = storagePathPrefix + "/" + oldUrl.Host + oldUrl.Path
	} else {
		return nil
	}

	newUrl := *oldUrl
	newUrl.Scheme = h.selfUrl.Scheme
	newUrl.Host = h.selfUrl.Host
	newUrl.Path = h.info.PathPrefix + path
	return &newUrl
}
