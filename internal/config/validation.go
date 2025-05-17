package config

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"golang.org/x/exp/slices"
	"net/url"
	"strings"
)

func (cfg *Config) validateValues() error {
	// Server
	if !slices.Contains(*cfg.Server.TrustedProxyIps, "*") {
		if _, err := utils.NewIpPool(*cfg.Server.TrustedProxyIps); err != nil {
			return fmt.Errorf("bad TrustedProxyIps value %+q: %v", *cfg.Server.TrustedProxyIps, err)
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
	for siteIdx, siteCfg := range cfg.Sites {
		if !cfg.Request.IpPool.Enabled && siteCfg.IpPoolStrategy != nil && *siteCfg.IpPoolStrategy != IpPoolStrategyNone {
			return fmt.Errorf("[site%d] IP pool is not enabled, but site IP pool strategy is set to %+q", siteIdx, *siteCfg.IpPoolStrategy)
		}
		if siteCfg.PathPrefix != "" && !strings.HasPrefix(siteCfg.PathPrefix, "/") {
			return fmt.Errorf("[site%d] pathPrefix %+q does not start with /", siteIdx, siteCfg.PathPrefix)
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

		switch *siteCfg.Mode {
		case SiteModeContainerRegistryProxy:
			settings := siteCfg.Settings.(*ContainerRegistrySettings)
			if err := checkUrl(settings.SelfUrl, "SelfUrl", false, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamAuthRealmUrl, "UpstreamAuthRealmUrl", true, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamV2Url, "UpstreamV2Url", true, false); err != nil {
				return err
			}
			for userIdx, userCfg := range settings.Authorization.Users {
				if userCfg.Name == "" {
					return fmt.Errorf("[site%d] Authorization.Users[%d].Name is empty", siteIdx, userIdx)
				}
				if userCfg.Password == "" {
					return fmt.Errorf("[site%d] Authorization.Users[%d].Password is empty", siteIdx, userIdx)
				}

				// check '$' for the upstream name / password split, ':' for basic auth
				if strings.Contains(userCfg.Name, "$") || strings.Contains(userCfg.Name, ":") {
					return fmt.Errorf("[site%d] Authorization.Users[%d].Name contains illegal char '$' or ':'", siteIdx, userIdx)
				}
				if strings.Contains(userCfg.Password, "$") || strings.Contains(userCfg.Password, ":") {
					return fmt.Errorf("[site%d] Authorization.Users[%d].Password contains illegal char '$' or ':'", siteIdx, userIdx)
				}
			}
		case SiteModeGithubDownloadProxy:
			settings := siteCfg.Settings.(*GithubDownloadProxySettings)
			_ = settings
		case SiteModeHttpGeneralProxy:
			settings := siteCfg.Settings.(*HttpGeneralProxySettings)
			_ = settings
		case SiteModePavonis:
			settings := siteCfg.Settings.(*PavonisSiteSettings)
			_ = settings
		case SiteModePypiProxy:
			settings := siteCfg.Settings.(*PypiRegistrySettings)
			if err := checkUrl(*settings.UpstreamSimpleUrl, "UpstreamSimpleUrl", true, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamFilesUrl, "UpstreamFilesUrl", true, false); err != nil {
				return err
			}
		case SiteModeSpeedTest:
			settings := siteCfg.Settings.(*SpeedTestSettings)
			_ = settings
		}
	}

	return nil
}
