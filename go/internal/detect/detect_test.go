package detect_test

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/jh125486/fldlint/go/internal/detect"
)

var probed = []string{"BoolVariation", "BoolVariationCtx", "IntVariation", "Close"}

// probe reports, for each call to a method with a probed name, either the
// recognised SDK method with its collected facts or "not an sdk call".
var probe = &analysis.Analyzer{
	Name:     "probe",
	Doc:      "reports detect results",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run: func(pass *analysis.Pass) (any, error) {
		env := &detect.Env{Fset: pass.Fset, Pkg: pass.Pkg, Info: pass.TypesInfo}
		files := map[string]detect.File{}
		for _, f := range pass.Files {
			name := pass.Fset.File(f.FileStart).Name()
			src, err := pass.ReadFile(name)
			if err != nil {
				return nil, err
			}
			files[name] = detect.File{AST: f, Src: src}
		}
		pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
			call := n.(*ast.CallExpr)
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !slices.Contains(probed, sel.Sel.Name) {
				return
			}
			method, ok := detect.Method(pass.TypesInfo, call)
			if !ok {
				pass.Reportf(call.Pos(), "not an sdk call")
				return
			}
			f := files[pass.Fset.File(call.Pos()).Name()]
			fa := env.Site(f, call, method).Facts
			pass.Reportf(call.Pos(), "method=%s static=%t key=%s ctx=%t eval=%s client=%s bound=%t default=%s typed=%t goctx=%s add=%v consumed=%d pure=%t local=%t",
				method, fa.KeyStatic, fa.Key, fa.ContextProven, fa.EvalCtx, fa.Client, fa.ClientBound, fa.Default, fa.DefaultProven, fa.GoCtx, fa.AddImports, len(fa.Consumed), fa.ReceiverPure, fa.Receiver != nil)
		})
		return nil, nil
	},
}

// keyProbe reports the fingerprint key (Facts.KeyValue) of every SDK call.
var keyProbe = &analysis.Analyzer{
	Name:     "keyprobe",
	Doc:      "reports the fingerprint key of every sdk call",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run: func(pass *analysis.Pass) (any, error) {
		env := &detect.Env{Fset: pass.Fset, Pkg: pass.Pkg, Info: pass.TypesInfo}
		files := map[string]detect.File{}
		for _, f := range pass.Files {
			name := pass.Fset.File(f.FileStart).Name()
			src, err := pass.ReadFile(name)
			if err != nil {
				return nil, err
			}
			files[name] = detect.File{AST: f, Src: src}
		}
		pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
			call := n.(*ast.CallExpr)
			method, ok := detect.Method(pass.TypesInfo, call)
			if !ok || method == "Close" {
				return
			}
			key := "absent"
			if kv := env.Site(files[pass.Fset.File(call.Pos()).Name()], call, method).Facts.KeyValue; kv != nil {
				key = strconv.Quote(*kv)
			}
			pass.Reportf(call.Pos(), "method=%s key=%s", method, key)
		})
		return nil, nil
	},
}

func testdata(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../../analyzer/testdata")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestMethod checks type-based identity: SDK methods are recognised, a
// same-named method on an unrelated type is not.
func TestMethod(t *testing.T) {
	t.Parallel()
	var got []string
	a := &analysis.Analyzer{
		Name:     "method",
		Doc:      "collects detect.Method results",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
				call := n.(*ast.CallExpr)
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && slices.Contains(probed, sel.Sel.Name) {
					m, ok := detect.Method(pass.TypesInfo, call)
					got = append(got, fmt.Sprintf("%s=%q,%t", sel.Sel.Name, m, ok))
				}
			})
			return nil, nil
		},
	}
	analysistest.Run(t, testdata(t), a, "detectplain")
	want := []string{
		`BoolVariation="BoolVariation",true`,
		`BoolVariationCtx="BoolVariationCtx",true`,
		`BoolVariation="BoolVariation",true`,
		`IntVariation="IntVariation",true`,
		`BoolVariation="",false`, // same name on an unrelated type
		`Close="",false`,
		`Close="",false`, // an SDK type, but not an evaluation method
		`BoolVariation="BoolVariation",true`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

// TestEnv_Site checks the collected facts (see the probe's want comments).
func TestEnv_Site(t *testing.T) {
	t.Parallel()
	analysistest.Run(t, testdata(t), probe, "detectprobe", "detectalias")
	// Report-only rules have a key too; the bulk rule and keys that are
	// dynamic or not valid UTF-8 have none.
	analysistest.Run(t, testdata(t), keyProbe, "detectkey")
}

// TestEnv_Removable checks that every reference to each named import is
// collected.
func TestEnv_Removable(t *testing.T) {
	t.Parallel()
	var got []string
	removable := &analysis.Analyzer{
		Name: "removable",
		Doc:  "collects ldcontext uses",
		Run: func(pass *analysis.Pass) (any, error) {
			env := &detect.Env{Fset: pass.Fset, Pkg: pass.Pkg, Info: pass.TypesInfo}
			for _, f := range pass.Files {
				for spc, ids := range env.Removable(detect.File{AST: f}) {
					got = append(got, fmt.Sprintf("%s:%d", spc.Path.Value, len(ids)))
				}
			}
			return nil, nil
		},
	}
	analysistest.Run(t, testdata(t), removable, "detectplain")
	slices.Sort(got) // map iteration order
	want := []string{
		`"context":1`,
		`"github.com/launchdarkly/go-sdk-common/v3/ldcontext":7`,
		`"github.com/launchdarkly/go-server-sdk/v7":1`,
		`"github.com/open-feature/go-sdk/openfeature":1`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
