package flags

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("fastpath")

var (
	retries  = 3
	ldClient *ldclient.LDClient
)

func UseFastPath() bool {
	fmt.Println("retries", retries)
	on, _ := ldClient.BoolVariation("fast-path", ldcontext.New("user"), true)
	return on
}
