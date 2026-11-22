package badctx

import (
	_ "context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

func Blank(k string) {
	_, _ = ld.BoolVariation("b", ldcontext.New(k), false) // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
}
