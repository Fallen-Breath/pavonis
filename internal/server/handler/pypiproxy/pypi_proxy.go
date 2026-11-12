package pypiproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
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
	reqPath, ok := utils.CutPathPrefix(r.URL.Path, settingPathPrefix)
	if !ok {
		panic(fmt.Errorf("r.URL.Path %v not started with prefix %v", r.URL.Path, settingPathPrefix))
	}

	var targetUrl *url.URL
	var pathPrefix string
	if strings.HasPrefix(reqPath, "/simple") {
		targetUrl = h.upstreamSimpleUrl
		pathPrefix = "/simple"
	} else if strings.HasPrefix(reqPath, "/files") {
		targetUrl = h.upstreamFilesUrl
		pathPrefix = "/files"
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	upstreamUrl := *r.URL
	upstreamUrl.Scheme = targetUrl.Scheme
	upstreamUrl.Host = targetUrl.Host
	upstreamUrl.Path = targetUrl.Path + reqPath[len(pathPrefix):]
	upstreamUrl.RawPath = ""
	localPath := (&url.URL{Path: settingPathPrefix + pathPrefix}).EscapedPath()
	if rest, ok := strings.CutPrefix(r.URL.EscapedPath(), localPath); ok {
		upstreamUrl.RawPath = targetUrl.EscapedPath() + rest
	}

	responseModifier := func(lastReq *http.Request, resp *http.Response) error {
		if !(resp.StatusCode == http.StatusOK && pathPrefix == "/simple") {
			return nil
		}

		isPypiJson := resp.Header.Get("Content-Type") == "application/vnd.pypi.simple.v1+json"
		if projectListPathPattern.MatchString(reqPath) {
			if isPypiJson {
				return nil
			}
			return common.ModifyResponseBody(ctx, resp, common.BodyReplacement{
				Search: `href="/simple/`, Replace: fmt.Sprintf(`href="%s/simple/`, settingPathPrefix),
			})
		}
		if !projectDetailPathPattern.MatchString(reqPath) {
			return nil
		}

		prefixes := []string{`href="`}
		if isPypiJson {
			prefixes = []string{`"url":"`, `"url": "`}
		}
		filePrefixes := simpleFileUrlPrefixes(lastReq.URL, h.upstreamFilesUrl)
		var replacements []common.BodyReplacement
		for _, prefix := range prefixes {
			for _, filePrefix := range filePrefixes {
				replacements = append(replacements, common.BodyReplacement{
					Search:  prefix + filePrefix,
					Replace: prefix + settingPathPrefix + "/files/",
				})
			}
		}
		return common.ModifyResponseBody(ctx, resp, replacements...)
	}

	h.helper.RunReverseProxy(ctx, w, r, &upstreamUrl, common.WithResponseModifier(responseModifier))
}

// Compute the prefixes once per index response; body rewriting stays literal.
func simpleFileUrlPrefixes(base, files *url.URL) []string {
	prefixes := []string{files.String() + "/"}
	if !strings.EqualFold(base.Scheme, files.Scheme) || !strings.EqualFold(base.Host, files.Host) {
		return prefixes
	}
	// An empty root prefix would also match protocol-relative URLs (//host/...).
	if files.Path != "" {
		prefixes = append(prefixes, files.EscapedPath()+"/")
	}
	baseDir := base.ResolveReference(&url.URL{Path: "."}).EscapedPath()
	from := strings.Split(strings.TrimPrefix(baseDir, "/"), "/")
	from = from[:len(from)-1]
	to := strings.Split(strings.TrimPrefix(files.EscapedPath()+"/", "/"), "/")
	to = to[:len(to)-1]
	common := 0
	for common < len(from) && common < len(to) && from[common] == to[common] {
		common++
	}
	relative := strings.Repeat("../", len(from)-common) + strings.Join(to[common:], "/")
	if relative == "" {
		relative = "."
	}
	if !strings.HasSuffix(relative, "/") {
		relative += "/"
	}
	return append(prefixes, relative)
}
