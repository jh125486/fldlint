package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("x")

func keyOf(b bool, err error) string { return "k" }

func Outer(c *ldclient.LDClient) bool {
	on, _ := c.BoolVariation("outer", ldcontext.New(keyOf(c.BoolVariation("inner", ldcontext.New("u"), false))), false)
	return on
}
