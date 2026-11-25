package config

import "time"

type HttpSiteMapping struct {
	Path        string `yaml:"path"`
	Destination string `yaml:"destination"`
}

type HttpSiteBodyReplacement struct {
	Search  string `yaml:"search"`
	Replace string `yaml:"replace"`
}

type HttpSiteBodyRewrite struct {
	Paths        []string                  `yaml:"paths"`         // path prefixes, empty means all paths
	ContentTypes []string                  `yaml:"content_types"` // "type/subtype", "type/*" or "*/*"
	Replacements []HttpSiteBodyReplacement `yaml:"replacements"`
}

type HttpSiteSettings struct {
	Destination    string                 `yaml:"destination"`
	Mappings       []*HttpSiteMapping     `yaml:"mappings"`
	RedirectAction *RedirectAction        `yaml:"redirect_action"`
	BodyRewrites   []*HttpSiteBodyRewrite `yaml:"body_rewrites"`
}

type GithubSiteSettings struct {
	SizeLimit         int64    `yaml:"size_limit"`
	RawTextUrlRewrite bool     `yaml:"raw_text_url_rewrite"`
	AllowPush         bool     `yaml:"allow_push"`
	ReposWhitelist    []string `yaml:"repos_whitelist"`
	ReposBlacklist    []string `yaml:"repos_blacklist"`
}

type AnyUrlSiteSettings struct {
	AllowedMethods      []string          `yaml:"allowed_methods"`
	DomainWhitelist     []string          `yaml:"domain_whitelist"`
	DomainBlacklist     []string          `yaml:"domain_blacklist"`
	AllowPrivateNetwork bool              `yaml:"allow_private_network"`
	SizeLimit           int64             `yaml:"size_limit"`
	Auth                *ClientAuthConfig `yaml:"auth"`
}

type HuggingFaceSiteSettings struct {
	SizeLimit int64 `yaml:"size_limit"`
}

type User struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

type UsersFile struct {
	Users []*User `yaml:"users"`
}

type ClientAuthConfig struct {
	Enabled                 bool           `yaml:"enabled"`
	Users                   []*User        `yaml:"users"`
	UsersFile               string         `yaml:"users_file"`
	UsersFileReloadInterval *time.Duration `yaml:"users_file_reload_interval"`
}

type ContainerRegistrySingleSiteSettings struct {
	UpstreamV1Url        *string           `yaml:"upstream_v1_url"`         // no trailing '/', might be nil
	UpstreamV2Url        *string           `yaml:"upstream_v2_url"`         // no trailing '/'
	UpstreamAuthRealmUrl *string           `yaml:"upstream_auth_realm_url"` // no trailing '/', might be nil
	Auth                 *ClientAuthConfig `yaml:"auth"`
	AllowPush            *bool             `yaml:"allow_push"`
	AllowList            *bool             `yaml:"allow_list"`
	ReposWhitelist       []string          `yaml:"repos_whitelist"`
	ReposBlacklist       []string          `yaml:"repos_blacklist"`
}

type ContainerRegistryAnySiteSettings struct {
	Auth           *ClientAuthConfig `yaml:"auth"`
	AllowList      *bool             `yaml:"allow_list"`
	ReposWhitelist []string          `yaml:"repos_whitelist"`
	ReposBlacklist []string          `yaml:"repos_blacklist"`
}

type PypiSiteSettings struct {
	UpstreamSimpleUrl *string `yaml:"upstream_simple_url"` // no trailing '/'
	UpstreamFilesUrl  *string `yaml:"upstream_files_url"`  // no trailing '/'
}

type SpeedTestSiteSettings struct {
	MaxUploadBytes   *int64 `yaml:"max_upload_bytes"`
	MaxDownloadBytes *int64 `yaml:"max_download_bytes"`
}

type StaticFileConfig struct {
	Path        string  `yaml:"path"`
	File        string  `yaml:"file"`
	Content     *string `yaml:"content"`
	ContentType string  `yaml:"content_type"`
}

type SiteConfig struct {
	Id             string              `yaml:"id"`
	Mode           *SiteMode           `yaml:"mode"`
	Host           SiteHosts           `yaml:"host"`
	SelfUrl        string              `yaml:"self_url"` // only scheme + host, not path (excluding path_prefix), not trailing '/'
	PathPrefix     string              `yaml:"path_prefix"`
	QuotaGroup     string              `yaml:"quota_group"`
	IpPoolStrategy *IpPoolStrategy     `yaml:"ip_pool_strategy"`
	StaticFiles    []*StaticFileConfig `yaml:"static_files"`
	Settings       interface{}         `yaml:"settings"`

	file string // only set on an unresolved site reference
}

type TLSCertificateConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type ServerTLSConfig struct {
	Certificates []TLSCertificateConfig `yaml:"certificates"`
}

type ServerConfig struct {
	TLS                 *ServerTLSConfig `yaml:"tls"`
	Listen              *string          `yaml:"listen"`
	MaxConnections      *int             `yaml:"max_connections"`
	ReadHeaderTimeout   *time.Duration   `yaml:"read_header_timeout"`
	IdleTimeout         *time.Duration   `yaml:"idle_timeout"`
	TrustedProxyIps     *[]string        `yaml:"trusted_proxy_ips"`
	TrustedProxyHeaders *[]string        `yaml:"trusted_proxy_headers"`
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

type RequestProxyServerConfig struct {
	Id            string        `yaml:"id"`
	Url           string        `yaml:"url"`
	ProxyProtocol ProxyProtocol `yaml:"proxy_protocol"`
	Weight        *int64        `yaml:"weight"`
}

type RequestProxyBlockConfig struct {
	FailureThreshold *int32         `yaml:"failure_threshold"`
	Cooldown         *time.Duration `yaml:"cooldown"`
	MaxCooldown      *time.Duration `yaml:"max_cooldown"`
}

type RequestProxyConfig struct {
	Strategy       *RequestProxyStrategy       `yaml:"strategy"`
	Servers        []*RequestProxyServerConfig `yaml:"servers"`
	DirectFallback bool                        `yaml:"direct_fallback"`
	ConnectTimeout *time.Duration              `yaml:"connect_timeout"`
	Block          *RequestProxyBlockConfig    `yaml:"block"`
}

type RequestConfig struct {
	Proxy  *RequestProxyConfig       `yaml:"proxy"`
	IpPool *IpPoolConfig             `yaml:"ip_pool"`
	Header *HeaderModificationConfig `yaml:"header"`
}

type ResponseConfig struct {
	Header      *HeaderModificationConfig `yaml:"header"`
	MaxRedirect *int32                    `yaml:"max_redirects"`
}

type ResourceLimitConfig struct {
	MaxConcurrent429Responses *int                  `yaml:"max_concurrent_429_responses"`
	IdleClientCacheSize       *int                  `yaml:"idle_client_cache_size"`
	Global                    ResourceQuotaConfig   `yaml:"global"`
	Groups                    []QuotaGroupConfig    `yaml:"groups"`
}

type QuotaGroupConfig struct {
	Id                  string `yaml:"id"`
	ResourceQuotaConfig `yaml:",inline"`
}

type ResourceQuotaConfig struct {
	RequestTimeout *time.Duration    `yaml:"request_timeout"`
	PerClient      ClientQuotaConfig `yaml:"per_client"`
	AllClients     ClientQuotaConfig `yaml:"all_clients"`
}

// ClientQuotaConfig is the limits under per_client or all_clients. nil fields mean unlimited
type ClientQuotaConfig struct {
	TrafficAvgMibps    *float64 `yaml:"traffic_avg_mibps"`
	TrafficBurstMib    *float64 `yaml:"traffic_burst_mib"`
	TrafficMaxMibps    *float64 `yaml:"traffic_max_mibps"`
	RequestConcurrency *int     `yaml:"request_concurrency"`
	RequestPerSecond   *float64 `yaml:"request_per_second"`
	RequestPerMinute   *float64 `yaml:"request_per_minute"`
	RequestPerHour     *float64 `yaml:"request_per_hour"`
}

type DiagnosticsConfig struct {
	Enabled bool    `yaml:"enabled"`
	Listen  *string `yaml:"listen"`
}

type Config struct {
	Debug         bool                 `yaml:"debug"`
	Server        *ServerConfig        `yaml:"server"`
	Request       *RequestConfig       `yaml:"request"`
	Response      *ResponseConfig      `yaml:"response"`
	ResourceLimit *ResourceLimitConfig `yaml:"resource_limit"`
	Diagnostics   *DiagnosticsConfig   `yaml:"diagnostics"`
	Sites         []*SiteConfig        `yaml:"sites"`
}
