package pkgrecvtwo

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"

	"example.com/pkgrecvtwo/clients"
)

var client = openfeature.NewClient("pkgrecvtwo")

func Enabled() (bool, bool) {
	a, _ := clients.LD.BoolVariation("a", ldcontext.New("u"), false) // want `Migrate boolean flag "a" to OpenFeature`
	b, _ := clients.LD.BoolVariation("b", ldcontext.New("u"), false) // want `Migrate boolean flag "b" to OpenFeature`
	return a, b
}
