package anyproxy

import "strings"

type domainList []string

func normalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(domain, "."))
}

func newDomainList(rules []string) domainList {
	list := make(domainList, 0, len(rules))
	for _, rule := range rules {
		list = append(list, normalizeDomain(rule))
	}
	return list
}

func (l domainList) matches(host string) bool {
	host = normalizeDomain(host)
	for _, rule := range l {
		if suffix, ok := strings.CutPrefix(rule, "*"); ok {
			if strings.HasSuffix(host, suffix) {
				return true
			}
		} else if host == rule {
			return true
		}
	}
	return false
}
