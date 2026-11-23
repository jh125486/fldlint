//go:build !ignore

package constraints

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"
)

var ofc = openfeature.NewClient("constraints")

// Same constraint as the client's file: bound.
func B() bool {
	on, _ := ld.BoolVariation("b", ldcontext.New("u"), false) // want `Migrate boolean flag "b" to OpenFeature`
	return on
}
