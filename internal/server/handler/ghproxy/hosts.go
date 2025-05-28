package ghproxy

import (
	"net/url"
	"regexp"
)

type hostDefinition struct {
	Parse func(url *url.URL) (author, repos string, ok bool)
}

var githubMainPathPattern = regexp.MustCompile("^/([^/]+)/([^/]+)/((releases|archive|blob|raw|info)/|git-upload-pack$)")
var githubRawPathPattern = regexp.MustCompile("^/([^/]+)/([^/]+)/[^/]+/") // author, repos, branch
var gistPathPattern = regexp.MustCompile("^/([^/]+)/[^/]+/")              // author, hash

func hostDefinition1(pattern *regexp.Regexp) hostDefinition {
	return hostDefinition{
		Parse: func(url *url.URL) (author, repos string, ok bool) {
			matches := pattern.FindStringSubmatch(url.Path)
			if matches == nil {
				return "", "", false
			}
			return matches[1], "", true
		},
	}
}

func hostDefinition2(pattern *regexp.Regexp) hostDefinition {
	return hostDefinition{
		Parse: func(url *url.URL) (author, repos string, ok bool) {
			matches := pattern.FindStringSubmatch(url.Path)
			if matches == nil {
				return "", "", false
			}
			return matches[1], matches[2], true
		},
	}
}

var allowedHosts = map[string]hostDefinition{
	"github.com":                 hostDefinition2(githubMainPathPattern),
	"raw.githubusercontent.com":  hostDefinition2(githubRawPathPattern),
	"gist.github.com":            hostDefinition1(gistPathPattern),
	"gist.githubusercontent.com": hostDefinition1(gistPathPattern),
}

// notes: remember to check req.Host after following-redirect
var rawTextUrlRewriteHosts = map[string]bool{
	"raw.githubusercontent.com":  true,
	"gist.githubusercontent.com": true,
}
