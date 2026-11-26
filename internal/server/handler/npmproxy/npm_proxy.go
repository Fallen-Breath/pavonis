package npmproxy

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
)

type proxyHandler struct {
	info         *handler.Info
	helper       *common.RequestHelper
	settings     *config.NpmSiteSettings
	registry     *url.URL
	packageRules []ioutils.JsonStringRule
	versionRules []ioutils.JsonStringRule
}

var _ handler.HttpHandler = &proxyHandler{}

func NewNpmHandler(info *handler.Info, helper *common.RequestHelper, settings *config.NpmSiteSettings) (handler.HttpHandler, error) {
	registry, err := url.Parse(*settings.UpstreamRegistryUrl)
	if err != nil {
		return nil, err
	}
	upstreamPrefix := registry.String() + "/"
	sitePrefix := info.SelfUrl + info.PathPrefix + "/"
	rewriteUrl := func(value string) (string, bool) {
		rest, ok := strings.CutPrefix(value, upstreamPrefix)
		if !ok {
			return "", false
		}
		return sitePrefix + rest, true
	}
	key := ioutils.NewJsonPathSegmentKey
	versionRules := []ioutils.JsonStringRule{
		{Path: []ioutils.JsonPathSegment{key("dist"), key("tarball")}, Rewrite: rewriteUrl},
		{Path: []ioutils.JsonPathSegment{key("dist"), key("attestations"), key("url")}, Rewrite: rewriteUrl},
	}
	var packageRules []ioutils.JsonStringRule
	for _, rule := range versionRules {
		rule.Path = append([]ioutils.JsonPathSegment{key("versions"), ioutils.NewJsonPathSegmentAnyKey()}, rule.Path...)
		packageRules = append(packageRules, rule)
	}
	return &proxyHandler{
		info:         info,
		helper:       helper,
		settings:     settings,
		registry:     registry,
		packageRules: packageRules,
		versionRules: versionRules,
	}, nil
}

func (h *proxyHandler) Info() *handler.Info { return h.info }
func (h *proxyHandler) Shutdown()           {}

type routeKind int

const (
	routeUnknown routeKind = iota
	routePackage
	routeVersion
	routePassthrough
	routeSearch
	routeAudit
)

const packageName = `(?:@[\w.~!'()*-]+/)?[\w~!'()*-][\w.~!'()*-]*`
const versionOrTag = `[\w~!'()*+-][\w.~!'()*+-]*`

var (
	packagePath     = regexp.MustCompile(`^/` + packageName + `$`)
	versionPath     = regexp.MustCompile(`^/` + packageName + `/` + versionOrTag + `$`)
	tarballPath     = regexp.MustCompile(`^/` + packageName + `/-/[^/\\]+\.tgz$`)
	distTagsPath    = regexp.MustCompile(`^/-/package/` + packageName + `/dist-tags$`)
	attestationPath = regexp.MustCompile(`^/-/npm/v1/attestations/` + packageName + `@` + versionOrTag + `$`)
)

func matchRoute(escapedPath string) routeKind {
	switch escapedPath {
	case "/-/v1/search":
		return routeSearch
	case "/-/npm/v1/security/advisories/bulk":
		return routeAudit
	case "/-/ping", "/-/npm/v1/keys":
		return routePassthrough
	}
	// npm uses both @scope%2fname and @scope/name. Decode for classification only
	path, _ := url.PathUnescape(escapedPath)
	switch {
	case distTagsPath.MatchString(path), attestationPath.MatchString(path):
		return routePassthrough
	case path == "/-", strings.HasPrefix(path, "/-/"), strings.HasPrefix(path, "/_"):
		return routeUnknown
	case packagePath.MatchString(path):
		return routePackage
	case versionPath.MatchString(path):
		return routeVersion
	case tarballPath.MatchString(path):
		return routePassthrough
	}
	return routeUnknown
}

func (h *proxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	escaped := h.info.ExtractEscapedSitePath(r.URL)
	route := matchRoute(escaped)
	if route == routeUnknown {
		http.NotFound(w, r)
		return
	}
	if (route == routeSearch && !h.settings.AllowSearch) || (route == routeAudit && !*h.settings.AllowAudit) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	methodAllowed := r.Method == http.MethodGet || r.Method == http.MethodHead
	if route == routeAudit {
		methodAllowed = r.Method == http.MethodPost
	}
	if !methodAllowed {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	target := *r.URL
	target.Scheme, target.Host = h.registry.Scheme, h.registry.Host
	utils.SetEscapedPath(&target, h.registry.EscapedPath()+escaped)

	var rules []ioutils.JsonStringRule
	switch route {
	case routePackage:
		rules = h.packageRules
	case routeVersion:
		rules = h.versionRules
	}
	opts := []common.ReverseProxyOption{common.WithRequestModifier(func(req *http.Request) {
		// Follow the transport's cross-host credential policy for npm's one-time password.
		if !strings.EqualFold(req.URL.Hostname(), h.registry.Hostname()) {
			req.Header.Del("Npm-Otp")
		}
		if rules != nil {
			req.Header.Del("Range")
			req.Header.Del("If-Range")
		}
	})}
	if rules != nil {
		opts = append(opts, common.WithResponseModifier(func(_ *http.Request, resp *http.Response) error {
			if resp.StatusCode == http.StatusPartialContent {
				return errors.New("unexpected partial response from npm registry after disabling Range requests")
			}
			if resp.StatusCode != http.StatusNotModified {
				mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
				if resp.StatusCode != http.StatusOK || (mediaType != "application/json" && mediaType != "application/vnd.npm.install-v1+json") {
					return nil
				}
			}
			resp.Header.Del("Accept-Ranges")
			return common.ModifyResponseBodyAdvanced(ctx, resp, func(body io.ReadCloser) io.ReadCloser {
				return ioutils.NewJsonRewriteReader(body, rules...)
			})
		}))
	}
	h.helper.RunReverseProxy(ctx, w, r, &target, opts...)
}
