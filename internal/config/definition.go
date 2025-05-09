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

type ServerConfig struct {
	Listen         *string `yaml:"listen"`
	TrustedProxies *string `yaml:"trusted_proxies"`
}

type IpPoolConfig struct {
	Enabled         bool            `yaml:"enabled"`
	DefaultStrategy *IpPoolStrategy `yaml:"default_strategy"`
	Subnets         []string        `yaml:"subnets"`
}

type ResourceLimitConfig struct {
	TrafficAvgMibps  *float64 `yaml:"traffic_avg_mibps"`
	TrafficBurstMib  *float64 `yaml:"traffic_burst_mib"`
	TrafficMaxMibps  *float64 `yaml:"traffic_max_mibps"`
	RequestPerSecond *float64 `yaml:"request_per_second"`
	RequestPerMinute *float64 `yaml:"request_per_minute"`
	RequestPerHour   *float64 `yaml:"request_per_hour"`
}

type Config struct {
	Debug         bool                 `yaml:"debug"`
	Server        *ServerConfig        `yaml:"server"`
	ResourceLimit *ResourceLimitConfig `yaml:"resource_limit"`
	IpPool        *IpPoolConfig        `yaml:"ip_pool"`
	Sites         []*SiteConfig        `yaml:"sites"`
}
