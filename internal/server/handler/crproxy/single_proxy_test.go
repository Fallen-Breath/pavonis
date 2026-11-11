package crproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/stretchr/testify/require"
)

func TestChallengeScope(t *testing.T) {
	for _, tc := range []struct {
		path, method, want string
	}{
		{"/v2/library/nginx/manifests/latest", http.MethodHead, "repository:library/nginx:pull"},
		{"/v2/library/nginx/blobs/sha256:abc", http.MethodGet, "repository:library/nginx:pull"},
		{"/v2/a/b/blobs/uploads/", http.MethodPost, "repository:a/b:pull,push"},
		{"/v2/a/b/manifests/v1", http.MethodPut, "repository:a/b:pull,push"},
		{"/v2/", http.MethodGet, ""},
		{"/v2/_catalog", http.MethodGet, ""},
	} {
		require.Equal(t, tc.want, challengeScope(tc.path, tc.method), tc.method+" "+tc.path)
	}
}

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
