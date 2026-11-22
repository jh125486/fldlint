package goctx

import (
	"context"
	"net/http"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("goctx")

var ld *ldclient.LDClient

func OneParam(ctx context.Context, userKey string) bool {
	on, _ := ld.BoolVariation("one", ldcontext.New(userKey), false) // want `Migrate boolean flag "one" to OpenFeature`
	return on
}

func NamedContext(context context.Context, userKey string) bool {
	on, _ := ld.BoolVariation("named", ldcontext.New(userKey), false) // want `Migrate boolean flag "named" to OpenFeature`
	return on
}

func TwoParams(a, b context.Context, userKey string) bool {
	on, _ := ld.BoolVariation("two", ldcontext.New(userKey), false) // want `Migrate boolean flag "two" to OpenFeature`
	return on
}

func Closure(ctx context.Context, userKey string) func() bool {
	return func() bool {
		on, _ := ld.BoolVariation("closure", ldcontext.New(userKey), false) // want `Migrate boolean flag "closure" to OpenFeature`
		return on
	}
}

func ClosureWithParam(userKey string) func(context.Context) bool {
	return func(c context.Context) bool {
		on, _ := ld.BoolVariation("withparam", ldcontext.New(userKey), false) // want `Migrate boolean flag "withparam" to OpenFeature`
		return on
	}
}

func ShadowedParam(ctx context.Context, userKey string) bool {
	{
		ctx := 1
		_ = ctx
		on, _ := ld.BoolVariation("shadowed-param", ldcontext.New(userKey), false) // want `Migrate boolean flag "shadowed-param" to OpenFeature`
		return on
	}
}

func Variant(r *http.Request) bool {
	on, _ := ld.BoolVariationCtx(r.Context(), "variant", ldcontext.New("u"), false) // want `Migrate boolean flag "variant" to OpenFeature`
	return on
}

func PointerContext(ctx *context.Context, userKey string) bool {
	on, _ := ld.BoolVariation("pointer", ldcontext.New(userKey), false) // want `Migrate boolean flag "pointer" to OpenFeature`
	return on
}

func RawKey(userKey string) bool {
	on, _ := ld.BoolVariation(`raw-key`, ldcontext.New(userKey), false) // want "Migrate boolean flag .raw-key. to OpenFeature"
	return on
}

var pkgLevel, _ = ld.BoolVariation("pkg-level", ldcontext.New("u"), false) // want `Migrate boolean flag "pkg-level" to OpenFeature`
