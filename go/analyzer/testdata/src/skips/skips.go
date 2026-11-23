package skips

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldmigration"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	"github.com/launchdarkly/go-server-sdk/v7"
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
