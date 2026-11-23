package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"

	"example.com/real/clients"
)

var client = openfeature.NewClient("x")

func Enabled() bool {
	on, _ := clients.LD.BoolVariation("a", ldcontext.New("u"), false)
	return on
}

func Other() bool {
	on, _ := clients.LD.BoolVariation("b", ldcontext.New("u"), false)
	return on
}
