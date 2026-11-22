package twocalls

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("two")

var ld *ldclient.LDClient

func Both(userKey string) (bool, string) {
	on, _ := ld.BoolVariation("a", ldcontext.New(userKey), false)        // want `Migrate boolean flag "a" to OpenFeature`
	s, _ := ld.StringVariation("b", ldcontext.New(userKey), "fallback") // want `Migrate string flag "b" to OpenFeature`
	fmt.Println(on, s)
	return on, s
}
