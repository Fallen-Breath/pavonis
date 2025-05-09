package proxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type reposListEntry struct {
	Author string
	Repos  string
}
type reposList []reposListEntry

func (le *reposListEntry) Check(author, repos string) bool {
	return (le.Author == "*" || le.Author == author) && (le.Repos == "*" || le.Repos == repos)
}

func (l *reposList) Check(author, repos string) bool {
	for _, ent := range *l {
		if ent.Check(author, repos) {
			return true
		}
	}
	return false
}

func newReposList(list []string) *reposList {
	reposList := make(reposList, 0, len(list))
	for _, ent := range list {
		parts := strings.SplitN(ent, "/", 2)
		if len(parts) < 2 {
			parts = append(parts, "")
		}
		reposList = append(reposList, reposListEntry{
			Author: parts[0],
			Repos:  parts[1],
		})
	}
	return &reposList
}

type GithubProxyHandler struct {
	name       string
	helper     *common.RequestHelper
	settings   *config.GithubDownloadProxySettings
	whitelist  *reposList
	blacklist  *reposList
	bypassList *reposList
}

var _ HttpHandler = &GithubProxyHandler{}

func NewGithubProxyHandler(name string, helper *common.RequestHelper, settings *config.GithubDownloadProxySettings) (*GithubProxyHandler, error) {
	return &GithubProxyHandler{
		name:       name,
		helper:     helper,
		settings:   settings,
		whitelist:  newReposList(settings.ReposWhitelist),
		blacklist:  newReposList(settings.ReposBlacklist),
		bypassList: newReposList(settings.ReposBypass),
	}, nil
}

func (h *GithubProxyHandler) Name() string {
	return h.name
}

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

func (h *GithubProxyHandler) parseTargetUrl(w http.ResponseWriter, r *http.Request) (*url.URL, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return nil, false
	}
	targetUrlStr := path[1:] // Remove leading "/"

	targetUrl, err := url.Parse(targetUrlStr)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return nil, false
	}
	if targetUrl.Scheme == "" {
		targetUrl.Scheme = "https"
	}
	if targetUrl.Scheme != "https" {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return nil, false
	}

	return targetUrl, true
}

func (h *GithubProxyHandler) ServeHttp(ctx *context.RequestContext, w http.ResponseWriter, r *http.Request) {
	targetUrl, ok := h.parseTargetUrl(w, r)
	if !ok {
		return
	}

	hd, ok := allowedHosts[targetUrl.Host]
	if !ok {
		http.Error(w, "Forbidden host", http.StatusForbidden)
		return
	}

	author, repos, ok := hd.Parse(targetUrl)
	if !ok {
		http.Error(w, "Forbidden url", http.StatusForbidden)
		return
	}
	if !h.checkAndApplyWhitelists(w, r, targetUrl, author, repos) {
		return
	}

	targetUrl.User = r.URL.User
	targetUrl.RawQuery = r.URL.RawQuery
	targetUrl.RawFragment = r.URL.RawFragment

	h.helper.RunReverseProxy(ctx, w, r, targetUrl, func(resp *http.Response) error {
		if h.settings.SizeLimit > 0 && resp.ContentLength > h.settings.SizeLimit {
			return common.NewHttpError(http.StatusBadRequest, "Response body too large")
		}
		return nil
	})
}

func (h *GithubProxyHandler) checkAndApplyWhitelists(w http.ResponseWriter, r *http.Request, targetUrl *url.URL, author string, repos string) bool {
	if len(*h.whitelist) > 0 && !h.whitelist.Check(author, repos) {
		http.Error(w, fmt.Sprintf("Repository %s/%s not in whitelist", author, repos), http.StatusForbidden)
		return false
	}
	if len(*h.blacklist) > 0 && h.blacklist.Check(author, repos) {
		http.Error(w, fmt.Sprintf("Repository %s/%s is in blacklist", author, repos), http.StatusForbidden)
		return false
	}
	if len(*h.bypassList) > 0 && h.bypassList.Check(author, repos) {
		http.Redirect(w, r, targetUrl.String(), http.StatusTemporaryRedirect)
		return false
	}
	return true
}
