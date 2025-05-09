package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/url"
	"strings"
)

func (cfg *Config) finalizeValues() error {
	siteSettingMapping := make(map[SiteMode]func() any)
	siteSettingMapping[HttpGeneralProxy] = func() any {
		return &HttpGeneralProxySettings{}
	}
	siteSettingMapping[GithubDownloadProxy] = func() any {
		return &GithubDownloadProxySettings{}
	}
	siteSettingMapping[ContainerRegistryProxy] = func() any {
		return &ContainerRegistrySettings{}
	}

	// Set sub-setting classes
	for siteIdx, site := range cfg.Sites {
		if site == nil {
			return fmt.Errorf("[site %d] site config is nil", siteIdx)
		}
		settingsData, _ := yaml.Marshal(site.Settings)
		if settingFactory, ok := siteSettingMapping[site.Mode]; ok {
			settings := settingFactory()
			if err := yaml.Unmarshal(settingsData, settings); err != nil {
				return fmt.Errorf("[site %d] failed to unmarshal settings mode %v: %v", siteIdx, site.Mode, err)
			}
			site.Settings = settings
		} else {
			return fmt.Errorf("[site %d] has invalid mode %v", siteIdx, site.Mode)
		}
	}

	// Set default values
	if cfg.Listen == "" {
		cfg.Listen = ":8009"
	}
	if cfg.IpPool == nil {
		cfg.IpPool = &IpPoolConfig{}
	}
	if cfg.IpPool.DefaultStrategy == nil {
		strategy := IpPoolStrategyNone
		cfg.IpPool.DefaultStrategy = &strategy
	}
	if cfg.RateLimit == nil {
		cfg.RateLimit = &RateLimitConfig{}
	}

	return nil
}

func (cfg *Config) applyConfig() error {
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}
	return nil
}

func (cfg *Config) validateValues() error {
	for siteIdx, site := range cfg.Sites {
		if !cfg.IpPool.Enabled && site.IpPoolStrategy != nil && *site.IpPoolStrategy != IpPoolStrategyNone {
			return fmt.Errorf("[site %d] IP pool is not enabled, but site IP pool strategy is set to %q", siteIdx, *site.IpPoolStrategy)
		}

		checkUrl := func(urlStr, what string, allowPath, allowTrailingSlash bool) error {
			urlObj, err := url.Parse(urlStr)
			if err != nil || urlObj == nil {
				return fmt.Errorf("[site %d] failed to parse %s %+q: %v", siteIdx, what, urlStr, err)
			}
			if urlObj.Scheme == "" {
				return fmt.Errorf("[site %d] bad %s %+q: scheme missing", siteIdx, what, urlStr)
			}
			if allowPath && !allowTrailingSlash && strings.HasSuffix(urlObj.Path, "/") {
				return fmt.Errorf("[site %d] bad %s %+q: trailing '/' is not allowed", siteIdx, what, urlStr)
			}
			if !allowPath && len(urlObj.Path) > 0 {
				return fmt.Errorf("[site %d] bad %s %+q: path is not allowed", siteIdx, what, urlStr)
			}
			return nil
		}

		switch site.Mode {
		case HttpGeneralProxy:
			settings := site.Settings.(*HttpGeneralProxySettings)
			_ = settings
		case GithubDownloadProxy:
			settings := site.Settings.(*GithubDownloadProxySettings)
			_ = settings
		case ContainerRegistryProxy:
			settings := site.Settings.(*ContainerRegistrySettings)
			if err := checkUrl(settings.SelfUrl, "SelfUrl", false, false); err != nil {
				return err
			}
			if err := checkUrl(settings.UpstreamTokenUrl, "UpstreamTokenUrl", true, false); err != nil {
				return err
			}
			if err := checkUrl(settings.UpstreamV2Url, "UpstreamV2Url", true, false); err != nil {
				return err
			}
		}
	}

	checkGreaterThanZero := func(value *float64, what string) error {
		if value != nil && *value <= 0 {
			return fmt.Errorf("%s cannot <= 0, value: %v", what, *value)
		}
		return nil
	}

	if err := checkGreaterThanZero(cfg.RateLimit.TrafficAvgMibps, "RateLimit.TrafficAvgMibps"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.RateLimit.TrafficBurstMib, "RateLimit.TrafficBurstMib"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.RateLimit.TrafficMaxMibps, "RateLimit.TrafficMaxMibps"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.RateLimit.RequestPerSecond, "RateLimit.RequestPerSecond"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.RateLimit.RequestPerMinute, "RateLimit.RequestPerMinute"); err != nil {
		return err
	}
	if err := checkGreaterThanZero(cfg.RateLimit.RequestPerHour, "RateLimit.RequestPerHour"); err != nil {
		return err
	}
	return nil
}

func (cfg *Config) Init() error {
	if err := cfg.finalizeValues(); err != nil {
		return fmt.Errorf("[config] Finalization failed: %v", err)
	}
	if err := cfg.validateValues(); err != nil {
		return fmt.Errorf("[config] Validation failed: %v", err)
	}
	if err := cfg.applyConfig(); err != nil {
		return fmt.Errorf("[config] Applying failed: %v", err)
	}
	return nil
}

func (cfg *Config) Dump() {
	if len(cfg.Sites) == 0 {
		log.Warning("No site defined in config")
	}
	for siteIdx, site := range cfg.Sites {
		log.Infof("site %d: mode=%s host=%s", siteIdx, site.Mode, site.Host)
		switch site.Mode {
		case HttpGeneralProxy:
			settings := site.Settings.(*HttpGeneralProxySettings)
			for _, mapping := range settings.Mappings {
				log.Infof("  %q -> %q", mapping.Path, mapping.Destination)
			}
		case GithubDownloadProxy:
			settings := site.Settings.(*GithubDownloadProxySettings)
			log.Infof("  %+v", settings)
		case ContainerRegistryProxy:
			settings := site.Settings.(*ContainerRegistrySettings)
			log.Infof("  %+v", settings)
		}
	}

	if cfg.RateLimit.RequestPerSecond != nil || cfg.RateLimit.RequestPerMinute != nil || cfg.RateLimit.RequestPerHour != nil {
		var parts []string
		if cfg.RateLimit.RequestPerSecond != nil {
			parts = append(parts, fmt.Sprintf("qps=%v", *cfg.RateLimit.RequestPerSecond))
		}
		if cfg.RateLimit.RequestPerMinute != nil {
			parts = append(parts, fmt.Sprintf("qpm=%v", *cfg.RateLimit.RequestPerMinute))
		}
		if cfg.RateLimit.RequestPerHour != nil {
			parts = append(parts, fmt.Sprintf("qph=%v", *cfg.RateLimit.RequestPerHour))
		}
		log.Infof("Request Limit: %s", strings.Join(parts, ", "))
	}
	if cfg.RateLimit.TrafficAvgMibps != nil || cfg.RateLimit.TrafficBurstMib != nil || cfg.RateLimit.TrafficMaxMibps != nil {
		var parts []string
		if cfg.RateLimit.TrafficAvgMibps != nil {
			parts = append(parts, fmt.Sprintf("avg=%vMiB/s", *cfg.RateLimit.TrafficAvgMibps))
		}
		if cfg.RateLimit.TrafficBurstMib != nil {
			parts = append(parts, fmt.Sprintf("burst=%vMiB", *cfg.RateLimit.TrafficBurstMib))
		}
		if cfg.RateLimit.TrafficMaxMibps != nil {
			parts = append(parts, fmt.Sprintf("max=%vMiB/s", *cfg.RateLimit.TrafficMaxMibps))
		}
		log.Infof("Traffic Limit: %s", strings.Join(parts, ", "))
	}
}
