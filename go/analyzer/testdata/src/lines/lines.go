package lines

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("lines")

var ld *ldclient.LDClient

//line generated.go:10
func Enabled() bool {
	on, _ := ld.BoolVariation("generated", ldcontext.New("u"), false) // want `Migrate boolean flag "generated" to OpenFeature`
	return on
}
