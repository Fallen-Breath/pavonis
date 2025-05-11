package crproxy

import (
	"github.com/Fallen-Breath/pavonis/internal/utils"
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
