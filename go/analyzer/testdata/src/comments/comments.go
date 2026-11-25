package comments

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("comments")

var ldClient *ldclient.LDClient

func Inline(userKey string) bool {
	on, _ := ldClient.BoolVariation("a" /* flag */, /* who */ ldcontext.New(userKey), false /* off */) // want `Migrate boolean flag "a" to OpenFeature`
	return on
}

func Lines(userKey string) bool {
	/* want `Migrate boolean flag "b" to OpenFeature` */ on, _ := ldClient.BoolVariation(
		"b", // the flag
		ldcontext.New(userKey), // who
		false, // default
	)
	return on
}

func Hoisted(userKey string) bool {
	on, _ := ldClient /* x */ . /* y */ BoolVariation /* z */ ("c", ldcontext.New(userKey), false) // want `Migrate boolean flag "c" to OpenFeature`
	return on
}

func OwnLine(userKey string) bool {
	/* want `Migrate boolean flag "d" to OpenFeature` */ on, _ := ldClient.BoolVariation(
		// which flag
		"d",
		ldcontext.New(userKey),
		false,
		// end
	)
	return on
}

func Verbatim(userKey string) bool {
	on, _ := ldClient.BoolVariation("e", ldcontext.New(string(/* kept with the key */ userKey)), false) // want `Migrate boolean flag "e" to OpenFeature`
	return on
}

func WithContext(ctx context.Context, userKey string) bool {
	on, _ := ldClient.BoolVariationCtx(ctx /* c */, /* flag */ "f", ldcontext.New(userKey), false) // want `Migrate boolean flag "f" to OpenFeature`
	return on
}
