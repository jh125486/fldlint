package flags

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("checkout")

var ldClient *ldclient.LDClient

func Details(ctx context.Context, userKey string) {
	_, _, _ = ldClient.BoolVariationDetail("plain", ldcontext.New(userKey), false)
	_, _, _ = ldClient.BoolVariationDetailCtx(ctx, "shifted", ldcontext.New(userKey), false)
}
