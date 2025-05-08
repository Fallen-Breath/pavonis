package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/url"
	"strings"
)

func (c *Config) Init() error {
	// logging level
	if c.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}

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
	for siteIdx, site := range c.Sites {
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
	if c.Listen == "" {
		c.Listen = ":8009"
	}
	if c.IpPool == nil {
		c.IpPool = &IpPoolConfig{
			Enabled:         false,
			DefaultStrategy: IpPoolStrategyNone,
		}
	}

	// Validate && Parse values
	for siteIdx, site := range c.Sites {
		if !c.IpPool.Enabled && site.IpPoolStrategy != nil && *site.IpPoolStrategy != IpPoolStrategyNone {
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

	return nil
}

func (c *Config) Dump() {
	if len(c.Sites) == 0 {
		log.Warning("No site defined in config")
	}
	for siteIdx, site := range c.Sites {
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
}
