package config

import "fmt"

type SiteMode string
type IpPoolStrategy string

const (
	SiteModeContainerRegistryProxy SiteMode = "container_registry"
	SiteModeGithubDownloadProxy    SiteMode = "github_proxy"
	SiteModeHttpGeneralProxy       SiteMode = "http"
	SiteModePavonis                SiteMode = "pavonis"
	SiteModePypiProxy              SiteMode = "pypi"
	SiteModeSpeedTest              SiteMode = "speed_test"

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
	return unmarshalStringEnum(s, unmarshal, "site mode", SiteModeHttpGeneralProxy, []SiteMode{
		SiteModeContainerRegistryProxy,
		SiteModeGithubDownloadProxy,
		SiteModeHttpGeneralProxy,
		SiteModePavonis,
		SiteModePypiProxy,
		SiteModeSpeedTest,
	})
}

func (s *IpPoolStrategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringEnum(s, unmarshal, "strategy", IpPoolStrategyNone, []IpPoolStrategy{
		IpPoolStrategyNone,
		IpPoolStrategyRandom,
		IpPoolStrategyIpHash,
	})
}

type SiteHosts []string

func unmarshalStringOrStringList[T ~[]string](obj *T, unmarshal func(interface{}) error, what string) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*obj = []string{single}
		return nil
	}

	var list []string
	if err := unmarshal(&list); err == nil {
		*obj = list
		return nil
	}

	return fmt.Errorf("invalid format for %s, should be a string or a list of string", what)
}

func (s *SiteHosts) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringOrStringList(s, unmarshal, "SiteHosts")
}

func (s *SiteHosts) IsWildcard() bool {
	return len(*s) == 1 && (*s)[0] == "*"
}
