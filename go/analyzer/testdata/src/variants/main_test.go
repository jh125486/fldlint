package variants

import (
	"testing"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var ofc = openfeature.NewClient("test")

func inTest(ld *ldclient.LDClient) {
	_, _ = ld.BoolVariation("in-test", ldcontext.New("u"), false) // want `Migrate boolean flag "in-test" to OpenFeature`
}

func TestEnabled(t *testing.T) { inTest(nil) }
