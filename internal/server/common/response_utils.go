package common

import (
	"errors"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type BodyReplacement = ioutils.Replacement

func modifyResponseBody(ctx *context.RequestContext, resp *http.Response, replacingReaderFactory func(io.ReadCloser) io.ReadCloser) error {
	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	decompressedReader, err := ioutils.NewDecompressReader(resp.Body, encoding)
	if err != nil {
		if errors.Is(err, ioutils.UnsupportedEncodingError) {
			return NewHttpError(http.StatusNotImplemented, fmt.Sprintf("Unsupported Content-Encoding %s", encoding))
		}
		return err
	}

	replacingReader := replacingReaderFactory(decompressedReader)

	newReader, err := ioutils.NewCompressReader(replacingReader, encoding)
	if err != nil {
		return err
	}

	resp.Body = newReader
	resp.Header.Del("Content-Length")
	resp.Header.Set("Transfer-Encoding", "chunked")

	return nil

}

func ModifyResponseBody(ctx *context.RequestContext, resp *http.Response, replacements ...BodyReplacement) error {
	hasReplacement := false
	for _, replacement := range replacements {
		logSuffix := ""
		if replacement.IsNoop() {
			logSuffix = " (skipped)"
		} else {
			hasReplacement = true
		}
		log.Debugf("%sModifying response body string: %+q -> %+q%s", ctx.LogPrefix, replacement.Search, replacement.Replace, logSuffix)
	}
	if !hasReplacement {
		return nil // no need for the actual modification
	}

	return modifyResponseBody(ctx, resp, func(reader io.ReadCloser) io.ReadCloser {
		return ioutils.NewRuleReplacingReader(reader, replacements...)
	})
}

// ModifyResponseBodyAdvanced replaces the (decompressed) response body with the reader returned by wrapReader
func ModifyResponseBodyAdvanced(ctx *context.RequestContext, resp *http.Response, wrapReader func(io.ReadCloser) io.ReadCloser) error {
	log.Debugf("%sModifying response body with custom reader", ctx.LogPrefix)
	return modifyResponseBody(ctx, resp, wrapReader)
}

// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Link
var linkUrlPattern = regexp.MustCompile(`<([^>]+)>`)

func RewriteLinkHeaderUrls(header *http.Header, rewriter func(oldUrl *url.URL) *url.URL, onUnknownUrl func(urlStr string)) {
	if linkHeader := header.Get("Link"); linkHeader != "" {
		modifiedSomething := false
		newLinkHeader := linkUrlPattern.ReplaceAllStringFunc(linkHeader, func(match string) string {
			submatches := linkUrlPattern.FindStringSubmatch(match)
			if len(submatches) < 2 {
				return match
			}

			urlStr := submatches[1]
			var newUrl *url.URL
			if oldUrl, err := url.Parse(urlStr); err == nil && oldUrl != nil {
				newUrl = rewriter(oldUrl)
			}

			if newUrl != nil {
				modifiedSomething = true
				return fmt.Sprintf("<%s>", newUrl.String())
			} else {
				if onUnknownUrl != nil {
					onUnknownUrl(urlStr)
				}
			}
			return match
		})
		if modifiedSomething {
			header.Set("Link", newLinkHeader)
		}
	}
}
