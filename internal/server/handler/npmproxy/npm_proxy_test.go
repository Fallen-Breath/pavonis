package npmproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/testutils"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/require"
)

type fakeRegistry func(http.ResponseWriter, *http.Request)

func (serve fakeRegistry) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	serve(w, r)
	resp := w.Result()
	resp.Request = r
	return resp, nil
}

func serveNpm(t *testing.T, req *http.Request, settings config.NpmSiteSettings, registry fakeRegistry) *httptest.ResponseRecorder {
	t.Helper()
	settings.UpstreamRegistryUrl = utils.ToPtr("https://up.test/registry")
	info := testutils.NewSiteInfo("npm", config.SiteModeNpm, "/npm", "https://mirror.test")
	h, err := NewNpmHandler(info, testutils.NewRequestHelperWithTransport(t, registry), &settings)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.NewRequestContext(), w, req)
	return w
}

func TestMatchRoute(t *testing.T) {
	for _, tc := range []struct {
		path string
		want routeKind
	}{
		{"/@scope%2fpkg", routePackage},
		{"/pkg/latest", routeVersion},
		{"/@scope/pkg/-/pkg-1.0.0.tgz", routePassthrough},
		{"/-/package/@scope%2fpkg/dist-tags", routePassthrough},
		{"/-/npm/v1/keys", routePassthrough},
		{"/-/npm/v1/attestations/@scope%2fpkg@1.0.0", routePassthrough},
		{"/-/ping", routePassthrough},
		{"/-/v1/search", routeSearch},
		{"/-/npm/v1/security/advisories/bulk", routeAudit},
		{"/-/whoami", routeUnknown},
	} {
		require.Equal(t, tc.want, matchRoute(tc.path), tc.path)
	}
}

func TestServeHttpRewritesPackageMetadata(t *testing.T) {
	const packument = `{"versions":{
		"1.0.0":{
			"dist":{"tarball":"https://up.test/registry/@scope/pkg/-/pkg.tgz","attestations":{"url":"https://up.test/registry/-/npm/v1/attestations/@scope%2fpkg@1.0.0"}},
			"dependencies":{"tarball":"https://up.test/registry/explicit.tgz"}
		},
		"2.0.0":{"dist":{"tarball":"https://cdn.test/external.tgz"}}
	}}`
	const expected = `{"versions":{
		"1.0.0":{
			"dist":{"tarball":"https://mirror.test/npm/@scope/pkg/-/pkg.tgz","attestations":{"url":"https://mirror.test/npm/-/npm/v1/attestations/@scope%2fpkg@1.0.0"}},
			"dependencies":{"tarball":"https://up.test/registry/explicit.tgz"}
		},
		"2.0.0":{"dist":{"tarball":"https://cdn.test/external.tgz"}}
	}}`
	req := httptest.NewRequest(http.MethodGet, "/npm/@scope%2fpkg", nil)
	w := serveNpm(t, req, config.NpmSiteSettings{}, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "https://up.test/registry/@scope%2fpkg", r.URL.String())
		w.Header().Set("Content-Type", "application/vnd.npm.install-v1+json")
		io.WriteString(w, packument)
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, expected, w.Body.String())
}

func TestServeHttpRewritesVersionMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/npm/pkg/latest", nil)
	w := serveNpm(t, req, config.NpmSiteSettings{}, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"dist":{"tarball":"https://up.test/registry/pkg/-/pkg.tgz","attestations":{"url":"https://up.test/registry/-/npm/v1/attestations/pkg@1.0.0"}}}`)
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"dist":{"tarball":"https://mirror.test/npm/pkg/-/pkg.tgz","attestations":{"url":"https://mirror.test/npm/-/npm/v1/attestations/pkg@1.0.0"}}}`, w.Body.String())
}

func TestServeHttpForwardsTarballRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/npm/@scope/pkg/-/pkg.tgz?download=1", nil)
	req.Header.Set("Range", "bytes=0-2")
	w := serveNpm(t, req, config.NpmSiteSettings{}, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "https://up.test/registry/@scope/pkg/-/pkg.tgz?download=1", r.URL.String())
		require.Equal(t, "bytes=0-2", r.Header.Get("Range"))
		w.Header().Set("Content-Range", "bytes 0-2/100")
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{0x1f, 0x8b, 0x08})
	})
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, []byte{0x1f, 0x8b, 0x08}, w.Body.Bytes())
}

func TestServeHttpAccess(t *testing.T) {
	const bulk = "/npm/-/npm/v1/security/advisories/bulk"
	const search = "/npm/-/v1/search?text=pkg"
	const payload = `{"pkg":["1.0.0"]}`
	for _, tc := range []struct {
		name, method, path, body string
		allowSearch, allowAudit  bool
		status                   int
	}{
		{"search enabled", http.MethodGet, search, "", true, false, 200},
		{"search disabled", http.MethodGet, search, "", false, true, 403},
		{"audit enabled", http.MethodPost, bulk, payload, false, true, 200},
		{"audit disabled", http.MethodPost, bulk, payload, true, false, 403},
		{"audit requires POST", http.MethodGet, bulk, "", false, true, 405},
		{"publish denied", http.MethodPut, "/npm/pkg", "", true, true, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			settings := config.NpmSiteSettings{AllowSearch: tc.allowSearch, AllowAudit: utils.ToPtr(tc.allowAudit)}
			w := serveNpm(t, req, settings, func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, tc.method, r.Method)
				if tc.body != "" {
					b, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.Equal(t, tc.body, string(b))
				}
				w.WriteHeader(http.StatusOK)
			})
			require.Equal(t, tc.status, w.Code)
			if tc.status == http.StatusOK {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}

func TestServeHttpPrivateTarballRedirect(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/npm/private/-/private.tgz", nil)
	req.Header.Set("Authorization", "Bearer read-token")
	req.Header.Set("Npm-Otp", "123456")
	w := serveNpm(t, req, config.NpmSiteSettings{}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Host {
		case "up.test":
			require.Equal(t, "Bearer read-token", r.Header.Get("Authorization"))
			require.Equal(t, "123456", r.Header.Get("Npm-Otp"))
			http.Redirect(w, r, "https://cdn.test/file", http.StatusFound)
		case "cdn.test":
			require.Empty(t, r.Header.Get("Npm-Otp"))
			io.WriteString(w, "tarball")
		default:
			t.Fatalf("unexpected upstream %s", r.URL)
		}
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "tarball", w.Body.String())
}
