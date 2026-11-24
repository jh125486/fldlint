package planner_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/fix"
	"github.com/jh125486/fldlint/go/internal/planner"
)

func call(t *testing.T) *ast.CallExpr { return callAt(t, 0) }

// callAt parses the call at a distinct source offset, so calls built for one
// file never share a range (a nested or duplicate range is a different case).
func callAt(t *testing.T, n int) *ast.CallExpr {
	t.Helper()
	e, err := parser.ParseExpr(strings.Repeat(" ", n*100) + `ld.BoolVariation("k", ldcontext.New("u"), false)`)
	if err != nil {
		t.Fatal(err)
	}
	return e.(*ast.CallExpr)
}

func parse(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return fset, file
}

// idents returns every identifier named name, in source order.
func idents(file ast.Node, name string) []*ast.Ident {
	var out []*ast.Ident
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			out = append(out, id)
		}
		return true
	})
	return out
}

func good() planner.Facts {
	return planner.Facts{
		KeyStatic: true, Key: `"k"`,
		Default: "false", DefaultProven: true,
		ContextProven: true, EvalCtx: `openfeature.NewEvaluationContext("u", nil)`,
		GoCtx:       "context.TODO()",
		ClientBound: true, Client: "client",
		ReceiverPure: true,
		AddImports:   []string{"context"},
	}
}

func TestPlan(t *testing.T) {
	t.Parallel()
	with := func(mut func(*planner.Facts)) planner.Facts {
		f := good()
		mut(&f)
		return f
	}
	tests := []struct {
		name         string
		method       string
		facts        planner.Facts
		wantKind     planner.Kind
		wantCategory string
		wantRule     string
		wantText     string
		wantMessage  string
	}{
		{
			name: "rewrite", method: "BoolVariation", facts: good(),
			wantKind: planner.KindRewrite, wantRule: "bool-variation",
			wantText:    `client.BooleanValue(context.TODO(), "k", false, openfeature.NewEvaluationContext("u", nil))`,
			wantMessage: `Migrate boolean flag "k" to OpenFeature`,
		},
		{
			name: "rewrite Ctx variant", method: "BoolVariationCtx", facts: with(func(f *planner.Facts) { f.GoCtx = "r.Context()" }),
			wantKind: planner.KindRewrite, wantRule: "bool-variation",
			wantText: `client.BooleanValue(r.Context(), "k", false, openfeature.NewEvaluationContext("u", nil))`,
		},
		{name: "unknown method", method: "Nope", facts: good(), wantKind: planner.KindSkip, wantCategory: "unbound-client"},
		{name: "detail is a rule-level skip", method: "BoolVariationDetail", facts: good(), wantKind: planner.KindSkip, wantCategory: "detail", wantRule: "bool-variation-detail"},
		{name: "bulk is a rule-level skip", method: "AllFlagsState", facts: good(), wantKind: planner.KindSkip, wantCategory: "bulk", wantRule: "all-flags-state"},
		{name: "int is a type mismatch", method: "IntVariation", facts: good(), wantKind: planner.KindSkip, wantCategory: "type-mismatch", wantRule: "int-variation"},
		{name: "json is a type mismatch", method: "JSONVariation", facts: good(), wantKind: planner.KindSkip, wantCategory: "type-mismatch", wantRule: "go-json-variation"},
		{name: "migration is a type mismatch", method: "MigrationVariation", facts: good(), wantKind: planner.KindSkip, wantCategory: "type-mismatch", wantRule: "migration-variation"},
		{
			name: "dynamic key", method: "BoolVariation", facts: with(func(f *planner.Facts) { f.KeyStatic = false }),
			wantKind: planner.KindSkip, wantCategory: "dynamic-key", wantRule: "bool-variation",
		},
		{
			name: "context unproven", method: "BoolVariation", facts: with(func(f *planner.Facts) { f.ContextProven = false }),
			wantKind: planner.KindSkip, wantCategory: "context-unproven", wantRule: "bool-variation",
		},
		{
			name: "unbound client", method: "BoolVariation", facts: with(func(f *planner.Facts) { f.ClientBound = false }),
			wantKind: planner.KindSkip, wantCategory: "unbound-client", wantRule: "bool-variation",
		},
		{
			name: "default type unproven", method: "BoolVariation", facts: with(func(f *planner.Facts) { f.DefaultProven = false }),
			wantKind: planner.KindSkip, wantCategory: "default-type-unproven", wantRule: "bool-variation",
		},
		{
			name: "dynamic key wins over everything else", method: "BoolVariation",
			facts:    planner.Facts{},
			wantKind: planner.KindSkip, wantCategory: "dynamic-key", wantRule: "bool-variation",
		},
		{
			name: "context-unproven wins over unbound client", method: "BoolVariation",
			facts:    planner.Facts{KeyStatic: true, Key: `"k"`},
			wantKind: planner.KindSkip, wantCategory: "context-unproven", wantRule: "bool-variation",
		},
		{
			name: "default type wins over unbound client", method: "BoolVariation",
			facts:    planner.Facts{KeyStatic: true, Key: `"k"`, ContextProven: true},
			wantKind: planner.KindSkip, wantCategory: "default-type-unproven", wantRule: "bool-variation",
		},
		{
			name: "unbound client wins over receiver", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.ClientBound = false; f.ReceiverPure = false }),
			wantKind: planner.KindSkip, wantCategory: "unbound-client", wantRule: "bool-variation",
		},
		{
			name: "impure receiver", method: "BoolVariation", facts: with(func(f *planner.Facts) { f.ReceiverPure = false }),
			wantKind: planner.KindSkip, wantCategory: "receiver-unproven", wantRule: "bool-variation",
		},
		{
			name: "an empty evaluation context is an error, not a hole", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.EvalCtx = "" }),
			wantKind: planner.KindSkip, wantCategory: "context-unproven", wantRule: "bool-variation",
		},
		{
			name: "an empty go context is an error", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.GoCtx = "" }),
			wantKind: planner.KindSkip, wantCategory: "context-unproven", wantRule: "bool-variation",
		},
		{
			name: "an empty default is an error", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.Default = "" }),
			wantKind: planner.KindSkip, wantCategory: "default-type-unproven", wantRule: "bool-variation",
		},
		{
			name: "an empty client is an error", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.Client = "" }),
			wantKind: planner.KindSkip, wantCategory: "unbound-client", wantRule: "bool-variation",
		},
		{
			name: "a comment on the context moves with it", method: "BoolVariation",
			facts: with(func(f *planner.Facts) {
				f.Notes = &fix.Notes{Args: map[string]*fix.ArgNotes{"evalctx": {Lead: []fix.Comment{{Text: "/* who */"}}}}}
			}),
			wantKind: planner.KindRewrite, wantRule: "bool-variation",
			wantText: `client.BooleanValue(context.TODO(), "k", false, /* who */ openfeature.NewEvaluationContext("u", nil))`,
		},
		{
			name: "a comment with no home reports the call without a fix", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.Notes = &fix.Notes{Unhomed: true} }),
			wantKind: planner.KindUnfixable, wantRule: "bool-variation",
			wantMessage: `Migrate boolean flag "k" to OpenFeature`,
		},
		{
			name: "a comment bound to an argument the template lacks has no home", method: "BoolVariation",
			facts: with(func(f *planner.Facts) {
				f.Notes = &fix.Notes{Args: map[string]*fix.ArgNotes{"nope": {Lead: []fix.Comment{{Text: "/* x */"}}}}}
			}),
			wantKind: planner.KindUnfixable, wantRule: "bool-variation",
		},
		{
			name: "a skip still wins over a comment with no home", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.KeyStatic = false; f.Notes = &fix.Notes{Unhomed: true} }),
			wantKind: planner.KindSkip, wantCategory: "dynamic-key", wantRule: "bool-variation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := planner.Plan(planner.CallSite{Call: call(t), Method: tt.method, Facts: tt.facts})
			if got.Kind != tt.wantKind || got.Category != tt.wantCategory || got.RuleID != tt.wantRule {
				t.Fatalf("got kind=%s category=%q rule=%q, want kind=%s category=%q rule=%q",
					got.Kind, got.Category, got.RuleID, tt.wantKind, tt.wantCategory, tt.wantRule)
			}
			if tt.wantKind == planner.KindSkip {
				if got.Edit != nil || got.Message == "" && tt.wantRule != "" {
					t.Fatalf("skip must have no edit and a message: %+v", got)
				}
				return
			}
			if tt.wantKind == planner.KindUnfixable {
				if got.Edit != nil || got.Imports != nil || got.Category != "" {
					t.Fatalf("unfixable must carry no edit, imports or category: %+v", got)
				}
				if tt.wantMessage != "" && got.Message != tt.wantMessage {
					t.Fatalf("message = %q, want %q", got.Message, tt.wantMessage)
				}
				return
			}
			if got.Edit == nil || string(got.Edit.NewText) != tt.wantText {
				t.Fatalf("edit = %+v, want text %q", got.Edit, tt.wantText)
			}
			if tt.wantMessage != "" && got.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", got.Message, tt.wantMessage)
			}
		})
	}
}

func TestPlanFile(t *testing.T) {
	t.Parallel()
	const importSrc = `package p

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"
)

func f() {
	fmt.Println(ldcontext.New("a"), ldcontext.New("b"), openfeature.NewClient("x"))
}
`
	const localSrc = `package p

func f() {
	ld := mk()
	ld.A()
	ld.B()
	ld.Close()
}

func g() {
	ld := mk()
	ld.A()
	ld.B()
}

func h() {
	ld := mk()
	ld.A()
}
`
	type want struct {
		kind     planner.Kind
		category string
		imports  bool
		nested   bool
	}
	rewrite := want{kind: planner.KindRewrite}
	rewriteImport := want{kind: planner.KindRewrite, imports: true}
	skipped := func(cat string) want { return want{kind: planner.KindSkip, category: cat} }

	// importCase builds one rewrite site per entry of consumed, each consuming
	// the ldcontext uses it lists; a nil entry is a dynamic-key skip.
	importCase := func(consumed ...[]int) func(*testing.T) (planner.FileInfo, []planner.CallSite) {
		return func(t *testing.T) (planner.FileInfo, []planner.CallSite) {
			fset, file := parse(t, importSrc)
			uses := idents(file, "ldcontext")
			var ldspec *ast.ImportSpec
			for _, s := range file.Imports {
				if strings.HasSuffix(s.Path.Value, `ldcontext"`) {
					ldspec = s
				}
			}
			var sites []planner.CallSite
			for _, c := range consumed {
				facts := good()
				facts.AddImports = nil
				for _, i := range c {
					facts.Consumed = append(facts.Consumed, uses[i])
				}
				if c == nil {
					facts.KeyStatic = false
				}
				sites = append(sites, planner.CallSite{Call: callAt(t, len(sites)), Method: "BoolVariation", Facts: facts})
			}
			return planner.FileInfo{Fset: fset, File: file, Src: []byte(importSrc), Removable: map[*ast.ImportSpec][]*ast.Ident{ldspec: uses}}, sites
		}
	}
	// localCase builds sites on the local `ld` of function fn. Each entry of
	// roots names which use of ld (0-based, after the declaration) is that
	// site's receiver root; a true entry of dynamic makes the site a
	// dynamic-key skip. The local's uses are all its references.
	localCase := func(fn string, roots []int, dynamic ...bool) func(*testing.T) (planner.FileInfo, []planner.CallSite) {
		return func(t *testing.T) (planner.FileInfo, []planner.CallSite) {
			fset, file := parse(t, localSrc)
			var uses []*ast.Ident
			for _, d := range file.Decls {
				if fd := d.(*ast.FuncDecl); fd.Name.Name == fn {
					uses = idents(fd, "ld")[1:]
				}
			}
			local := &planner.Local{Uses: uses}
			var sites []planner.CallSite
			for i, r := range roots {
				facts := good()
				facts.AddImports = nil
				facts.ReceiverRoot, facts.Receiver = uses[r], local
				if i < len(dynamic) && dynamic[i] {
					facts.KeyStatic = false
				}
				sites = append(sites, planner.CallSite{Call: callAt(t, len(sites)), Method: "BoolVariation", Facts: facts})
			}
			return planner.FileInfo{Fset: fset, File: file, Src: []byte(localSrc)}, sites
		}
	}
	// nestedCase builds two rewrite sites where the first call contains the
	// second (`a(b())`), in the given order.
	nestedCase := func(innerFirst bool) func(*testing.T) (planner.FileInfo, []planner.CallSite) {
		return func(t *testing.T) (planner.FileInfo, []planner.CallSite) {
			e, err := parser.ParseExpr(`a(b())`)
			if err != nil {
				t.Fatal(err)
			}
			outer := e.(*ast.CallExpr)
			inner := outer.Args[0].(*ast.CallExpr)
			sites := []planner.CallSite{
				{Call: outer, Method: "BoolVariation", Facts: good()},
				{Call: inner, Method: "BoolVariation", Facts: good()},
			}
			if innerFirst {
				sites[0], sites[1] = sites[1], sites[0]
			}
			fset, file := parse(t, importSrc)
			return planner.FileInfo{Fset: fset, File: file, Src: []byte(importSrc)}, sites
		}
	}
	tests := []struct {
		name  string
		setup func(*testing.T) (planner.FileInfo, []planner.CallSite)
		want  []want
	}{
		{name: "a fix that consumes every use removes the import", setup: importCase([]int{0, 1}), want: []want{rewriteImport}},
		{name: "a remaining use keeps the import", setup: importCase([]int{0}), want: []want{rewrite}},
		{name: "fixes that share the import each leave it to the driver", setup: importCase([]int{0}, []int{1}), want: []want{rewrite, rewrite}},
		{name: "skipped sites consume nothing", setup: importCase(nil), want: []want{skipped("dynamic-key")}},
		{
			name:  "a local whose only use is the rewritten call is stranded",
			setup: localCase("h", []int{0}),
			want:  []want{skipped("receiver-unproven")},
		},
		{
			name:  "every rewritable call on a stranded local is skipped, never a subset",
			setup: localCase("g", []int{0, 1}),
			want:  []want{skipped("receiver-unproven"), skipped("receiver-unproven")},
		},
		{
			name:  "another use of the local keeps the calls rewritable",
			setup: localCase("f", []int{0, 1}),
			want:  []want{rewrite, rewrite},
		},
		{
			name:  "of nested rewrites only the inner call keeps its fix",
			setup: nestedCase(false),
			want:  []want{{kind: planner.KindRewrite, nested: true}, {kind: planner.KindRewrite, imports: true}},
		},
		{
			name:  "nesting does not depend on source order of the sites",
			setup: nestedCase(true),
			want:  []want{{kind: planner.KindRewrite, imports: true}, {kind: planner.KindRewrite, nested: true}},
		},
		{
			// local-client-survivor-skip: the dynamic-key call keeps the local
			// used, so the first call is rewritten and the second is skipped
			// for its own reason.
			name:  "a skipped call is a surviving use",
			setup: localCase("g", []int{0, 1}, false, true),
			want:  []want{rewrite, skipped("dynamic-key")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fi, sites := tt.setup(t)
			res := planner.PlanFile(fi, sites)
			if len(res) != len(tt.want) {
				t.Fatalf("got %d results, want %d", len(res), len(tt.want))
			}
			for i, w := range tt.want {
				if res[i].Kind != w.kind || res[i].Category != w.category {
					t.Errorf("site %d: got kind=%s category=%q, want kind=%s category=%q", i, res[i].Kind, res[i].Category, w.kind, w.category)
				}
				if res[i].Nested != w.nested {
					t.Errorf("site %d: nested = %v, want %v", i, res[i].Nested, w.nested)
				}
				if got := len(res[i].Imports) > 0; got != w.imports {
					t.Errorf("site %d: import edits present = %v, want %v (%+v)", i, got, w.imports, res[i])
				}
				if w.kind == planner.KindSkip && (res[i].Edit != nil || res[i].Message == "") {
					t.Errorf("site %d: a skip has a message and no edit: %+v", i, res[i])
				}
			}
		})
	}
}
