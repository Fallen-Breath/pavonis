package config

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"net/url"
	"strings"
)

func (cfg *Config) validateValues() error {
	// Server
	if *cfg.Server.TrustedProxies != "*" {
		if _, err := utils.NewIpPool(strings.Split(*cfg.Server.TrustedProxies, ",")); err != nil {
			return fmt.Errorf("bad TrustedProxies value %+q: %v", *cfg.Server.TrustedProxies, err)
		}
	}

	// ResourceLimit
	checkGreaterThanZero := func(value *float64, what string) error {
		if value != nil && *value <= 0 {
			return fmt.Errorf("%s cannot <= 0, value: %v", what, *value)
		}
		return nil
	}

	if err := checkGreaterThanZero(cfg.ResourceLimit.TrafficAvgMibps, "RateLimit.TrafficAvgMibps"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.TrafficBurstMib, "RateLimit.TrafficBurstMib"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.TrafficMaxMibps, "RateLimit.TrafficMaxMibps"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.RequestPerSecond, "RateLimit.RequestPerSecond"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.RequestPerMinute, "RateLimit.RequestPerMinute"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.RequestPerHour, "RateLimit.RequestPerHour"); err != nil {
		return err
	}

	// Request
	if cfg.Request.Proxy != "" {
		if urlObj, err := url.Parse(cfg.Request.Proxy); err != nil || urlObj == nil {
			return fmt.Errorf("failed to parse Request.Proxy %+q: %v", cfg.Request.Proxy, err)
		}
	}
	if cfg.Request.IpPool.Enabled && len(cfg.Request.IpPool.Subnets) == 0 {
		return fmt.Errorf("IpPool enabled but no subnets specified")
	}

	// Site
	for siteIdx, site := range cfg.Sites {
		if !cfg.Request.IpPool.Enabled && site.IpPoolStrategy != nil && *site.IpPoolStrategy != IpPoolStrategyNone {
			return fmt.Errorf("[site%d] IP pool is not enabled, but site IP pool strategy is set to %+q", siteIdx, *site.IpPoolStrategy)
		}

		checkUrl := func(urlStr, what string, allowPath, allowTrailingSlash bool) error {
			urlObj, err := url.Parse(urlStr)
			if err != nil || urlObj == nil {
				return fmt.Errorf("[site%d] failed to parse %s %+q: %v", siteIdx, what, urlStr, err)
			}
			if urlObj.Scheme == "" {
				return fmt.Errorf("[site%d] bad %s %+q: scheme missing", siteIdx, what, urlStr)
			}
			if allowPath && !allowTrailingSlash && strings.HasSuffix(urlObj.Path, "/") {
				return fmt.Errorf("[site%d] bad %s %+q: trailing '/' is not allowed", siteIdx, what, urlStr)
			}
			if !allowPath && len(urlObj.Path) > 0 {
				return fmt.Errorf("[site%d] bad %s %+q: path is not allowed", siteIdx, what, urlStr)
			}
			return nil
		}

		switch site.Mode {
		case SiteModeContainerRegistryProxy:
			settings := site.Settings.(*ContainerRegistrySettings)
			if err := checkUrl(settings.SelfUrl, "SelfUrl", false, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamTokenUrl, "UpstreamTokenUrl", true, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamV2Url, "UpstreamV2Url", true, false); err != nil {
				return err
			}
			if *settings.AllowPush && settings.Authorization.Enabled {
				return fmt.Errorf("[site%d] cannot enable Push if customized Authorization is enabled", siteIdx)
			}
		case SiteModeGithubDownloadProxy:
			settings := site.Settings.(*GithubDownloadProxySettings)
			_ = settings
		case SiteModeHttpGeneralProxy:
			settings := site.Settings.(*HttpGeneralProxySettings)
			_ = settings
		case SiteModePypiProxy:
			settings := site.Settings.(*PypiRegistrySettings)
			if err := checkUrl(*settings.UpstreamSimpleUrl, "UpstreamSimpleUrl", true, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamFilesUrl, "UpstreamFilesUrl", true, false); err != nil {
				return err
			}
		case SiteModeSpeedTest:
			settings := site.Settings.(*SpeedTestSettings)
			_ = settings
		}
	}

	return nil
}
