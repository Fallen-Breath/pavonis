package ghproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/testutils"
	"github.com/stretchr/testify/require"
)

const selfUrl = "https://proxy.example"

type upstreamRequest struct {
	Method string
	Url    string // scheme + host + path + query, as the upstream sees it
	Body   string
}

// fakeGithub serves all the upstream hosts in tests. Requests are routed by their original url
type fakeGithub struct {
	mu       sync.Mutex
	requests []upstreamRequest
	handle   func(w http.ResponseWriter, r *http.Request)
}

func (g *fakeGithub) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
	}
	u := *req.URL
	u.Fragment = ""
	g.mu.Lock()
	g.requests = append(g.requests, upstreamRequest{Method: req.Method, Url: u.String(), Body: string(body)})
	g.mu.Unlock()

	r := req.Clone(req.Context())
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	if g.handle != nil {
		g.handle(w, r)
	} else {
		_, _ = w.Write([]byte("upstream body"))
	}
	resp := w.Result()
	resp.Request = req
	return resp, nil
}

func (g *fakeGithub) Requests() []upstreamRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]upstreamRequest(nil), g.requests...)
}

func newTestHandler(t *testing.T, settings *config.GithubDownloadProxySettings, gh *fakeGithub) handler.HttpHandler {
	info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "", selfUrl)
	h, err := NewGithubProxyHandler(info, testutils.NewRequestHelperWithTransport(t, gh), settings)
	require.NoError(t, err)
	return h
}

func serve(h handler.HttpHandler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/", body)
	r.URL.Path = "/" + target
	if idx := strings.IndexByte(target, '?'); idx >= 0 {
		r.URL.Path = "/" + target[:idx]
		r.URL.RawQuery = target[idx+1:]
	}
	w := httptest.NewRecorder()
	h.ServeHttp(testutils.Context(), w, r)
	return w
}

func get(h handler.HttpHandler, target string) *httptest.ResponseRecorder {
	return serve(h, http.MethodGet, target, nil)
}

func newResolver(t *testing.T, settings *config.GithubDownloadProxySettings) *proxyHandler {
	info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, "", selfUrl)
	h, err := NewGithubProxyHandler(info, nil, settings)
	require.NoError(t, err)
	return h.(*proxyHandler)
}

// resolve returns the url to forward to, or the status code of the rejection
func resolve(t *testing.T, settings *config.GithubDownloadProxySettings, target string) (string, int) {
	path, rawQuery, _ := strings.Cut(target, "?")
	u, httpErr := newResolver(t, settings).resolveTarget("/"+path, rawQuery)
	if httpErr != nil {
		require.Nil(t, u)
		return "", httpErr.Status
	}
	return u.String(), 0
}

func TestResolveTargetForwardsServedUrls(t *testing.T) {
	for _, tc := range []struct {
		name, target, expected string
	}{
		{"as is", "https://github.com/cli/cli/raw/trunk/README.md", "https://github.com/cli/cli/raw/trunk/README.md"},
		{"query kept", "https://github.com/cli/cli.git/info/refs?service=git-upload-pack", "https://github.com/cli/cli.git/info/refs?service=git-upload-pack"},
		{"without scheme", "github.com/cli/cli/raw/trunk/README.md", "https://github.com/cli/cli/raw/trunk/README.md"},
		{"host case", "https://GitHub.com/cli/cli/raw/trunk/README.md", "https://github.com/cli/cli/raw/trunk/README.md"},
		{"blob to raw", "https://github.com/cli/cli/blob/trunk/docs/README.md", "https://github.com/cli/cli/raw/trunk/docs/README.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, status := resolve(t, &config.GithubDownloadProxySettings{}, tc.target)
			require.Zero(t, status)
			require.Equal(t, tc.expected, u)
		})
	}
}

func TestResolveTargetRejects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings config.GithubDownloadProxySettings
		target   string
		status   int
	}{
		{"leading slash", config.GithubDownloadProxySettings{}, "/github.com/cli/cli/raw/trunk/README.md", http.StatusBadRequest},
		{"opaque url", config.GithubDownloadProxySettings{}, "https:github.com/cli/cli/raw/trunk/README.md", http.StatusBadRequest},
		{"http", config.GithubDownloadProxySettings{}, "http://github.com/cli/cli/raw/trunk/README.md", http.StatusBadRequest},
		{"port", config.GithubDownloadProxySettings{}, "github.com:443/cli/cli/raw/trunk/README.md", http.StatusBadRequest},
		{"userinfo", config.GithubDownloadProxySettings{}, "user:pass@github.com/cli/cli/raw/trunk/README.md", http.StatusBadRequest},
		{"unsupported host", config.GithubDownloadProxySettings{}, "https://example.com/cli/cli/raw/trunk/README.md", http.StatusNotFound},
		{"unsupported path", config.GithubDownloadProxySettings{}, "https://github.com/cli/cli/issues/1", http.StatusNotFound},
		{"push not allowed", config.GithubDownloadProxySettings{}, "https://github.com/cli/cli.git/git-receive-pack", http.StatusForbidden},
		{"not whitelisted", config.GithubDownloadProxySettings{ReposWhitelist: []string{"cli/cli"}}, "https://github.com/cli/other/raw/trunk/README.md", http.StatusForbidden},
		{"blacklisted", config.GithubDownloadProxySettings{ReposBlacklist: []string{"cli/cli"}}, "https://github.com/cli/cli/raw/trunk/README.md", http.StatusForbidden},
		{"whitelisted and blacklisted", config.GithubDownloadProxySettings{ReposWhitelist: []string{"cli/*"}, ReposBlacklist: []string{"cli/cli"}}, "https://github.com/cli/cli/raw/trunk/README.md", http.StatusForbidden},
		{"lists apply to push", config.GithubDownloadProxySettings{AllowPush: true, ReposWhitelist: []string{"cli/cli"}}, "https://github.com/cli/other.git/git-receive-pack", http.StatusForbidden},
		{"lists apply to gists", config.GithubDownloadProxySettings{ReposWhitelist: []string{"cli/*"}}, "https://gist.github.com/schacon/1/raw", http.StatusForbidden},
		{"gist without owner, whitelist", config.GithubDownloadProxySettings{ReposWhitelist: []string{"*/*"}}, "https://gist.github.com/1.git/info/refs?service=git-upload-pack", http.StatusForbidden},
		{"gist without owner, blacklist", config.GithubDownloadProxySettings{ReposBlacklist: []string{"schacon/*"}}, "https://gist.github.com/1.git/info/refs?service=git-upload-pack", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, status := resolve(t, &tc.settings, tc.target)
			require.Empty(t, u)
			require.Equal(t, tc.status, status)
		})
	}
}

func TestResolveTargetAllows(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings config.GithubDownloadProxySettings
		target   string
	}{
		{"push allowed", config.GithubDownloadProxySettings{AllowPush: true}, "https://github.com/cli/cli.git/git-receive-pack"},
		{"whitelisted", config.GithubDownloadProxySettings{ReposWhitelist: []string{"cli/cli"}}, "https://github.com/cli/cli/raw/trunk/README.md"},
		{"not blacklisted", config.GithubDownloadProxySettings{ReposBlacklist: []string{"cli/cli"}}, "https://github.com/cli/other/raw/trunk/README.md"},
		{"gist owner whitelisted", config.GithubDownloadProxySettings{ReposWhitelist: []string{"schacon/*"}}, "https://gist.githubusercontent.com/schacon/1/raw"},
		{"gist without owner, no lists", config.GithubDownloadProxySettings{}, "https://gist.github.com/1.git/info/refs?service=git-upload-pack"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, status := resolve(t, &tc.settings, tc.target)
			require.Zero(t, status)
		})
	}
}

func TestServeHttpForwards(t *testing.T) {
	gh := &fakeGithub{handle: func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}}
	h := newTestHandler(t, &config.GithubDownloadProxySettings{AllowPush: true}, gh)
	w := serve(h, http.MethodPost, "https://github.com/cli/cli/blob/trunk/a?x=1", strings.NewReader("data"))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "data", w.Body.String())
	require.Equal(t, []upstreamRequest{{Method: http.MethodPost, Url: "https://github.com/cli/cli/raw/trunk/a?x=1", Body: "data"}}, gh.Requests())
}

func TestServeHttpRejects(t *testing.T) {
	gh := &fakeGithub{}
	w := get(newTestHandler(t, &config.GithubDownloadProxySettings{}, gh), "https://github.com/cli/cli/issues/1")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Empty(t, gh.Requests())
}

func TestServeHttpSizeLimit(t *testing.T) {
	gh := &fakeGithub{handle: func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte(strings.Repeat("a", 100)))
	}}
	h := newTestHandler(t, &config.GithubDownloadProxySettings{SizeLimit: 10, AllowPush: true}, gh)

	require.Equal(t, http.StatusForbidden, get(h, "https://github.com/cli/cli/raw/trunk/README.md").Code)

	w := serve(h, http.MethodPost, "https://github.com/cli/cli.git/git-receive-pack", strings.NewReader(strings.Repeat("b", 100)))
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestServeHttpRawTextUrlRewrite(t *testing.T) {
	const content = "https://github.com/cli/cli/raw/trunk/a"
	gh := &fakeGithub{handle: func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cli/cli/raw/trunk/redirect.sh" {
			http.Redirect(w, r, "https://raw.githubusercontent.com/cli/cli/trunk/install.sh", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(content))
	}}
	for _, pathPrefix := range []string{"", "/gh"} {
		t.Run(fmt.Sprintf("pathPrefix=%q", pathPrefix), func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				enabled  bool
				target   string
				expected string
			}{
				{"raw host", true, "https://raw.githubusercontent.com/cli/cli/trunk/install.sh", selfUrl + pathPrefix + "/" + content},
				{"raw host after redirect", true, "https://github.com/cli/cli/raw/trunk/redirect.sh", selfUrl + pathPrefix + "/" + content},
				{"not a raw host", true, "https://github.com/cli/cli/raw/trunk/install.sh", content},
				{"disabled", false, "https://raw.githubusercontent.com/cli/cli/trunk/install.sh", content},
			} {
				t.Run(tc.name, func(t *testing.T) {
					info := testutils.SiteInfo("gh", config.SiteModeGithubDownloadProxy, pathPrefix, selfUrl)
					h, err := NewGithubProxyHandler(info, testutils.NewRequestHelperWithTransport(t, gh), &config.GithubDownloadProxySettings{RawTextUrlRewrite: tc.enabled})
					require.NoError(t, err)
					w := httptest.NewRecorder()
					h.ServeHttp(testutils.Context(), w, httptest.NewRequest(http.MethodGet, pathPrefix+"/"+tc.target, nil))
					require.Equal(t, http.StatusOK, w.Code)
					require.Equal(t, tc.expected, w.Body.String())
				})
			}
		})
	}
}

func TestServeHttpRawTextRangePolicy(t *testing.T) {
	const content = "#!/bin/sh\ncurl https://github.com/cli/cli/raw/trunk/a\n"
	const rawTarget = "https://raw.githubusercontent.com/cli/cli/trunk/install.sh"
	const textType = "text/plain; charset=utf-8"
	for _, tc := range []struct {
		name, target, contentType, byteRange string
		enabled, partial                     bool
	}{
		{"raw text", rawTarget, textType, "bytes=0-19", true, false},
		{"suffix range", rawTarget, textType, "bytes=-10", true, false},
		{"multiple ranges", rawTarget, textType, "bytes=0-1,10-11", true, false},
		{"unsatisfiable range", rawTarget, textType, "bytes=9999-", true, false},
		{"without range", rawTarget, textType, "", true, false},
		{"raw binary", rawTarget, "application/octet-stream", "bytes=0-19", true, false},
		{"gist raw", "https://gist.githubusercontent.com/cli/1/raw/install.sh", textType, "bytes=0-19", true, false},
		{"redirect to raw", "https://github.com/cli/cli/raw/trunk/redirect.sh", textType, "bytes=0-19", true, false},
		{"redirect to gist raw", "https://gist.github.com/cli/1/raw/redirect.sh", textType, "bytes=0-19", true, false},
		{"disabled", rawTarget, textType, "bytes=0-19", false, true},
		{"disabled after redirect", "https://github.com/cli/cli/raw/trunk/redirect.sh", textType, "bytes=0-19", false, true},
		{"release download", "https://github.com/cli/cli/releases/download/v1/file", "application/octet-stream", "bytes=0-19", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gh := &fakeGithub{handle: func(w http.ResponseWriter, r *http.Request) {
				if tc.enabled && (r.URL.Hostname() == "raw.githubusercontent.com" || r.URL.Hostname() == "gist.githubusercontent.com") {
					require.Empty(t, r.Header.Get("Range"))
					require.Empty(t, r.Header.Get("If-Range"))
				} else {
					require.Equal(t, tc.byteRange, r.Header.Get("Range"))
					require.Equal(t, `"origin-tag"`, r.Header.Get("If-Range"))
				}
				if strings.HasSuffix(r.URL.Path, "/redirect.sh") {
					target := rawTarget
					if r.URL.Hostname() == "gist.github.com" {
						target = "https://gist.githubusercontent.com/cli/1/raw/install.sh"
					}
					http.Redirect(w, r, target, http.StatusFound)
					return
				}
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("ETag", `"origin-tag"`)
				http.ServeContent(w, r, "file", time.Time{}, strings.NewReader(content))
			}}
			h := newTestHandler(t, &config.GithubDownloadProxySettings{RawTextUrlRewrite: tc.enabled}, gh)
			r := httptest.NewRequest(http.MethodGet, "/"+tc.target, nil)
			if tc.byteRange != "" {
				r.Header.Set("Range", tc.byteRange)
			}
			r.Header.Set("If-Range", `"origin-tag"`)
			w := httptest.NewRecorder()
			h.ServeHttp(testutils.Context(), w, r)

			if tc.partial {
				require.Equal(t, http.StatusPartialContent, w.Code)
				require.Equal(t, fmt.Sprintf("bytes 0-19/%d", len(content)), w.Header().Get("Content-Range"))
				require.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
				require.Equal(t, content[:20], w.Body.String())
			} else {
				require.Equal(t, http.StatusOK, w.Code)
				require.Empty(t, w.Header().Get("Content-Range"))
				require.Equal(t, "none", w.Header().Get("Accept-Ranges"))
				expected := content
				if tc.contentType == textType {
					expected = strings.ReplaceAll(content, "https://github.com/", selfUrl+"/https://github.com/")
				}
				require.Equal(t, expected, w.Body.String())
			}
		})
	}
}

func TestServeHttpRawTextRejectsUnexpectedPartialContent(t *testing.T) {
	gh := &fakeGithub{handle: func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Range", "bytes 0-9/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("incomplete"))
	}}
	h := newTestHandler(t, &config.GithubDownloadProxySettings{RawTextUrlRewrite: true}, gh)
	w := get(h, "https://raw.githubusercontent.com/cli/cli/trunk/install.sh")
	require.Equal(t, http.StatusBadGateway, w.Code)
	require.NotContains(t, w.Body.String(), "incomplete")
}
