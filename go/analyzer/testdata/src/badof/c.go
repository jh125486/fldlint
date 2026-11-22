package badof

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	. "github.com/open-feature/go-sdk/openfeature"
)

var _ = NewClient

func Dot(k string) {
	_, _ = ld.BoolVariation("b", ldcontext.New(k), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
}
