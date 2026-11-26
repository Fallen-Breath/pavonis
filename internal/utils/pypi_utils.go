package utils

import (
	"regexp"
	"strings"
)

// https://packaging.python.org/en/latest/specifications/name-normalization/
var (
	validPypiPackageName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)
	pypiNameSeparators   = regexp.MustCompile(`[-_.]+`)
)

func IsValidPypiPackageName(name string) bool {
	return validPypiPackageName.MatchString(name)
}

func NormalizePypiPackageName(name string) string {
	return pypiNameSeparators.ReplaceAllString(strings.ToLower(name), "-")
}
