package handler

type Info struct {
	Id         string
	PathPrefix string
}

func NewSiteInfo(id, pathPrefix string) *Info {
	return &Info{
		Id:         id,
		PathPrefix: pathPrefix,
	}
}
