package flags

import (

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("shop")

var ldClient *ldclient.LDClient

func CheckoutEnabled() bool {
	on, _ := ldClient.BoolVariation("new-checkout", ldcontext.New("user"), false)
	return on
}

func SearchEnabled() bool {
	on, _ := ldClient.BoolVariation("new-search", ldcontext.New("user"), false)
	return on
}
