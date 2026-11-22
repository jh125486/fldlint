package detectprobe

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("probe")

var ld *ldclient.LDClient

type fake struct{}

func (fake) BoolVariation(key string, c ldcontext.Context, d bool) (bool, error) { return d, nil }
func (fake) Close() error                                                        { return nil }

func PtrCtx(ctx *context.Context, userKey string) {
	_, _ = ld.BoolVariation("p", ldcontext.New(userKey), false) // want `method=BoolVariation static=true key="p" ctx=true eval=openfeature.NewEvaluationContext\(userKey, nil\) client=client bound=true default=false typed=true goctx=context.TODO\(\) add=\[\] consumed=1 pure=true local=false`
}

func Probe(ctx context.Context, userKey string, f fake, c ldcontext.Context) {
	_, _ = ld.BoolVariation("a", ldcontext.New(userKey), false)     // want `method=BoolVariation static=true key="a" ctx=true eval=openfeature.NewEvaluationContext\(userKey, nil\) client=client bound=true default=false typed=true goctx=ctx add=\[\] consumed=1 pure=true local=false`
	_, _ = ld.BoolVariationCtx(ctx, "b", ldcontext.New("u"), true)  // want `method=BoolVariationCtx static=true key="b" ctx=true eval=openfeature.NewEvaluationContext\("u", nil\) client=client bound=true default=true typed=true goctx=ctx add=\[\] consumed=1 pure=true local=false`
	_, _ = ld.BoolVariation("c", c, false)                          // want `method=BoolVariation static=true key="c" ctx=false eval= client=client bound=true default=false typed=true goctx=ctx add=\[\] consumed=0 pure=true local=false`
	_, _ = ld.IntVariation("d", ldcontext.New(userKey), 1)          // want `method=IntVariation static=false key= ctx=false eval= client= bound=false default= typed=false goctx= add=\[\] consumed=0 pure=false local=false`
	_, _ = f.BoolVariation("e", ldcontext.New(userKey), false)      // want `not an sdk call`
	_ = f.Close()                                                   // want `not an sdk call`
	_ = ld.Close()                                                  // want `not an sdk call`
	_, _ = ld.BoolVariation(userKey, ldcontext.New(userKey), false) // want `method=BoolVariation static=false key= ctx=true eval=openfeature.NewEvaluationContext\(userKey, nil\) client=client bound=true default=false typed=true goctx=ctx add=\[\] consumed=1 pure=true local=false`
}
