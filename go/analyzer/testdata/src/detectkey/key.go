package detectkey

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-server-sdk/v7"
)

var ld *ldclient.LDClient

// Report-only rules carry a fingerprint key; the bulk rule stays keyless.
func Keys(ctx context.Context, name string, c ldcontext.Context) {
	_, _ = ld.IntVariation("flag-a", c, 1)                                // want `method=IntVariation key="flag-a"`
	_, _, _ = ld.BoolVariationDetail("flag-b", c, false)                  // want `method=BoolVariationDetail key="flag-b"`
	_, _, _ = ld.MigrationVariationCtx(ctx, "flag-c", c, ldmigration.Off) // want `method=MigrationVariationCtx key="flag-c"`
	_, _ = ld.IntVariation(name, c, 1)                                    // want `method=IntVariation key=absent$`
	_ = ld.AllFlagsState(c)                                               // want `method=AllFlagsState key=absent$`
	_, _ = ld.IntVariation("a\x41", c, 1)                                 // want `method=IntVariation key="aA"`
	_, _ = ld.IntVariation(`aA`, c, 1)                                    // want `method=IntVariation key="aA"`
	_, _ = ld.IntVariation("a\xff", c, 1)                                 // want `method=IntVariation key=absent$`
	_, _ = ld.BoolVariation("flag-d", c, false)                           // want `method=BoolVariation key="flag-d"`
	_, _ = ld.BoolVariation("a\xed\xa0\x80", c, false)                    // want `method=BoolVariation key=absent-static`
	_, _ = (*ldclient.LDClient).BoolVariation(ld, "flag-e", c, false)     // want `method=BoolVariation key="flag-e"`
	_, _ = ld.BoolVariation(("flag-f"), c, false)                         // want `method=BoolVariation key="flag-f"`
}
