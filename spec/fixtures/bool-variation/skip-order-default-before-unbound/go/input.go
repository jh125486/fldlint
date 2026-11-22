package flags

import (

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
)

var ldClient *ldclient.LDClient

func FeatureOn(fallback any) bool {
	on, _ := ldClient.BoolVariation("new-feature", ldcontext.New("user"), fallback)
	return on
}
