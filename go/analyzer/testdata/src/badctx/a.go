package badctx

import (
	stdctx "context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("badctx")

var ld *ldclient.LDClient

var _ stdctx.Context

func Shadowed(k string) {
	stdctx := 1
	_ = stdctx
	_, _ = ld.BoolVariation("a", ldcontext.New(k), false) // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
}
