package forkuse

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("fork")

var ld *ldclient.LDClient

// Every signature below differs from the real SDK: no finding, so no
// diagnostic, whatever the call site looks like.
func Forked(userKey string) {
	_, _ = ld.BoolVariation(false, "a", ldcontext.New(userKey))
	_ = ld.StringVariation("b", ldcontext.New(userKey), "x")
	_, _ = ld.Float64Variation("c", ldcontext.New(userKey), 1.5, 0)
	_, _ = ld.BoolVariationCtx("ctx", "d", ldcontext.New(userKey), false)
}
