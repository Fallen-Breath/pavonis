package config

import "time"

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

type User struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

type CrAuthConfig struct {
	// TODO: read from file for hot reload
	Enabled bool    `yaml:"enabled"`
	Users   []*User `yaml:"users"`
}

type ContainerRegistrySettings struct {
	PathPrefix       string        `yaml:"path_prefix"`
	SelfUrl          string        `yaml:"self_url"`
	UpstreamV2Url    *string       `yaml:"upstream_v2_url"`    // no trailing '/'
	UpstreamTokenUrl *string       `yaml:"upstream_token_url"` // no trailing '/'
	Authorization    *CrAuthConfig `yaml:"authorization"`      // if enabled, push is not allowed
	AllowPush        *bool         `yaml:"allow_push"`
}

type PypiRegistrySettings struct {
	PathPrefix        string  `yaml:"path_prefix"`
	UpstreamSimpleUrl *string `yaml:"upstream_simple_url"` // no trailing '/'
	UpstreamFilesUrl  *string `yaml:"upstream_files_url"`  // no trailing '/'
}

type SpeedTestSettings struct {
	MaxUploadBytes   *int64 `yaml:"max_upload_bytes"`
	MaxDownloadBytes *int64 `yaml:"max_download_bytes"`
}

type SiteConfig struct {
	Mode           SiteMode        `yaml:"mode"`
	Host           SiteHosts       `yaml:"host"`
	IpPoolStrategy *IpPoolStrategy `yaml:"ip_pool_strategy"`
	Settings       interface{}     `yaml:"settings"`
}

type ServerConfig struct {
	Listen              *string   `yaml:"listen"`
	TrustedProxyIps     *[]string `yaml:"trusted_proxy_ips"`
	TrustedProxyHeaders *[]string `yaml:"trusted_proxy_headers"`
}

type IpPoolConfig struct {
	Enabled         bool            `yaml:"enabled"`
	DefaultStrategy *IpPoolStrategy `yaml:"default_strategy"`
	Subnets         []string        `yaml:"subnets"`
}

type HeaderModificationConfig struct {
	Modify *map[string]string `yaml:"modify"`
	Delete *[]string          `yaml:"delete"`
}

type RequestConfig struct {
	Proxy  string                    `yaml:"proxy"`
	IpPool *IpPoolConfig             `yaml:"ip_pool"`
	Header *HeaderModificationConfig `yaml:"header"`
}

type ResponseConfig struct {
	Header *HeaderModificationConfig `yaml:"header"`
}

// ResourceLimitConfig fields will be null in runtime
type ResourceLimitConfig struct {
	TrafficAvgMibps  *float64       `yaml:"traffic_avg_mibps"`
	TrafficBurstMib  *float64       `yaml:"traffic_burst_mib"`
	TrafficMaxMibps  *float64       `yaml:"traffic_max_mibps"`
	RequestPerSecond *float64       `yaml:"request_per_second"`
	RequestPerMinute *float64       `yaml:"request_per_minute"`
	RequestPerHour   *float64       `yaml:"request_per_hour"`
	RequestTimeout   *time.Duration `yaml:"request_timeout"`
}

type Config struct {
	Debug         bool                 `yaml:"debug"`
	Server        *ServerConfig        `yaml:"server"`
	Request       *RequestConfig       `yaml:"request"`
	Response      *ResponseConfig      `yaml:"response"`
	ResourceLimit *ResourceLimitConfig `yaml:"resource_limit"`
	Sites         []*SiteConfig        `yaml:"sites"`
}
