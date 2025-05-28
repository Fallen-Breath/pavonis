package config

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"golang.org/x/exp/slices"
	"net/url"
	"strings"
	"time"
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

		var checkSelfUrlReason *string

		switch *siteCfg.Mode {
		case SiteModeContainerRegistryProxy:
			settings := siteCfg.Settings.(*ContainerRegistrySettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			if err := checkUrl(*settings.UpstreamAuthRealmUrl, "UpstreamAuthRealmUrl", true, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamV2Url, "UpstreamV2Url", true, false); err != nil {
				return err
			}
			if settings.Auth.Enabled {
				for userIdx, userCfg := range settings.Auth.Users {
					if err := ValidateUser(userCfg); err != nil {
						return fmt.Errorf("[site%d] Auth.Users[%d] validation failed: %v", siteIdx, userIdx, err)
					}
				}
				if settings.Auth.UsersFile != "" && !utils.IsFile(settings.Auth.UsersFile) {
					return fmt.Errorf("[site%d] Auth.UsersFile %+q is not a valid file", siteIdx, settings.Auth.UsersFile)
				}
				if settings.Auth.UsersFileReloadInterval != nil && *settings.Auth.UsersFileReloadInterval <= 1*time.Second {
					return fmt.Errorf("[site%d] Auth.UsersFileReloadInterval %q is too small", siteIdx, settings.Auth.UsersFileReloadInterval.String())
				}
			}
		case SiteModeGithubDownloadProxy:
			settings := siteCfg.Settings.(*GithubDownloadProxySettings)
			if settings.RawTextUrlRewrite {
				checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("RawTextUrlRewrite is %v", settings.RawTextUrlRewrite))
			}
		case SiteModeHuggingFaceProxy:
			settings := siteCfg.Settings.(*HuggingFaceProxySettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			_ = settings
		case SiteModeHttpGeneralProxy:
			settings := siteCfg.Settings.(*HttpGeneralProxySettings)
			for i, mapping := range settings.Mappings {
				if mapping == nil {
					return fmt.Errorf("[site%d] Mappings[%d] is nil", siteIdx, i)
				}
			}
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

		if checkSelfUrlReason == nil && siteCfg.SelfUrl != "" {
			checkSelfUrlReason = utils.ToPtr("SelfUrl is not empty")
		}
		if checkSelfUrlReason != nil {
			// XXX: should path be allowed here? maybe the user has configured another upper-level reversed proxy
			if err := checkUrl(siteCfg.SelfUrl, "SelfUrl", false, false); err != nil {
				return fmt.Errorf("%v, check reason: %s", err, *checkSelfUrlReason)
			}
		}
	}

	return nil
}

func ValidateUser(userCfg *User) error {
	if userCfg == nil {
		return fmt.Errorf("userCfg is nil")
	}

	if userCfg.Name == "" {
		return fmt.Errorf("name is empty")
	}
	if userCfg.Password == "" {
		return fmt.Errorf("password is empty")
	}

	// check '$' for the upstream name / password split, ':' for basic auth
	if strings.Contains(userCfg.Name, "$") || strings.Contains(userCfg.Name, ":") {
		return fmt.Errorf("name contains illegal char '$' or ':'")
	}
	if strings.Contains(userCfg.Password, "$") || strings.Contains(userCfg.Password, ":") {
		return fmt.Errorf("password contains illegal char '$' or ':'")
	}

	return nil
}
