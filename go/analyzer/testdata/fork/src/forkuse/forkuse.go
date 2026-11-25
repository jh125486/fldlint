package forkuse

import (
	"context"

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

// Report-only methods are pinned by argCount rather than a template: a
// forked signature whose argument count is outside the range is no finding.
// BoolVariationDetail and AllFlagsState keep the SDK count and are reported.
func ForkedReportOnly(userKey string, ctx context.Context) {
	_, _, _ = ld.IntVariationDetail("a", ldcontext.New(userKey))
	_, _, _ = ld.StringVariationDetail("b", ldcontext.New(userKey), "x", 0)
	_, _ = ld.IntVariationCtx("c", ldcontext.New(userKey), 1)
	_, _, _ = ld.BoolVariationDetailCtx(ctx, "d", ldcontext.New(userKey), false, 0)
	_, _, _ = ld.BoolVariationDetail("e", ldcontext.New(userKey), false) // want `Detail call; reason/variation metadata has no 1:1 rewrite \[skip: detail\]`
	_ = ld.AllFlagsState(ldcontext.New(userKey))                         // want `Bulk evaluation; migrate manually \[skip: bulk\]`
}
