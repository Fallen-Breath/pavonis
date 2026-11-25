package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

// dumpTable collects labeled rows, and logs them with the labels aligned in a column, e.g.
//
//	upstream  https://example.com
//	access    pull only, list off
type dumpTable struct {
	indent string
	lines  []dumpTableLine
}

type dumpTableLine struct {
	label string // empty for continuation lines
	text  string
}

func newDumpTable(indent string) *dumpTable {
	return &dumpTable{indent: indent}
}

// add adds a single-line row with non-empty parts joined by ", ". The row is skipped if all parts are empty
func (t *dumpTable) add(label string, parts ...string) {
	var nonEmptyParts []string
	for _, part := range parts {
		if part != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}
	if len(nonEmptyParts) > 0 {
		t.addLines(label, strings.Join(nonEmptyParts, ", "))
	}
}

// addLines adds a multi-line row with the label shown on its first line only. Empty lines are skipped
func (t *dumpTable) addLines(label string, lines ...string) {
	for _, line := range lines {
		if line == "" {
			continue
		}
		t.lines = append(t.lines, dumpTableLine{label: label, text: line})
		label = ""
	}
}

func (t *dumpTable) log() {
	labelWidth := 0
	for _, line := range t.lines {
		labelWidth = max(labelWidth, len(line.label))
	}
	for _, line := range t.lines {
		log.Infof("%s%-*s  %s", t.indent, labelWidth, line.label, line.text)
	}
}

func (cfg *Config) Dump() {
	log.Info("Global:")
	cfg.dumpGlobal()

	log.Infof("Sites (%d):", len(cfg.Sites))
	if len(cfg.Sites) == 0 {
		log.Warning("No site defined in config")
	}
	for siteIdx, siteCfg := range cfg.Sites {
		log.Infof("  #%d [%s] %s: %s", siteIdx, siteCfg.Id, *siteCfg.Mode, siteCfg.dumpLocations())
		table := newDumpTable("    ")
		siteCfg.Settings.(siteSettingsDumper).dump(table)
		table.add("quota group", siteCfg.QuotaGroup)
		table.addLines("static", siteCfg.dumpStaticFiles()...)
		table.add("network", includeStringIf(siteCfg.SelfUrl != "", "self url "+siteCfg.SelfUrl), prefixOptionalString("ip pool", siteCfg.IpPoolStrategy))
		table.log()
	}
}

func (cfg *Config) dumpGlobal() {
	table := newDumpTable("  ")
	table.add("debug", includeStringIf(cfg.Debug, "on"))
	table.add("listen", *cfg.Server.Listen)
	if cfg.Server.TLS != nil {
		table.add("tls", formatCount(len(cfg.Server.TLS.Certificates), "certificate"))
	}
	table.add("connections", fmt.Sprintf("max %d, read header timeout %s, idle timeout %s",
		*cfg.Server.MaxConnections, *cfg.Server.ReadHeaderTimeout, *cfg.Server.IdleTimeout))

	trustedIps := *cfg.Server.TrustedProxyIps
	if slices.Contains(trustedIps, "*") {
		trustedIps = []string{"any"}
	}
	table.addLines("trusted", "ips "+joinOr(trustedIps, "none"), "headers "+joinOr(*cfg.Server.TrustedProxyHeaders, "none"))

	proxyCfg := cfg.Request.Proxy
	if len(proxyCfg.Servers) > 0 {
		hasNonDefaultWeight := slices.ContainsFunc(proxyCfg.Servers, func(server *RequestProxyServerConfig) bool {
			return *server.Weight != defaultRequestProxyWeight
		})

		var lines []string
		for idx, server := range proxyCfg.Servers {
			addr := server.Url
			if proxyUrl, err := url.Parse(server.Url); err == nil {
				addr = proxyUrl.Redacted()
			}
			line := fmt.Sprintf("#%d [%s] %s", idx, server.Id, addr)
			if server.ProxyProtocol != ProxyProtocolNone {
				line += fmt.Sprintf(" (proxy protocol %s)", server.ProxyProtocol)
			}
			if hasNonDefaultWeight {
				line += fmt.Sprintf(" (weight %d)", *server.Weight)
			}
			lines = append(lines, line)
		}
		lines = append(lines, fmt.Sprintf("strategy %s, direct fallback %s, connect timeout %s", *proxyCfg.Strategy, formatOnOff(proxyCfg.DirectFallback), *proxyCfg.ConnectTimeout))
		table.addLines("proxy", lines...)
	}
	if ipPoolCfg := cfg.Request.IpPool; ipPoolCfg.Enabled {
		table.addLines("ip pool", strings.Join(ipPoolCfg.Subnets, ", "), fmt.Sprintf("default strategy %s", *ipPoolCfg.DefaultStrategy))
	}

	table.addLines("request", cfg.Request.Header.dumpLines()...)
	table.addLines("response", append([]string{fmt.Sprintf("max redirects %d", *cfg.Response.MaxRedirect)}, cfg.Response.Header.dumpLines()...)...)

	limitCfg := cfg.ResourceLimit
	table.addLines("limits", append(limitCfg.Global.dumpLines(),
		fmt.Sprintf("idle client cache %d", *limitCfg.IdleClientCacheSize),
		fmt.Sprintf("max concurrent 429 responses %d", *limitCfg.MaxConcurrent429Responses))...)
	for _, group := range limitCfg.Groups {
		table.addLines("limit group "+group.Id, group.dumpLines()...)
	}

	if cfg.Diagnostics.Enabled {
		table.add("diagnostics", *cfg.Diagnostics.Listen)
	}
	table.log()
}

func (h *HeaderModificationConfig) dumpLines() []string {
	var modifiedHeaders []string
	for name := range *h.Modify {
		modifiedHeaders = append(modifiedHeaders, name)
	}
	slices.Sort(modifiedHeaders) // header values are not shown, they might contain credentials
	return []string{
		includeStringIf(len(*h.Delete) > 0, "delete "+formatCount(len(*h.Delete), "header")),
		prefixNonEmptyString("modify headers", strings.Join(modifiedHeaders, ", ")),
	}
}

// dumpLocations returns where the site is mounted, i.e. host + path prefix
func (s *SiteConfig) dumpLocations() string {
	if len(s.Host) == 0 {
		return "no host"
	}
	var locations []string
	for _, host := range s.Host {
		locations = append(locations, host+s.PathPrefix)
	}
	return strings.Join(locations, ", ")
}

func (s *SiteConfig) dumpStaticFiles() []string {
	var lines []string
	for _, staticFile := range s.StaticFiles {
		source := "file " + staticFile.File
		if staticFile.Content != nil {
			source = formatCount(len(*staticFile.Content), "byte") + " content"
		}
		lines = append(lines, fmt.Sprintf("%s -> %s", staticFile.Path, source)+prefixNonEmptyString(", type", staticFile.ContentType))
	}
	return lines
}

type siteSettingsDumper interface {
	dump(table *dumpTable)
}

var (
	_ siteSettingsDumper = &ContainerRegistrySingleSiteSettings{}
	_ siteSettingsDumper = &ContainerRegistryAnySiteSettings{}
	_ siteSettingsDumper = &GithubSiteSettings{}
	_ siteSettingsDumper = &HuggingFaceSiteSettings{}
	_ siteSettingsDumper = &HttpSiteSettings{}
	_ siteSettingsDumper = &AnyUrlSiteSettings{}
	_ siteSettingsDumper = &NpmSiteSettings{}
	_ siteSettingsDumper = &PypiSiteSettings{}
	_ siteSettingsDumper = &SpeedTestSiteSettings{}
)

func (s *ContainerRegistrySingleSiteSettings) dump(table *dumpTable) {
	table.addLines("upstream", prefixOptionalString("v1", s.UpstreamV1Url), prefixOptionalString("v2", s.UpstreamV2Url), prefixOptionalString("auth realm", s.UpstreamAuthRealmUrl))
	table.add("access", chooseString(*s.AllowPush, "pull and push", "pull only"), "list "+formatOnOff(*s.AllowList))
	s.Auth.dump(table)
	dumpRepos(table, s.ReposWhitelist, s.ReposBlacklist)
}

func (s *ContainerRegistryAnySiteSettings) dump(table *dumpTable) {
	table.add("access", "pull only", "list "+formatOnOff(*s.AllowList))
	s.Auth.dump(table)
	dumpRepos(table, s.ReposWhitelist, s.ReposBlacklist)
}

func (s *GithubSiteSettings) dump(table *dumpTable) {
	table.add("limits", includeStringIf(s.SizeLimit > 0, "size <= "+utils.FormatByteSize(s.SizeLimit)))
	table.add("rewrite", includeStringIf(s.RawTextUrlRewrite, "raw text urls"))
	table.add("access", chooseString(s.AllowPush, "pull and push", "pull only"))
	dumpRepos(table, s.ReposWhitelist, s.ReposBlacklist)
}

func (s *HuggingFaceSiteSettings) dump(table *dumpTable) {
	table.add("limits", includeStringIf(s.SizeLimit > 0, "size <= "+utils.FormatByteSize(s.SizeLimit)))
}

func (s *HttpSiteSettings) dump(table *dumpTable) {
	var lines []string
	for _, mapping := range s.Mappings {
		lines = append(lines, fmt.Sprintf("%s -> %s", mapping.Path, mapping.Destination))
	}
	if s.Destination != "" {
		lines = append(lines, fmt.Sprintf("* -> %s", s.Destination))
	}
	table.addLines("upstream", lines...)
	table.add("redirect", string(*s.RedirectAction))

	var rewriteLines []string
	for _, rewrite := range s.BodyRewrites {
		summary := formatCount(len(rewrite.Replacements), "replacement")
		if len(rewrite.JsonPaths) > 0 {
			summary += " on " + formatCount(len(rewrite.JsonPaths), "JSON path")
		}
		rewriteLines = append(rewriteLines, fmt.Sprintf("%s [%s]: %s",
			joinOr(rewrite.Paths, "*"), strings.Join(rewrite.ContentTypes, ", "), summary,
		))
	}
	table.addLines("rewrite", rewriteLines...)
}

func (s *AnyUrlSiteSettings) dump(table *dumpTable) {
	table.addLines("access",
		"methods "+strings.Join(s.AllowedMethods, ", "),
		prefixNonEmptyString("domain whitelist", strings.Join(s.DomainWhitelist, ", ")),
		prefixNonEmptyString("domain blacklist", strings.Join(s.DomainBlacklist, ", ")),
		includeStringIf(s.AllowPrivateNetwork, "private network allowed"),
	)
	table.add("limits", includeStringIf(s.SizeLimit > 0, "size <= "+utils.FormatByteSize(s.SizeLimit)))
	s.Auth.dump(table)
}

func (s *NpmSiteSettings) dump(table *dumpTable) {
	table.add("upstream", *s.UpstreamRegistryUrl)
	table.add("access", "search "+formatOnOff(s.AllowSearch), "audit "+formatOnOff(*s.AllowAudit))
}

func (s *PypiSiteSettings) dump(table *dumpTable) {
	table.addLines("upstream", prefixOptionalString("simple", s.UpstreamSimpleUrl), prefixOptionalString("files", s.UpstreamFilesUrl))
	table.addLines("packages", prefixNonEmptyString("whitelist", strings.Join(s.PackagesWhitelist, ", ")), prefixNonEmptyString("blacklist", strings.Join(s.PackagesBlacklist, ", ")))
}

func (s *SpeedTestSiteSettings) dump(table *dumpTable) {
	table.add("limits", "upload <= "+utils.FormatByteSize(*s.MaxUploadBytes), "download <= "+utils.FormatByteSize(*s.MaxDownloadBytes))
}

func (a *ClientAuthConfig) dump(table *dumpTable) {
	if !a.Enabled {
		table.add("auth", "off")
		return
	}
	usersFile := ""
	if a.UsersFile != "" {
		usersFile = "users file " + a.UsersFile
		if a.UsersFileReloadInterval != nil {
			usersFile += fmt.Sprintf(" (reload every %s)", *a.UsersFileReloadInterval)
		}
	}
	table.add("auth", "on", includeStringIf(len(a.Users) > 0, formatCount(len(a.Users), "user")), usersFile)
}

func dumpRepos(table *dumpTable, whitelist, blacklist []string) {
	table.addLines("repos", prefixNonEmptyString("whitelist", strings.Join(whitelist, ", ")), prefixNonEmptyString("blacklist", strings.Join(blacklist, ", ")))
}

func formatOnOff(b bool) string {
	return chooseString(b, "on", "off")
}

func chooseString(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func includeStringIf(cond bool, s string) string {
	return chooseString(cond, s, "")
}

// prefixNonEmptyString returns "<prefix> <s>", or "" if s is empty
func prefixNonEmptyString(prefix string, s string) string {
	return includeStringIf(s != "", prefix+" "+s)
}

// prefixOptionalString returns "<prefix> <*p>", or "" if p is nil
func prefixOptionalString[T ~string](prefix string, p *T) string {
	if p == nil {
		return ""
	}
	return prefixNonEmptyString(prefix, string(*p))
}

func joinOr(list []string, fallback string) string {
	return chooseString(len(list) > 0, strings.Join(list, ", "), fallback)
}

func formatCount(n int, noun string) string {
	return fmt.Sprintf("%d %s", n, chooseString(n == 1, noun, noun+"s"))
}

func (q *ResourceQuotaConfig) dumpLines() []string {
	return []string{
		formatLimit(q.RequestTimeout, "timeout %s"),
		prefixNonEmptyString("per client", q.PerClient.formatLimits()),
		prefixNonEmptyString("all clients", q.AllClients.formatLimits()),
	}
}

func (q *ClientQuotaConfig) formatLimits() string {
	return joinLimits(
		formatLimit(q.RequestConcurrency, "%d concurrent"),
		formatLimit(q.RequestPerSecond, "%v req/s"),
		formatLimit(q.RequestPerMinute, "%v req/min"),
		formatLimit(q.RequestPerHour, "%v req/h"),
		formatLimit(q.TrafficAvgMibps, "%v MiB/s avg"),
		formatLimit(q.TrafficBurstMib, "%v MiB burst"),
		formatLimit(q.TrafficMaxMibps, "%v MiB/s max"),
	)
}

func formatLimit[T any](value *T, format string) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf(format, *value)
}

func joinLimits(limits ...string) string {
	return strings.Join(slices.DeleteFunc(limits, func(s string) bool { return s == "" }), ", ")
}
