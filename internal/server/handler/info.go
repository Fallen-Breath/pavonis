package handler

type Info struct {
	Name       string
	PathPrefix string
}

func NewSiteInfo(name, pathPrefix string) *Info {
	return &Info{
		Name:       name,
		PathPrefix: pathPrefix,
	}
}
