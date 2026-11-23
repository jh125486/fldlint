package nested

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("nested")

func must(b bool, err error) bool { return b }

func Outer(c *ldclient.LDClient) bool {
	on, _ := c.BoolVariation("outer", ldcontext.New("u"), must(c.BoolVariation("inner", ldcontext.New("u"), false))) // want `Migrate boolean flag "outer" to OpenFeature` `Migrate boolean flag "inner" to OpenFeature`
	return on
}
