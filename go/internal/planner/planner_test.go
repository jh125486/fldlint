package planner_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/planner"
)

func call(t *testing.T) *ast.CallExpr {
	t.Helper()
	e, err := parser.ParseExpr(`ld.BoolVariation("k", ldcontext.New("u"), false)`)
	if err != nil {
		t.Fatal(err)
	}
	return e.(*ast.CallExpr)
}

func good() planner.Facts {
	return planner.Facts{
		KeyStatic: true, Key: `"k"`,
		Default: "false", DefaultProven: true,
		ContextProven: true, EvalCtx: `openfeature.NewEvaluationContext("u", nil)`,
		GoCtx:       "context.TODO()",
		ClientBound: true, Client: "client",
		AddImports: []string{"context"},
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
			name: "unbound client wins over default type", method: "BoolVariation",
			facts:    planner.Facts{KeyStatic: true, Key: `"k"`, ContextProven: true},
			wantKind: planner.KindSkip, wantCategory: "unbound-client", wantRule: "bool-variation",
		},
		{
			name: "an empty value still expands", method: "BoolVariation",
			facts:    with(func(f *planner.Facts) { f.EvalCtx = "" }),
			wantKind: planner.KindRewrite, wantRule: "bool-variation",
			wantText: `client.BooleanValue(context.TODO(), "k", false, )`,
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
	const src = `package p

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/open-feature/go-sdk/openfeature"
)

func f() {
	fmt.Println(ldcontext.New("a"), ldcontext.New("b"), openfeature.NewClient("x"))
}
`
	tests := []struct {
		name       string
		consumed   int // how many of the two ldcontext uses the rewrites consume
		rewrite    bool
		wantImport bool
	}{
		{name: "all uses consumed removes the import", consumed: 2, rewrite: true, wantImport: true},
		{name: "a remaining use keeps the import", consumed: 1, rewrite: true},
		{name: "skipped sites consume nothing", consumed: 2, rewrite: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			var uses []*ast.Ident
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "ldcontext" {
					uses = append(uses, id)
				}
				return true
			})
			var ldspec *ast.ImportSpec
			for _, s := range file.Imports {
				if strings.HasSuffix(s.Path.Value, `ldcontext"`) {
					ldspec = s
				}
			}
			facts := good()
			facts.AddImports = nil
			facts.Consumed = uses[:tt.consumed]
			if !tt.rewrite {
				facts.KeyStatic = false
			}
			e, _ := parser.ParseExpr(`ld.BoolVariation("k", ldcontext.New("u"), false)`)
			fi := planner.FileInfo{Fset: fset, File: file, Src: []byte(src), Removable: map[*ast.ImportSpec][]*ast.Ident{ldspec: uses}}
			// The call positions are not in this file; PlanFile only needs
			// them for the call edit, which this test does not inspect.
			res := planner.PlanFile(fi, []planner.CallSite{{Call: e.(*ast.CallExpr), Method: "BoolVariation", Facts: facts}})
			if len(res) != 1 {
				t.Fatalf("got %d results", len(res))
			}
			if got := len(res[0].Imports) > 0; got != tt.wantImport {
				t.Fatalf("import edits present = %v, want %v (%+v)", got, tt.wantImport, res[0])
			}
		})
	}
}
