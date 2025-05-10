package config

import "github.com/Fallen-Breath/pavonis/internal/utils"

// setDefaultValues fills all nil values with the defaults
func (cfg *Config) setDefaultValues() error {
	// Server
	if cfg.Server == nil {
		cfg.Server = &ServerConfig{}
	}
	if cfg.Server.Listen == nil {
		cfg.Server.Listen = utils.ToPtr(":8009")
	}
	if cfg.Server.TrustedProxies == nil {
		cfg.Server.TrustedProxies = utils.ToPtr("127.0.0.1/24")
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
			// reversed proxy stuffs
			"via", // caddy v2.10.0 adds this
			"x-forwarded-for",
			"x-forwarded-proto",
			"x-forwarded-host",

			// reversed proxy stuffs (cloudflare)
			"cdn-loop",
			"cf-connecting-ip",
			"cf-connecting-ipv6",
			"cf-ew-via",
			"cf-ipcountry",
			"cf-pseudo-ipv4",
			"cf-ray",
			"cf-visitor",
			"cf-warp-tag-id",
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

	// ResourceLimit
	if cfg.ResourceLimit == nil {
		cfg.ResourceLimit = &ResourceLimitConfig{}
	}

	return nil
}
