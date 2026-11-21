package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("items")

var ldClient *ldclient.LDClient

func MaxItems(userKey string) int {
	n, _ := ldClient.IntVariation("max-items", ldcontext.New(userKey), 10)
	return n
}
