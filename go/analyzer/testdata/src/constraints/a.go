package constraints

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
)

var ld *ldclient.LDClient

// The only client is declared under a build constraint this file does not
// have, so it may be absent where this file is built.
func A() bool {
	on, _ := ld.BoolVariation("a", ldcontext.New("u"), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	return on
}
