package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"net/http"
)

type RunReverseProxyConfig struct {
	ResponseModifier responseModifier
	RedirectHandler  RedirectHandler
}

type ReverseProxyOption func(*RunReverseProxyConfig)

func WithResponseModifier(responseModifier responseModifier) ReverseProxyOption {
	return func(cfg *RunReverseProxyConfig) {
		cfg.ResponseModifier = responseModifier
	}
}

func WithRedirectHandler(redirectHandler RedirectHandler) ReverseProxyOption {
	return func(cfg *RunReverseProxyConfig) {
		cfg.RedirectHandler = redirectHandler
	}
}

func WithRedirectFollowAll() ReverseProxyOption {
	return WithRedirectHandler(func(resp *http.Response) *RedirectResult {
		return &RedirectResult{Decision: RedirectDecisionFollow}
	})
}

func WithRedirectFollowNone() ReverseProxyOption {
	return WithRedirectHandler(func(resp *http.Response) *RedirectResult {
		return &RedirectResult{Decision: RedirectDecisionReturn}
	})
}

func WithRedirectAction(action config.RedirectAction, locationRewriter func(*http.Response) *string) ReverseProxyOption {
	switch action {
	case config.RedirectActionFollowAll:
		return WithRedirectFollowAll()
	case config.RedirectActionNone:
		return WithRedirectFollowNone()
	}

	if locationRewriter == nil {
		panic("rewriter is nil")
	}

	return WithRedirectHandler(func(resp *http.Response) *RedirectResult {
		if rewrittenLocation := locationRewriter(resp); rewrittenLocation != nil {
			return &RedirectResult{Decision: RedirectDecisionRewrite, Value: *rewrittenLocation}
		}

		switch action {
		case config.RedirectActionRewriteOrFollow:
			return &RedirectResult{Decision: RedirectDecisionFollow}
		case config.RedirectActionRewriteOnly:
			return &RedirectResult{Decision: RedirectDecisionReturn}
		default:
			panic(fmt.Errorf("unknown redirect action %v", action))
		}
	})
}
