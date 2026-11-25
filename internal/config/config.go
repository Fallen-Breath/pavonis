package config

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"
)

type siteSettingsType struct {
	new    func() any
	decode func(unmarshal func(any) error) (any, error)
}

func siteSettingsOf[T any]() siteSettingsType {
	return siteSettingsType{
		new: func() any { return new(T) },
		decode: func(unmarshal func(any) error) (any, error) {
			var site struct {
				Settings *T                   `yaml:"settings"`
				Others   map[string]yaml.Node `yaml:",inline"`
			}
			if err := unmarshal(&site); err != nil {
				return nil, err
			}
			if site.Settings == nil {
				return new(T), nil
			}
			return site.Settings, nil
		},
	}
}

var siteSettingsTypes = map[SiteMode]siteSettingsType{
	SiteModeContainerRegistrySingle: siteSettingsOf[ContainerRegistrySingleSiteSettings](),
	SiteModeContainerRegistryAny:    siteSettingsOf[ContainerRegistryAnySiteSettings](),
	SiteModeGithub:                  siteSettingsOf[GithubSiteSettings](),
	SiteModeHuggingFace:             siteSettingsOf[HuggingFaceSiteSettings](),
	SiteModeHttp:                    siteSettingsOf[HttpSiteSettings](),
	SiteModeAnyUrl:                  siteSettingsOf[AnyUrlSiteSettings](),
	SiteModePypi:                    siteSettingsOf[PypiSiteSettings](),
	SiteModeSpeedTest:               siteSettingsOf[SpeedTestSiteSettings](),
}

// UnmarshalYAML decodes settings into the struct of the site mode. It reuses the decoder of
// the whole config, so that the strict mode and the line numbers in errors are retained
func (s *SiteConfig) UnmarshalYAML(unmarshal func(any) error) error {
	var ref struct {
		File   *string              `yaml:"file"`
		Others map[string]yaml.Node `yaml:",inline"`
	}
	if err := unmarshal(&ref); err != nil {
		return err
	}
	if ref.File != nil {
		if len(ref.Others) > 0 {
			return fmt.Errorf("site reference %+q cannot have other fields: %s", *ref.File, strings.Join(slices.Sorted(maps.Keys(ref.Others)), ", "))
		}
		if *ref.File == "" {
			return fmt.Errorf("site reference has an empty file")
		}
		s.file = *ref.File
		return nil
	}

	type siteConfig SiteConfig
	if err := unmarshal((*siteConfig)(s)); err != nil {
		return err
	}
	if s.Mode == nil {
		return nil
	}
	settings, err := siteSettingsTypes[*s.Mode].decode(unmarshal)
	if err != nil {
		return err
	}
	s.Settings = settings
	return nil
}

func (cfg *Config) finalizeValues() error {
	for siteIdx, siteCfg := range cfg.Sites {
		if siteCfg == nil {
			return fmt.Errorf("[site%d] site config is nil", siteIdx)
		}
		if siteCfg.Mode == nil {
			return fmt.Errorf("[site%d] site mode is not provided", siteIdx)
		}
		settingsType, ok := siteSettingsTypes[*siteCfg.Mode]
		if !ok {
			return fmt.Errorf("[site%d] has invalid mode %v", siteIdx, *siteCfg.Mode)
		}
		if siteCfg.Settings == nil {
			siteCfg.Settings = settingsType.new()
		} else if want := reflect.TypeOf(settingsType.new()); reflect.TypeOf(siteCfg.Settings) != want {
			return fmt.Errorf("[site%d] settings type %T does not match mode %v, should be %v", siteIdx, siteCfg.Settings, *siteCfg.Mode, want)
		}
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

func (cfg *Config) Init() error {
	if err := cfg.finalizeValues(); err != nil {
		return fmt.Errorf("[config] Finalization failed: %v", err)
	}
	if err := cfg.applyDefaultsAndNormalize(); err != nil {
		return fmt.Errorf("[config] Applying defaults and normalizing values failed: %v", err)
	}
	if err := cfg.validateValues(); err != nil {
		return fmt.Errorf("[config] Validation failed: %v", err)
	}
	if err := cfg.applyConfig(); err != nil {
		return fmt.Errorf("[config] Applying failed: %v", err)
	}
	return nil
}

// ResolveRequestTimeout returns the request timeout of a site in the given quota group. "" means no group
func (c *ResourceLimitConfig) ResolveRequestTimeout(group string) time.Duration {
	timeout := *c.Global.RequestTimeout
	for _, g := range c.Groups {
		if g.Id == group && g.RequestTimeout != nil {
			timeout = min(timeout, *g.RequestTimeout)
		}
	}
	return timeout
}
