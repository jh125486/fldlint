package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("checkout")

var ldClient *ldclient.LDClient

func WhyEnabled() string {
	_, detail, _ := ldClient.BoolVariationDetail("new-checkout", ldcontext.New("user"), false)
	return string(detail.Reason.GetKind())
}
