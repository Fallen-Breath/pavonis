package config

import "fmt"

type SiteMode string
type IpPoolStrategy string

const (
	HttpGeneralProxy       SiteMode = "http"
	GithubDownloadProxy    SiteMode = "github_proxy"
	ContainerRegistryProxy SiteMode = "container_registry"
	PypiProxy              SiteMode = "pypi"

	IpPoolStrategyNone   IpPoolStrategy = "none"
	IpPoolStrategyRandom IpPoolStrategy = "random"
	IpPoolStrategyIpHash IpPoolStrategy = "ip_hash"
)

func unmarshalStringEnum[T ~string](obj *T, unmarshal func(interface{}) error, what string, defaultValue T, values []T) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}
	if str == "" {
		*obj = defaultValue
	} else {
		*obj = T(str)
		for _, value := range values {
			if *obj == value {
				return nil
			}
		}
		return fmt.Errorf("invalid %s: %s", what, str)
	}
	return nil
}

func (s *SiteMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringEnum(s, unmarshal, "strategy", HttpGeneralProxy, []SiteMode{
		HttpGeneralProxy,
		GithubDownloadProxy,
		ContainerRegistryProxy,
		PypiProxy,
	})
}

func (s *IpPoolStrategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringEnum(s, unmarshal, "strategy", IpPoolStrategyNone, []IpPoolStrategy{
		IpPoolStrategyNone,
		IpPoolStrategyRandom,
		IpPoolStrategyIpHash,
	})
}
