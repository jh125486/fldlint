package variants

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
)

// No OpenFeature client exists in the non-test files: the call is unbound in
// the package and in its test variant alike.
func Enabled(ld *ldclient.LDClient) bool {
	on, _ := ld.BoolVariation("k", ldcontext.New("u"), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	return on
}
