package ioutils

import (
	"encoding/json/jsontext"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// RFC 9535 name shorthand and child selectors, restricted to names and wildcards.
var jsonPathName = regexp.MustCompile(`^(?:[a-zA-Z_\x{80}-\x{10ffff}][a-zA-Z_0-9\x{80}-\x{10ffff}]*|\*)`)
var jsonPathBracket = regexp.MustCompile(`^\[[ \t\r\n]*(\*|"(?:[^"\\\x00-\x1f]|\\["\\/bfnrtu])*"|'(?:[^'\\\x00-\x1f]|\\['\\/bfnrtu])*')[ \t\r\n]*\]`)

// Convert a single-quoted selector to JSON string contents, preserving escaped backslashes.
var jsonPathSingleQuoteEscapes = strings.NewReplacer(`\\`, `\\`, `\'`, `'`, `"`, `\"`)

// ParseJsonPath parses child selectors from the root, e.g. $.files[*].url or files[*].url.
func ParseJsonPath(path string) ([]JsonPathSegment, error) {
	if path == "" || !utf8.ValidString(path) {
		return nil, fmt.Errorf("expected a JSON field path")
	}
	rest, rooted := strings.CutPrefix(path, "$")
	var segments []JsonPathSegment
	for rest != "" {
		var token string
		if rest[0] == '[' {
			match := jsonPathBracket.FindStringSubmatch(rest)
			if match == nil {
				return nil, fmt.Errorf("expected a quoted key or wildcard at %q", rest)
			}
			token, rest = match[1], rest[len(match[0]):]
		} else {
			if rooted || len(segments) > 0 {
				var ok bool
				if rest, ok = strings.CutPrefix(rest, "."); !ok {
					return nil, fmt.Errorf("expected '.' or '[' in %q", path)
				}
			}
			token = jsonPathName.FindString(rest)
			if token == "" {
				return nil, fmt.Errorf("expected a field name or wildcard at %q", rest)
			}
			rest = rest[len(token):]
		}

		if token == "*" {
			segments = append(segments, JsonPathSegment{kind: jsonPathSegmentWildcard})
			continue
		}
		if token[0] == '\'' {
			token = `"` + jsonPathSingleQuoteEscapes.Replace(token[1:len(token)-1]) + `"`
		}
		if token[0] == '"' {
			key, err := jsontext.AppendUnquote(nil, []byte(token))
			if err != nil {
				return nil, fmt.Errorf("invalid quoted field name: %w", err)
			}
			token = string(key)
		}
		segments = append(segments, NewJsonPathSegmentKey(token))
	}
	return segments, nil
}
