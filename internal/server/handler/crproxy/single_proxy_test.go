package crproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCheckAllowPushOnlyBlocksV2(t *testing.T) {
	h := &singleProxyHandler{settings: &config.ContainerRegistrySingleProxySettings{AllowPush: boolPtr(false)}}
	for _, tc := range []struct {
		method string
		route  routePrefix
		want   bool
	}{
		{http.MethodPost, routePrefixAuthRealm, true},
		{http.MethodGet, routePrefixV2, true},
		{http.MethodHead, routePrefixV2, true},
		{http.MethodPost, routePrefixV2, false},
		{http.MethodPut, routePrefixV2, false},
	} {
		w := httptest.NewRecorder()
		require.Equal(t, tc.want, h.checkAllowPush(w, httptest.NewRequest(tc.method, "/", nil), tc.route), tc.method+" "+string(tc.route))
		if !tc.want {
			require.Equal(t, http.StatusMethodNotAllowed, w.Code)
		}
	}
}
