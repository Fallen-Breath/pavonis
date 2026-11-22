package limits

import (
	"net/http"

	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/server/metrics"
)

// ServeRequest calls serve while holding a lease, or responds 429 if the request is not admitted
func (p *Policy) ServeRequest(ctx *context.RequestContext, w http.ResponseWriter, serve func()) {
	if len(p.scopes) == 0 {
		serve()
		return
	}
	lease, rejection := p.AcquireLease(ctx.ClientAddr)
	if rejection != nil {
		metrics.GetResourceLimitRejectionCounter(p.site, rejection.ResourceGroup, rejection.Quota, rejection.Limit).Inc()
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	defer lease.Release()
	ctx.TrafficLimiter = lease.TrafficLimiter
	serve()
}

type limitedHandler struct {
	handler.HttpHandler
	policy *Policy
}

func WrapHandler(inner handler.HttpHandler, policy *Policy) handler.HttpHandler {
	return &limitedHandler{HttpHandler: inner, policy: policy}
}

func (h *limitedHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	h.policy.ServeRequest(ctx, w, func() {
		h.HttpHandler.ServeHttp(ctx, w, r)
	})
}
