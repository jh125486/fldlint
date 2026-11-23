package initcycle

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var ld = &ldclient.LDClient{}

var ofc = newOF()

// newOF is referenced by ofc's initializer: binding the call to ofc would be
// an initialization cycle.
func newOF() *openfeature.Client {
	if helper() {
		return openfeature.NewClient("a")
	}
	on, _ := ld.BoolVariation("direct", ldcontext.New("u"), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	_ = on
	return openfeature.NewClient("b")
}

// helper is reached transitively from ofc's initializer.
func helper() bool {
	on, _ := ld.BoolVariation("transitive", ldcontext.New("u"), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	return on
}

// Elsewhere is not reachable from the initializer, so ofc is fine.
func Elsewhere() bool {
	on, _ := ld.BoolVariation("elsewhere", ldcontext.New("u"), false) // want `Migrate boolean flag "elsewhere" to OpenFeature`
	return on
}
