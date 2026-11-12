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
		table.add("network", ifThen(siteCfg.SelfUrl != "", "self url "+siteCfg.SelfUrl), prefixedPtr("ip pool", siteCfg.IpPoolStrategy))
		table.log()
	}
}

func (cfg *Config) dumpGlobal() {
	table := newDumpTable("  ")
	table.add("debug", ifThen(cfg.Debug, "on"))
	table.add("listen", *cfg.Server.Listen)

	trustedIps := *cfg.Server.TrustedProxyIps
	if slices.Contains(trustedIps, "*") {
		trustedIps = []string{"any"}
	}
	table.addLines("trusted", "ips "+joinOr(trustedIps, "none"), "headers "+joinOr(*cfg.Server.TrustedProxyHeaders, "none"))

	proxyCfg := cfg.Request.Proxy
	if len(proxyCfg.Servers) > 0 {
		var lines []string
		for _, server := range proxyCfg.Servers {
			line := server.Url
			if proxyUrl, err := url.Parse(server.Url); err == nil {
				line = proxyUrl.Redacted()
			}
			if server.ProxyProtocol != ProxyProtocolNone {
				line += fmt.Sprintf(" (proxy protocol %s)", server.ProxyProtocol)
			}
			lines = append(lines, line)
		}
		lines = append(lines, fmt.Sprintf("direct fallback %s, connect timeout %s", onOff(proxyCfg.DirectFallback), *proxyCfg.ConnectTimeout))
		table.addLines("proxy", lines...)
	}
	if ipPoolCfg := cfg.Request.IpPool; ipPoolCfg.Enabled {
		table.addLines("ip pool", strings.Join(ipPoolCfg.Subnets, ", "), fmt.Sprintf("default strategy %s", *ipPoolCfg.DefaultStrategy))
	}

	table.addLines("request", cfg.Request.Header.dumpLines()...)
	table.addLines("response", append([]string{fmt.Sprintf("max redirects %d", *cfg.Response.MaxRedirect)}, cfg.Response.Header.dumpLines()...)...)

	limitCfg := cfg.ResourceLimit
	table.addLines("limits",
		fmt.Sprintf("timeout %s", *limitCfg.RequestTimeout),
		prefixed("per client", joinLimits(
			formatLimit(limitCfg.RequestPerSecond, "%v req/s"),
			formatLimit(limitCfg.RequestPerMinute, "%v req/min"),
			formatLimit(limitCfg.RequestPerHour, "%v req/h"),
			formatLimit(limitCfg.TrafficAvgMibps, "%v MiB/s avg"),
			formatLimit(limitCfg.TrafficBurstMib, "%v MiB burst"),
			formatLimit(limitCfg.TrafficMaxMibps, "%v MiB/s max"),
		)),
		prefixed("global", joinLimits(
			formatLimit(limitCfg.GlobalTrafficAvgMibps, "%v MiB/s avg"),
			formatLimit(limitCfg.GlobalTrafficBurstMib, "%v MiB burst"),
			formatLimit(limitCfg.GlobalTrafficMaxMibps, "%v MiB/s max"),
		)),
	)

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
		ifThen(len(*h.Delete) > 0, "delete "+countOf(len(*h.Delete), "header")),
		prefixed("modify headers", strings.Join(modifiedHeaders, ", ")),
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

type siteSettingsDumper interface {
	dump(table *dumpTable)
}

func (s *ContainerRegistrySingleSiteSettings) dump(table *dumpTable) {
	table.addLines("upstream", prefixedPtr("v1", s.UpstreamV1Url), prefixedPtr("v2", s.UpstreamV2Url), prefixedPtr("auth realm", s.UpstreamAuthRealmUrl))
	table.add("access", ifElse(*s.AllowPush, "pull and push", "pull only"), "list "+onOff(*s.AllowList))
	s.Auth.dump(table)
	dumpRepos(table, s.ReposWhitelist, s.ReposBlacklist)
}

func (s *ContainerRegistryAnySiteSettings) dump(table *dumpTable) {
	table.add("access", "pull only")
	s.Auth.dump(table)
	dumpRepos(table, s.ReposWhitelist, s.ReposBlacklist)
}

func (s *GithubSiteSettings) dump(table *dumpTable) {
	table.add("limits", ifThen(s.SizeLimit > 0, "size <= "+utils.PrettyByteSize(s.SizeLimit)))
	table.add("rewrite", ifThen(s.RawTextUrlRewrite, "raw text urls"))
	table.add("access", ifElse(s.AllowPush, "pull and push", "pull only"))
	dumpRepos(table, s.ReposWhitelist, s.ReposBlacklist)
}

func (s *HuggingFaceSiteSettings) dump(table *dumpTable) {
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
}

func (s *AnyUrlSiteSettings) dump(table *dumpTable) {
	table.addLines("access",
		"methods "+strings.Join(s.AllowedMethods, ", "),
		prefixed("domain whitelist", strings.Join(s.DomainWhitelist, ", ")),
		prefixed("domain blacklist", strings.Join(s.DomainBlacklist, ", ")),
		ifThen(s.AllowPrivateNetwork, "private network allowed"),
	)
	table.add("limits", ifThen(s.SizeLimit > 0, "size <= "+utils.PrettyByteSize(s.SizeLimit)))
	s.Auth.dump(table)
}

func (s *PypiSiteSettings) dump(table *dumpTable) {
	table.addLines("upstream", prefixedPtr("simple", s.UpstreamSimpleUrl), prefixedPtr("files", s.UpstreamFilesUrl))
}

func (s *SpeedTestSiteSettings) dump(table *dumpTable) {
	table.add("limits", "upload <= "+utils.PrettyByteSize(*s.MaxUploadBytes), "download <= "+utils.PrettyByteSize(*s.MaxDownloadBytes))
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
	table.add("auth", "on", ifThen(len(a.Users) > 0, countOf(len(a.Users), "user")), usersFile)
}

func dumpRepos(table *dumpTable, whitelist, blacklist []string) {
	table.addLines("repos", prefixed("whitelist", strings.Join(whitelist, ", ")), prefixed("blacklist", strings.Join(blacklist, ", ")))
}

func onOff(b bool) string {
	return ifElse(b, "on", "off")
}

func ifElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func ifThen(cond bool, s string) string {
	return ifElse(cond, s, "")
}

// prefixed returns "<prefix> <s>", or "" if s is empty
func prefixed(prefix string, s string) string {
	return ifThen(s != "", prefix+" "+s)
}

// prefixedPtr returns "<prefix> <*p>", or "" if p is nil
func prefixedPtr[T ~string](prefix string, p *T) string {
	if p == nil {
		return ""
	}
	return prefixed(prefix, string(*p))
}

func joinOr(list []string, fallback string) string {
	return ifElse(len(list) > 0, strings.Join(list, ", "), fallback)
}

func countOf(n int, noun string) string {
	return fmt.Sprintf("%d %s", n, ifElse(n == 1, noun, noun+"s"))
}

func formatLimit(value *float64, format string) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf(format, *value)
}

func joinLimits(limits ...string) string {
	return strings.Join(slices.DeleteFunc(limits, func(s string) bool { return s == "" }), ", ")
}
