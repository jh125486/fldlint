package flags

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("billing")

var ldClient *ldclient.LDClient

func InvoicesEnabled(ctx context.Context, userKey string) bool {
	on, _ := ldClient.BoolVariation("new-invoices", ldcontext.New(userKey), false)
	return on
}
