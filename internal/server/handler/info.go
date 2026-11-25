package handler

import (
	"fmt"
	"net/url"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
)

type Info struct {
	Id         string
	PathPrefix string
	SelfUrl    string

	RequestTimeout time.Duration
}

// ExtractEscapedSitePath returns the escaped path relative to the mount selected by the router.
func (info *Info) ExtractEscapedSitePath(u *url.URL) string {
	path := u.EscapedPath()
	rest, ok := utils.CutPathPrefix(path, info.PathPrefix)
	if !ok {
		panic(fmt.Errorf("path %q does not match site prefix %q", path, info.PathPrefix))
	}
	return rest
}

func NewSiteInfo(id string, siteCfg *config.SiteConfig, requestTimeout time.Duration) *Info {
	return &Info{
		Id:         id,
		PathPrefix: siteCfg.PathPrefix,
		SelfUrl:    siteCfg.SelfUrl,

		RequestTimeout: requestTimeout,
	}
}
