package crproxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	log "github.com/sirupsen/logrus"
)

type reposListEntry []string
type reposList []reposListEntry

func (le *reposListEntry) Check(repos []string) bool {
	if len(repos) < len(*le) {
		// An entry may omit trailing repository segments, which act as '*'.
		// An actual repository cannot be shorter than an explicit entry.
		return false
	}

	for i := range *le {
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

// A recognized route returns ok=true. A nil name then denotes a registry-wide
// endpoint, while ok=false means the repository cannot be determined safely.
func extractReposNameFromV2Path(path string) (name []string, ok bool) {
	// https://distribution.github.io/distribution/spec/api/#detail
	// Possible paths:
	//
	//     "/v2/_catalog"
	//     "/v2/<name>/blobs/<digest>"
	//     "/v2/<name>/blobs/uploads/"
	//     "/v2/<name>/blobs/uploads/<uuid>"
	//     "/v2/<name>/manifests/<reference>"
	//     "/v2/<name>/tags/list"
	if path == "/v2/" || path == "/v2/_catalog" {
		return nil, true
	}
	if !strings.HasPrefix(path, "/v2/") {
		return nil, false
	}
	// Registries may redirect trailing-slash variants to the same resource.
	// Ignore those slashes for access checks without changing the forwarded path.
	parts := strings.Split(strings.TrimRight(path[len("/v2/"):], "/"), "/")
	n := len(parts)
	if n < 3 {
		return nil, false
	}
	// Do not guess which repository an upstream would choose after cleaning a
	// path with empty or dot segments, including segments in operation parameters.
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, false
		}
	}

	// Match the complete operation suffix: the repository itself may contain any
	// of these path segments, including "blobs/uploads" and "tags/list".
	switch {
	// /v2/<name>/blobs/uploads/<uuid>
	case n >= 4 && parts[n-3] == "blobs" && parts[n-2] == "uploads":
		name = parts[:n-3]
	// /v2/<name>/blobs/uploads/
	case parts[n-2] == "blobs" && parts[n-1] == "uploads":
		name = parts[:n-2]
	// /v2/<name>/tags/list
	case parts[n-2] == "tags" && parts[n-1] == "list":
		name = parts[:n-2]
	// /v2/<name>/manifests/<reference> or /v2/<name>/blobs/<digest>
	case parts[n-2] == "manifests" || parts[n-2] == "blobs":
		name = parts[:n-2]
	// No supported repository operation suffix.
	default:
		return nil, false
	}
	return name, true
}

func checkAndApplyWhitelists(whitelist *reposList, blacklist *reposList, w http.ResponseWriter, reposName []string) bool {
	if len(*whitelist) > 0 && !whitelist.Check(reposName) {
		http.Error(w, fmt.Sprintf("Repository '%s' is not whitelisted", strings.Join(reposName, "/")), http.StatusForbidden)
		return false
	}
	if len(*blacklist) > 0 && blacklist.Check(reposName) {
		http.Error(w, fmt.Sprintf("Repository '%s' is blacklisted", strings.Join(reposName, "/")), http.StatusForbidden)
		return false
	}
	return true
}

func (h *singleProxyHandler) checkReposWhitelist(ctx *context.RequestContext, w http.ResponseWriter, reqPath string, routePrefix routePrefix) bool {
	if len(*h.whitelist) == 0 && len(*h.blacklist) == 0 {
		return true
	}

	var reposName *[]string
	if routePrefix == routePrefixV1 {
		reposName = extractReposNameFromV1Path(reqPath)
	} else if routePrefix == routePrefixV2 {
		name, ok := extractReposNameFromV2Path(reqPath)
		if !ok {
			http.Error(w, "Cannot determine repository for access control", http.StatusForbidden)
			return false
		}
		if len(name) == 0 {
			return true
		}
		reposName = &name
	} else {
		return true
	}

	log.Debugf("%sExtracted reposName from reqPath %+q: %+v", ctx.LogPrefix, reqPath, reposName)
	if reposName != nil && !checkAndApplyWhitelists(h.whitelist, h.blacklist, w, *reposName) {
		return false
	}

	return true
}
