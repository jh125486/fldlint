package parenonly

import (
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

var client = openfeature.NewClient("parenonly")

var ld *ldclient.LDClient

func Enabled() bool {
	on, _ := ld.BoolVariation("paren", ldcontext.New("u"), false) // want `Migrate boolean flag "paren" to OpenFeature`
	return on
}
