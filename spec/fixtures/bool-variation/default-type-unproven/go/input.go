package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("fallback")

var ldClient *ldclient.LDClient

func FeatureOn(fallback any) bool {
	on, _ := ldClient.BoolVariation("new-feature", ldcontext.New("user"), fallback)
	return on
}
