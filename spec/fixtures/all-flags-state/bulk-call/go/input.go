package flags

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("snapshot")

var ldClient *ldclient.LDClient

func Snapshot() any {
	state := ldClient.AllFlagsState(ldcontext.New("user"))
	return state.ToValuesMap()
}
