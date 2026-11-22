package addctx

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("addctx")

var ld *ldclient.LDClient

func Enabled(userKey string) bool {
	on, _ := ld.BoolVariation("new-ui", ldcontext.New(userKey), false) // want `Migrate boolean flag "new-ui" to OpenFeature`
	return on
}
