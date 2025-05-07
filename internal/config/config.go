package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/url"
)

type HttpGeneralProxyMapping struct {
	Path        string `yaml:"path"`
	Destination string `yaml:"destination"`
}

type HttpGeneralProxySettings struct {
	Mappings []HttpGeneralProxyMapping `yaml:"mappings"`
}

type GithubDownloadProxySettings struct {
	SizeLimit      int      `yaml:"size_limit"`
	ReposWhitelist []string `yaml:"repos_whitelist"`
	ReposBlacklist []string `yaml:"repos_blacklist"`
	ReposBypass    []string `yaml:"repos_bypass"`
}

type ContainerRegistrySettings struct {
	SelfUrl          string `yaml:"self_url"`
	UpstreamV2Url    string `yaml:"upstream_v2_url"`
	UpstreamTokenUrl string `yaml:"upstream_token_url"`

	ParsedUpstreamV2Url    *url.URL
	ParsedUpstreamTokenUrl *url.URL
}

type SiteConfig struct {
	Mode     SiteMode    `yaml:"mode"`
	Host     string      `yaml:"host"`
	Settings interface{} `yaml:"settings"`
}

type IpPoolConfig struct {
	Strategy IpPoolStrategy `yaml:"strategy"`
	Subnets  []string       `yaml:"subnets"`
}

func NewIpPoolConfig() *IpPoolConfig {
	return &IpPoolConfig{Strategy: IpPoolStrategyNone}
}

type Config struct {
	Listen string        `yaml:"listen"`
	Sites  []*SiteConfig `yaml:"sites"`
	IpPool *IpPoolConfig `yaml:"ip_pool"`
}

func (c *Config) Init() error {
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
		c.Listen = ":8080"
	}
	if c.IpPool == nil {
		c.IpPool = NewIpPoolConfig()
	}

	// Validate && Parse values
	for siteIdx, site := range c.Sites {
		switch site.Mode {
		case HttpGeneralProxy:
			settings := site.Settings.(*HttpGeneralProxySettings)
			_ = settings
		case GithubDownloadProxy:
			settings := site.Settings.(*GithubDownloadProxySettings)
			_ = settings
		case ContainerRegistryProxy:
			settings := site.Settings.(*ContainerRegistrySettings)
			var err error
			if _, err = url.Parse(settings.SelfUrl); err != nil {
				return fmt.Errorf("[site %d] failed to parse self url %v: %v", siteIdx, settings.SelfUrl, err)
			}
			if settings.ParsedUpstreamTokenUrl, err = url.Parse(settings.UpstreamTokenUrl); err != nil {
				return fmt.Errorf("[site %d] failed to parse upstream /token url %v: %v", siteIdx, settings.UpstreamTokenUrl, err)
			}
			if settings.ParsedUpstreamV2Url, err = url.Parse(settings.UpstreamV2Url); err != nil {
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
