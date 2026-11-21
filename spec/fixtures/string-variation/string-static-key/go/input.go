package flags

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("layout")

var ldClient *ldclient.LDClient

func CheckoutLayout(ctx context.Context, userKey string) string {
	layout, _ := ldClient.StringVariation("checkout-layout", ldcontext.New(userKey), "classic")
	return layout
}
