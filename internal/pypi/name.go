package pypi

import (
	"regexp"
	"strings"
)

// https://packaging.python.org/en/latest/specifications/name-normalization/
var (
	validName      = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)
	nameSeparators = regexp.MustCompile(`[-_.]+`)
)

func IsValidName(name string) bool {
	return validName.MatchString(name)
}

func NormalizeName(name string) string {
	return nameSeparators.ReplaceAllString(strings.ToLower(name), "-")
}
