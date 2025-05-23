package config

import "fmt"

type SiteMode string
type IpPoolStrategy string
type RedirectAction string

const (
	SiteModeContainerRegistryProxy SiteMode = "container_registry"
	SiteModeGithubDownloadProxy    SiteMode = "github_proxy"
	SiteModeHttpGeneralProxy       SiteMode = "http"
	SiteModeHuggingFaceProxy       SiteMode = "hugging_face"
	SiteModePavonis                SiteMode = "pavonis"
	SiteModePypiProxy              SiteMode = "pypi"
	SiteModeSpeedTest              SiteMode = "speed_test"

	IpPoolStrategyNone   IpPoolStrategy = "none"
	IpPoolStrategyRandom IpPoolStrategy = "random"
	IpPoolStrategyIpHash IpPoolStrategy = "ip_hash"

	RedirectActionFollowAll       RedirectAction = "follow_all"        // follow all
	RedirectActionRewriteOrFollow RedirectAction = "rewrite_or_follow" // rewrite relative, follow external,
	RedirectActionRewriteOnly     RedirectAction = "rewrite_only"      // rewrite relative only
	RedirectActionNone            RedirectAction = "none"              // do nothing
)

func unmarshalStringEnum[T ~string](obj *T, unmarshal func(interface{}) error, what string, values []T) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	strValue := T(str)
	for _, value := range values {
		if strValue == value {
			*obj = strValue
			return nil
		}
	}

	return fmt.Errorf("invalid %s: %s", what, str)
}

func (s *SiteMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringEnum(s, unmarshal, "site mode", []SiteMode{
		SiteModeContainerRegistryProxy,
		SiteModeGithubDownloadProxy,
		SiteModeHttpGeneralProxy,
		SiteModeHuggingFaceProxy,
		SiteModePavonis,
		SiteModePypiProxy,
		SiteModeSpeedTest,
	})
}

func (s *IpPoolStrategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringEnum(s, unmarshal, "strategy", []IpPoolStrategy{
		IpPoolStrategyNone,
		IpPoolStrategyRandom,
		IpPoolStrategyIpHash,
	})
}

func (s *RedirectAction) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshalStringEnum(s, unmarshal, "redirect action", []RedirectAction{
		RedirectActionFollowAll,
		RedirectActionRewriteOrFollow,
		RedirectActionRewriteOnly,
		RedirectActionNone,
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
