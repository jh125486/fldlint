package flags

import (
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("checkout")

func CheckoutEnabled() (bool, error) {
	ld, err := ldclient.MakeClient("sdk-key", 5*time.Second)
	if err != nil {
		return false, err
	}
	on, _ := ld.BoolVariation("new-checkout", ldcontext.New("user"), false)
	return on, nil
}
