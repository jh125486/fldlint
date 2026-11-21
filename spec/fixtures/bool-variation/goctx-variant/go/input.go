package flags

import (
	"net/http"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("beta")

var ldClient *ldclient.LDClient

func Beta(r *http.Request) bool {
	on, _ := ldClient.BoolVariationCtx(r.Context(), "beta-program", ldcontext.New("user"), false)
	return on
}
