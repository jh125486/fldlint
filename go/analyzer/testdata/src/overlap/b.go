package overlap

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"

	ld "github.com/launchdarkly/go-server-sdk/v7"
)

func F(c *ld.LDClient) bool {
	v, _ := c.BoolVariation("a", ldcontext.New("u"), false) // want `Migrate boolean flag "a" to OpenFeature`
	return v
}
