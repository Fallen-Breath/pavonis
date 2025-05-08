package config

type HttpGeneralProxyMapping struct {
	Path        string `yaml:"path"`
	Destination string `yaml:"destination"`
}

type HttpGeneralProxySettings struct {
	Destination string                    `yaml:"destination"`
	Mappings    []HttpGeneralProxyMapping `yaml:"mappings"`
}

type GithubDownloadProxySettings struct {
	SizeLimit      int64    `yaml:"size_limit"`
	ReposWhitelist []string `yaml:"repos_whitelist"`
	ReposBlacklist []string `yaml:"repos_blacklist"`
	ReposBypass    []string `yaml:"repos_bypass"`
}

type ContainerRegistrySettings struct {
	PathPrefix       string `yaml:"path_prefix"`
	SelfUrl          string `yaml:"self_url"`
	UpstreamV2Url    string `yaml:"upstream_v2_url"`    // no trailing '/'
	UpstreamTokenUrl string `yaml:"upstream_token_url"` // no trailing '/'
}

type SiteConfig struct {
	Mode           SiteMode        `yaml:"mode"`
	Host           string          `yaml:"host"`
	IpPoolStrategy *IpPoolStrategy `yaml:"ip_pool_strategy"`
	Settings       interface{}     `yaml:"settings"`
}

type IpPoolConfig struct {
	Enabled         bool           `yaml:"enabled"`
	DefaultStrategy IpPoolStrategy `yaml:"default_strategy"`
	Subnets         []string       `yaml:"subnets"`
}

type Config struct {
	Listen string        `yaml:"listen"`
	Debug  bool          `yaml:"debug"`
	Sites  []*SiteConfig `yaml:"sites"`
	IpPool *IpPoolConfig `yaml:"ip_pool"`
}
