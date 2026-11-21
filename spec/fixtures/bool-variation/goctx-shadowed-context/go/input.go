package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("flagged")

var ldClient *ldclient.LDClient

func Flagged(context string) bool {
	on, _ := ldClient.BoolVariation("flagged", ldcontext.New(context), false)
	return on
}
