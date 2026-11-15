package httpproxy

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
)

type bodyRewrite struct {
	paths        []string // empty means all paths
	contentTypes []string // lower-cased "type/subtype", "type/*" or "*/*"
	replacements []common.BodyReplacement
}

func newBodyRewrites(rewriteConfigs []*config.HttpSiteBodyRewrite) []*bodyRewrite {
	var rewrites []*bodyRewrite
	for _, rewriteConfig := range rewriteConfigs {
		rewrite := &bodyRewrite{paths: rewriteConfig.Paths}
		for _, contentType := range rewriteConfig.ContentTypes {
			rewrite.contentTypes = append(rewrite.contentTypes, strings.ToLower(contentType))
		}
		hasEffect := false
		for _, r := range rewriteConfig.Replacements {
			replacement := common.BodyReplacement{Search: r.Search, Replace: r.Replace}
			hasEffect = hasEffect || !replacement.IsNoop()
			rewrite.replacements = append(rewrite.replacements, replacement)
		}
		// a rule that changes nothing must not have side effects either, e.g. stripping the Range header
		if hasEffect {
			rewrites = append(rewrites, rewrite)
		}
	}
	return rewrites
}

func (rw *bodyRewrite) matchPath(reqPath string) bool {
	if len(rw.paths) == 0 {
		return true
	}
	for _, path := range rw.paths {
		if _, ok := utils.CutPathPrefix(reqPath, path); ok {
			return true
		}
	}
	return false
}

func (rw *bodyRewrite) matchContentType(mediaType string) bool {
	mainType, _, _ := strings.Cut(mediaType, "/")
	for _, pattern := range rw.contentTypes {
		if pattern == "*/*" || pattern == mediaType || pattern == mainType+"/*" {
			return true
		}
	}
	return false
}

// matchBodyRewrites returns the rewrites whose paths match the request path (without the site path prefix)
func (h *proxyHandler) matchBodyRewrites(reqPath string) []*bodyRewrite {
	var matched []*bodyRewrite
	for _, rewrite := range h.bodyRewrites {
		if rewrite.matchPath(reqPath) {
			matched = append(matched, rewrite)
		}
	}
	return matched
}

// stripRangeHeaders makes the upstream return the full content, since partial content cannot be rewritten
func stripRangeHeaders(req *http.Request) {
	req.Header.Del("Range")
	req.Header.Del("If-Range")
}

// rewriteResponse rewrites the body of the response that carries the full content.
// Bodyless responses that describe the content (HEAD, 304) get their headers aligned with the rewritten ones
func rewriteResponse(ctx *context.RequestContext, lastReq *http.Request, resp *http.Response, rewrites []*bodyRewrite) error {
	status := resp.StatusCode
	if status == http.StatusNotModified {
		// 304 usually has no Content-Type, so it's not filtered by content types
		weakenETag(resp.Header)
		return nil
	}
	if status == http.StatusNoContent || status == http.StatusResetContent || status == http.StatusPartialContent {
		return nil
	}
	if !(status >= 200 && status < 300) && !(status >= 400) {
		return nil
	}
	if resp.ContentLength == 0 {
		return nil
	}

	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil
	}
	var replacements []common.BodyReplacement
	for _, rewrite := range rewrites {
		if rewrite.matchContentType(mediaType) {
			replacements = append(replacements, rewrite.replacements...)
		}
	}
	if len(replacements) == 0 {
		return nil
	}

	if lastReq.Method != http.MethodHead {
		if err := common.ModifyResponseBody(ctx, resp, replacements...); err != nil {
			if errors.Is(err, ioutils.UnsupportedEncodingError) {
				log.Warnf("%sSkipped response body rewrite: %v", ctx.LogPrefix, err)
				return nil
			}
			return err
		}
	}
	resp.Header.Del("Content-Length")
	resp.Header.Del("Accept-Ranges")
	weakenETag(resp.Header)
	return nil
}

func weakenETag(header http.Header) {
	if etag := header.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		header.Set("ETag", "W/"+etag)
	}
}
