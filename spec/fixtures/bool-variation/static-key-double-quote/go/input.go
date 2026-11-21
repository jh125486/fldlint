package flags

import (
	"fmt"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("checkout")

func CheckoutEnabled() bool {
	ldClient, err := ldclient.MakeClient("sdk-key", 5*time.Second)
	if err != nil {
		return false
	}
	enabled, err := ldClient.BoolVariation("new-checkout", ldcontext.New("user"), false)
	if err != nil {
		fmt.Println("flag error:", err)
	}
	return enabled
}
