package config

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/utils"
	"golang.org/x/exp/slices"
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
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return fmt.Errorf("%s must be finite, value: %v", what, *value)
		}
		if value != nil && *value <= 0 {
			return fmt.Errorf("%s cannot <= 0, value: %v", what, *value)
		}
		return nil
	}
	checkRequestRate := func(value *float64, what string, interval time.Duration) error {
		if err := checkGreaterThanZero(value, what); err != nil {
			return err
		}
		// Require at least one request per day to bound request-rate refill time.
		const maxRequestInterval = 24 * time.Hour
		minimum := interval.Seconds() / maxRequestInterval.Seconds()
		if value != nil && *value < minimum {
			return fmt.Errorf("%s must be >= %g (at least one request per %s), value: %v", what, minimum, maxRequestInterval, *value)
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
	if err := checkRequestRate(cfg.ResourceLimit.RequestPerSecond, "RateLimit.RequestPerSecond", time.Second); err != nil {
		return err
	}
	if err := checkRequestRate(cfg.ResourceLimit.RequestPerMinute, "RateLimit.RequestPerMinute", time.Minute); err != nil {
		return err
	}
	if err := checkRequestRate(cfg.ResourceLimit.RequestPerHour, "RateLimit.RequestPerHour", time.Hour); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.GlobalTrafficAvgMibps, "RateLimit.GlobalTrafficAvgMibps"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.GlobalTrafficBurstMib, "RateLimit.GlobalTrafficBurstMib"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.ResourceLimit.GlobalTrafficMaxMibps, "RateLimit.GlobalTrafficMaxMibps"); err != nil {
		return err
	}

	// Request
	if err := cfg.Request.Proxy.validate(); err != nil {
		return err
	}
	if cfg.Request.IpPool.Enabled && len(cfg.Request.IpPool.Subnets) == 0 {
		return fmt.Errorf("IpPool enabled but no subnets specified")
	}

	// Site
	if len(cfg.Sites) == 0 {
		return fmt.Errorf("no site is configured")
	}
	siteIdxById := make(map[string]int)
	for siteIdx, siteCfg := range cfg.Sites {
		if prevIdx, ok := siteIdxById[siteCfg.Id]; ok {
			return fmt.Errorf("[site%d] id %+q is already used by site%d", siteIdx, siteCfg.Id, prevIdx)
		}
		siteIdxById[siteCfg.Id] = siteIdx
		if !cfg.Request.IpPool.Enabled && siteCfg.IpPoolStrategy != nil && *siteCfg.IpPoolStrategy != IpPoolStrategyNone {
			return fmt.Errorf("[site%d] IP pool is not enabled, but site IP pool strategy is set to %+q", siteIdx, *siteCfg.IpPoolStrategy)
		}
		if err := validatePathPrefix(siteCfg.PathPrefix); err != nil {
			return fmt.Errorf("[site%d] bad pathPrefix: %v", siteIdx, err)
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
		case SiteModeContainerRegistrySingleProxy:
			settings := siteCfg.Settings.(*ContainerRegistrySingleProxySettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			if settings.UpstreamAuthRealmUrl != nil {
				if err := checkUrl(*settings.UpstreamAuthRealmUrl, "UpstreamAuthRealmUrl", true, false); err != nil {
					return err
				}
			}
			if err := checkUrl(*settings.UpstreamV2Url, "UpstreamV2Url", true, false); err != nil {
				return err
			}
			if err := settings.Auth.validate(); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
		case SiteModeContainerRegistryAnyProxy:
			settings := siteCfg.Settings.(*ContainerRegistryAnyProxySettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			if err := settings.Auth.validate(); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
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
			if settings.Destination == "" && len(settings.Mappings) == 0 {
				return fmt.Errorf("[site%d] neither Destination nor Mappings is provided", siteIdx)
			}
			for i, mapping := range settings.Mappings {
				if mapping == nil {
					return fmt.Errorf("[site%d] Mappings[%d] is nil", siteIdx, i)
				}
				if err := validatePathPrefix(mapping.Path); err != nil {
					return fmt.Errorf("[site%d] bad Mappings[%d].Path: %v", siteIdx, i, err)
				}
			}
		case SiteModeAnyProxy:
			settings := siteCfg.Settings.(*AnyProxySettings)
			if err := validateDomainRules("DomainWhitelist", settings.DomainWhitelist); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
			if err := validateDomainRules("DomainBlacklist", settings.DomainBlacklist); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
			if err := settings.Auth.validate(); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
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

func (c *RequestProxyConfig) validate() error {
	for idx, server := range c.Servers {
		urlObj, err := url.Parse(server.Url)
		if err != nil {
			return fmt.Errorf("failed to parse Request.Proxy.Servers[%d].Url: %v", idx, err)
		}
		switch urlObj.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("Request.Proxy.Servers[%d].Url %+q has unsupported scheme %+q, should be one of http, https, socks5, socks5h", idx, urlObj.Redacted(), urlObj.Scheme)
		}
		if urlObj.Hostname() == "" {
			return fmt.Errorf("Request.Proxy.Servers[%d].Url %+q has no host", idx, urlObj.Redacted())
		}
	}
	if c.DirectFallback && len(c.Servers) == 0 {
		return fmt.Errorf("Request.Proxy.DirectFallback is enabled but no proxy server is configured")
	}
	if *c.ConnectTimeout <= 0 {
		return fmt.Errorf("Request.Proxy.ConnectTimeout should be > 0, value: %v", *c.ConnectTimeout)
	}
	if *c.Block.FailureThreshold < 1 {
		return fmt.Errorf("Request.Proxy.Block.FailureThreshold should be >= 1, value: %v", *c.Block.FailureThreshold)
	}
	if *c.Block.Cooldown <= 0 {
		return fmt.Errorf("Request.Proxy.Block.Cooldown should be > 0, value: %v", *c.Block.Cooldown)
	}
	if *c.Block.MaxCooldown < *c.Block.Cooldown {
		return fmt.Errorf("Request.Proxy.Block.MaxCooldown (%v) should be >= Request.Proxy.Block.Cooldown (%v)", *c.Block.MaxCooldown, *c.Block.Cooldown)
	}
	return nil
}

func (a *ClientAuthConfig) validate() error {
	if !a.Enabled {
		return nil
	}
	if len(a.Users) == 0 && a.UsersFile == "" {
		return fmt.Errorf("Auth is enabled but has no users")
	}
	for userIdx, userCfg := range a.Users {
		if err := ValidateUser(userCfg); err != nil {
			return fmt.Errorf("Auth.Users[%d] validation failed: %v", userIdx, err)
		}
	}
	if a.UsersFile != "" && !utils.IsFile(a.UsersFile) {
		return fmt.Errorf("Auth.UsersFile %+q is not a valid file", a.UsersFile)
	}
	if a.UsersFileReloadInterval != nil && a.UsersFile == "" {
		return fmt.Errorf("Auth.UsersFileReloadInterval is set but Auth.UsersFile is not")
	}
	if a.UsersFileReloadInterval != nil && *a.UsersFileReloadInterval <= 1*time.Second {
		return fmt.Errorf("Auth.UsersFileReloadInterval %q is too small", a.UsersFileReloadInterval.String())
	}
	return nil
}

func validateDomainRules(what string, rules []string) error {
	for idx, rule := range rules {
		domain := strings.TrimPrefix(rule, "*.")
		if domain == "" || strings.ContainsAny(domain, "*/: ") {
			return fmt.Errorf("%s[%d] %+q is neither a domain nor a *.domain pattern", what, idx, rule)
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

func validatePathPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if prefix == "/" {
		return fmt.Errorf("%+q is not allowed, leave it empty to match all paths", prefix)
	}
	if !strings.HasPrefix(prefix, "/") {
		return fmt.Errorf("%+q does not start with '/'", prefix)
	}
	if strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("%+q must not end with '/'", prefix)
	}
	return nil
}
