package flags

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("search")

var c *ldclient.LDClient

func ShowSearch() bool {
	on, err := c.BoolVariation("new-search", ldcontext.New("user"), false)
	if err != nil {
		fmt.Println("flag error:", err)
	}
	return on
}
