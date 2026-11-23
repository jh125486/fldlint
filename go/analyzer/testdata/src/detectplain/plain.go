package detectplain

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

func Probe(ctx context.Context, userKey string, f fake, c ldcontext.Context) {
	_, _ = ld.BoolVariation("a", ldcontext.New(userKey), false)
	_, _ = ld.BoolVariationCtx(ctx, "b", ldcontext.New("u"), true)
	_, _ = ld.BoolVariation("c", c, false)
	_, _ = ld.IntVariation("d", ldcontext.New(userKey), 1)
	_, _ = f.BoolVariation("e", ldcontext.New(userKey), false)
	_ = f.Close()
	_ = ld.Close()
	_, _ = ld.BoolVariation(userKey, ldcontext.New(userKey), false)
}
