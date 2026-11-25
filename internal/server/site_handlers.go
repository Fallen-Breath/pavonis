package server

import (
	"fmt"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/anyproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/crproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/ghproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/hfproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/httpproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/npmproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/pypiproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/speedtest"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/staticfile"
	"github.com/Fallen-Breath/pavonis/internal/server/limits"
)

func createSiteHandler(cfg *config.Config, siteCfg *config.SiteConfig, helper *common.RequestHelper, limitManager *limits.Manager) (handler.HttpHandler, error) {
	info := handler.NewSiteInfo(siteCfg.Id, siteCfg, cfg.ResourceLimit.ResolveRequestTimeout(siteCfg.QuotaGroup))
	hdl, err := createSiteHttpHandler(*siteCfg.Mode, info, helper, siteCfg.Settings)
	if err != nil {
		return nil, err
	}
	if *siteCfg.Mode == config.SiteModeSpeedTest && len(siteCfg.StaticFiles) == 0 {
		return hdl, nil
	}
	policy := limitManager.CreateSitePolicy(siteCfg.Id, siteCfg.QuotaGroup)
	// speed test measures the bandwidth without the limits
	if *siteCfg.Mode != config.SiteModeSpeedTest {
		hdl = limits.WrapHandler(hdl, policy)
	}
	if len(siteCfg.StaticFiles) > 0 {
		hdl = staticfile.WrapHandler(hdl, policy, siteCfg.StaticFiles)
	}
	return hdl, nil
}

func createSiteHttpHandler(mode config.SiteMode, info *handler.Info, helper *common.RequestHelper, settings interface{}) (handler.HttpHandler, error) {
	switch mode {

	case config.SiteModeContainerRegistrySingle:
		return crproxy.NewContainerRegistrySingleHandler(info, helper, settings.(*config.ContainerRegistrySingleSiteSettings))
	case config.SiteModeContainerRegistryAny:
		return crproxy.NewContainerRegistryAnyHandler(info, helper, settings.(*config.ContainerRegistryAnySiteSettings))
	case config.SiteModeGithub:
		return ghproxy.NewGithubHandler(info, helper, settings.(*config.GithubSiteSettings))
	case config.SiteModeHuggingFace:
		return hfproxy.NewHuggingFaceHandler(info, helper, settings.(*config.HuggingFaceSiteSettings))
	case config.SiteModeHttp:
		return httpproxy.NewHttpHandler(info, helper, settings.(*config.HttpSiteSettings))
	case config.SiteModeAnyUrl:
		return anyproxy.NewAnyUrlHandler(info, helper, settings.(*config.AnyUrlSiteSettings))
	case config.SiteModeNpm:
		return npmproxy.NewNpmHandler(info, helper, settings.(*config.NpmSiteSettings))
	case config.SiteModePypi:
		return pypiproxy.NewPypiHandler(info, helper, settings.(*config.PypiSiteSettings))
	case config.SiteModeSpeedTest:
		return speedtest.NewSpeedTestHandler(info, helper, settings.(*config.SpeedTestSiteSettings))

	default:
		return nil, fmt.Errorf("unknown mode %s", mode)
	}
}
