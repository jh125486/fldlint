package shadowedof

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("shadowedof")

var ld *ldclient.LDClient

func Shadowed(userKey string) bool {
	openfeature := 1
	_ = openfeature
	on, _ := ld.BoolVariation("a", ldcontext.New(userKey), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	return on
}

func ShadowedUnprovenContext(c ldcontext.Context) bool {
	openfeature := 1
	_ = openfeature
	on, _ := ld.BoolVariation("b", c, false) // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
	return on
}
