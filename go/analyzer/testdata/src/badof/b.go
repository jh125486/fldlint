package badof

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	_ "github.com/open-feature/go-sdk/openfeature"
)

var ld *ldclient.LDClient

func Blank(k string) {
	_, _ = ld.BoolVariation("a", ldcontext.New(k), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
}
