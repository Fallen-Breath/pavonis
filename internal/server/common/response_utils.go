package common

import (
	"errors"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils/ioutils"
	log "github.com/sirupsen/logrus"
	"net/http"
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
