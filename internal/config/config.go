package config

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"strings"
)

func (cfg *Config) finalizeValues() error {
	siteSettingMapping := make(map[SiteMode]func() any)
	siteSettingMapping[SiteModeContainerRegistryProxy] = func() any {
		return &ContainerRegistrySettings{}
	}
	siteSettingMapping[SiteModeGithubDownloadProxy] = func() any {
		return &GithubDownloadProxySettings{}
	}
	siteSettingMapping[SiteModeHttpGeneralProxy] = func() any {
		return &HttpGeneralProxySettings{}
	}
	siteSettingMapping[SiteModePypiProxy] = func() any {
		return &PypiRegistrySettings{}
	}
	siteSettingMapping[SiteModeSpeedTest] = func() any {
		return &SpeedTestSettings{}
	}
	siteSettingMapping[SiteModePavonis] = func() any {
		return &PavonisSiteSettings{}
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

func (cfg *Config) Dump() {
	if len(cfg.Sites) == 0 {
		log.Warning("No site defined in config")
	}
	for siteIdx, siteCfg := range cfg.Sites {
		siteInfo := []string{
			fmt.Sprintf("mode=%s", *siteCfg.Mode),
			fmt.Sprintf("host=%s", siteCfg.Host),
		}
		if siteCfg.PathPrefix != "" {
			siteInfo = append(siteInfo, "path_prefix="+siteCfg.PathPrefix)
		}
		log.Infof("site%d (id=%s): %s", siteIdx, siteCfg.Id, strings.Join(siteInfo, " "))

		switch *siteCfg.Mode {
		case SiteModeContainerRegistryProxy:
			settings := siteCfg.Settings.(*ContainerRegistrySettings)
			log.Infof("  %+v", settings)
		case SiteModeGithubDownloadProxy:
			settings := siteCfg.Settings.(*GithubDownloadProxySettings)
			log.Infof("  %+v", settings)
		case SiteModeHttpGeneralProxy:
			settings := siteCfg.Settings.(*HttpGeneralProxySettings)
			if settings.Destination != "" {
				log.Infof("  -> %+q", settings.Destination)
			}
			for _, mapping := range settings.Mappings {
				log.Infof("  %+q -> %+q", mapping.Path, mapping.Destination)
			}
		case SiteModePavonis:
			settings := siteCfg.Settings.(*PavonisSiteSettings)
			_ = settings
		case SiteModePypiProxy:
			settings := siteCfg.Settings.(*PypiRegistrySettings)
			log.Infof("  %+v", settings)
		case SiteModeSpeedTest:
			settings := siteCfg.Settings.(*SpeedTestSettings)
			log.Infof("  MaxUpload=%s, MaxDownload=%s", utils.PrettyByteSize(*settings.MaxUploadBytes), utils.PrettyByteSize(*settings.MaxUploadBytes))
		}
	}

	if cfg.ResourceLimit.RequestPerSecond != nil || cfg.ResourceLimit.RequestPerMinute != nil || cfg.ResourceLimit.RequestPerHour != nil {
		var parts []string
		if cfg.ResourceLimit.RequestPerSecond != nil {
			parts = append(parts, fmt.Sprintf("qps=%v", *cfg.ResourceLimit.RequestPerSecond))
		}
		if cfg.ResourceLimit.RequestPerMinute != nil {
			parts = append(parts, fmt.Sprintf("qpm=%v", *cfg.ResourceLimit.RequestPerMinute))
		}
		if cfg.ResourceLimit.RequestPerHour != nil {
			parts = append(parts, fmt.Sprintf("qph=%v", *cfg.ResourceLimit.RequestPerHour))
		}
		log.Infof("Request Limit: %s", strings.Join(parts, ", "))
	}
	if cfg.ResourceLimit.TrafficAvgMibps != nil || cfg.ResourceLimit.TrafficBurstMib != nil || cfg.ResourceLimit.TrafficMaxMibps != nil {
		var parts []string
		if cfg.ResourceLimit.TrafficAvgMibps != nil {
			parts = append(parts, fmt.Sprintf("avg=%vMiB/s", *cfg.ResourceLimit.TrafficAvgMibps))
		}
		if cfg.ResourceLimit.TrafficBurstMib != nil {
			parts = append(parts, fmt.Sprintf("burst=%vMiB", *cfg.ResourceLimit.TrafficBurstMib))
		}
		if cfg.ResourceLimit.TrafficMaxMibps != nil {
			parts = append(parts, fmt.Sprintf("max=%vMiB/s", *cfg.ResourceLimit.TrafficMaxMibps))
		}
		log.Infof("Traffic Limit: %s", strings.Join(parts, ", "))
	}
}
