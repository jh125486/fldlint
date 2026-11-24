package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("banner")

var ldClient *ldclient.LDClient

func BannerEnabled(userKey string) bool {
	on, _ := ldClient /* a */ . /* b */ BoolVariation /* p */ ("promo", ldcontext.New(userKey), false)
	return on
}
