package aliases

import (
	stdctx "context"

	ldc "github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	of "github.com/open-feature/go-sdk/openfeature"
)

var client = of.NewClient("aliases")

var ld *ldclient.LDClient

func Enabled(ctx stdctx.Context, userKey string) bool {
	on, _ := ld.BoolVariation("aliased", ldc.New(userKey), false) // want `Migrate boolean flag "aliased" to OpenFeature`
	return on
}

func Other(userKey string) bool {
	on, _ := ld.BoolVariation("aliased-two", ldc.New(userKey), false) // want `Migrate boolean flag "aliased-two" to OpenFeature`
	return on
}
