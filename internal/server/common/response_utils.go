package common

import (
	"errors"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

func ModifyResponseBody(ctx *context.RequestContext, resp *http.Response, search, replace string) error {
	encoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	decompressedReader, err := ioutils.NewDecompressReader(resp.Body, encoding)
	if err != nil {
		if errors.Is(err, ioutils.UnsupportedEncodingError) {
			return NewHttpError(http.StatusNotImplemented, fmt.Sprintf("Unsupported Content-Encoding %s", encoding))
		}
		return err
	}

	log.Debugf("%sModifying response body string: %+q -> %+q", ctx.LogPrefix, search, replace)

	replacingReader := ioutils.NewReplacingReader(decompressedReader, []byte(search), []byte(replace))

	newReader, err := ioutils.NewCompressReader(replacingReader, encoding)
	if err != nil {
		return err
	}

	resp.Body = newReader
	resp.Header.Del("Content-Length")
	resp.Header.Set("Transfer-Encoding", "chunked")

	return nil
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
