package handler

import (
	"fmt"
	"net/url"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
)

type Info struct {
	Id         string
	PathPrefix string
	SelfUrl    string
}

// EscapedSitePath returns the escaped path relative to the mount selected by the router.
func (info *Info) EscapedSitePath(u *url.URL) string {
	path := u.EscapedPath()
	rest, ok := utils.CutPathPrefix(path, info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("path %q does not match site prefix %q", path, info.PathPrefix))
	}
	return rest
}

func NewSiteInfo(id string, siteCfg *config.SiteConfig) *Info {
	return &Info{
		Id:         id,
		PathPrefix: siteCfg.PathPrefix,
		SelfUrl:    siteCfg.SelfUrl,
	}
}
