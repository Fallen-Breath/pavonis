package config

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"strconv"
	"time"
)

// setDefaultValues fills all nil values with the defaults
func (cfg *Config) setDefaultValues() error {
	// Server
	if cfg.Server == nil {
		cfg.Server = &ServerConfig{}
	}
	if cfg.Server.Listen == nil {
		cfg.Server.Listen = utils.ToPtr(":8009")
	}
	if cfg.Server.TrustedProxyIps == nil {
		cfg.Server.TrustedProxyIps = utils.ToPtr([]string{"127.0.0.1/24"})
	}
	if cfg.Server.TrustedProxyHeaders == nil {
		cfg.Server.TrustedProxyHeaders = utils.ToPtr([]string{
			"CF-Connecting-IP", // Cloudflare
			"X-Forwarded-For",  // Standard proxy header
			"X-Real-IP",        // Common alternative
		})
	}

	// Request
	if cfg.Request == nil {
		cfg.Request = &RequestConfig{}
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
		cfg.Response.MaxRedirect = utils.ToPtr(10)
	}

	// ResourceLimit
	if cfg.ResourceLimit == nil {
		cfg.ResourceLimit = &ResourceLimitConfig{}
	}
	if cfg.ResourceLimit.RequestTimeout == nil {
		cfg.ResourceLimit.RequestTimeout = utils.ToPtr(1 * time.Hour)
	}

	// Diagnostics
	if cfg.Diagnostics == nil {
		cfg.Diagnostics = &DiagnosticsConfig{}
	}
	if cfg.Diagnostics.Listen == nil {
		cfg.Diagnostics.Listen = utils.ToPtr("127.0.0.1:6009")
	}

	// Site
	existingSiteIds := map[string]bool{}
	for _, site := range cfg.Sites {
		if site.Id != "" {
			existingSiteIds[site.Id] = true
		}
	}
	for siteIdx, siteCfg := range cfg.Sites {
		if siteCfg.Mode == nil {
			return fmt.Errorf("[site%d] site mode is not provided", siteIdx)
		}

		if siteCfg.Id == "" {
			newIdBase := fmt.Sprintf("site%d", siteIdx)
			attempt := 1
			for {
				newId := newIdBase
				if attempt > 1 {
					newId += "_" + strconv.Itoa(attempt)
				}
				if _, ok := existingSiteIds[newId]; !ok {
					siteCfg.Id = newId
					existingSiteIds[newId] = true
					break
				}
				attempt++
				if attempt == len(cfg.Sites)+10 {
					panic("impossible")
				}
			}
		}

		switch *siteCfg.Mode {
		case SiteModeContainerRegistryProxy:
			settings := siteCfg.Settings.(*ContainerRegistrySettings)

			// All valid url inputs
			// V1   V2   AuthRealm
			// -    -    -
			// -    x    x
			// x    x    x
			if (settings.UpstreamV2Url == nil) != (settings.UpstreamAuthRealmUrl == nil) {
				return fmt.Errorf("[site%d] UpstreamV2Url and UpstreamAuthRealmUrl not all-set or all-unset", siteIdx)
			}
			if settings.UpstreamV1Url != nil && settings.UpstreamV2Url == nil {
				return fmt.Errorf("[site%d] UpstreamV2Url is nil while UpstreamV1Url is not nil", siteIdx)
			}
			// default to Docker Hub
			if settings.UpstreamV1Url == nil && settings.UpstreamV2Url == nil && settings.UpstreamAuthRealmUrl == nil {
				settings.UpstreamV1Url = utils.ToPtr("https://registry.hub.docker.com/v1")
				settings.UpstreamV2Url = utils.ToPtr("https://registry.hub.docker.com/v2")
				settings.UpstreamAuthRealmUrl = utils.ToPtr("https://auth.docker.io/token")
			}
			if settings.UpstreamV2Url == nil || settings.UpstreamAuthRealmUrl == nil {
				panic("impossible")
			}

			if settings.Auth == nil {
				settings.Auth = &ContainerRegistryAuthConfig{}
			}
			settings.Auth.Users = cleanNil(settings.Auth.Users)
			if settings.AllowPush == nil {
				settings.AllowPush = utils.ToPtr(false)
			}
			if settings.AllowList == nil {
				settings.AllowList = utils.ToPtr(false)
			}
		case SiteModeHttpGeneralProxy:
			settings := siteCfg.Settings.(*HttpGeneralProxySettings)
			if settings.RedirectAction == nil {
				settings.RedirectAction = utils.ToPtr(RedirectActionRewriteOrFollow)
			}
		case SiteModePypiProxy:
			settings := siteCfg.Settings.(*PypiRegistrySettings)
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
			settings := siteCfg.Settings.(*SpeedTestSettings)
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
