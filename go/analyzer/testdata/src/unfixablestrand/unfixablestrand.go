package unfixablestrand

import (
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("unfixablestrand")

func mk() *ldclient.LDClient { c, _ := ldclient.MakeClient("k", time.Second); return c }

// The same call reports the same skip with and without a comment: a comment
// with no home must not hide receiver-unproven.
func Without(userKey string) bool {
	ld := mk()
	on, _ := ld.BoolVariation("promo", ldcontext.New(userKey), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return on
}

func With(userKey string) bool {
	ld, _ := ldclient.MakeClient("k", 0)
	on, _ := ld.BoolVariation("promo", ldcontext.New(userKey /* id */), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return on
}

// Siblings keep their own outcome: "a" is rewritten (the commented call still
// uses the local), and the commented call is judged as if it were fixable, so
// it reports the skip it would report without the comment.
func Sibling(userKey string) bool {
	ld := mk()
	on, _ := ld.BoolVariation("a", ldcontext.New(userKey), false)         // want `Migrate boolean flag "a" to OpenFeature`
	_, _ = ld.BoolVariation("b", ldcontext.New(userKey /* id */), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return on
}

// Another surviving use: the commented call is just unfixable.
func Kept(k, userKey string) bool {
	ld := mk()
	on, _ := ld.BoolVariation("c", ldcontext.New(userKey /* id */), false) // want `Migrate boolean flag "c" to OpenFeature \[no-fix: comment\]`
	_, _ = ld.BoolVariation(k, ldcontext.New(userKey), false)              // want `Flag key is not a static string \[skip: dynamic-key\]`
	return on
}
