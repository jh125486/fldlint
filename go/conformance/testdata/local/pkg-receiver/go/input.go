package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"

	"example.com/real/clients"
)

var client = openfeature.NewClient("x")

func Enabled() bool {
	on, _ := clients.H.C.BoolVariation("pkg", ldcontext.New("u"), false)
	return on
}
