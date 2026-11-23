package pkgrecv

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"

	"example.com/pkgrecv/clients"
)

var client = openfeature.NewClient("pkgrecv")

func Enabled() bool {
	on, _ := clients.H.C.BoolVariation("pkg", ldcontext.New("u"), false) // want `Migrate boolean flag "pkg" to OpenFeature`
	return on
}
