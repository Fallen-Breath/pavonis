package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInitAppliesGlobalDefaults(t *testing.T) {
	mode := SiteModeSpeedTest
	cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}}}}
	require.NoError(t, cfg.Init())
	require.Equal(t, ":8009", *cfg.Server.Listen)
	require.Equal(t, time.Hour, *cfg.ResourceLimit.RequestTimeout)
	require.Equal(t, 10, *cfg.Response.MaxRedirect)
	require.Equal(t, IpPoolStrategyNone, *cfg.Request.IpPool.DefaultStrategy)
	require.Empty(t, *cfg.Server.TrustedProxyHeaders)
}

func TestInitAppliesRequestProxyDefaults(t *testing.T) {
	mode := SiteModeSpeedTest
	cfg := &Config{
		Sites:   []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}}},
		Request: &RequestConfig{Proxy: &RequestProxyConfig{Servers: []*RequestProxyServerConfig{{Url: "http://10.0.0.1:7890"}}}},
	}
	require.NoError(t, cfg.Init())
	require.Equal(t, RequestProxyStrategyByOrder, *cfg.Request.Proxy.Strategy)
	require.Equal(t, int64(1000), *cfg.Request.Proxy.Servers[0].Weight)
}

func TestRequestProxyConfig(t *testing.T) {
	initYaml := func(proxyYaml string) error {
		var cfg Config
		proxyYaml = "    " + strings.ReplaceAll(strings.TrimSpace(proxyYaml), "\n", "\n    ")
		if err := yaml.Unmarshal([]byte("sites:\n  - host: '*'\n    mode: speed_test\nrequest:\n  proxy:\n"+proxyYaml+"\n"), &cfg); err != nil {
			return err
		}
		return cfg.Init()
	}

	for _, tc := range []struct {
		name      string
		proxyYaml string
	}{
		{"full", `
servers:
  - url: http://10.0.0.1:7890
    proxy_protocol: v2
  - url: socks5://10.0.0.2:1080
    proxy_protocol: v1
  - url: socks5h://10.0.0.3:1080
    proxy_protocol: none
direct_fallback: true
connect_timeout: 3s
block:
  failure_threshold: 5
  cooldown: 30s
  max_cooldown: 10m
`},
		{"minimal", "servers:\n  - url: http://10.0.0.1:7890"},
		{"partial block", "servers:\n  - url: http://10.0.0.1:7890\nblock:\n  cooldown: 1m"},
		{"https", "servers:\n  - url: https://proxy.example:8443"},
		{"upper case scheme", "servers:\n  - url: HTTP://10.0.0.1:7890"},
		{"ipv6", "servers:\n  - url: socks5://[::1]:1080"},
		{"userinfo", "servers:\n  - url: http://user:pass@10.0.0.1:7890"},
		{"no port", "servers:\n  - url: http://proxy.example"},
	} {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			require.NoError(t, initYaml(tc.proxyYaml))
		})
	}

	for _, tc := range []struct {
		name      string
		proxyYaml string
	}{
		{"missing scheme", "servers:\n  - url: 10.0.0.1:7890"},
		{"missing scheme with hostname", "servers:\n  - url: localhost:7890"},
		{"empty scheme", "servers:\n  - url: //10.0.0.1:7890"},
		{"unsupported scheme", "servers:\n  - url: socks4://10.0.0.1:1080"},
		{"empty url", "servers:\n  - url: ''"},
		{"empty host", "servers:\n  - url: http://:7890"},
		{"invalid proxy protocol", "servers:\n  - url: http://10.0.0.1:7890\n    proxy_protocol: v3"},
		{"zero weight", "servers:\n  - url: http://10.0.0.1:7890\n    weight: 0"},
		{"direct fallback without servers", "direct_fallback: true"},
		{"zero connect timeout", "servers:\n  - url: http://10.0.0.1:7890\nconnect_timeout: 0s"},
		{"negative connect timeout", "servers:\n  - url: http://10.0.0.1:7890\nconnect_timeout: -1s"},
		{"zero failure threshold", "servers:\n  - url: http://10.0.0.1:7890\nblock:\n  failure_threshold: 0"},
		{"zero cooldown", "servers:\n  - url: http://10.0.0.1:7890\nblock:\n  cooldown: 0s"},
		{"max cooldown less than cooldown", "servers:\n  - url: http://10.0.0.1:7890\nblock:\n  cooldown: 1m\n  max_cooldown: 30s"},
	} {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			require.Error(t, initYaml(tc.proxyYaml))
		})
	}

	for _, url := range []string{"socks4://user:s3cret@10.0.0.1:1080", "http://user:s3cret@:7890"} {
		t.Run("hides password "+url, func(t *testing.T) {
			err := initYaml("servers:\n  - url: " + url)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "s3cret")
		})
	}
}

func TestInitFinalizesSettingsForEverySiteMode(t *testing.T) {
	cases := []struct {
		name string
		mode SiteMode
		want interface{}
	}{
		{"registry single", SiteModeContainerRegistrySingle, &ContainerRegistrySingleSiteSettings{}},
		{"registry any", SiteModeContainerRegistryAny, &ContainerRegistryAnySiteSettings{}},
		{"github", SiteModeGithub, &GithubSiteSettings{}},
		{"huggingface", SiteModeHuggingFace, &HuggingFaceSiteSettings{}},
		{"http", SiteModeHttp, &HttpSiteSettings{}},
		{"any", SiteModeAnyUrl, &AnyUrlSiteSettings{}},
		{"pypi", SiteModePypi, &PypiSiteSettings{}},
		{"speed test", SiteModeSpeedTest, &SpeedTestSiteSettings{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}}}}
			if mode == SiteModeContainerRegistrySingle || mode == SiteModeContainerRegistryAny || mode == SiteModeHuggingFace {
				cfg.Sites[0].SelfUrl = "https://proxy.example.test"
			}
			if mode == SiteModeHttp {
				cfg.Sites[0].Settings = &HttpSiteSettings{Destination: "https://upstream.example.test"}
			}
			require.NoError(t, cfg.Init())
			require.IsType(t, tc.want, cfg.Sites[0].Settings)
		})
	}
}

func TestInitAppliesSiteDefaults(t *testing.T) {
	mode := SiteModeContainerRegistrySingle
	pypiMode := SiteModePypi
	speedMode := SiteModeSpeedTest
	httpMode := SiteModeHttp
	anyMode := SiteModeAnyUrl
	cfg := &Config{Sites: []*SiteConfig{
		{Mode: &mode, Host: SiteHosts{"docker.example"}, SelfUrl: "https://docker.example"},
		{Mode: &pypiMode, Host: SiteHosts{"pypi.example"}},
		{Mode: &speedMode, Host: SiteHosts{"speed.example"}},
		{Mode: &httpMode, Host: SiteHosts{"http.example"}, Settings: &HttpSiteSettings{Destination: "https://upstream.example"}},
		{Mode: &anyMode, Host: SiteHosts{"any.example"}},
	}}
	require.NoError(t, cfg.Init())
	registry := cfg.Sites[0].Settings.(*ContainerRegistrySingleSiteSettings)
	require.Equal(t, "https://registry.hub.docker.com/v2", *registry.UpstreamV2Url)
	require.False(t, *registry.AllowPush)
	require.False(t, *registry.AllowList)
	pypi := cfg.Sites[1].Settings.(*PypiSiteSettings)
	require.Equal(t, "https://pypi.org/simple", *pypi.UpstreamSimpleUrl)
	require.Equal(t, "https://files.pythonhosted.org", *pypi.UpstreamFilesUrl)
	speed := cfg.Sites[2].Settings.(*SpeedTestSiteSettings)
	require.Equal(t, int64(1<<30), *speed.MaxDownloadBytes)
	require.Equal(t, int64(1<<30), *speed.MaxUploadBytes)
	require.Equal(t, RedirectActionRewriteOrFollow, *cfg.Sites[3].Settings.(*HttpSiteSettings).RedirectAction)
	require.Equal(t, []string{"GET", "HEAD"}, cfg.Sites[4].Settings.(*AnyUrlSiteSettings).AllowedMethods)
}

func TestInitGeneratesUniqueSiteIds(t *testing.T) {
	mode := SiteModeSpeedTest
	cfg := &Config{Sites: []*SiteConfig{
		{Id: "site0", Mode: &mode, Host: SiteHosts{"a.example"}},
		{Mode: &mode, Host: SiteHosts{"b.example"}},
	}}
	require.NoError(t, cfg.Init())
	require.Equal(t, "site0", cfg.Sites[0].Id)
	require.Equal(t, "site1", cfg.Sites[1].Id)
}

func TestInitRejectsInvalidSiteValues(t *testing.T) {
	t.Run("incomplete percent escape in site prefix", func(t *testing.T) {
		mode := SiteModeSpeedTest
		cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, PathPrefix: "/bad%"}}}
		require.Error(t, cfg.Init())
	})

	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"bad trusted proxy", &Config{Server: &ServerConfig{TrustedProxyIps: ToPtr([]string{"bad"})}}, "TrustedProxyIps"},
		{"bad request rate limit", &Config{ResourceLimit: &ResourceLimitConfig{RequestPerSecond: ToPtr(float64(0))}}, "cannot <= 0"},
		{"ip pool without subnet", &Config{Request: &RequestConfig{IpPool: &IpPoolConfig{Enabled: true}}}, "no subnets"},
		{"pypi half configured", func() *Config {
			mode := SiteModePypi
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &PypiSiteSettings{UpstreamSimpleUrl: ToPtr("https://a")}}}}
		}(), "not all-set"},
		{"root path prefix", func() *Config {
			mode := SiteModeSpeedTest
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, PathPrefix: "/"}}}
		}(), "pathPrefix"},
		{"path prefix with trailing slash", func() *Config {
			mode := SiteModeSpeedTest
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, PathPrefix: "/a/"}}}
		}(), "pathPrefix"},
		{"mapping path with trailing slash", func() *Config {
			mode := SiteModeHttp
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &HttpSiteSettings{
				Mappings: []*HttpSiteMapping{{Path: "/a/", Destination: "https://upstream.example"}},
			}}}}
		}(), "Mappings[0].Path"},
		{"relative path prefix", func() *Config {
			mode := SiteModeSpeedTest
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, PathPrefix: "api"}}}
		}(), "pathPrefix"},
		{"self url with path", func() *Config {
			mode := SiteModeSpeedTest
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, SelfUrl: "https://proxy.example/base"}}}
		}(), "path is not allowed"},
		{"upstream url with trailing slash", func() *Config {
			mode := SiteModePypi
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &PypiSiteSettings{
				UpstreamSimpleUrl: ToPtr("https://pypi.example/simple/"), UpstreamFilesUrl: ToPtr("https://files.example"),
			}}}}
		}(), "trailing '/' is not allowed"},
		{"auth without users", func() *Config {
			mode := SiteModeAnyUrl
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &AnyUrlSiteSettings{Auth: &ClientAuthConfig{Enabled: true}}}}}
		}(), "has no users"},
		{"bad any_url domain rule", func() *Config {
			mode := SiteModeAnyUrl
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &AnyUrlSiteSettings{DomainWhitelist: []string{"a.*.example.com"}}}}}
		}(), "DomainWhitelist[0]"},
		{"nil body rewrite", func() *Config {
			mode := SiteModeHttp
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &HttpSiteSettings{
				Destination: "https://upstream.example", BodyRewrites: []*HttpSiteBodyRewrite{nil},
			}}}}
		}(), "BodyRewrites[0] is nil"},
		{"body rewrite path with trailing slash", func() *Config {
			mode := SiteModeHttp
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &HttpSiteSettings{
				Destination: "https://upstream.example", BodyRewrites: []*HttpSiteBodyRewrite{{Paths: []string{"/a/"}}},
			}}}}
		}(), "BodyRewrites[0].Paths[0]"},
		{"body rewrite bad content type", func() *Config {
			mode := SiteModeHttp
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &HttpSiteSettings{
				Destination: "https://upstream.example", BodyRewrites: []*HttpSiteBodyRewrite{{ContentTypes: []string{"text"}}},
			}}}}
		}(), "BodyRewrites[0].ContentTypes[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { require.ErrorContains(t, tc.cfg.Init(), tc.want) })
	}
}

func TestInitValidatesStaticFiles(t *testing.T) {
	file := filepath.Join(t.TempDir(), "index.html")
	require.NoError(t, os.WriteFile(file, []byte("hi"), 0o644))
	newConfig := func(staticFiles ...*StaticFileConfig) *Config {
		mode := SiteModeSpeedTest
		return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, StaticFiles: staticFiles}}}
	}

	t.Run("accepted", func(t *testing.T) {
		require.NoError(t, newConfig(
			&StaticFileConfig{Path: "/", File: file},
			&StaticFileConfig{Path: "/dir/", Content: ToPtr("x")},
			&StaticFileConfig{Path: "/empty", Content: ToPtr("")},
		).Init())
	})

	tests := []struct {
		name        string
		staticFiles []*StaticFileConfig
		want        string
	}{
		{"path with query", []*StaticFileConfig{{Path: "/a?b", Content: ToPtr("x")}}, "StaticFiles[0].Path"},
		{"duplicate path", []*StaticFileConfig{{Path: "/a", Content: ToPtr("x")}, {Path: "/a", Content: ToPtr("y")}}, "StaticFiles[1].Path"},
		{"both file and content", []*StaticFileConfig{{Path: "/a", File: file, Content: ToPtr("x")}}, "StaticFiles[0]"},
		{"neither file nor content", []*StaticFileConfig{{Path: "/a"}}, "StaticFiles[0]"},
		{"file is a directory", []*StaticFileConfig{{Path: "/a", File: t.TempDir()}}, "StaticFiles[0].File"},
		{"bad content type", []*StaticFileConfig{{Path: "/a", Content: ToPtr("x"), ContentType: "text/"}}, "StaticFiles[0].ContentType"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { require.ErrorContains(t, newConfig(tc.staticFiles...).Init(), tc.want) })
	}
}

func TestEnumAndHostsYamlParsing(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte("request:\n  ip_pool:\n    default_strategy: random\nsites:\n  - host: [a.example, b.example]\n    mode: http\n    settings:\n      destination: https://upstream.example\n      redirect_action: rewrite_only\n"), &cfg)
	require.NoError(t, err)
	require.NoError(t, cfg.Init())
	require.Equal(t, SiteHosts{"a.example", "b.example"}, cfg.Sites[0].Host)
	require.Equal(t, IpPoolStrategyRandom, *cfg.Request.IpPool.DefaultStrategy)
	require.Equal(t, RedirectActionRewriteOnly, *cfg.Sites[0].Settings.(*HttpSiteSettings).RedirectAction)

	var bad Config
	require.ErrorContains(t, yaml.Unmarshal([]byte("sites:\n  - host: x\n    mode: unknown\n"), &bad), "invalid site mode")
}

func TestInitAppliesHttpBodyRewriteDefaults(t *testing.T) {
	mode := SiteModeHttp
	settings := &HttpSiteSettings{
		Destination: "https://upstream.example",
		BodyRewrites: []*HttpSiteBodyRewrite{
			{},
			{ContentTypes: []string{"text/html"}},
		},
	}
	cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: settings}}}
	require.NoError(t, cfg.Init())
	require.Equal(t, []string{"text/*", "application/json", "application/javascript", "application/xml"}, settings.BodyRewrites[0].ContentTypes)
	require.Equal(t, []string{"text/html"}, settings.BodyRewrites[1].ContentTypes)
}

func TestValidateMediaTypePattern(t *testing.T) {
	for _, pattern := range []string{"TEXT/Plain", "*/*"} {
		t.Run(pattern, func(t *testing.T) { require.NoError(t, validateMediaTypePattern(pattern)) })
	}
	for _, pattern := range []string{" text/plain ", "text", "*/plain"} {
		t.Run(pattern, func(t *testing.T) { require.Error(t, validateMediaTypePattern(pattern)) })
	}
}

func ToPtr[T any](v T) *T { return &v }

func TestYamlSiteSettingsAreFinalized(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(strings.TrimSpace(`
sites:
  - host: example.test
    mode: speed_test
    settings:
      max_download_bytes: 123
`)), &cfg)
	require.NoError(t, err)
	require.NoError(t, cfg.Init())
	settings, ok := cfg.Sites[0].Settings.(*SpeedTestSiteSettings)
	require.True(t, ok)
	require.Equal(t, int64(123), *settings.MaxDownloadBytes)
	require.NotNil(t, settings.MaxUploadBytes)
}

func TestValidateUser(t *testing.T) {
	cases := []struct {
		name string
		user *User
		want string
	}{
		{"nil", nil, "userCfg is nil"},
		{"empty name", &User{Password: "secret"}, "name is empty"},
		{"empty password", &User{Name: "alice"}, "password is empty"},
		{"name dollar", &User{Name: "al$ice", Password: "secret"}, "illegal char"},
		{"name colon", &User{Name: "al:ice", Password: "secret"}, "illegal char"},
		{"password dollar", &User{Name: "alice", Password: "sec$ret"}, "illegal char"},
		{"password colon", &User{Name: "alice", Password: "sec:ret"}, "illegal char"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUser(tc.user)
			require.ErrorContains(t, err, tc.want)
		})
	}

	require.NoError(t, ValidateUser(&User{Name: "alice", Password: "secret"}))
}

func writeFile(t *testing.T, path, content string) {
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestLoadConfigResolvesSiteReferences(t *testing.T) {
	t.Setenv(envVarConfigContent, "")
	require.NoError(t, os.Unsetenv(envVarConfigContent))
	configDir, workDir := t.TempDir(), t.TempDir()
	t.Chdir(workDir)
	writeFile(t, filepath.Join(workDir, "static/index.html"), "hi")

	const httpSite = `host: b.example
mode: http
static_files:
  - path: /
    file: static/index.html
settings:
  destination: https://upstream.example
`
	writeFile(t, filepath.Join(configDir, "sites/http.yml"), httpSite)
	writeFile(t, filepath.Join(configDir, "inline.yml"), "sites:\n  - "+strings.ReplaceAll(strings.TrimSpace(httpSite), "\n", "\n    ")+"\n  - host: a.example\n    mode: speed_test\n")
	writeFile(t, filepath.Join(configDir, "split.yml"), "sites:\n  - file: sites/http.yml\n  - host: a.example\n    mode: speed_test\n")

	inline, err := loadConfig(filepath.Join(configDir, "inline.yml"))
	require.NoError(t, err)
	split, err := loadConfig(filepath.Join(configDir, "split.yml"))
	require.NoError(t, err)
	require.Equal(t, inline.Sites, split.Sites)

	t.Run("absolute path", func(t *testing.T) {
		siteFile := filepath.Join(t.TempDir(), "site.yml")
		writeFile(t, siteFile, "host: c.example\nmode: speed_test\n")
		writeFile(t, filepath.Join(configDir, "absolute.yml"), fmt.Sprintf("sites:\n  - file: %q\n", siteFile))

		cfg, err := loadConfig(filepath.Join(configDir, "absolute.yml"))
		require.NoError(t, err)
		require.Equal(t, SiteHosts{"c.example"}, cfg.Sites[0].Host)
	})
}

func TestLoadConfigResolvesSiteReferencesFromWorkingDirWithEnvConfig(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	writeFile(t, filepath.Join(workDir, "site.yml"), "host: a.example\nmode: speed_test\n")
	t.Setenv(envVarConfigContent, "sites:\n  - file: site.yml\n")

	cfg, err := loadConfig(filepath.Join(t.TempDir(), "config.yml"))
	require.NoError(t, err)
	require.Equal(t, SiteHosts{"a.example"}, cfg.Sites[0].Host)
}

func TestSiteReferenceRejectsOtherFields(t *testing.T) {
	var cfg Config
	require.Error(t, unmarshalStrict([]byte("sites:\n  - file: site.yml\n    host: a.example\n"), &cfg, "test"))
}

func TestResolveReferencesRejectsNestedReference(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "outer.yml"), "file: inner.yml\n")
	writeFile(t, filepath.Join(dir, "inner.yml"), "host: a.example\nmode: speed_test\n")

	cfg := &Config{Sites: []*SiteConfig{{file: "outer.yml"}}}
	require.Error(t, cfg.resolveReferences(dir))
}

func TestResolveReferencesParsesSiteFileStrictly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "site.yml"), "host: a.example\nmode: speed_test\nunknown: 1\n")

	cfg := &Config{Sites: []*SiteConfig{{file: "site.yml"}}}
	require.Error(t, cfg.resolveReferences(dir))
}
