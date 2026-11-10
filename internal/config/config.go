package config

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func (cfg *Config) finalizeValues() error {
	siteSettingMapping := make(map[SiteMode]func() any)
	siteSettingMapping[SiteModeContainerRegistrySingleProxy] = func() any {
		return &ContainerRegistrySingleProxySettings{}
	}
	siteSettingMapping[SiteModeContainerRegistryAnyProxy] = func() any {
		return &ContainerRegistryAnyProxySettings{}
	}
	siteSettingMapping[SiteModeGithubDownloadProxy] = func() any {
		return &GithubDownloadProxySettings{}
	}
	siteSettingMapping[SiteModeHuggingFaceProxy] = func() any {
		return &HuggingFaceProxySettings{}
	}
	siteSettingMapping[SiteModeHttpGeneralProxy] = func() any {
		return &HttpGeneralProxySettings{}
	}
	siteSettingMapping[SiteModeAnyProxy] = func() any { return &AnyProxySettings{} }
	siteSettingMapping[SiteModePypiProxy] = func() any {
		return &PypiRegistrySettings{}
	}
	siteSettingMapping[SiteModeSpeedTest] = func() any {
		return &SpeedTestSettings{}
	}

	// Set sub-setting classes
	for siteIdx, siteCfg := range cfg.Sites {
		if siteCfg == nil {
			return fmt.Errorf("[site%d] site config is nil", siteIdx)
		}
		settingsData, _ := yaml.Marshal(siteCfg.Settings)
		if settingFactory, ok := siteSettingMapping[*siteCfg.Mode]; ok {
			settings := settingFactory()
			if err := yaml.Unmarshal(settingsData, settings); err != nil {
				return fmt.Errorf("[site%d] failed to unmarshal settings mode %v: %v", siteIdx, *siteCfg.Mode, err)
			}
			siteCfg.Settings = settings
		} else {
			return fmt.Errorf("[site%d] has invalid mode %v", siteIdx, *siteCfg.Mode)
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
	if err := cfg.setDefaultValues(); err != nil {
		return fmt.Errorf("[config] Default value initalization failed: %v", err)
	}
	if err := cfg.validateValues(); err != nil {
		return fmt.Errorf("[config] Validation failed: %v", err)
	}
	if err := cfg.applyConfig(); err != nil {
		return fmt.Errorf("[config] Applying failed: %v", err)
	}
	return nil
}
