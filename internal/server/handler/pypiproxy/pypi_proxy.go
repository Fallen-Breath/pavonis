package pypiproxy

import (
	"errors"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type proxyHandler struct {
	info     *handler.Info
	helper   *common.RequestHelper
	settings *config.PypiRegistrySettings

	upstreamSimpleUrl *url.URL
	upstreamFilesUrl  *url.URL
}

var _ handler.HttpHandler = &proxyHandler{}

func NewProxyHandler(info *handler.Info, helper *common.RequestHelper, settings *config.PypiRegistrySettings) (handler.HttpHandler, error) {
	var err error
	var upstreamSimpleUrl, upstreamTokenUrl *url.URL
	if upstreamSimpleUrl, err = url.Parse(*settings.UpstreamSimpleUrl); err != nil {
		return nil, fmt.Errorf("invalid UpstreamSimpleUrl %v: %v", settings.UpstreamSimpleUrl, err)
	}
	if upstreamTokenUrl, err = url.Parse(*settings.UpstreamFilesUrl); err != nil {
		return nil, fmt.Errorf("invalid UpstreamFilesUrl %v: %v", settings.UpstreamFilesUrl, err)
	}

	return &proxyHandler{
		info:              info,
		helper:            helper,
		settings:          settings,
		upstreamSimpleUrl: upstreamSimpleUrl,
		upstreamFilesUrl:  upstreamTokenUrl,
	}, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
}

// https://peps.python.org/pep-0691/#project-list
// https://peps.python.org/pep-0691/#project-detail
var projectListPathPattern = regexp.MustCompile("^/simple/?$")
var projectDetailPathPattern = regexp.MustCompile("^/simple/[^/]+/?$")

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	settingPathPrefix := h.info.PathPrefix
	if !strings.HasPrefix(r.URL.Path, settingPathPrefix) {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, settingPathPrefix))
	}
	reqPath := r.URL.Path[len(settingPathPrefix):]

	var targetURL *url.URL
	var pathPrefix string
	if strings.HasPrefix(reqPath, "/simple") {
		targetURL = h.upstreamSimpleUrl
		pathPrefix = "/simple"
	} else if strings.HasPrefix(reqPath, "/files") {
		targetURL = h.upstreamFilesUrl
		pathPrefix = "/files"
	} else {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	downstreamUrl := *r.URL
	downstreamUrl.Scheme = targetURL.Scheme
	downstreamUrl.Host = targetURL.Host
	downstreamUrl.Path = targetURL.Path + reqPath[len(pathPrefix):]

	responseModifier := func(resp *http.Response) error {
		if !(resp.StatusCode == http.StatusOK && pathPrefix == "/simple") {
			return nil
		}

		isPypiJson := resp.Header.Get("Content-Type") == "application/vnd.pypi.simple.v1+json"
		if projectListPathPattern.MatchString(reqPath) {
			if isPypiJson {
				// do nothing
			} else {
				return h.modifyResponse(resp, `href="/simple/`, fmt.Sprintf(`href="%s/simple/`, settingPathPrefix))
			}
		} else if projectDetailPathPattern.MatchString(reqPath) {
			if isPypiJson {
				return h.modifyResponse(resp, fmt.Sprintf(`"url":"%s/`, *h.settings.UpstreamFilesUrl), fmt.Sprintf(`"url":"%s/files/`, settingPathPrefix))
			} else {
				return h.modifyResponse(resp, fmt.Sprintf(`href="%s/`, *h.settings.UpstreamFilesUrl), fmt.Sprintf(`href="%s/files/`, settingPathPrefix))
			}
		}

		return nil
	}

	h.helper.RunReverseProxy(ctx, w, r, &downstreamUrl, common.WithResponseModifier(responseModifier))
}

func (h *proxyHandler) modifyResponse(resp *http.Response, search, replace string) error {
	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	decompressedReader, err := newDecompressReader(resp.Body, encoding)
	if err != nil {
		if errors.Is(err, unsupportedEncodingError) {
			return common.NewHttpError(http.StatusNotImplemented, fmt.Sprintf("Unsupported Content-Encoding %s", encoding))
		}
		return err
	}

	replacingReader := NewReplacingReader(decompressedReader, []byte(search), []byte(replace))

	newReader, err := newCompressReader(replacingReader, encoding)
	if err != nil {
		return err
	}

	resp.Body = newReader
	resp.Header.Del("Content-Length")
	resp.Header.Set("Transfer-Encoding", "chunked")

	return nil
}
