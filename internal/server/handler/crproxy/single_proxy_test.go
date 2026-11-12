package crproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestGetRoute(t *testing.T) {
	v1, v2 := &url.URL{Host: "v1.test"}, &url.URL{Host: "v2.test"}
	for _, tc := range []struct {
		path      string
		withV1    bool
		wantRoute routePrefix
		wantUrl   *url.URL
		wantOk    bool
	}{
		{"/v1/_ping", true, routePrefixV1, v1, true},
		{"/v1/search", true, routePrefixV1, v1, true},
		{"/v1/repositories/library/nginx/tags", true, routePrefixV1, v1, true},
		{"/v1/images/search", true, routePrefixV1, v1, false},
		{"/v1/_ping", false, "", nil, false},
		{"/v2/library/nginx/manifests/latest", true, routePrefixV2, v2, true},
		{"/auth", true, routePrefixAuthRealm, nil, true},
		{"/other", true, "", nil, false},
	} {
		h := &singleProxyHandler{upstreamV2Url: v2}
		if tc.withV1 {
			h.upstreamV1Url = v1
		}
		w := httptest.NewRecorder()
		targetUrl, route, ok := h.getRoute(w, tc.path)
		require.Equal(t, tc.wantOk, ok, tc.path)
		if !ok {
			require.Equal(t, http.StatusNotFound, w.Code, tc.path)
			continue
		}
		require.Equal(t, tc.wantRoute, route, tc.path)
		require.Equal(t, tc.wantUrl, targetUrl, tc.path)
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

func applyV2ResponseModifier(t *testing.T, h *singleProxyHandler, status int, header http.Header) http.Header {
	resp := &http.Response{StatusCode: status, Header: header}
	require.NoError(t, h.createResponseModifier(testutils.Context(), routePrefixV2)(httptest.NewRequest(http.MethodGet, "https://registry.test/v2/", nil), resp))
	return resp.Header
}

func TestSingleProxyResponseModifierRewritesBearerRealm(t *testing.T) {
	h := newTestSingleHandler(t, &fakeRegistry{}, false)
	header := applyV2ResponseModifier(t, h, http.StatusUnauthorized, http.Header{"Www-Authenticate": {`Bearer realm="https://auth.test/token",service="registry"`}})
	require.Equal(t, `Bearer realm="http://proxy.test/r/auth",service="registry"`, header.Get("Www-Authenticate"))
}

func TestSingleProxyResponseModifierHandlesBasicChallenge(t *testing.T) {
	const basicChallenge = `Basic realm="Registry Realm"`
	for _, authEnabled := range []bool{true, false} {
		h := newTestSingleHandler(t, &fakeRegistry{}, authEnabled)
		header := applyV2ResponseModifier(t, h, http.StatusUnauthorized, http.Header{"Www-Authenticate": {basicChallenge}})
		if authEnabled {
			// The request carried a valid Pavonis token, so the client must not retry with Basic auth
			require.Empty(t, header.Values("Www-Authenticate"))
		} else {
			require.Equal(t, basicChallenge, header.Get("Www-Authenticate"))
		}
	}
}

func TestSingleProxyResponseModifierRewritesUploadLocation(t *testing.T) {
	for _, upstreamV2Url := range []string{"https://registry.test", "https://registry.test/v2"} {
		settings := testSingleSettings(false)
		settings.UpstreamV2Url = &upstreamV2Url
		h := newSingleHandler(t, &fakeRegistry{}, settings)
		header := applyV2ResponseModifier(t, h, http.StatusAccepted, http.Header{"Location": {"https://registry.test/v2/library/alpine/blobs/uploads/abc"}})
		require.Equal(t, "http://proxy.test/r/v2/library/alpine/blobs/uploads/abc", header.Get("Location"), upstreamV2Url)
	}
}

func TestSingleProxyServeHttpRewritesResponseHeaders(t *testing.T) {
	w := serveRequest(newTestSingleHandler(t, &fakeRegistry{}, false), newRequest(http.MethodGet, "/r/v2/library/alpine/manifests/latest", ""))
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, `Bearer realm="http://proxy.test/r/auth",service="svc"`, w.Header().Get("Www-Authenticate"))
}

func TestSingleProxyForwardsV2(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	w := serveRequest(newTestSingleHandler(t, reg, false), newRequest(http.MethodGet, "/r/v2/library/alpine/manifests/latest", ""))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "upstream body", w.Body.String())
	require.Equal(t, "https://registry.test/v2/library/alpine/manifests/latest", reg.LastRequest(t).URL.String())
}

func TestSingleProxyRejectsPushAndNonWhitelisted(t *testing.T) {
	reg := &fakeRegistry{anonymous: map[string]bool{"registry.test": true}}
	settings := testSingleSettings(false)
	settings.ReposWhitelist = []string{"library/alpine"}
	h := newSingleHandler(t, reg, settings)

	require.Equal(t, http.StatusMethodNotAllowed, serveRequest(h, newRequest(http.MethodPut, "/r/v2/library/alpine/manifests/latest", "")).Code)
	require.Equal(t, http.StatusForbidden, serveRequest(h, newRequest(http.MethodGet, "/r/v2/library/ubuntu/manifests/latest", "")).Code)
	require.Empty(t, reg.Requests())
}
