package testutils

import (
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/common"
	servercontext "github.com/Fallen-Breath/pavonis/internal/server/context"
	"github.com/Fallen-Breath/pavonis/internal/server/handler"
)

// NewRequestHelper creates the real request helper with production defaults and
// registers its cleanup on the current test.
func NewRequestHelper(t testing.TB) *common.RequestHelper {
	t.Helper()
	cfg := &config.Config{}
	if err := cfg.Init(); err != nil {
		t.Fatalf("init test config: %v", err)
	}
	factory, err := common.NewRequestHelperFactory(cfg)
	if err != nil {
		t.Fatalf("create test request helper: %v", err)
	}
	t.Cleanup(factory.Shutdown)
	return factory.NewRequestHelper(nil)
}

func SiteInfo(id string, mode config.SiteMode, pathPrefix, selfURL string) *handler.Info {
	site := &config.SiteConfig{
		Id: id, Mode: &mode, Host: config.SiteHosts{"*"},
		PathPrefix: pathPrefix, SelfUrl: selfURL,
	}
	return handler.NewSiteInfo(id, site)
}

func Context() *servercontext.RequestContext {
	return servercontext.NewRequestContext("test", "127.0.0.1")
}
