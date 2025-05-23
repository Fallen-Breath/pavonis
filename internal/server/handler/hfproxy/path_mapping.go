package hfproxy

import (
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"net/url"
)

type pathMapping struct {
	PathPrefix  string
	Destination *url.URL
}

var hfUrl = utils.MustParseUrl("https://huggingface.co")

var pmCasServer = &pathMapping{
	PathPrefix:  "/.csxhc",
	Destination: utils.MustParseUrl("https://cas-server.xethub.hf.co"),
}
var pmTransfer = &pathMapping{
	PathPrefix:  "/.txhc",
	Destination: utils.MustParseUrl("https://transfer.xethub.hf.co"),
}

var pathMappings = []*pathMapping{
	{
		PathPrefix:  "/.cbxhc",
		Destination: utils.MustParseUrl("https://cas-bridge.xethub.hf.co"),
	},
	pmCasServer,
	pmTransfer,
	{
		PathPrefix:  "",
		Destination: hfUrl,
	},
}

func init() {
	for _, pm := range pathMappings {
		if pm.Destination.Path != "" {
			panic(fmt.Errorf("bad path mapping %+v", pm))
		}
	}
}
