package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("pricing")

var ldClient *ldclient.LDClient

func DiscountRate(userKey string) float64 {
	rate, _ := ldClient.Float64Variation("discount-rate", ldcontext.New(userKey), 0.1)
	return rate
}
