package defaulttype

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("defaulttype")

var ld *ldclient.LDClient

type flag bool

// This package does not type-check against the real SDK signatures (the
// default is not a bool); it exists to exercise the type-unproven path on
// partially typed code, so the test runs the analyzer despite errors.
func Mismatch(fallback any, named flag) {
	_, _ = ld.BoolVariation("a", ldcontext.New("u"), fallback) // want `Default value type not proven \[skip: default-type-unproven\]`
	_, _ = ld.BoolVariation("b", ldcontext.New("u"), named)    // want `Default value type not proven \[skip: default-type-unproven\]`
}
