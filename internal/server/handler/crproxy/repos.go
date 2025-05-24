package crproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

type reposListEntry []string
type reposList []reposListEntry

func (le *reposListEntry) Check(repos []string) bool {
	if len(*le) < len(repos) {
		// actual repos is longer than the entry??
		return false
	}

	// If the entry is shorter than the actual repos, treat the missing parts as "*"

	n := utils.Min(len(repos), len(*le))
	for i := 0; i < n; i++ {
		if (*le)[i] != "*" && (*le)[i] != repos[i] {
			return false
		}
	}

	return true
}

func (l *reposList) Check(repos []string) bool {
	for _, ent := range *l {
		if ent.Check(repos) {
			return true
		}
	}
	return false
}

func newReposList(list []string) *reposList {
	reposList := make(reposList, 0, len(list))
	for _, ent := range list {
		parts := strings.Split(ent, "/")
		reposList = append(reposList, parts)
	}
	return &reposList
}

func extractReposNameFromV1Path(path string) *[]string {
	// https://docs.docker.com/reference/api/hub/deprecated/
	// Possible paths:
	//
	//     "/v1/repositories/{name}/images"
	//     "/v1/repositories/{name}/tags"
	//     "/v1/repositories/{name}/tags/{tag_name}"
	//     "/v1/repositories/{namespace}/{name}/images"
	//     "/v1/repositories/{namespace}/{name}/tags"
	//     "/v1/repositories/{namespace}/{name}/tags/{tag_name}"
	if !strings.HasPrefix(path, "/v1/repositories/") {
		return nil
	}
	path = path[len("/v1/repositories/"):]

	for _, suffix := range []string{"/images", "/tags"} {
		if strings.HasSuffix(path, suffix) {
			name := path[:len(path)-len(suffix)]
			return utils.ToPtr(strings.Split(name, "/"))
		}
	}

	idx := strings.LastIndex(path, "/tags/")
	if idx != -1 {
		pre := path[:idx]
		post := path[idx+len("/tags/"):]
		if pre != "" && post != "" && !strings.Contains(post, "/") {
			return utils.ToPtr(strings.Split(pre, "/"))
		}
	}

	return nil
}

func extractReposNameFromV2Path(path string) *[]string {
	// https://distribution.github.io/distribution/spec/api/#detail
	// Possible paths:
	//
	//     "/v2/_catalog"
	//     "/v2/<name>/blobs/<digest>"
	//     "/v2/<name>/blobs/uploads/"
	//     "/v2/<name>/blobs/uploads/<uuid>"
	//     "/v2/<name>/manifests/<reference>"
	//     "/v2/<name>/tags/list"
	if !strings.HasPrefix(path, "/v2/") {
		return nil
	}
	path = path[len("/v2/"):]

	for _, keyword := range []string{
		"/blobs/uploads/",
		"/blobs/",
		"/tags/list",
		"/manifests/",
	} {
		idx := strings.Index(path, keyword)
		if idx != -1 {
			name := path[:idx]
			return utils.ToPtr(strings.Split(name, "/"))
		}
	}

	return nil
}

func (h *proxyHandler) checkAndApplyWhitelists(w http.ResponseWriter, reposName []string) bool {
	if len(*h.whitelist) > 0 && !h.whitelist.Check(reposName) {
		http.Error(w, fmt.Sprintf("Repository '%s' is not whitelisted", strings.Join(reposName, "/")), http.StatusForbidden)
		return false
	}
	if len(*h.blacklist) > 0 && h.blacklist.Check(reposName) {
		http.Error(w, fmt.Sprintf("Repository '%s' is blacklisted", strings.Join(reposName, "/")), http.StatusForbidden)
		return false
	}
	return true
}

func (h *proxyHandler) checkReposWhitelist(ctx *context.RequestContext, w http.ResponseWriter, reqPath string, routePrefix routePrefix) bool {
	if len(*h.whitelist) == 0 && len(*h.blacklist) == 0 {
		return true
	}

	var reposName *[]string
	if routePrefix == routePrefixV1 {
		reposName = extractReposNameFromV1Path(reqPath)
	} else if routePrefix == routePrefixV2 {
		reposName = extractReposNameFromV2Path(reqPath)
	} else {
		return true
	}

	log.Debugf("%sExtracted reposName from reqPath %+q: %+v", ctx.LogPrefix, reqPath, reposName)
	if reposName != nil && !h.checkAndApplyWhitelists(w, *reposName) {
		return false
	}

	return true
}
