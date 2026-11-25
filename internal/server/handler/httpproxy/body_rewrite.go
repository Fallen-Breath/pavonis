package httpproxy

import (
	"errors"
	"fmt"
	"io"
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
	jsonRules    []ioutils.JsonStringRule
}

func newBodyRewrites(rewriteConfigs []*config.HttpSiteBodyRewrite) ([]*bodyRewrite, error) {
	var rewrites []*bodyRewrite
	for _, rewriteConfig := range rewriteConfigs {
		rewrite := &bodyRewrite{paths: rewriteConfig.Paths}
		for _, contentType := range rewriteConfig.ContentTypes {
			rewrite.contentTypes = append(rewrite.contentTypes, strings.ToLower(contentType))
		}
		var replacements []common.BodyReplacement
		hasEffect := false
		for _, r := range rewriteConfig.Replacements {
			replacement := common.BodyReplacement{Search: r.Search, Replace: r.Replace}
			hasEffect = hasEffect || !replacement.IsNoop()
			replacements = append(replacements, replacement)
		}
		// An ineffective rule must not strip Range or shadow another JSON rule.
		if !hasEffect {
			continue
		}
		if len(rewriteConfig.JsonPaths) == 0 {
			rewrite.replacements = replacements
		} else {
			var pairs []string
			for _, r := range replacements {
				if !r.IsNoop() {
					pairs = append(pairs, r.Search, r.Replace)
				}
			}
			replacer := strings.NewReplacer(pairs...)
			replace := func(value string) (string, bool) {
				result := replacer.Replace(value)
				return result, result != value
			}
			for _, path := range rewriteConfig.JsonPaths {
				segments, err := ioutils.ParseJsonPath(path)
				if err != nil {
					return nil, fmt.Errorf("invalid JSON path %q: %w", path, err)
				}
				rewrite.jsonRules = append(rewrite.jsonRules, ioutils.JsonStringRule{Path: segments, Rewrite: replace})
			}
		}
		rewrites = append(rewrites, rewrite)
	}
	return rewrites, nil
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

// rewriteResponse rewrites the response that carries or describes (HEAD, 304) the full content
func rewriteResponse(ctx *context.RequestContext, resp *http.Response, rewrites []*bodyRewrite) error {
	status := resp.StatusCode
	if status != http.StatusNotModified {
		if status == http.StatusNoContent || status == http.StatusResetContent || status == http.StatusPartialContent {
			return nil
		}
		if !(status >= 200 && status < 300) && !(status >= 400) {
			return nil
		}
		if resp.ContentLength == 0 {
			return nil
		}
	}

	contentType := resp.Header.Get("Content-Type")
	// 304 usually has no Content-Type, so it can only be matched by the path
	matchAnyType := status == http.StatusNotModified && contentType == ""
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil && !matchAnyType {
		return nil
	}
	var jsonRules []ioutils.JsonStringRule
	var replacements []common.BodyReplacement
	for _, rewrite := range rewrites {
		if matchAnyType || rewrite.matchContentType(mediaType) {
			jsonRules = append(jsonRules, rewrite.jsonRules...)
			replacements = append(replacements, rewrite.replacements...)
		}
	}
	switch {
	case len(jsonRules) != 0:
		// Match JSON paths against the upstream document before any whole-body substitutions.
		err = common.ModifyResponseBodyAdvanced(ctx, resp, func(body io.ReadCloser) io.ReadCloser {
			body = ioutils.NewJsonRewriteReader(body, jsonRules...)
			if len(replacements) != 0 {
				body = ioutils.NewRuleReplacingReader(body, replacements...)
			}
			return body
		})
	case len(replacements) != 0:
		err = common.ModifyResponseBody(ctx, resp, replacements...)
	default:
		return nil
	}
	if err != nil {
		if errors.Is(err, ioutils.UnsupportedEncodingError) {
			log.Warnf("%sSkipped response body rewrite: %v", ctx.LogPrefix, err)
			return nil
		}
		return err
	}
	resp.Header.Del("Accept-Ranges")
	return nil
}
