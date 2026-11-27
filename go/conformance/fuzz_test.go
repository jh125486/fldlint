package conformance_test

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The planner fuzzer generates one small Go file per input from a template
// grammar (receiver kind x key shape x context shape x client-binding shape x
// `context` shadowing x method), runs the analyzer through the same machinery
// as the conformance harness and checks the safety properties of issue #21:
//
//   - no panic and no analysis error;
//   - every rewritten file parses and type-checks against the SDK stubs;
//   - rewriting is idempotent (a converged file has nothing left to fix);
//   - every skipped call leaves the file byte-identical.
//
// Native fuzzing needs no dependency. Seeds below run as a normal unit test
// (`go test`); `go test -run=^$ -fuzz=FuzzPlanner -fuzztime=30s ./conformance`
// explores further. A crash is a real bug: commit the minimized input that
// the fuzzer writes under testdata/fuzz/FuzzPlanner as a regression seed.

// plannerCase is one point of the grammar. Every index is reduced modulo the
// size of its dimension, so any byte is valid.
type plannerCase struct {
	recv, key, ctx, bind, shadow, method uint8
	badDefault, two, stranded            bool
	keyLit                               string
}

// Dimension sizes.
const (
	nRecv   = 5 // local, package var, struct field, call result, parameter
	nKey    = 6 // literal, const ident, raw string, concatenation, param, call
	nCtx    = 6 // New("u"), New(uid), Builder, ldcontext.Context param, NewWithKind, New(uid + "x")
	nBind   = 5 // none, package var, function-local, shadowed, two package vars
	nShadow = 5 // none, string param `context`, local `context`, ctx param, `context context.Context` param
	nMethod = 6 // Bool, BoolCtx, String, Float64, Int (report-only), AllFlagsState
)

type method struct {
	name, defVal, defType string
	ctx, keyless          bool
}

var plannerMethods = [nMethod]method{
	{name: "BoolVariation", defVal: "false", defType: "bool"},
	{name: "BoolVariationCtx", defVal: "false", defType: "bool", ctx: true},
	{name: "StringVariation", defVal: `"dv"`, defType: "string"},
	{name: "Float64Variation", defVal: "1.5", defType: "float64"},
	{name: "IntVariation", defVal: "1", defType: "int"},
	{name: "AllFlagsState", keyless: true},
}

var ctxShapes = [nCtx]string{
	`ldcontext.New("u")`,
	`ldcontext.New(uid)`,
	`ldcontext.NewBuilder("u").Build()`,
	`uctx`,
	`ldcontext.NewWithKind("org", "o")`,
	`ldcontext.New(uid + "x")`,
}

// source renders the case as gofmt-formatted Go source.
func (p plannerCase) source() string {
	recv, key, ctx, bind, shadow := p.recv%nRecv, p.key%nKey, p.ctx%nCtx, p.bind%nBind, p.shadow%nShadow
	m := plannerMethods[p.method%nMethod]

	// Does the function take a Go context, and under which name?
	goCtx := ""
	switch {
	case shadow == 4:
		goCtx = "context"
	case m.ctx || shadow == 3:
		goCtx = "ctx"
	}

	var imports, pkgDecls, params, locals []string
	imports = append(imports,
		`"github.com/launchdarkly/go-sdk-common/v3/ldcontext"`,
		`"github.com/launchdarkly/go-server-sdk/v7"`)
	if bind != 0 {
		imports = append(imports, `"github.com/open-feature/go-sdk/openfeature"`)
	}
	if goCtx != "" {
		imports = append(imports, `"context"`)
		if goCtx == "ctx" {
			params = append(params, "ctx context.Context")
		} else {
			params = append(params, "context context.Context")
		}
	}

	recvExpr := "lc"
	switch recv {
	case 0:
		locals = append(locals, `lc, _ := ldclient.MakeClient("sdk", 0)`)
		if !p.stranded {
			// Another use keeps the local alive after the call is rewritten.
			locals = append(locals, "defer lc.Close()")
		}
	case 1:
		pkgDecls = append(pkgDecls, "var lc *ldclient.LDClient")
	case 2:
		pkgDecls = append(pkgDecls, "type holder struct{ lc *ldclient.LDClient }")
		params = append(params, "h holder")
		recvExpr = "h.lc"
	case 3:
		pkgDecls = append(pkgDecls, "func get() *ldclient.LDClient { return nil }")
		recvExpr = "get()"
	case 4:
		params = append(params, "lc *ldclient.LDClient")
	}

	keyExpr := strconv.Quote(p.keyLit)
	switch key {
	case 1:
		pkgDecls = append(pkgDecls, `const flagKey = "const-key"`)
		keyExpr = "flagKey"
	case 2:
		keyExpr = "`raw-key`"
	case 3:
		keyExpr = `"a" + "b"`
	case 4:
		params = append(params, "k string")
		keyExpr = "k"
	case 5:
		pkgDecls = append(pkgDecls, `func keyFn() string { return "x" }`)
		keyExpr = "keyFn()"
	}

	switch ctx {
	case 1, 5:
		params = append(params, "uid string")
	case 3:
		params = append(params, "uctx ldcontext.Context")
	}

	switch bind {
	case 1:
		pkgDecls = append(pkgDecls, `var client = openfeature.NewClient("fz")`)
	case 2:
		locals = append(locals, `client := openfeature.NewClient("fz")`, "_ = client")
	case 3:
		pkgDecls = append(pkgDecls, `var client = openfeature.NewClient("fz")`)
		locals = append(locals, "client := 1", "_ = client")
	case 4:
		pkgDecls = append(pkgDecls, `var client = openfeature.NewClient("fz")`, `var other = openfeature.NewClient("other")`)
	}

	switch shadow {
	case 1:
		params = append(params, "context string")
	case 2:
		locals = append(locals, `context := "x"`, "_ = context")
	}

	def := m.defVal
	if p.badDefault && !m.keyless {
		params = append(params, "dflt "+m.defType)
		def = "dflt"
	}

	call := func(key, ctxExpr, def string) string {
		var args []string
		if m.ctx {
			args = append(args, goCtx)
		}
		if !m.keyless {
			args = append(args, key)
		}
		args = append(args, ctxExpr)
		if !m.keyless {
			args = append(args, def)
		}
		return recvExpr + "." + m.name + "(" + strings.Join(args, ", ") + ")"
	}
	assign := func(v, expr string) string {
		if m.keyless {
			return v + " := " + expr
		}
		return v + ", _ := " + expr
	}

	var b strings.Builder
	b.WriteString("package flags\n\nimport (\n")
	for _, i := range imports {
		b.WriteString("\t" + i + "\n")
	}
	b.WriteString(")\n\n")
	for _, d := range pkgDecls {
		b.WriteString(d + "\n\n")
	}
	fmt.Fprintf(&b, "func F(%s) bool {\n", strings.Join(params, ", "))
	for _, l := range locals {
		b.WriteString("\t" + l + "\n")
	}
	b.WriteString("\t" + assign("v", call(keyExpr, ctxShapes[ctx], def)) + "\n\t_ = v\n")
	if p.two {
		// The second call always has an unproven context (a builder), so a
		// skip sits next to whatever the first call does.
		b.WriteString("\t" + assign("w", call(`"second"`, ctxShapes[2], def)) + "\n\t_ = w\n")
	}
	b.WriteString("\treturn true\n}\n")

	out, err := format.Source([]byte(b.String()))
	if err != nil {
		// A generator bug, not a product bug: fail loudly with the text.
		panic(fmt.Sprintf("generated source does not parse: %v\n%s", err, b.String()))
	}
	return string(out)
}

// Each fuzz byte is mapped through a weighted table, so the rewrite path is
// common (a uniform byte hits a skip almost every time: the receiver, the
// context and the client must all be provable). Every dimension value appears
// at least once in its table.
var (
	recvW   = []uint8{0, 0, 0, 1, 2, 4, 3}
	keyW    = []uint8{0, 0, 0, 1, 2, 3, 0, 4, 5}
	ctxW    = []uint8{0, 0, 0, 1, 1, 3, 2, 4, 5}
	bindW   = []uint8{1, 1, 1, 1, 2, 0, 3, 4}
	shadowW = []uint8{0, 0, 0, 0, 1, 2, 3, 4}
	methodW = []uint8{0, 0, 0, 1, 2, 3, 4, 5}
)

// pick maps a fuzz byte to a dimension value.
func pick(table []uint8, b uint8) uint8 { return table[int(b)%len(table)] }

// Flag bits of the last fuzz byte: a bad default on 1 of 8, then two bits.
const (
	flagBadDefault = 1 // flags%8 == 1
	flagTwo        = 8
	flagStranded   = 16
)

// decodePlanner maps the fuzz bytes to a grammar point.
func decodePlanner(recv, key, ctx, bind, shadow, method, flags uint8, keyLit string) plannerCase {
	return plannerCase{
		recv: pick(recvW, recv), key: pick(keyW, key), ctx: pick(ctxW, ctx), bind: pick(bindW, bind),
		shadow: pick(shadowW, shadow), method: pick(methodW, method),
		badDefault: flags%8 == flagBadDefault, two: flags&flagTwo != 0, stranded: flags&flagStranded != 0,
		keyLit: keyLit,
	}
}

// plannerSeed is one point of the grammar with the outcome the analyzer must
// produce for it, one entry per finding in source order: "rewrite" (a fix is
// offered) or the skip category.
type plannerSeed struct {
	name string
	plannerCase
	want []string
}

const (
	rewrite = "rewrite"
	ctxUnpr = "context-unproven"
)

var plannerSeeds = []plannerSeed{
	{name: "stranded local client", plannerCase: plannerCase{stranded: true, bind: 1, keyLit: "stranded"}, want: []string{"receiver-unproven"}},
	{name: "basic rewrite", plannerCase: plannerCase{bind: 1, keyLit: "basic"}, want: []string{rewrite}},
	{name: "package var receiver", plannerCase: plannerCase{recv: 1, bind: 1, keyLit: "pkg-recv"}, want: []string{rewrite}},
	{name: "struct field receiver, local client", plannerCase: plannerCase{recv: 2, bind: 2, ctx: 1, keyLit: "field"}, want: []string{rewrite}},
	{name: "call-result receiver", plannerCase: plannerCase{recv: 3, bind: 1, keyLit: "impure"}, want: []string{"receiver-unproven"}},
	{name: "parameter receiver rewrites", plannerCase: plannerCase{recv: 4, bind: 1, keyLit: "param"}, want: []string{rewrite}},
	{name: "parameter receiver, ldcontext param", plannerCase: plannerCase{recv: 4, bind: 1, ctx: 3, keyLit: "param"}, want: []string{ctxUnpr}},
	{name: "raw string key", plannerCase: plannerCase{key: 2, bind: 1}, want: []string{rewrite}},
	{name: "concatenated key", plannerCase: plannerCase{key: 3, bind: 1}, want: []string{"dynamic-key"}},
	{name: "dynamic key param", plannerCase: plannerCase{key: 4, bind: 1}, want: []string{"dynamic-key"}},
	{name: "builder context", plannerCase: plannerCase{ctx: 2, bind: 1, keyLit: "builder"}, want: []string{ctxUnpr}},
	{name: "no OpenFeature client", plannerCase: plannerCase{bind: 0, keyLit: "no-client"}, want: []string{"unbound-client"}},
	{name: "function-local client", plannerCase: plannerCase{bind: 2, keyLit: "local-client"}, want: []string{rewrite}},
	{name: "client shadowed by an int", plannerCase: plannerCase{bind: 3, keyLit: "shadowed-client"}, want: []string{"unbound-client"}},
	{name: "two package clients", plannerCase: plannerCase{bind: 4, keyLit: "two-clients"}, want: []string{"unbound-client"}},
	{name: "context shadowed by a string param: no context.TODO", plannerCase: plannerCase{bind: 1, shadow: 1, keyLit: "ctx-pkg-shadowed"}, want: []string{ctxUnpr}},
	{name: "context shadowed by a local: no context.TODO", plannerCase: plannerCase{bind: 1, shadow: 2, keyLit: "ctx-local-shadowed"}, want: []string{ctxUnpr}},
	{name: "ctx param, plain method", plannerCase: plannerCase{bind: 1, shadow: 3, keyLit: "go-ctx-param"}, want: []string{rewrite}},
	{name: "param named context", plannerCase: plannerCase{bind: 1, shadow: 4, keyLit: "ctx-named-context"}, want: []string{rewrite}},
	{name: "Ctx method rewrites although context is shadowed", plannerCase: plannerCase{bind: 1, shadow: 1, method: 1, keyLit: "ctx-method-shadowed"}, want: []string{rewrite}},
	{name: "Ctx method", plannerCase: plannerCase{bind: 1, method: 1, keyLit: "ctx-method"}, want: []string{rewrite}},
	{name: "string flag", plannerCase: plannerCase{bind: 1, method: 2, keyLit: "string"}, want: []string{rewrite}},
	{name: "number flag", plannerCase: plannerCase{bind: 1, method: 3, keyLit: "float"}, want: []string{rewrite}},
	{name: "report-only rule", plannerCase: plannerCase{bind: 1, method: 4, keyLit: "int"}, want: []string{"type-mismatch"}},
	{name: "bulk rule", plannerCase: plannerCase{bind: 1, method: 5, keyLit: "bulk"}, want: []string{"bulk"}},
	{name: "dflt parameter default still rewrites", plannerCase: plannerCase{bind: 1, badDefault: true, keyLit: "bad-default"}, want: []string{rewrite}},
	{name: "rewrite next to a context skip", plannerCase: plannerCase{bind: 1, two: true, keyLit: "two-calls"}, want: []string{rewrite, ctxUnpr}},
	{name: "context shadowed, Ctx method rewrites next to a skip", plannerCase: plannerCase{bind: 1, two: true, method: 1, shadow: 2, keyLit: "two-shadow-local"}, want: []string{rewrite, ctxUnpr}},
	{name: "two calls, Ctx method, context shadowed", plannerCase: plannerCase{bind: 1, two: true, method: 1, shadow: 1, keyLit: "two-shadow"}, want: []string{rewrite, ctxUnpr}},
	{name: "non-ASCII key", plannerCase: plannerCase{bind: 1, keyLit: "café \U0001F600"}, want: []string{rewrite}},
	{name: "key needing escapes", plannerCase: plannerCase{bind: 1, keyLit: "a\x00b\"c\\d\n"}, want: []string{rewrite}},
	{name: "key that is not valid UTF-8", plannerCase: plannerCase{bind: 1, keyLit: "bad-\xff-utf8"}, want: []string{rewrite}},
}

// FuzzPlanner is the planner/analyzer property fuzzer (issue #21, part 1).
func FuzzPlanner(f *testing.F) {
	// The initial corpus is testdata/fuzz/FuzzPlanner (kept small: every
	// input costs several package loads); TestPlannerSeeds holds the full
	// seed table with expected outcomes.
	f.Fuzz(func(t *testing.T, recv, key, ctx, bind, shadow, method, flags uint8, keyLit string) {
		t.Parallel()
		checkPlanner(t, decodePlanner(recv, key, ctx, bind, shadow, method, flags, keyLit))
	})
}

// TestPlannerSeeds runs every seed and pins the outcome the analyzer must
// reach for it, so a seed cannot silently stop covering the path it names.
func TestPlannerSeeds(t *testing.T) {
	t.Parallel()
	for _, s := range plannerSeeds {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			if got := checkPlanner(t, s.plannerCase); !slices.Equal(got, s.want) {
				t.Errorf("outcomes = %q, want %q\n%s", got, s.want, s.source())
			}
		})
	}
}

// checkPlanner asserts the planner properties for one generated file and
// returns the outcome of each finding in source order.
func checkPlanner(t *testing.T, p plannerCase) []string {
	t.Helper()
	text := p.source()
	src := []byte(text)
	if _, err := parser.ParseFile(token.NewFileSet(), "input.go", src, parser.ParseComments); err != nil {
		t.Fatalf("generated source does not parse (generator bug): %v\n%s", err, src)
	}

	// runSource fails the test unless the generated input type-checks against
	// the stubs, then runs the analyzer: a panic or analysis error is a crash.
	got, offset := runSource(t, src, ".", false)

	var fixes int
	var outcomes []string
	for _, f := range got {
		switch {
		case f.hasFix && f.skip != "":
			t.Errorf("%d:%d: skip %q carries a fix", f.line, f.col, f.skip)
		case f.hasFix:
			fixes++
			outcomes = append(outcomes, rewrite)
		default:
			outcomes = append(outcomes, f.skip)
		}
	}

	// Rewrite to a fixed point: every step parses and compiles, and converge
	// only returns after a run that offers no fix (so the result is
	// idempotent). With no fix on offer the file is unchanged by definition.
	final := src
	if fixes > 0 {
		final = convergeFrom(t, src, ".", got, offset)
	}

	// A skipped call is byte-identical in the output, even when another call
	// in the same file is rewritten: its exact text (the diagnostic's range)
	// survives.
	for _, f := range got {
		if f.skip == "" {
			continue
		}
		if call := src[f.start:f.end]; !bytes.Contains(final, call) {
			t.Errorf("skipped call at %d:%d was altered: %q missing from\n%s", f.line, f.col, call, final)
		}
	}
	if fixes == 0 && !bytes.Equal(final, src) {
		t.Errorf("only skips, but the file changed")
	}
	return outcomes
}
