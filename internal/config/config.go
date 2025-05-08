package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/url"
)

func (c *Config) Init() error {
	// logging level
	if c.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}

	// Set sub-setting classes
	for siteIdx, site := range c.Sites {
		if site == nil {
			return fmt.Errorf("[site %d] site config is nil", siteIdx)
		}
		settingsData, _ := yaml.Marshal(site.Settings)
		switch site.Mode {
		case HttpGeneralProxy:
			settings := HttpGeneralProxySettings{}
			if err := yaml.Unmarshal(settingsData, &settings); err != nil {
				return fmt.Errorf("[site %d] failed to unmarshal settings mode %v: %v", siteIdx, site.Mode, err)
			}
			site.Settings = &settings
		case GithubDownloadProxy:
			settings := GithubDownloadProxySettings{}
			if err := yaml.Unmarshal(settingsData, &settings); err != nil {
				return fmt.Errorf("[site %d] failed to unmarshal settings mode %v: %v", siteIdx, site.Mode, err)
			}
			site.Settings = &settings
		case ContainerRegistryProxy:
			settings := ContainerRegistrySettings{}
			if err := yaml.Unmarshal(settingsData, &settings); err != nil {
				return fmt.Errorf("[site %d] failed to unmarshal settings mode %v: %v", siteIdx, site.Mode, err)
			}
			site.Settings = &settings

		default:
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

		switch site.Mode {
		case HttpGeneralProxy:
			settings := site.Settings.(*HttpGeneralProxySettings)
			_ = settings
		case GithubDownloadProxy:
			settings := site.Settings.(*GithubDownloadProxySettings)
			_ = settings
		case ContainerRegistryProxy:
			settings := site.Settings.(*ContainerRegistrySettings)
			if _, err := url.Parse(settings.SelfUrl); err != nil {
				return fmt.Errorf("[site %d] failed to parse self url %v: %v", siteIdx, settings.SelfUrl, err)
			}
			if _, err := url.Parse(settings.UpstreamTokenUrl); err != nil {
				return fmt.Errorf("[site %d] failed to parse upstream /token url %v: %v", siteIdx, settings.UpstreamTokenUrl, err)
			}
			if _, err := url.Parse(settings.UpstreamV2Url); err != nil {
				return fmt.Errorf("[site %d] failed to parse upstream /v2 url %v: %v", siteIdx, settings.UpstreamV2Url, err)
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
