package pypiproxy

import (
	"net/url"
	"slices"
	"strings"

	"github.com/Fallen-Breath/pavonis/internal/pypi"
)

func buildPackageSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[pypi.NormalizeName(name)] = true
	}
	return set
}

func (h *proxyHandler) checkPackageAccess(path requestPath) bool {
	if (len(h.whitelist) == 0 && len(h.blacklist) == 0) || path.kind == routeProjectList {
		return true
	}
	name := path.project
	if path.kind == routeFiles {
		name = extractPackageNameFromFilePath(path.escapedRest)
	}
	if !pypi.IsValidName(name) {
		return false
	}
	name = pypi.NormalizeName(name)
	return (len(h.whitelist) == 0 || h.whitelist[name]) && !h.blacklist[name]
}

func extractPackageNameFromFilePath(escapedPath string) string {
	// An encoded separator or dot segment can change the upstream path's meaning.
	var filename string
	for component := range strings.SplitSeq(strings.TrimPrefix(escapedPath, "/"), "/") {
		decoded, err := url.PathUnescape(component)
		if err != nil || decoded == "" || decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "/\\") {
			return ""
		}
		filename = decoded
	}
	return extractPackageNameFromFilename(filename)
}

// A wheel's first field is the distribution name. A standard sdist has exactly
// one hyphen separating name and version. Historical archives may be ambiguous.
// https://packaging.python.org/en/latest/specifications/binary-distribution-format/#file-name-convention
// https://packaging.python.org/en/latest/specifications/source-distribution-format/#source-distribution-file-name
func extractPackageNameFromFilename(filename string) string {
	filename = strings.TrimSuffix(filename, ".metadata")
	var name string
	switch {
	case strings.HasSuffix(filename, ".whl"):
		fields := strings.Split(strings.TrimSuffix(filename, ".whl"), "-")
		if (len(fields) != 5 && len(fields) != 6) || slices.Contains(fields, "") {
			return ""
		}
		name = fields[0]
	case strings.HasSuffix(filename, ".tar.gz"):
		var version string
		name, version, _ = strings.Cut(strings.TrimSuffix(filename, ".tar.gz"), "-")
		if version == "" || strings.Contains(version, "-") {
			return ""
		}
	}
	if !pypi.IsValidName(name) {
		return ""
	}
	return name
}
