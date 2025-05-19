package handler

import "github.com/Fallen-Breath/pavonis/internal/config"

type Info struct {
	Id         string
	PathPrefix string
	SelfUrl    string
}

func NewSiteInfo(id string, siteCfg *config.SiteConfig) *Info {
	return &Info{
		Id:         id,
		PathPrefix: siteCfg.PathPrefix,
		SelfUrl:    siteCfg.SelfUrl,
	}
}
