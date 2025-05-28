package common

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"net/http"
	"net/url"
)

type RunReverseProxyConfig struct {
	ResponseModifier ResponseModifier
	RedirectHandler  RedirectHandler
}

type ReverseProxyOption func(*RunReverseProxyConfig)

func WithResponseModifier(responseModifier ResponseModifier) ReverseProxyOption {
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

func withRedirectRewriteRedirect(destUrl *url.URL, checker func(*url.URL) bool, fallbackDecision RedirectDecision) ReverseProxyOption {
	return WithRedirectHandler(func(resp *http.Response) *RedirectResult {
		if location, err := resp.Location(); err == nil && location != nil {
			if checker == nil || checker(location) {
				location.Scheme = destUrl.Scheme
				location.Host = destUrl.Host
				return &RedirectResult{Decision: RedirectDecisionRewrite, Value: location.String()}
			}
		}
		return &RedirectResult{Decision: fallbackDecision}
	})
}

// WithRedirectRewriteOrFollow only checks Scheme and Host
func WithRedirectRewriteOrFollow(destUrl *url.URL, checker func(*url.URL) bool) ReverseProxyOption {
	return withRedirectRewriteRedirect(destUrl, checker, RedirectDecisionFollow)
}

// WithRedirectRewriteOnly only checks Scheme and Host
func WithRedirectRewriteOnly(destUrl *url.URL, checker func(*url.URL) bool) ReverseProxyOption {
	return withRedirectRewriteRedirect(destUrl, checker, RedirectDecisionReturn)
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
