package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("dynamic")

var ldClient *ldclient.LDClient

func FeatureOn(feature string) bool {
	on, _ := ldClient.BoolVariation("feature-"+feature, ldcontext.New("user"), false)
	return on
}
