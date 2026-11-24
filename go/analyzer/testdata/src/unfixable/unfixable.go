package unfixable

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("unfixable")

var ldClient *ldclient.LDClient

// A comment inside the ldcontext.New(...) call has no home in the converted
// context: the diagnostic stays, the fix is withheld.
func InContext(userKey string) bool {
	on, _ := ldClient.BoolVariation("a", ldcontext.New(userKey /* user id */), false) // want `Migrate boolean flag "a" to OpenFeature`
	return on
}

// A // comment between the receiver and the method cannot be hoisted.
func LineInCallee(userKey string) bool {
	/* want `Migrate boolean flag "b" to OpenFeature` */ on, _ := ldClient. // note
		BoolVariation("b", ldcontext.New(userKey), false)
	return on
}

// A multi-line block comment between receiver and method cannot be hoisted.
func BlockInCallee(userKey string) bool {
	/* want `Migrate boolean flag "c" to OpenFeature` */ on, _ := ldClient. /* a
	b */ BoolVariation("c", ldcontext.New(userKey), false)
	return on
}

// The unfixable call still uses ldClient and ldcontext, so nothing is stranded
// and the neighbouring call is still rewritten.
func Neighbour(userKey string) bool {
	on, _ := ldClient.BoolVariation("d", ldcontext.New(userKey), false) // want `Migrate boolean flag "d" to OpenFeature`
	return on
}
