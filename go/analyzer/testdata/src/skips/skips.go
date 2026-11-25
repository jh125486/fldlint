package skips

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/interfaces/flagstate"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("skips")

var ld *ldclient.LDClient

const constKey = "const-key"

func RuleLevel(userKey string) {
	_, _, _ = ld.BoolVariationDetail("detail", ldcontext.New(userKey), false)       // want `Detail call; reason/variation metadata has no 1:1 rewrite \[skip: detail\]`
	_ = ld.AllFlagsState(ldcontext.New(userKey))                                    // want `Bulk evaluation; migrate manually \[skip: bulk\]`
	_, _ = ld.IntVariation("int", ldcontext.New(userKey), 1)                        // want `Return/default type differs between SDKs \[skip: type-mismatch\]`
	_, _ = ld.JSONVariation("json", ldcontext.New(userKey), ldvalue.Null())         // want `Return/default type differs between SDKs \[skip: type-mismatch\]`
	_, _, _ = ld.MigrationVariation("mig", ldcontext.New(userKey), ldmigration.Off) // want `Return/default type differs between SDKs \[skip: type-mismatch\]`
}

// A spread into a variadic report-only method is still reported; the spread
// may be empty, so only the arguments before it must reach the minimum.
func Variadic(userKey string, opts ...flagstate.Option) {
	c := ldcontext.New(userKey)
	_ = ld.AllFlagsState(c, opts...)                                      // want `Bulk evaluation; migrate manually \[skip: bulk\]`
	_ = ld.AllFlagsState(c, flagstate.Option(nil), flagstate.Option(nil)) // want `Bulk evaluation; migrate manually \[skip: bulk\]`
}

// A method expression passes the receiver as its first argument; it does not
// count toward the method's own arity.
func MethodExpr(userKey string) {
	_, _, _ = (*ldclient.LDClient).BoolVariationDetail(ld, "detail", ldcontext.New(userKey), false) // want `Detail call; reason/variation metadata has no 1:1 rewrite \[skip: detail\]`
}

func recvAndContext(userKey string) (*ldclient.LDClient, ldcontext.Context) {
	return ld, ldcontext.New(userKey)
}

// A single call supplying the receiver and the context counts both values.
func MethodExprTuple(userKey string) {
	_ = (*ldclient.LDClient).AllFlagsState(recvAndContext(userKey)) // want `Bulk evaluation; migrate manually \[skip: bulk\]`
}

func DynamicKey(userKey, suffix string) {
	_, _ = ld.BoolVariation("a-"+suffix, ldcontext.New(userKey), false) // want `Flag key is not a static string \[skip: dynamic-key\]`
	_, _ = ld.BoolVariation(suffix, ldcontext.New(userKey), false)      // want `Flag key is not a static string \[skip: dynamic-key\]`
	_, _ = ld.BoolVariation(constKey, ldcontext.New(userKey), false)    // want `Flag key is not a static string \[skip: dynamic-key\]`
}

func ContextUnproven(userKey string, c ldcontext.Context) {
	_, _ = ld.BoolVariation("a", c, false)                                          // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
	_, _ = ld.BoolVariation("b", ldcontext.NewBuilder(userKey).Build(), false)      // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
	_, _ = ld.BoolVariation("c", ldcontext.NewWithKind("org", userKey), false)      // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
	_, _ = ld.BoolVariation("d", ldcontext.NewMulti(ldcontext.New(userKey)), false) // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
	_, _ = ld.StringVariation("e", c, "x")                                          // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
	fmt.Println()
}

func ContextShadowed(context string) {
	_, _ = ld.BoolVariation("a", ldcontext.New(context), false) // want `Cannot prove LD context -> EvaluationContext conversion \[skip: context-unproven\]`
}

func ShadowedClient(userKey string) {
	client := fmt.Sprint(userKey)
	_ = client
	_, _ = ld.BoolVariation("a", ldcontext.New(userKey), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
}
