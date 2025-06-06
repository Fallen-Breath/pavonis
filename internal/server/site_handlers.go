package server

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/crproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/ghproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/hfproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/httpproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/pypiproxy"
	"github.com/Fallen-Breath/pavonis/internal/server/handler/speedtest"
)

func createSiteHttpHandler(mode config.SiteMode, info *handler.Info, helper *common.RequestHelper, settings interface{}) (handler.HttpHandler, error) {
	switch mode {

	case config.SiteModeContainerRegistryProxy:
		return crproxy.NewContainerRegistryProxyHandler(info, helper, settings.(*config.ContainerRegistrySettings))
	case config.SiteModeGithubDownloadProxy:
		return ghproxy.NewGithubProxyHandler(info, helper, settings.(*config.GithubDownloadProxySettings))
	case config.SiteModeHuggingFaceProxy:
		return hfproxy.NewHuggingFaceProxyHandler(info, helper, settings.(*config.HuggingFaceProxySettings))
	case config.SiteModeHttpGeneralProxy:
		return httpproxy.NewProxyHandler(info, helper, settings.(*config.HttpGeneralProxySettings))
	case config.SiteModePypiProxy:
		return pypiproxy.NewProxyHandler(info, helper, settings.(*config.PypiRegistrySettings))
	case config.SiteModeSpeedTest:
		return speedtest.NewSpeedTestHandler(info, helper, settings.(*config.SpeedTestSettings))

	default:
		return nil, fmt.Errorf("unknown mode %s", mode)
	}
}
