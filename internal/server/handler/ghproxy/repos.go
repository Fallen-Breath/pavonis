package ghproxy

import "strings"

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
