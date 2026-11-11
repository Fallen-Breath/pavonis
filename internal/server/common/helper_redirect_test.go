package common

import (
	"net/http"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRedirectOptions(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://upstream.example/path"}}}
	for _, tc := range []struct {
		name   string
		option ReverseProxyOption
		want   RedirectDecision
	}{
		{"follow all", WithRedirectAction(config.RedirectActionFollowAll, nil), RedirectDecisionFollow},
		{"none", WithRedirectAction(config.RedirectActionNone, nil), RedirectDecisionReturn},
		{"rewrite or follow", WithRedirectAction(config.RedirectActionRewriteOrFollow, func(*http.Response) *string { return nil }), RedirectDecisionFollow},
		{"rewrite only", WithRedirectAction(config.RedirectActionRewriteOnly, func(*http.Response) *string { return nil }), RedirectDecisionReturn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &RunReverseProxyConfig{}
			tc.option(cfg)
			result := cfg.RedirectHandler(resp)
			require.Equal(t, RedirectDecision(tc.want), result.Decision)
		})
	}
}

func TestIsStatusCodeRedirect(t *testing.T) {
	for _, code := range []int{301, 302, 303, 307, 308} {
		require.True(t, IsStatusCodeRedirect(code))
	}
	for _, code := range []int{200, 300, 304, 400, 500} {
		require.False(t, IsStatusCodeRedirect(code))
	}
}
