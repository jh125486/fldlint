package twofiles

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
)

var ld *ldclient.LDClient

func Enabled(userKey string) bool {
	on, _ := ld.BoolVariation("two-files", ldcontext.New(userKey), false) // want `Migrate boolean flag "two-files" to OpenFeature`
	return on
}
