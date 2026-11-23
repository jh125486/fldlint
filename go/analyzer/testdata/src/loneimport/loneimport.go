package loneimport

import "github.com/launchdarkly/go-sdk-common/v3/ldcontext"
import "github.com/launchdarkly/go-server-sdk/v7"
import "github.com/open-feature/go-sdk/openfeature"

var client = openfeature.NewClient("lone")

var ld *ldclient.LDClient

func Enabled() bool {
	on, _ := ld.BoolVariation("lone", ldcontext.New("u"), false) // want `Migrate boolean flag "lone" to OpenFeature`
	return on
}
