package pypiproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPackageNameFromFilename(t *testing.T) {
	for _, tc := range []struct{ filename, want string }{
		{"linphone-3.7.0_711_gc8bd7e1-cp27-none-win32.whl", "linphone"},
		{"zope.interface-6.0-1-py3-none-any.whl.metadata", "zope.interface"},
		{"flit_core-3.9.0.tar.gz", "flit_core"},
		{"flit_core-3.9.0.tar.gz.metadata", "flit_core"},
		{"cffi-1.0.2-2.tar.gz", ""},
		{"flit_core-.tar.gz", ""},
		{"broken!-1.0-py3-none-any.whl", ""},
		{"demo-1.0.whl", ""},
		{"demo-1.0.zip", ""},
		{"demo-1.0.tar.gz.metadata.metadata", ""},
	} {
		require.Equal(t, tc.want, extractPackageNameFromFilename(tc.filename), tc.filename)
	}
}

func TestExtractPackageNameFromFilePath(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/a/my_pkg-1.0.tar.gz", "my_pkg"},
		{"/a/../my_pkg-1.0.tar.gz", ""},
		{"/a%2Fb/my_pkg-1.0.tar.gz", ""},
	} {
		require.Equal(t, tc.want, extractPackageNameFromFilePath(tc.path), tc.path)
	}
}

func TestCheckPackageAccess(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		whitelist, blacklist []string
		path                 string
		allow                bool
	}{
		{"normalized whitelist match", []string{"My._-Pkg"}, nil, "/simple/MY_pkg", true},
		{"whitelist requires exact match", []string{"My._-Pkg"}, nil, "/simple/my-pkg-extra", false},
		{"blacklist overrides whitelist", []string{"my-pkg"}, []string{"My.Pkg"}, "/simple/my-pkg", false},
		{"blacklist permits other packages", nil, []string{"my-pkg"}, "/simple/other", true},
		{"unknown file without lists", nil, nil, "/files/legacy.zip", true},
		{"unknown file with blacklist", nil, []string{"my-pkg"}, "/files/legacy.zip", false},
		{"project list with whitelist", []string{"my-pkg"}, nil, "/simple/", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &proxyHandler{
				whitelist: buildPackageSet(tc.whitelist),
				blacklist: buildPackageSet(tc.blacklist),
			}
			require.Equal(t, tc.allow, h.checkPackageAccess(parseRequestPath(tc.path)))
		})
	}
}
