package flags

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("sync")

var ldClient *ldclient.LDClient

func SyncEnabled(reqCtx, jobCtx context.Context, userKey string) bool {
	on, _ := ldClient.BoolVariation("new-sync", ldcontext.New(userKey), false)
	return on
}
