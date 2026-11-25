package pypiproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
)

type proxyHandler struct {
	info      *handler.Info
	helper    *common.RequestHelper
	whitelist map[string]bool
	blacklist map[string]bool

	upstreamSimpleUrl *url.URL
	upstreamFilesUrl  *url.URL
}

var _ handler.HttpHandler = &proxyHandler{}

func NewPypiHandler(info *handler.Info, helper *common.RequestHelper, settings *config.PypiSiteSettings) (handler.HttpHandler, error) {
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
		whitelist:         buildPackageSet(settings.PackagesWhitelist),
		blacklist:         buildPackageSet(settings.PackagesBlacklist),
		upstreamSimpleUrl: upstreamSimpleUrl,
		upstreamFilesUrl:  upstreamTokenUrl,
	}, nil
}

func (h *proxyHandler) Info() *handler.Info {
	return h.info
}

func (h *proxyHandler) Shutdown() {
}

type routeKind int

const (
	routeUnknown routeKind = iota
	routeProjectList
	routeProjectDetail
	routeSimpleOther
	routeFiles
)

type requestPath struct {
	kind        routeKind
	escapedRest string
	project     string
}

func parseRequestPath(escapedPath string) requestPath {
	if rest, ok := utils.CutPathPrefix(escapedPath, "/files"); ok {
		return requestPath{kind: routeFiles, escapedRest: rest}
	}
	rest, ok := utils.CutPathPrefix(escapedPath, "/simple")
	if !ok {
		return requestPath{}
	}
	if rest == "" || rest == "/" {
		return requestPath{kind: routeProjectList, escapedRest: rest}
	}
	project, err := url.PathUnescape(strings.TrimSuffix(rest[1:], "/"))
	if err == nil && project != "" && !strings.Contains(project, "/") {
		return requestPath{kind: routeProjectDetail, escapedRest: rest, project: project}
	}
	// Other Simple paths are forwarded without document rewriting.
	return requestPath{kind: routeSimpleOther, escapedRest: rest}
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	settingPathPrefix := h.info.PathPrefix
	path := parseRequestPath(h.info.EscapedSitePath(r.URL))
	if path.kind == routeUnknown {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if !h.checkPackageAccess(path) {
		http.Error(w, "Package access denied", http.StatusForbidden)
		return
	}

	targetUrl := h.upstreamSimpleUrl
	if path.kind == routeFiles {
		targetUrl = h.upstreamFilesUrl
	}
	upstreamUrl := *r.URL
	upstreamUrl.Scheme = targetUrl.Scheme
	upstreamUrl.Host = targetUrl.Host
	utils.SetEscapedPath(&upstreamUrl, targetUrl.EscapedPath()+path.escapedRest)

	responseModifier := func(lastReq *http.Request, resp *http.Response) error {
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
			return nil
		}

		// A 304 usually has no Content-Type, so it can't be recognized as JSON and gets the
		// HTML rules. It has no body, so the rules can only weaken its ETag. On project pages
		// its ETag then matches a 200 in either format, since both formats have their file
		// links rewritten. On the project list with a site prefix, its ETag becomes weak even
		// for JSON, whose 200 is passed through with the strong ETag.
		//
		// We accept this because a strong ETag would be wrong for rewritten HTML. The weak
		// ETag still works with If-None-Match, and mirror clients such as devpi-server 6.21
		// and bandersnatch 8.2.0 don't use it for anything else.
		isPypiJson := resp.Header.Get("Content-Type") == "application/vnd.pypi.simple.v1+json"
		if path.kind == routeProjectList {
			if isPypiJson {
				return nil
			}
			return common.ModifyResponseBody(ctx, resp, common.BodyReplacement{
				Search: `href="/simple/`, Replace: fmt.Sprintf(`href="%s/simple/`, settingPathPrefix),
			})
		}
		if path.kind != routeProjectDetail {
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

// The prefixes cover the file link forms seen on real indexes: absolute (PyPI,
// SJTUG, PyTorch), root-relative (piwheels) and page-relative like ../../packages/
// (TUNA, BFSU, USTC, NJU, Aliyun, Tencent Cloud, Huawei Cloud). Literal matching
// keeps rewriting streaming without per-file URL parsing or allocation. A per-link
// parser benchmarked much slower with no gain on these layouts, so other spellings
// should wait for a real upstream that uses them.
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
