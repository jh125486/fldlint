package flags

import (

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("checkout")

func newLD() *ldclient.LDClient {
	c, _ := ldclient.MakeClient("sdk-key", 0)
	return c
}

func CheckoutEnabled() bool {
	on, _ := newLD().BoolVariation("new-checkout", ldcontext.New("user"), false)
	return on
}
