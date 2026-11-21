package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("ui")

var ldClient *ldclient.LDClient

func UIConfig(userKey string) string {
	cfg, _ := ldClient.JSONVariation("ui-config", ldcontext.New(userKey), ldvalue.Null())
	return cfg.JSONString()
}
