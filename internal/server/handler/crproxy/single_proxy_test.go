package crproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
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

func TestSingleProxyResponseModifierHandlesBasicChallenge(t *testing.T) {
	const basicChallenge = `Basic realm="Registry Realm"`
	for _, authEnabled := range []bool{true, false} {
		h := newTestSingleHandler(t, &fakeRegistry{}, authEnabled).(*singleProxyHandler)
		resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header)}
		resp.Header.Set("Www-Authenticate", basicChallenge)
		require.NoError(t, h.createResponseModifier(testutils.Context(), routePrefixV2)(httptest.NewRequest(http.MethodGet, "/", nil), resp))
		if authEnabled {
			// The request carried a valid Pavonis token, so the client must not retry with Basic auth
			require.Empty(t, resp.Header.Values("Www-Authenticate"))
		} else {
			require.Equal(t, basicChallenge, resp.Header.Get("Www-Authenticate"))
		}
	}
}
