package config

import (
	"fmt"
	"math"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/utils"
	"golang.org/x/exp/slices"
)

func (cfg *Config) validateValues() error {
	// Server
	if *cfg.Server.Listen == "" {
		return fmt.Errorf("server.listen must not be empty")
	}
	if *cfg.Server.MaxConnections <= 0 {
		return fmt.Errorf("server.max_connections must be > 0, value: %v", *cfg.Server.MaxConnections)
	}
	if *cfg.Server.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("server.read_header_timeout must be > 0, value: %v", *cfg.Server.ReadHeaderTimeout)
	}
	if *cfg.Server.IdleTimeout <= 0 {
		return fmt.Errorf("server.idle_timeout must be > 0, value: %v", *cfg.Server.IdleTimeout)
	}
	if cfg.Diagnostics.Enabled && *cfg.Diagnostics.Listen == "" {
		return fmt.Errorf("diagnostics.listen must not be empty when diagnostics is enabled")
	}
	if !slices.Contains(*cfg.Server.TrustedProxyIps, "*") {
		if _, err := utils.NewIpPool(*cfg.Server.TrustedProxyIps); err != nil {
			return fmt.Errorf("bad TrustedProxyIps value %+q: %v", *cfg.Server.TrustedProxyIps, err)
		}
	}

	// ResourceLimit
	if *cfg.ResourceLimit.MaxConcurrent429Responses <= 0 {
		return fmt.Errorf("resource_limit.max_concurrent_429_responses must be > 0, value: %v", *cfg.ResourceLimit.MaxConcurrent429Responses)
	}
	if *cfg.ResourceLimit.IdleClientCacheSize <= 0 {
		return fmt.Errorf("resource_limit.idle_client_cache_size must be > 0, value: %v", *cfg.ResourceLimit.IdleClientCacheSize)
	}
	if err := cfg.ResourceLimit.Global.validate("resource_limit.global"); err != nil {
		return err
	}
	groupIds := make(map[string]bool)
	for groupIdx, group := range cfg.ResourceLimit.Groups {
		path := fmt.Sprintf("resource_limit.groups[%d]", groupIdx)
		if group.Id == "" {
			return fmt.Errorf("%s.id is required", path)
		}
		if groupIds[group.Id] {
			return fmt.Errorf("%s.id %+q is duplicated", path, group.Id)
		}
		groupIds[group.Id] = true
		if err := group.validate(path); err != nil {
			return err
		}
	}

	// Request
	if err := cfg.Request.Proxy.validate(); err != nil {
		return err
	}
	if cfg.Request.IpPool.Enabled && len(cfg.Request.IpPool.Subnets) == 0 {
		return fmt.Errorf("IpPool enabled but no subnets specified")
	}

	// Site
	if len(cfg.Sites) == 0 {
		return fmt.Errorf("no site is configured")
	}
	siteIdxById := make(map[string]int)
	for siteIdx, siteCfg := range cfg.Sites {
		if siteCfg.ResourceGroup != "" && !groupIds[siteCfg.ResourceGroup] {
			return fmt.Errorf("sites[%d].resource_group %+q is not defined in resource_limit.groups", siteIdx, siteCfg.ResourceGroup)
		}
		if len(siteCfg.Host) == 0 {
			return fmt.Errorf("sites[%d].host is required", siteIdx)
		}
		for hostIdx, host := range siteCfg.Host {
			if host == "" {
				return fmt.Errorf("sites[%d].host[%d] must not be empty", siteIdx, hostIdx)
			}
		}
		if prevIdx, ok := siteIdxById[siteCfg.Id]; ok {
			return fmt.Errorf("[site%d] id %+q is already used by site%d", siteIdx, siteCfg.Id, prevIdx)
		}
		siteIdxById[siteCfg.Id] = siteIdx
		if !cfg.Request.IpPool.Enabled && siteCfg.IpPoolStrategy != nil && *siteCfg.IpPoolStrategy != IpPoolStrategyNone {
			return fmt.Errorf("[site%d] IP pool is not enabled, but site IP pool strategy is set to %+q", siteIdx, *siteCfg.IpPoolStrategy)
		}
		if err := validatePathPrefix(siteCfg.PathPrefix); err != nil {
			return fmt.Errorf("[site%d] bad pathPrefix: %v", siteIdx, err)
		}
		if err := validateStaticFiles(siteCfg.StaticFiles); err != nil {
			return fmt.Errorf("[site%d] %v", siteIdx, err)
		}

		checkUrl := func(urlStr, what string, allowPath, allowTrailingSlash bool) error {
			if err := validateHttpUrl(urlStr, allowPath, allowTrailingSlash); err != nil {
				return fmt.Errorf("sites[%d].%s: %v", siteIdx, what, err)
			}
			return nil
		}

		var checkSelfUrlReason *string

		switch *siteCfg.Mode {
		case SiteModeContainerRegistrySingle:
			settings := siteCfg.Settings.(*ContainerRegistrySingleSiteSettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			if settings.UpstreamV1Url != nil {
				if err := checkUrl(*settings.UpstreamV1Url, "settings.upstream_v1_url", true, false); err != nil {
					return err
				}
			}
			if settings.UpstreamAuthRealmUrl != nil {
				if err := checkUrl(*settings.UpstreamAuthRealmUrl, "settings.upstream_auth_realm_url", true, false); err != nil {
					return err
				}
			}
			if err := checkUrl(*settings.UpstreamV2Url, "settings.upstream_v2_url", true, false); err != nil {
				return err
			}
			if err := settings.Auth.validate(); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
		case SiteModeContainerRegistryAny:
			settings := siteCfg.Settings.(*ContainerRegistryAnySiteSettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			if err := settings.Auth.validate(); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
		case SiteModeGithub:
			settings := siteCfg.Settings.(*GithubSiteSettings)
			if settings.RawTextUrlRewrite {
				checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("RawTextUrlRewrite is %v", settings.RawTextUrlRewrite))
			}
		case SiteModeHuggingFace:
			settings := siteCfg.Settings.(*HuggingFaceSiteSettings)
			checkSelfUrlReason = utils.ToPtr(fmt.Sprintf("site mode is %s", *siteCfg.Mode))
			_ = settings
		case SiteModeHttp:
			settings := siteCfg.Settings.(*HttpSiteSettings)
			checkDestination := func(destination, field string) error {
				if err := checkUrl(destination, field, true, true); err != nil {
					return err
				}
				if strings.ContainsAny(destination, "?#") {
					return fmt.Errorf("sites[%d].%s: query or fragment is not allowed", siteIdx, field)
				}
				return nil
			}
			if settings.Destination == "" && len(settings.Mappings) == 0 {
				return fmt.Errorf("[site%d] neither Destination nor Mappings is provided", siteIdx)
			}
			if settings.Destination != "" {
				if err := checkDestination(settings.Destination, "settings.destination"); err != nil {
					return err
				}
			}
			for i, mapping := range settings.Mappings {
				if mapping == nil {
					return fmt.Errorf("[site%d] Mappings[%d] is nil", siteIdx, i)
				}
				if err := validatePathPrefix(mapping.Path); err != nil {
					return fmt.Errorf("[site%d] bad Mappings[%d].Path: %v", siteIdx, i, err)
				}
				if err := checkDestination(mapping.Destination, fmt.Sprintf("settings.mappings[%d].destination", i)); err != nil {
					return err
				}
			}
			for i, rewrite := range settings.BodyRewrites {
				if rewrite == nil {
					return fmt.Errorf("[site%d] BodyRewrites[%d] is nil", siteIdx, i)
				}
				for j, path := range rewrite.Paths {
					if err := validatePathPrefix(path); err != nil {
						return fmt.Errorf("[site%d] bad BodyRewrites[%d].Paths[%d]: %v", siteIdx, i, j, err)
					}
				}
				for j, contentType := range rewrite.ContentTypes {
					if err := validateMediaTypePattern(contentType); err != nil {
						return fmt.Errorf("[site%d] bad BodyRewrites[%d].ContentTypes[%d]: %v", siteIdx, i, j, err)
					}
				}
			}
		case SiteModeAnyUrl:
			settings := siteCfg.Settings.(*AnyUrlSiteSettings)
			if err := validateDomainRules("DomainWhitelist", settings.DomainWhitelist); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
			if err := validateDomainRules("DomainBlacklist", settings.DomainBlacklist); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
			if err := settings.Auth.validate(); err != nil {
				return fmt.Errorf("[site%d] %v", siteIdx, err)
			}
		case SiteModePypi:
			settings := siteCfg.Settings.(*PypiSiteSettings)
			if err := checkUrl(*settings.UpstreamSimpleUrl, "settings.upstream_simple_url", true, false); err != nil {
				return err
			}
			if err := checkUrl(*settings.UpstreamFilesUrl, "settings.upstream_files_url", true, false); err != nil {
				return err
			}
		case SiteModeSpeedTest:
			settings := siteCfg.Settings.(*SpeedTestSiteSettings)
			_ = settings
		}

		if checkSelfUrlReason == nil && siteCfg.SelfUrl != "" {
			checkSelfUrlReason = utils.ToPtr("SelfUrl is not empty")
		}
		if checkSelfUrlReason != nil {
			// XXX: should path be allowed here? maybe the user has configured another upper-level reversed proxy
			if err := checkUrl(siteCfg.SelfUrl, "self_url", false, false); err != nil {
				return fmt.Errorf("%v, check reason: %s", err, *checkSelfUrlReason)
			}
		}
	}

	return nil
}

func (q *ResourceQuotaConfig) validate(path string) error {
	if q.RequestTimeout != nil && *q.RequestTimeout <= 0 {
		return fmt.Errorf("%s.request_timeout must be > 0, value: %v", path, *q.RequestTimeout)
	}
	if err := q.PerClient.validate(path + ".per_client"); err != nil {
		return err
	}
	return q.AllClients.validate(path + ".all_clients")
}

func (q *ClientQuotaConfig) validate(path string) error {
	if q.RequestConcurrency != nil && *q.RequestConcurrency <= 0 {
		return fmt.Errorf("%s.request_concurrency must be > 0, value: %v", path, *q.RequestConcurrency)
	}
	for _, check := range []struct {
		value *float64
		name  string
	}{
		{q.TrafficAvgMibps, "traffic_avg_mibps"},
		{q.TrafficBurstMib, "traffic_burst_mib"},
		{q.TrafficMaxMibps, "traffic_max_mibps"},
	} {
		if err := checkPositiveLimit(check.value, path+"."+check.name); err != nil {
			return err
		}
	}
	for _, check := range []struct {
		value    *float64
		name     string
		interval time.Duration
	}{
		{q.RequestPerSecond, "request_per_second", time.Second},
		{q.RequestPerMinute, "request_per_minute", time.Minute},
		{q.RequestPerHour, "request_per_hour", time.Hour},
	} {
		if err := checkRequestRateLimit(check.value, path+"."+check.name, check.interval); err != nil {
			return err
		}
	}
	return nil
}

func checkPositiveLimit(value *float64, what string) error {
	if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
		return fmt.Errorf("%s must be finite, value: %v", what, *value)
	}
	if value != nil && *value <= 0 {
		return fmt.Errorf("%s cannot <= 0, value: %v", what, *value)
	}
	return nil
}

func checkRequestRateLimit(value *float64, what string, interval time.Duration) error {
	if err := checkPositiveLimit(value, what); err != nil {
		return err
	}
	// Require at least one request per day to bound request-rate refill time
	const maxRequestInterval = 24 * time.Hour
	minimum := interval.Seconds() / maxRequestInterval.Seconds()
	if value != nil && *value < minimum {
		return fmt.Errorf("%s must be >= %g (at least one request per %s), value: %v", what, minimum, maxRequestInterval, *value)
	}
	return nil
}

func validateHttpUrl(rawUrl string, allowPath, allowTrailingSlash bool) error {
	urlObj, err := url.Parse(rawUrl)
	if err != nil {
		return err
	}
	if urlObj.Scheme != "http" && urlObj.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", urlObj.Scheme)
	}
	if urlObj.Hostname() == "" {
		return fmt.Errorf("host is missing")
	}
	if err := validateUrlPort(urlObj); err != nil {
		return err
	}
	if allowPath && !allowTrailingSlash && strings.HasSuffix(urlObj.Path, "/") {
		return fmt.Errorf("trailing '/' is not allowed")
	}
	if !allowPath && len(urlObj.Path) > 0 {
		return fmt.Errorf("path is not allowed")
	}
	return nil
}

func validateUrlPort(urlObj *url.URL) error {
	if port := urlObj.Port(); port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return fmt.Errorf("port must be between 1 and 65535, got %q", port)
		}
	}
	return nil
}

func (c *RequestProxyConfig) validate() error {
	serverIdxById := make(map[string]int)
	for idx, server := range c.Servers {
		if prevIdx, ok := serverIdxById[server.Id]; ok {
			return fmt.Errorf("request.proxy.servers[%d].id %+q is already used by request.proxy.servers[%d]", idx, server.Id, prevIdx)
		}
		serverIdxById[server.Id] = idx
		urlObj, err := url.Parse(server.Url)
		if err != nil {
			return fmt.Errorf("failed to parse Request.Proxy.Servers[%d].Url: %v", idx, err)
		}
		switch urlObj.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("Request.Proxy.Servers[%d].Url %+q has unsupported scheme %+q, should be one of http, https, socks5, socks5h", idx, urlObj.Redacted(), urlObj.Scheme)
		}
		if urlObj.Hostname() == "" {
			return fmt.Errorf("Request.Proxy.Servers[%d].Url %+q has no host", idx, urlObj.Redacted())
		}
		if err := validateUrlPort(urlObj); err != nil {
			return fmt.Errorf("request.proxy.servers[%d].url: %v", idx, err)
		}
		if *server.Weight < 1 || *server.Weight > maxRequestProxyWeight {
			return fmt.Errorf("Request.Proxy.Servers[%d].Weight should be between 1 and %d, value: %v", idx, maxRequestProxyWeight, *server.Weight)
		}
	}
	if c.DirectFallback && len(c.Servers) == 0 {
		return fmt.Errorf("Request.Proxy.DirectFallback is enabled but no proxy server is configured")
	}
	if *c.ConnectTimeout <= 0 {
		return fmt.Errorf("Request.Proxy.ConnectTimeout should be > 0, value: %v", *c.ConnectTimeout)
	}
	if *c.Block.FailureThreshold < 1 {
		return fmt.Errorf("Request.Proxy.Block.FailureThreshold should be >= 1, value: %v", *c.Block.FailureThreshold)
	}
	if *c.Block.Cooldown <= 0 {
		return fmt.Errorf("Request.Proxy.Block.Cooldown should be > 0, value: %v", *c.Block.Cooldown)
	}
	if *c.Block.MaxCooldown < *c.Block.Cooldown {
		return fmt.Errorf("Request.Proxy.Block.MaxCooldown (%v) should be >= Request.Proxy.Block.Cooldown (%v)", *c.Block.MaxCooldown, *c.Block.Cooldown)
	}
	return nil
}

func (a *ClientAuthConfig) validate() error {
	if !a.Enabled {
		return nil
	}
	if len(a.Users) == 0 && a.UsersFile == "" {
		return fmt.Errorf("Auth is enabled but has no users")
	}
	for userIdx, userCfg := range a.Users {
		if err := ValidateUser(userCfg); err != nil {
			return fmt.Errorf("Auth.Users[%d] validation failed: %v", userIdx, err)
		}
	}
	if a.UsersFile != "" && !utils.IsFile(a.UsersFile) {
		return fmt.Errorf("Auth.UsersFile %+q is not a valid file", a.UsersFile)
	}
	if a.UsersFileReloadInterval != nil && a.UsersFile == "" {
		return fmt.Errorf("Auth.UsersFileReloadInterval is set but Auth.UsersFile is not")
	}
	if a.UsersFileReloadInterval != nil && *a.UsersFileReloadInterval <= 1*time.Second {
		return fmt.Errorf("Auth.UsersFileReloadInterval %q is too small", a.UsersFileReloadInterval.String())
	}
	return nil
}

func validateDomainRules(what string, rules []string) error {
	for idx, rule := range rules {
		domain := strings.TrimPrefix(rule, "*.")
		if domain == "" || strings.ContainsAny(domain, "*/: ") {
			return fmt.Errorf("%s[%d] %+q is neither a domain nor a *.domain pattern", what, idx, rule)
		}
	}
	return nil
}

func ValidateUser(userCfg *User) error {
	if userCfg == nil {
		return fmt.Errorf("userCfg is nil")
	}

	if userCfg.Name == "" {
		return fmt.Errorf("name is empty")
	}
	if userCfg.Password == "" {
		return fmt.Errorf("password is empty")
	}

	// check '$' for the upstream name / password split, ':' for basic auth
	if strings.Contains(userCfg.Name, "$") || strings.Contains(userCfg.Name, ":") {
		return fmt.Errorf("name contains illegal char '$' or ':'")
	}
	if strings.Contains(userCfg.Password, "$") || strings.Contains(userCfg.Password, ":") {
		return fmt.Errorf("password contains illegal char '$' or ':'")
	}

	return nil
}

// validateMediaTypePattern accepts "type/subtype", "type/*" and "*/*" only, case-insensitively
func validateMediaTypePattern(pattern string) error {
	mediaType, _, err := mime.ParseMediaType(pattern)
	if err != nil {
		return fmt.Errorf("%+q is not a valid media type: %v", pattern, err)
	}
	// mime.ParseMediaType tolerates whitespaces, parameters and a trailing ';', which are not allowed here
	if strings.ToLower(pattern) != mediaType {
		return fmt.Errorf("%+q must be a bare media type, without whitespaces or parameters", pattern)
	}
	mainType, subType, ok := strings.Cut(mediaType, "/")
	if !ok || mainType == "" || subType == "" {
		return fmt.Errorf("%+q is not in the form of type/subtype", pattern)
	}
	if mainType == "*" && subType != "*" {
		return fmt.Errorf("%+q is not allowed, use */* to match all types", pattern)
	}
	return nil
}

func validateStaticFiles(staticFiles []*StaticFileConfig) error {
	seenPaths := make(map[string]int)
	for i, staticFile := range staticFiles {
		if staticFile == nil {
			return fmt.Errorf("StaticFiles[%d] is nil", i)
		}
		if err := validateEscapedPath(staticFile.Path); err != nil {
			return fmt.Errorf("bad StaticFiles[%d].Path: %v", i, err)
		}
		if prevIdx, ok := seenPaths[staticFile.Path]; ok {
			return fmt.Errorf("StaticFiles[%d].Path %+q is already used by StaticFiles[%d]", i, staticFile.Path, prevIdx)
		}
		seenPaths[staticFile.Path] = i

		if (staticFile.File != "") == (staticFile.Content != nil) {
			return fmt.Errorf("StaticFiles[%d] must have exactly one of File and Content", i)
		}
		if staticFile.File != "" {
			f, _, err := utils.OpenRegularFile(staticFile.File)
			if err != nil {
				return fmt.Errorf("bad StaticFiles[%d].File: %v", i, err)
			}
			_ = f.Close()
		}
		if staticFile.ContentType != "" {
			if _, _, err := mime.ParseMediaType(staticFile.ContentType); err != nil {
				return fmt.Errorf("bad StaticFiles[%d].ContentType %+q: %v", i, staticFile.ContentType, err)
			}
		}
	}
	return nil
}

// validateEscapedPath accepts a bare escaped URL path starting with '/', without query or fragment
func validateEscapedPath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("%+q does not start with '/'", path)
	}
	u, err := url.ParseRequestURI(path)
	if err != nil || u.EscapedPath() != path {
		return fmt.Errorf("%+q is not a valid escaped URL path", path)
	}
	return nil
}

func validatePathPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if prefix == "/" {
		return fmt.Errorf("%+q is not allowed, leave it empty to match all paths", prefix)
	}
	if strings.HasPrefix(prefix, "/") && strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("%+q must not end with '/'", prefix)
	}
	return validateEscapedPath(prefix)
}
