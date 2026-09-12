package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInitAppliesGlobalDefaults(t *testing.T) {
	cfg := &Config{}
	require.NoError(t, cfg.Init())
	require.Equal(t, ":8009", *cfg.Server.Listen)
	require.Equal(t, time.Hour, *cfg.ResourceLimit.RequestTimeout)
	require.Equal(t, 10, *cfg.Response.MaxRedirect)
	require.Equal(t, IpPoolStrategyNone, *cfg.Request.IpPool.DefaultStrategy)
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
		Auth: &ContainerRegistryAuthConfig{Enabled: true},
	}}}}
	require.ErrorContains(t, cfg.Init(), "has no users")
}

func TestInitAppliesSiteDefaults(t *testing.T) {
	mode := SiteModeContainerRegistrySingleProxy
	pypiMode := SiteModePypiProxy
	speedMode := SiteModeSpeedTest
	httpMode := SiteModeHttpGeneralProxy
	cfg := &Config{Sites: []*SiteConfig{
		{Mode: &mode, Host: SiteHosts{"docker.example"}, SelfUrl: "https://docker.example"},
		{Mode: &pypiMode, Host: SiteHosts{"pypi.example"}},
		{Mode: &speedMode, Host: SiteHosts{"speed.example"}},
		{Mode: &httpMode, Host: SiteHosts{"http.example"}},
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { require.ErrorContains(t, tc.cfg.Init(), tc.want) })
	}
}

func TestEnumAndHostsYamlParsing(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte("request:\n  ip_pool:\n    default_strategy: random\nsites:\n  - host: [a.example, b.example]\n    mode: http\n    settings:\n      redirect_action: rewrite_only\n"), &cfg)
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
