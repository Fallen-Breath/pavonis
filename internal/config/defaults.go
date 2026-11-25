package config

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

const defaultRequestProxyWeight int64 = 1000
const maxRequestProxyWeight int64 = 1e12

func fillDefaultIds[T any](entries []T, prefix string, idField func(T) *string) {
	used := make(map[string]bool)
	for _, entry := range entries {
		used[*idField(entry)] = true
	}
	for idx, entry := range entries {
		id := idField(entry)
		if *id != "" {
			continue
		}
		base := prefix + strconv.Itoa(idx)
		candidate := base
		for suffix := 2; used[candidate]; suffix++ {
			candidate = base + "_" + strconv.Itoa(suffix)
		}
		*id = candidate
		used[candidate] = true
	}
}

// applyDefaultsAndNormalize fills missing values and normalizes configured values before validation
func (cfg *Config) applyDefaultsAndNormalize() error {
	// Server
	if cfg.Server == nil {
		cfg.Server = &ServerConfig{}
	}
	if cfg.Server.Listen == nil {
		cfg.Server.Listen = utils.ToPtr(":8009")
	}
	if cfg.Server.MaxConnections == nil {
		cfg.Server.MaxConnections = utils.ToPtr(1024)
	}
	if cfg.Server.ReadHeaderTimeout == nil {
		cfg.Server.ReadHeaderTimeout = utils.ToPtr(15 * time.Second)
	}
	if cfg.Server.IdleTimeout == nil {
		cfg.Server.IdleTimeout = utils.ToPtr(30 * time.Second)
	}
	if cfg.Server.TrustedProxyIps == nil {
		cfg.Server.TrustedProxyIps = utils.ToPtr([]string{"127.0.0.1/24"})
	}
	if cfg.Server.TrustedProxyHeaders == nil {
		cfg.Server.TrustedProxyHeaders = utils.ToPtr([]string{})
	}

	// Request
	if cfg.Request == nil {
		cfg.Request = &RequestConfig{}
	}
	if cfg.Request.Proxy == nil {
		cfg.Request.Proxy = &RequestProxyConfig{}
	}
	if cfg.Request.Proxy.Strategy == nil {
		cfg.Request.Proxy.Strategy = utils.ToPtr(RequestProxyStrategyByOrder)
	}
	cfg.Request.Proxy.Servers = cleanNil(cfg.Request.Proxy.Servers)
	fillDefaultIds(cfg.Request.Proxy.Servers, "proxy", func(s *RequestProxyServerConfig) *string { return &s.Id })
	for _, server := range cfg.Request.Proxy.Servers {
		if server.ProxyProtocol == "" {
			server.ProxyProtocol = ProxyProtocolNone
		}
		if server.Weight == nil {
			server.Weight = utils.ToPtr(defaultRequestProxyWeight)
		}
	}
	if cfg.Request.Proxy.ConnectTimeout == nil {
		cfg.Request.Proxy.ConnectTimeout = utils.ToPtr(5 * time.Second)
	}
	if cfg.Request.Proxy.Block == nil {
		cfg.Request.Proxy.Block = &RequestProxyBlockConfig{}
	}
	if cfg.Request.Proxy.Block.FailureThreshold == nil {
		cfg.Request.Proxy.Block.FailureThreshold = utils.ToPtr[int32](3)
	}
	if cfg.Request.Proxy.Block.Cooldown == nil {
		cfg.Request.Proxy.Block.Cooldown = utils.ToPtr(10 * time.Second)
	}
	if cfg.Request.Proxy.Block.MaxCooldown == nil {
		cfg.Request.Proxy.Block.MaxCooldown = utils.ToPtr(5 * time.Minute)
	}
	if cfg.Request.IpPool == nil {
		cfg.Request.IpPool = &IpPoolConfig{}
	}
	if cfg.Request.IpPool.DefaultStrategy == nil {
		cfg.Request.IpPool.DefaultStrategy = utils.ToPtr(IpPoolStrategyNone)
	}
	if cfg.Request.Header == nil {
		cfg.Request.Header = &HeaderModificationConfig{}
	}
	if cfg.Request.Header.Delete == nil {
		cfg.Request.Header.Delete = &[]string{
			// reversed proxy stuffs (common)
			"Via", // caddy v2.10.0 adds this
			"X-Forwarded-For",
			"X-Forwarded-Proto",
			"X-Forwarded-Host",

			// reversed proxy stuffs (cloudflare)
			// https://developers.cloudflare.com/fundamentals/reference/http-headers/
			"CDN-Loop",
			"CF-Connecting-IP",
			"CF-Connecting-IPv6",
			"CF-EW-Via",
			"CF-IPCountry",
			"CF-Pseudo-IPv4",
			"Cf-Ray",
			"CF-Visitor",
			"Cf-Warp-Tag-Id",
		}
	}
	if cfg.Request.Header.Modify == nil {
		cfg.Request.Header.Modify = &map[string]string{}
	}

	// Response
	if cfg.Response == nil {
		cfg.Response = &ResponseConfig{}
	}
	if cfg.Response.Header == nil {
		cfg.Response.Header = &HeaderModificationConfig{}
	}
	if cfg.Response.Header.Delete == nil {
		cfg.Response.Header.Delete = &[]string{}
	}
	if cfg.Response.Header.Modify == nil {
		cfg.Response.Header.Modify = &map[string]string{}
	}
	if cfg.Response.MaxRedirect == nil {
		cfg.Response.MaxRedirect = utils.ToPtr[int32](10)
	}

	// ResourceLimit
	if cfg.ResourceLimit == nil {
		cfg.ResourceLimit = &ResourceLimitConfig{}
	}
	if cfg.ResourceLimit.MaxConcurrent429Responses == nil {
		cfg.ResourceLimit.MaxConcurrent429Responses = utils.ToPtr(64)
	}
	if cfg.ResourceLimit.IdleClientCacheSize == nil {
		cfg.ResourceLimit.IdleClientCacheSize = utils.ToPtr(10240)
	}
	// Groups keep nil timeouts, so a group without one follows the global timeout
	if cfg.ResourceLimit.Global.RequestTimeout == nil {
		cfg.ResourceLimit.Global.RequestTimeout = utils.ToPtr(1 * time.Hour)
	}

	// Diagnostics
	if cfg.Diagnostics == nil {
		cfg.Diagnostics = &DiagnosticsConfig{}
	}
	if cfg.Diagnostics.Listen == nil {
		cfg.Diagnostics.Listen = utils.ToPtr("127.0.0.1:6009")
	}

	// Site
	fillDefaultIds(cfg.Sites, "site", func(s *SiteConfig) *string { return &s.Id })
	for siteIdx, siteCfg := range cfg.Sites {
		if siteCfg.Mode == nil {
			return fmt.Errorf("[site%d] site mode is not provided", siteIdx)
		}

		switch *siteCfg.Mode {
		case SiteModeContainerRegistrySingle:
			settings := siteCfg.Settings.(*ContainerRegistrySingleSiteSettings)

			// All valid url inputs (v means valid)
			// V1   V2   AuthRealm
			// -    -    -
			// -    v    -
			// -    v    v
			// v    v    -
			// v    v    v
			if settings.UpstreamV2Url == nil && settings.UpstreamAuthRealmUrl != nil {
				return fmt.Errorf("[site%d] UpstreamV2Url is nil while UpstreamAuthRealmUrl is not nil", siteIdx)
			}
			if settings.UpstreamV2Url == nil && settings.UpstreamV1Url != nil {
				return fmt.Errorf("[site%d] UpstreamV2Url is nil while UpstreamV1Url is not nil", siteIdx)
			}
			// default to Docker Hub
			if settings.UpstreamV1Url == nil && settings.UpstreamV2Url == nil && settings.UpstreamAuthRealmUrl == nil {
				settings.UpstreamV1Url = utils.ToPtr("https://registry.hub.docker.com/v1")
				settings.UpstreamV2Url = utils.ToPtr("https://registry.hub.docker.com/v2")
				settings.UpstreamAuthRealmUrl = utils.ToPtr("https://auth.docker.io/token")
			}
			if settings.UpstreamV2Url == nil {
				panic("impossible")
			}

			if settings.Auth == nil {
				settings.Auth = &ClientAuthConfig{}
			}
			settings.Auth.Users = cleanNil(settings.Auth.Users)
			if settings.AllowPush == nil {
				settings.AllowPush = utils.ToPtr(false)
			}
			if settings.AllowList == nil {
				settings.AllowList = utils.ToPtr(false)
			}
		case SiteModeContainerRegistryAny:
			settings := siteCfg.Settings.(*ContainerRegistryAnySiteSettings)
			if settings.Auth == nil {
				settings.Auth = &ClientAuthConfig{}
			}
			settings.Auth.Users = cleanNil(settings.Auth.Users)
			if settings.AllowList == nil {
				settings.AllowList = utils.ToPtr(false)
			}
		case SiteModeHttp:
			settings := siteCfg.Settings.(*HttpSiteSettings)
			trimDestination := func(destination *string, field string) {
				trimmed := strings.TrimRight(*destination, "/")
				if removed := len(*destination) - len(trimmed); removed > 0 {
					*destination = trimmed
					log.Warnf("Trimmed trailing '/' from sites[%d].settings.%s (count=%d)", siteIdx, field, removed)
				}
			}
			trimDestination(&settings.Destination, "destination")
			for i, mapping := range settings.Mappings {
				if mapping != nil {
					trimDestination(&mapping.Destination, fmt.Sprintf("mappings[%d].destination", i))
				}
			}
			if settings.RedirectAction == nil {
				settings.RedirectAction = utils.ToPtr(RedirectActionRewriteOrFollow)
			}
			for _, rewrite := range settings.BodyRewrites {
				if rewrite != nil && len(rewrite.ContentTypes) == 0 {
					if len(rewrite.JsonPaths) > 0 {
						rewrite.ContentTypes = []string{"application/json"}
					} else {
						rewrite.ContentTypes = []string{"text/*", "application/json", "application/javascript", "application/xml"}
					}
				}
			}
		case SiteModeAnyUrl:
			settings := siteCfg.Settings.(*AnyUrlSiteSettings)
			if len(settings.AllowedMethods) == 0 {
				settings.AllowedMethods = []string{http.MethodGet, http.MethodHead}
			}
			if settings.Auth == nil {
				settings.Auth = &ClientAuthConfig{}
			}
			settings.Auth.Users = cleanNil(settings.Auth.Users)
		case SiteModeNpm:
			settings := siteCfg.Settings.(*NpmSiteSettings)
			if settings.UpstreamRegistryUrl == nil {
				settings.UpstreamRegistryUrl = utils.ToPtr("https://registry.npmjs.org")
			} else {
				*settings.UpstreamRegistryUrl = strings.TrimRight(*settings.UpstreamRegistryUrl, "/")
			}
			if settings.AllowAudit == nil {
				settings.AllowAudit = utils.ToPtr(true)
			}
		case SiteModePypi:
			settings := siteCfg.Settings.(*PypiSiteSettings)
			if (settings.UpstreamSimpleUrl == nil) != (settings.UpstreamFilesUrl == nil) {
				return fmt.Errorf("[site%d] UpstreamSimpleUrl and UpstreamFilesUrl not all-set or all-unset", siteIdx)
			}
			// default to PyPI
			if settings.UpstreamSimpleUrl == nil {
				settings.UpstreamSimpleUrl = utils.ToPtr("https://pypi.org/simple")
			}
			if settings.UpstreamFilesUrl == nil {
				settings.UpstreamFilesUrl = utils.ToPtr("https://files.pythonhosted.org")
			}
		case SiteModeSpeedTest:
			settings := siteCfg.Settings.(*SpeedTestSiteSettings)
			_1GiB := int64(1) * 1024 * 1024 * 1024
			if settings.MaxDownloadBytes == nil {
				settings.MaxDownloadBytes = utils.ToPtr(_1GiB)
			}
			if settings.MaxUploadBytes == nil {
				settings.MaxUploadBytes = utils.ToPtr(_1GiB)
			}
		}
	}

	return nil
}
