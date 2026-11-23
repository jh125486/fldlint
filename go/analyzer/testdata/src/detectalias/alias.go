package detectalias

import (
	stdctx "context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("alias")

var ld *ldclient.LDClient

var _ stdctx.Context

type holder struct{ ld *ldclient.LDClient }

func mk() *ldclient.LDClient { return nil }

func Alias(k string) {
	_, _ = ld.BoolVariation("a", ldcontext.New(k), false) // want `method=BoolVariation static=true key="a" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=true local=false`
}

func ShadowedOpenFeature(k string) {
	openfeature := 1
	_ = openfeature
	_, _ = ld.BoolVariation("b", ldcontext.New(k), false) // want `method=BoolVariation static=true key="b" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=false default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=true local=false`
}

func Receivers(k string, p *ldclient.LDClient, hs []holder) {
	l := mk()
	h := holder{ld: l}
	_, _ = l.BoolVariation("c", ldcontext.New(k), false)        // want `method=BoolVariation static=true key="c" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=true local=true`
	_, _ = h.ld.BoolVariation("d", ldcontext.New(k), false)     // want `method=BoolVariation static=true key="d" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=true local=true`
	_, _ = p.BoolVariation("e", ldcontext.New(k), false)        // want `method=BoolVariation static=true key="e" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=true local=false`
	_, _ = hs[0].ld.BoolVariation("f", ldcontext.New(k), false) // want `method=BoolVariation static=true key="f" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=false local=false`
	_, _ = mk().BoolVariation("g", ldcontext.New(k), false)     // want `method=BoolVariation static=true key="g" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=false local=false`
	_, _ = (*p).BoolVariation("h", ldcontext.New(k), false)     // want `method=BoolVariation static=true key="h" ctx=true eval=openfeature.NewEvaluationContext\(k, nil\) client=client bound=true default=false typed=true goctx=stdctx.TODO\(\) add=\[\] consumed=1 pure=false local=false`
	_ = h
}
