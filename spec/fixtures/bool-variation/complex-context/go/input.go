package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("org")

var ldClient *ldclient.LDClient

func OrgFeature(orgKey string) bool {
	c := ldcontext.NewBuilder(orgKey).Kind("organization").Build()
	on, _ := ldClient.BoolVariation("org-feature", c, false)
	return on
}
