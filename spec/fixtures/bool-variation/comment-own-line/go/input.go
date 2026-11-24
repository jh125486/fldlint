package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("banner")

var ldClient *ldclient.LDClient

func BannerEnabled(userKey string) bool {
	on, _ := ldClient.BoolVariation(
		// which flag
		"promo",
		ldcontext.New(userKey),
		false,
		// end
	)
	return on
}
