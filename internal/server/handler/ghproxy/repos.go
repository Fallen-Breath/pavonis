package ghproxy

import "strings"

type reposListEntry struct {
	Author string
	Repos  string
}
type reposList []reposListEntry

// GitHub names are case-insensitive, and "repo.git" is the same repository as "repo"
func normalizeReposName(name string) string {
	return strings.TrimSuffix(strings.ToLower(name), ".git")
}

func (le *reposListEntry) Check(author, repos string) bool {
	return (le.Author == "*" || le.Author == author) && (le.Repos == "*" || le.Repos == repos)
}

func (l *reposList) Check(author, repos string) bool {
	author, repos = normalizeReposName(author), normalizeReposName(repos)
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
			Author: normalizeReposName(parts[0]),
			Repos:  normalizeReposName(parts[1]),
		})
	}
	return &reposList
}
