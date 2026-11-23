package initcycleself

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var ld = &ldclient.LDClient{}

var ofc = func() *openfeature.Client {
	_, _ = ld.BoolVariation("self", ldcontext.New("u"), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	return openfeature.NewClient("a")
}()
