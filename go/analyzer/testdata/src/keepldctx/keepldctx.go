package keepldctx

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("keep")

var ld *ldclient.LDClient

// other still uses ldcontext, so the import must stay.
var other = ldcontext.NewBuilder("k").Kind("org").Build()

func Enabled(userKey string) bool {
	fmt.Println(other)
	on, _ := ld.BoolVariation("keep", ldcontext.New(userKey), true) // want `Migrate boolean flag "keep" to OpenFeature`
	return on
}
