package config

import (
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
		{"registry single", SiteModeContainerRegistrySingleProxy, &ContainerRegistrySingleProxySettings{}},
		{"registry any", SiteModeContainerRegistryAnyProxy, &ContainerRegistryAnyProxySettings{}},
		{"github", SiteModeGithubDownloadProxy, &GithubDownloadProxySettings{}},
		{"huggingface", SiteModeHuggingFaceProxy, &HuggingFaceProxySettings{}},
		{"http", SiteModeHttpGeneralProxy, &HttpGeneralProxySettings{}},
		{"any", SiteModeAnyProxy, &AnyProxySettings{}},
		{"pypi", SiteModePypiProxy, &PypiRegistrySettings{}},
		{"speed test", SiteModeSpeedTest, &SpeedTestSettings{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}}}}
			if mode == SiteModeContainerRegistrySingleProxy || mode == SiteModeContainerRegistryAnyProxy || mode == SiteModeHuggingFaceProxy {
				cfg.Sites[0].SelfUrl = "https://proxy.example.test"
			}
			if mode == SiteModeHttpGeneralProxy {
				cfg.Sites[0].Settings = &HttpGeneralProxySettings{Destination: "https://upstream.example.test"}
			}
			require.NoError(t, cfg.Init())
			require.IsType(t, tc.want, cfg.Sites[0].Settings)
		})
	}
}

func TestInitRejectsInvalidPathPrefix(t *testing.T) {
	mode := SiteModeGithubDownloadProxy
	cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, PathPrefix: "api"}}}
	require.ErrorContains(t, cfg.Init(), "pathPrefix")
}

func TestInitRejectsInvalidAnyProxyAuth(t *testing.T) {
	mode := SiteModeAnyProxy
	cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &AnyProxySettings{
		Auth: &ClientAuthConfig{Enabled: true},
	}}}}
	require.ErrorContains(t, cfg.Init(), "has no users")
}

func TestInitAppliesSiteDefaults(t *testing.T) {
	mode := SiteModeContainerRegistrySingleProxy
	pypiMode := SiteModePypiProxy
	speedMode := SiteModeSpeedTest
	httpMode := SiteModeHttpGeneralProxy
	anyMode := SiteModeAnyProxy
	cfg := &Config{Sites: []*SiteConfig{
		{Mode: &mode, Host: SiteHosts{"docker.example"}, SelfUrl: "https://docker.example"},
		{Mode: &pypiMode, Host: SiteHosts{"pypi.example"}},
		{Mode: &speedMode, Host: SiteHosts{"speed.example"}},
		{Mode: &httpMode, Host: SiteHosts{"http.example"}, Settings: &HttpGeneralProxySettings{Destination: "https://upstream.example"}},
		{Mode: &anyMode, Host: SiteHosts{"any.example"}},
	}}
	require.NoError(t, cfg.Init())
	registry := cfg.Sites[0].Settings.(*ContainerRegistrySingleProxySettings)
	require.Equal(t, "https://registry.hub.docker.com/v2", *registry.UpstreamV2Url)
	require.False(t, *registry.AllowPush)
	require.False(t, *registry.AllowList)
	pypi := cfg.Sites[1].Settings.(*PypiRegistrySettings)
	require.Equal(t, "https://pypi.org/simple", *pypi.UpstreamSimpleUrl)
	require.Equal(t, "https://files.pythonhosted.org", *pypi.UpstreamFilesUrl)
	speed := cfg.Sites[2].Settings.(*SpeedTestSettings)
	require.Equal(t, int64(1<<30), *speed.MaxDownloadBytes)
	require.Equal(t, int64(1<<30), *speed.MaxUploadBytes)
	require.Equal(t, RedirectActionRewriteOrFollow, *cfg.Sites[3].Settings.(*HttpGeneralProxySettings).RedirectAction)
	require.Equal(t, []string{"GET", "HEAD"}, cfg.Sites[4].Settings.(*AnyProxySettings).AllowedMethods)
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
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{"bad trusted proxy", &Config{Server: &ServerConfig{TrustedProxyIps: ToPtr([]string{"bad"})}}, "TrustedProxyIps"},
		{"bad timeout limit", &Config{ResourceLimit: &ResourceLimitConfig{RequestPerSecond: ToPtr(float64(0))}}, "cannot <= 0"},
		{"ip pool without subnet", &Config{Request: &RequestConfig{IpPool: &IpPoolConfig{Enabled: true}}}, "no subnets"},
		{"pypi half configured", func() *Config {
			mode := SiteModePypiProxy
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &PypiRegistrySettings{UpstreamSimpleUrl: ToPtr("https://a")}}}}
		}(), "not all-set"},
		{"bad any proxy domain rule", func() *Config {
			mode := SiteModeAnyProxy
			return &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &AnyProxySettings{DomainWhitelist: []string{"a.*.example.com"}}}}}
		}(), "DomainWhitelist[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { require.ErrorContains(t, tc.cfg.Init(), tc.want) })
	}
}

func TestEnumAndHostsYamlParsing(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte("request:\n  ip_pool:\n    default_strategy: random\nsites:\n  - host: [a.example, b.example]\n    mode: http\n    settings:\n      destination: https://upstream.example\n      redirect_action: rewrite_only\n"), &cfg)
	require.NoError(t, err)
	require.NoError(t, cfg.Init())
	require.Equal(t, SiteHosts{"a.example", "b.example"}, cfg.Sites[0].Host)
	require.Equal(t, IpPoolStrategyRandom, *cfg.Request.IpPool.DefaultStrategy)
	require.Equal(t, RedirectActionRewriteOnly, *cfg.Sites[0].Settings.(*HttpGeneralProxySettings).RedirectAction)

	var bad Config
	require.ErrorContains(t, yaml.Unmarshal([]byte("sites:\n  - host: x\n    mode: unknown\n"), &bad), "invalid site mode")
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
	settings, ok := cfg.Sites[0].Settings.(*SpeedTestSettings)
	require.True(t, ok)
	require.Equal(t, int64(123), *settings.MaxDownloadBytes)
	require.NotNil(t, settings.MaxUploadBytes)
}

func TestInitRejectsInvalidUpstreamUrlShape(t *testing.T) {
	mode := SiteModePypiProxy
	simple := "https://pypi.example/simple/"
	files := "https://files.example"
	cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, Settings: &PypiRegistrySettings{UpstreamSimpleUrl: &simple, UpstreamFilesUrl: &files}}}}
	require.ErrorContains(t, cfg.Init(), "trailing '/' is not allowed")
}

func TestInitRejectsSelfUrlPathWhenRequired(t *testing.T) {
	mode := SiteModeSpeedTest
	cfg := &Config{Sites: []*SiteConfig{{Mode: &mode, Host: SiteHosts{"*"}, SelfUrl: "https://proxy.example/base"}}}
	require.ErrorContains(t, cfg.Init(), "path is not allowed")
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
