package anyproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomainList(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rules    []string
		host     string
		expected bool
	}{
		{"exact", []string{"example.com"}, "example.com", true},
		{"other domain", []string{"example.com"}, "example.org", false},
		{"subdomain is not exact", []string{"example.com"}, "a.example.com", false},
		{"any rule", []string{"a.test", "example.com"}, "example.com", true},
		{"case insensitive", []string{"Example.COM"}, "EXAMPLE.com", true},
		{"trailing dot in host", []string{"example.com"}, "example.com.", true},
		{"trailing dot in rule", []string{"example.com."}, "example.com", true},
		{"wildcard matches subdomain", []string{"*.example.com"}, "a.example.com", true},
		{"wildcard matches nested subdomain", []string{"*.example.com"}, "a.b.example.com", true},
		{"wildcard excludes the domain itself", []string{"*.example.com"}, "example.com", false},
		{"wildcard requires a label boundary", []string{"*.example.com"}, "aexample.com", false},
		{"ip", []string{"127.0.0.1"}, "127.0.0.1", true},
		{"empty list", nil, "example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, newDomainList(tc.rules).matches(tc.host))
		})
	}
}
