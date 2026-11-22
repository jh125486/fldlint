package clients

import (
	"context"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/interfaces"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("clients")

func InterfaceReceiver(ld interfaces.LDClientInterface, userKey string) bool {
	on, _ := ld.BoolVariation("iface", ldcontext.New(userKey), false) // want `Migrate boolean flag "iface" to OpenFeature`
	return on
}

func EvaluationInterface(ld interfaces.LDClientEvaluations, userKey string) bool {
	on, _ := ld.BoolVariation("eval-iface", ldcontext.New(userKey), false) // want `Migrate boolean flag "eval-iface" to OpenFeature`
	return on
}

func Generic[C interfaces.LDClientInterface](ld C, userKey string) bool {
	on, _ := ld.BoolVariation("generic", ldcontext.New(userKey), false) // want `Migrate boolean flag "generic" to OpenFeature`
	return on
}

type withInterface struct {
	interfaces.LDClientInterface
}

func EmbeddedInterface(w withInterface, userKey string) bool {
	on, _ := w.BoolVariation("embedded-iface", ldcontext.New(userKey), false) // want `Migrate boolean flag "embedded-iface" to OpenFeature`
	return on
}

type service struct {
	ld *ldclient.LDClient
}

func (s *service) StructField(userKey string) bool {
	on, _ := s.ld.BoolVariation("field", ldcontext.New(userKey), false) // want `Migrate boolean flag "field" to OpenFeature`
	return on
}

type embedded struct {
	*ldclient.LDClient
}

func Embedded(e embedded, userKey string) bool {
	on, _ := e.BoolVariation("embedded", ldcontext.New(userKey), false) // want `Migrate boolean flag "embedded" to OpenFeature`
	return on
}

func Factory() *ldclient.LDClient { c, _ := ldclient.MakeClient("k", 0); return c }

func ThroughFactory(userKey string) bool {
	on, _ := Factory().BoolVariation("factory", ldcontext.New(userKey), false) // want `LaunchDarkly client expression cannot be removed safely \[skip: receiver-unproven\]`
	return on
}

func ParamClient(ctx context.Context, of *openfeature.Client, ld *ldclient.LDClient, userKey string) bool {
	on, _ := ld.BoolVariation("param", ldcontext.New(userKey), false) // want `Migrate boolean flag "param" to OpenFeature`
	return on
}

func InterfaceClient(of openfeature.IClient, ld *ldclient.LDClient, userKey string) bool {
	on, _ := ld.BoolVariation("iclient", ldcontext.New(userKey), false) // want `Migrate boolean flag "iclient" to OpenFeature`
	return on
}

type wrapper interface {
	openfeature.IClient
}

func WrappedInterface(w wrapper, ld *ldclient.LDClient, userKey string) bool {
	on, _ := ld.BoolVariation("wrapped", ldcontext.New(userKey), false) // want `Migrate boolean flag "wrapped" to OpenFeature`
	return on
}

func LocalClient(ld *ldclient.LDClient, userKey string) bool {
	local := openfeature.NewClient("local")
	on, _ := ld.BoolVariation("local", ldcontext.New(userKey), false) // want `Migrate boolean flag "local" to OpenFeature`
	return on && local != nil
}

func LocalBeforeUse(ld *ldclient.LDClient, userKey string) bool {
	on, _ := ld.BoolVariation("declared-later", ldcontext.New(userKey), false) // want `Migrate boolean flag "declared-later" to OpenFeature`
	later := openfeature.NewClient("later")
	return on && later != nil
}

func InnerShadowsOuter(ld *ldclient.LDClient, userKey string) bool {
	client := openfeature.NewClient("inner")
	on, _ := ld.BoolVariation("inner", ldcontext.New(userKey), false) // want `Migrate boolean flag "inner" to OpenFeature`
	return on && client != nil
}

func Ambiguous(a, b *openfeature.Client, ld *ldclient.LDClient, userKey string) bool {
	on, _ := ld.BoolVariation("ambiguous", ldcontext.New(userKey), false) // want `OpenFeature client binding not proven in scope \[skip: unbound-client\]`
	return on
}

func MethodValue(ld *ldclient.LDClient, userKey string) bool {
	f := ld.BoolVariation
	on, _ := f("method-value", ldcontext.New(userKey), false)
	return on
}
