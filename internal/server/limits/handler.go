package limits

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/server/metrics"
	log "github.com/sirupsen/logrus"
)

func (p *Policy) ServeRequest(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request, serve func()) {
	if len(p.scopes) == 0 {
		serve()
		return
	}
	lease, rejection := p.AcquireLease(ctx.ClientAddr)
	if rejection != nil {
		metrics.GetResourceLimitRejectionCounter(p.site, rejection.ResourceGroup, rejection.Quota, rejection.Limit).Inc()
		p.manager.rejectRequest(ctx, w, r)
		return
	}
	defer lease.Release()
	ctx.TrafficLimiter = lease.TrafficLimiter
	serve()
}

const rejectionResponseTimeout = time.Second

func (m *Manager) rejectRequest(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	controller := http.NewResponseController(w)
	hasBody := r.Body != nil && r.Body != http.NoBody
	select {
	case m.rejectionResponses <- struct{}{}:
		defer func() { <-m.rejectionResponses }()
	default:
		ctx.AbortReason = "429 response limit"
		if hasBody {
			// net/http can drain the body while unwinding an aborted handler.
			_ = controller.SetReadDeadline(time.Now())
		}
		panic(http.ErrAbortHandler)
	}

	deadline := time.Now().Add(rejectionResponseTimeout)
	if end, ok := r.Context().Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if hasBody {
		if err := controller.SetReadDeadline(deadline); err != nil {
			log.Warnf("%sFailed to set rejected request read deadline: %v", ctx.LogPrefix, err)
		}
	}
	if err := controller.SetWriteDeadline(deadline); err != nil {
		log.Warnf("%sFailed to set rejection response write deadline: %v", ctx.LogPrefix, err)
	}

	body := http.StatusText(http.StatusTooManyRequests) + "\n"
	header := w.Header()
	if r.ProtoMajor == 1 {
		header.Set("Connection", "close")
	}
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusTooManyRequests)
	if r.Method != http.MethodHead {
		if _, err := io.WriteString(w, body); err != nil {
			panic(http.ErrAbortHandler)
		}
	}
	// A small 429 can still be buffered when WriteString returns.
	if err := controller.Flush(); err != nil {
		panic(http.ErrAbortHandler)
	}
}

type limitedHandler struct {
	handler.HttpHandler
	policy *Policy
}

func WrapHandler(inner handler.HttpHandler, policy *Policy) handler.HttpHandler {
	return &limitedHandler{HttpHandler: inner, policy: policy}
}

func (h *limitedHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	h.policy.ServeRequest(ctx, w, r, func() {
		h.HttpHandler.ServeHttp(ctx, w, r)
	})
}
