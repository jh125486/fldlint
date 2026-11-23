package stranded

import (
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("stranded")

var pkgLD *ldclient.LDClient

type holder struct{ ld *ldclient.LDClient }

func mk() *ldclient.LDClient { c, _ := ldclient.MakeClient("k", time.Second); return c }

// Only use: stranded.
func OnlyUse() bool {
	ld := mk()
	on, _ := ld.BoolVariation("a", ldcontext.New("u"), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return on
}

// local-client-survivor-skip: the dynamic-key call keeps the local used, so
// the rewritable call is rewritten and the other is skipped dynamic-key.
func Survivor(k string) bool {
	ld := mk()
	on, _ := ld.BoolVariation("b", ldcontext.New("u"), false) // want `Migrate boolean flag "b" to OpenFeature`
	_, _ = ld.BoolVariation(k, ldcontext.New("u"), false)     // want `Flag key is not a static string \[skip: dynamic-key\]`
	return on
}

// A plain assignment is not a use.
func PlainAssign() bool {
	ld := mk()
	on, _ := ld.BoolVariation("c", ldcontext.New("u"), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	ld = nil
	return on
}

// A blank assignment is a use.
func BlankAssign() bool {
	ld := mk()
	on, _ := ld.BoolVariation("d", ldcontext.New("u"), false) // want `Migrate boolean flag "d" to OpenFeature`
	_ = ld
	return on
}

// A read inside a closure is a use.
func ClosureRead() bool {
	ld := mk()
	on, _ := ld.BoolVariation("e", ldcontext.New("u"), false) // want `Migrate boolean flag "e" to OpenFeature`
	defer func() { _ = ld.Close() }()
	return on
}

// A field store through the root keeps it used.
func FieldStore() bool {
	h := &holder{}
	on, _ := h.ld.BoolVariation("f", ldcontext.New("u"), false) // want `Migrate boolean flag "f" to OpenFeature`
	h.ld = nil
	return on
}

// The root of a selector chain strands like a plain local.
func RootStranded() bool {
	h := holder{ld: mk()}
	on, _ := h.ld.BoolVariation("g", ldcontext.New("u"), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return on
}

// Package variables, parameters and named results never strand.
func Never(p *ldclient.LDClient) (on bool) {
	on, _ = pkgLD.BoolVariation("h", ldcontext.New("u"), false) // want `Migrate boolean flag "h" to OpenFeature`
	on, _ = p.BoolVariation("i", ldcontext.New("u"), false)     // want `Migrate boolean flag "i" to OpenFeature`
	return on
}

func NamedResult() (ld *ldclient.LDClient, on bool) {
	on, _ = ld.BoolVariation("j", ldcontext.New("u"), false) // want `Migrate boolean flag "j" to OpenFeature`
	return
}

// Impure receivers are never rewritten.
func Impure(hs []holder) bool {
	a, _ := hs[0].ld.BoolVariation("k", ldcontext.New("u"), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	b, _ := (*pkgLD).BoolVariation("l", ldcontext.New("u"), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return a && b
}
