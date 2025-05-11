package config

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
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

	// Site
	for siteIdx, site := range cfg.Sites {
		switch site.Mode {
		case SiteModeContainerRegistryProxy:
			settings := site.Settings.(*ContainerRegistrySettings)
			if (settings.UpstreamV2Url == nil) != (settings.UpstreamTokenUrl == nil) {
				return fmt.Errorf("[site%d] UpstreamV2Url and UpstreamTokenUrl not all-set or all-unset", siteIdx)
			}
			// default to Docker Hub
			if settings.UpstreamV2Url == nil {
				settings.UpstreamV2Url = utils.ToPtr("https://registry.hub.docker.com/v2")
			}
			if settings.UpstreamTokenUrl == nil {
				settings.UpstreamTokenUrl = utils.ToPtr("https://auth.docker.io/token")
			}
			if settings.Authorization == nil {
				settings.Authorization = &CrAuthConfig{}
			}
			settings.Authorization.Users = cleanNil(settings.Authorization.Users)
			if settings.AllowPush == nil {
				settings.AllowPush = utils.ToPtr(!settings.Authorization.Enabled)
			}
		case SiteModePypiProxy:
			settings := site.Settings.(*PypiRegistrySettings)
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
			settings := site.Settings.(*SpeedTestSettings)
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
