package ghproxy

import (
	"net/url"
	"regexp"
	"strings"
)

type targetMatch struct {
	Owner string // empty if the url does not contain the owner, e.g. gist clone urls without the username
	Repo  string // empty for gists
	Push  bool

	// non-empty if the path should be replaced before forwarding
	RewrittenPath string
}

type pathRule struct {
	pattern *regexp.Regexp // optional named groups: owner, repo, rest
	service string         // if non-empty, the "service" query parameter must equal to it
	push    bool           // only allowed if push is enabled
	rewrite func(m *targetMatch, rest string) string
}

func newPathRule(pattern string) pathRule {
	return pathRule{pattern: regexp.MustCompile(pattern)}
}

func (r pathRule) withService(service string) pathRule {
	r.service = service
	return r
}

func (r pathRule) forPush() pathRule {
	r.push = true
	return r
}

func (r pathRule) match(path string, query url.Values) (*targetMatch, bool) {
	if r.service != "" && query.Get("service") != r.service {
		return nil, false
	}
	matches := r.pattern.FindStringSubmatch(path)
	if matches == nil {
		return nil, false
	}
	group := func(name string) string {
		if idx := r.pattern.SubexpIndex(name); idx >= 0 {
			return matches[idx]
		}
		return ""
	}
	tm := &targetMatch{Owner: group("owner"), Repo: group("repo"), Push: r.push}
	if r.rewrite != nil {
		tm.RewrittenPath = r.rewrite(tm, group("rest"))
	}
	return tm, true
}

var githubMainRules = []pathRule{
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/releases/download/.+$`),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/releases/latest/download/.+$`),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/archive/.+$`),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/raw/.+$`),
	{
		// blob pages are html, serve the raw file instead
		pattern: regexp.MustCompile(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/blob/(?P<rest>.+)$`),
		rewrite: func(m *targetMatch, rest string) string {
			return "/" + m.Owner + "/" + m.Repo + "/raw/" + rest
		},
	},
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/info/refs$`).withService("git-upload-pack"),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/git-upload-pack$`),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/info/refs$`).withService("git-receive-pack").forPush(),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/git-receive-pack$`).forPush(),
	// TODO: the batch response contains object transfer urls on other hosts, which clients access directly.
	//       Rewrite them to go through pavonis, and check the operation for allow_push
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/info/lfs/objects/batch$`),
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/info/lfs/locks(?:/.*)?$`).forPush(),
}

var githubRawRules = []pathRule{
	newPathRule(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/[^/]+/.+$`), // owner, repo, ref, file path
}

var gistMainRules = []pathRule{
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+/raw(?:/.*)?$`),
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+/archive/.+$`),
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+/download$`),
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+(?:\.git)?/info/refs$`).withService("git-upload-pack"),
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+(?:\.git)?/git-upload-pack$`),
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+(?:\.git)?/info/refs$`).withService("git-receive-pack").forPush(),
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+(?:\.git)?/git-receive-pack$`).forPush(),
	// the default clone url, which has no username
	newPathRule(`^/[0-9a-fA-F]+(?:\.git)?/info/refs$`).withService("git-upload-pack"),
	newPathRule(`^/[0-9a-fA-F]+(?:\.git)?/git-upload-pack$`),
	newPathRule(`^/[0-9a-fA-F]+(?:\.git)?/info/refs$`).withService("git-receive-pack").forPush(),
	newPathRule(`^/[0-9a-fA-F]+(?:\.git)?/git-receive-pack$`).forPush(),
}

var gistRawRules = []pathRule{
	newPathRule(`^/(?P<owner>[^/]+)/[0-9a-fA-F]+/raw(?:/.*)?$`),
}

var allowedHosts = map[string][]pathRule{
	"github.com":                 githubMainRules,
	"raw.githubusercontent.com":  githubRawRules,
	"gist.github.com":            gistMainRules,
	"gist.githubusercontent.com": gistRawRules,
}

// notes: remember to check req.Host after following-redirect
var rawTextUrlRewriteHosts = map[string]bool{
	"raw.githubusercontent.com":  true,
	"gist.githubusercontent.com": true,
}

func hasDotSegment(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}

// matchTarget checks if the url is something the github mode serves, regardless of the repos lists and allow_push.
// hostOk is false if the host itself is unsupported
func matchTarget(host, path string, query url.Values) (tm *targetMatch, hostOk bool) {
	rules, ok := allowedHosts[host]
	if !ok {
		return nil, false
	}
	if hasDotSegment(path) {
		return nil, true
	}
	for _, rule := range rules {
		if tm, ok := rule.match(path, query); ok {
			return tm, true
		}
	}
	return nil, true
}
