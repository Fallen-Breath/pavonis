package proxy

import (
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"net/http"
)

type HttpHandler interface {
	ServeHttp(ctx *context.HttpContext, w http.ResponseWriter, r *http.Request)
}
