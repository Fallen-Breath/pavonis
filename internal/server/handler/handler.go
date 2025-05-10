package handler

import (
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"net/http"
)

type HttpHandler interface {
	Name() string
	ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request)
}
