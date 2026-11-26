package conformance_test

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
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

func (p plannerCase) flags() uint8 {
	var f uint8
	if p.badDefault {
		f |= 1
	}
	if p.two {
		f |= 2
	}
	if p.stranded {
		f |= 4
	}
	return f
}

// plannerSeeds covers every value of every dimension at least once, plus the
// interesting pairs (shadowed `context` with a rewritable call, ambiguous
// clients, impure receivers) and a few awkward literal keys.
var plannerSeeds = []plannerCase{
	{stranded: true, bind: 1, keyLit: "stranded"},                    // local receiver only used by the call
	{bind: 1, keyLit: "basic"},                                       // local receiver, literal key, rewrite
	{recv: 1, bind: 1, keyLit: "pkg-recv"},                           // package var receiver + package client
	{recv: 2, bind: 2, ctx: 1, keyLit: "field"},                      // struct field receiver, local client
	{recv: 3, bind: 1, keyLit: "impure"},                             // call-result receiver
	{recv: 4, bind: 1, ctx: 3, keyLit: "param"},                      // parameter receiver, ldcontext param
	{key: 1, bind: 1, keyLit: ""},                                    // const key identifier
	{key: 2, bind: 1, keyLit: ""},                                    // raw string key
	{key: 3, bind: 1, keyLit: ""},                                    // concatenated key
	{key: 4, bind: 1, keyLit: ""},                                    // dynamic key
	{key: 5, bind: 1, keyLit: ""},                                    // call key
	{ctx: 2, bind: 1, keyLit: "builder"},                             // unproven context
	{ctx: 4, bind: 1, keyLit: "kind"},                                // non-user kind
	{ctx: 5, bind: 1, keyLit: "concat-ctx"},                          // computed context key
	{bind: 0, keyLit: "no-client"},                                   // no OpenFeature client
	{bind: 2, keyLit: "local-client"},                                // function-local client
	{bind: 3, keyLit: "shadowed-client"},                             // `client` shadowed by an int
	{bind: 4, keyLit: "two-clients"},                                 // two package clients
	{bind: 1, shadow: 1, keyLit: "ctx-pkg-shadowed"},                 // `context` is a string param
	{bind: 1, shadow: 2, keyLit: "ctx-local-shadowed"},               // `context` is a local
	{bind: 1, shadow: 3, keyLit: "go-ctx-param"},                     // ctx param, plain method
	{bind: 1, shadow: 4, keyLit: "ctx-named-context"},                // param `context context.Context`
	{bind: 1, shadow: 1, method: 1, keyLit: "ctx-method-shadowed"},   // Ctx method, `context` shadowed
	{bind: 1, method: 1, keyLit: "ctx-method"},                       // Ctx method
	{bind: 1, method: 2, keyLit: "string"},                           // string flag
	{bind: 1, method: 3, keyLit: "float"},                            // number flag
	{bind: 1, method: 4, keyLit: "int"},                              // report-only rule
	{bind: 1, method: 5, keyLit: "bulk"},                             // bulk rule, no key
	{bind: 1, badDefault: true, keyLit: "bad-default"},               // default type unproven
	{bind: 1, two: true, keyLit: "two-calls"},                        // a skip next to a rewrite
	{bind: 1, two: true, method: 2, shadow: 1, keyLit: "two-shadow"}, // two calls, shadowed context
	{bind: 1, keyLit: "café \U0001F600"},                             // non-ASCII key
	{bind: 1, keyLit: "a\x00b\"c\\d\n"},                              // key needing escapes
	{bind: 1, keyLit: "bad-\xff-utf8"},                               // key that is not valid UTF-8
}

// FuzzPlanner is the planner/analyzer property fuzzer (issue #21, part 1).
func FuzzPlanner(f *testing.F) {
	for _, s := range plannerSeeds {
		f.Add(s.recv, s.key, s.ctx, s.bind, s.shadow, s.method, s.flags(), s.keyLit)
	}
	f.Fuzz(func(t *testing.T, recv, key, ctx, bind, shadow, method, flags uint8, keyLit string) {
		checkPlanner(t, plannerCase{
			recv: recv, key: key, ctx: ctx, bind: bind, shadow: shadow, method: method,
			badDefault: flags&1 != 0, two: flags&2 != 0, stranded: flags&4 != 0, keyLit: keyLit,
		})
	})
}

// checkPlanner asserts the planner properties for one generated file.
func checkPlanner(t *testing.T, p plannerCase) {
	t.Helper()
	text := p.source()
	src := []byte(text)
	if _, err := parser.ParseFile(token.NewFileSet(), "input.go", src, parser.ParseComments); err != nil {
		t.Fatalf("generated source does not parse (generator bug): %v\n%s", err, src)
	}

	// runSource fails the test unless the generated input type-checks against
	// the stubs, then runs the analyzer: a panic or analysis error is a crash.
	got, _ := runSource(t, src, ".", false)

	var fixes int
	for _, f := range got {
		if f.skip != "" && f.hasFix {
			t.Errorf("%d:%d: skip %q carries a fix", f.line, f.col, f.skip)
		}
		if f.hasFix {
			fixes++
		}
	}

	// Rewrite to a fixed point: every step parses and compiles (converge).
	// With no fix on offer the file is, by definition, unchanged: check that
	// no edits are attached either.
	final := src
	if fixes == 0 {
		for _, f := range got {
			if len(f.edits) != 0 {
				t.Errorf("%d:%d: no fix offered but %d edits attached", f.line, f.col, len(f.edits))
			}
		}
	} else {
		final = converge(t, src, ".")
	}

	// A skipped call's line survives verbatim, even when another call in the
	// same file is rewritten.
	lines := strings.Split(text, "\n")
	for _, f := range got {
		if f.skip == "" {
			continue
		}
		if line := lines[f.line-1]; !bytes.Contains(final, []byte(line)) {
			t.Errorf("skipped call at %d:%d was altered: line %q missing from\n%s", f.line, f.col, line, final)
		}
	}

	// Idempotent: the converged file offers no fix (nothing left to rewrite,
	// so a further run changes no byte). Skipped-only files are their own
	// fixed point.
	if fixes > 0 {
		again, _ := runSource(t, final, ".", false)
		for _, f := range again {
			if f.hasFix {
				t.Errorf("converged file still offers a fix at %d:%d (%s)\n%s", f.line, f.col, f.ruleID, final)
			}
		}
	}
}
