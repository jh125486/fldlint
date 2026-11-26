package conformance_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/detect"
	"github.com/jh125486/fldlint/go/internal/fingerprint"
	"github.com/jh125486/fldlint/go/internal/planner"
	"github.com/jh125486/fldlint/go/internal/position"
	"github.com/jh125486/fldlint/go/internal/spec"
)

// Metamorphic tests (issue #21, part 1): over every fixture, a change that
// does not touch the calls must not change what is found. The multiset of
// (ruleId, skip, key) and the fingerprints stay equal under
//
//   - blank lines and comments added elsewhere in the file (fingerprint.md,
//     "Stability");
//   - a consistent rename of the LaunchDarkly client variable.
//
// Positions may change and are not compared.

// identity is one finding as the baseline sees it.
type identity struct {
	ruleID, skip string
	key          *string
	line, col    int
}

func (i identity) String() string {
	k := "<none>"
	if i.key != nil {
		k = fmt.Sprintf("%q", *i.key)
	}
	return fmt.Sprintf("%s skip=%q key=%s", i.ruleID, i.skip, k)
}

// identities runs detection and planning exactly as the analyzer does and
// returns each finding with its key (the engine's fingerprint input).
func identities(t *testing.T, src []byte, caseDir string, tolerate bool) []identity {
	t.Helper()
	pkg := newWorkspace(t, "input.go", src, caseDir).load(t)
	if !tolerate && len(pkg.Errors) > 0 {
		t.Fatalf("source does not type-check against the SDK stubs: %v\n%s", pkg.Errors, src)
	}
	env := &detect.Env{Fset: pkg.Fset, Pkg: pkg.Types, Info: pkg.TypesInfo, Files: pkg.Syntax}
	var out []identity
	for _, f := range pkg.Syntax {
		df := detect.File{AST: f, Src: src}
		var sites []planner.CallSite
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if method, ok := detect.Method(pkg.TypesInfo, call); ok {
					sites = append(sites, env.Site(df, call, method))
				}
			}
			return true
		})
		fi := planner.FileInfo{Fset: pkg.Fset, File: f, Src: src, Removable: env.Removable(df)}
		for i, res := range planner.PlanFile(fi, sites) {
			p := pkg.Fset.Position(sites[i].Call.Pos())
			id := identity{ruleID: res.RuleID, key: sites[i].Facts.KeyValue, line: p.Line, col: position.UTF16Col(src, p.Offset)}
			switch res.Kind {
			case planner.KindSkip:
				id.skip = res.Category
			case planner.KindUnfixable:
				id.skip = "no-fix"
			}
			out = append(out, id)
		}
	}
	return out
}

// summary returns the sorted identity multiset and the sorted fingerprints.
func summary(ids []identity) (multiset, fingerprints []string) {
	pos := make([]fingerprint.Positioned, len(ids))
	for i, id := range ids {
		pos[i] = fingerprint.Positioned{RuleID: id.ruleID, Key: id.key, Line: id.line, Col: id.col}
	}
	occ := fingerprint.AssignOccurrences(pos)
	for i, id := range ids {
		multiset = append(multiset, id.String())
		fingerprints = append(fingerprints, fingerprint.Fingerprint(fingerprint.Input{RuleID: id.ruleID, Path: "input.go", Key: id.key, Occ: occ[i]}))
	}
	sort.Strings(multiset)
	sort.Strings(fingerprints)
	return multiset, fingerprints
}

// withNoise adds blank lines and comments before every top-level declaration
// and at the end of the file, never inside a call.
func withNoise(t *testing.T, src []byte, _ string) ([]byte, bool) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "input.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("fixture does not parse: %v", err)
	}
	const noise = "// metamorphic noise\n\n\n/* more noise */\n\n"
	var at []int
	for _, d := range f.Decls {
		pos := d.Pos()
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Doc != nil {
				pos = d.Doc.Pos()
			}
		case *ast.FuncDecl:
			if d.Doc != nil {
				pos = d.Doc.Pos()
			}
		}
		off := fset.Position(pos).Offset
		at = append(at, bytes.LastIndexByte(src[:off], '\n')+1)
	}
	slices.Sort(at)
	at = slices.Compact(at)
	var out []byte
	last := 0
	for _, o := range at {
		out = append(out, src[last:o]...)
		out = append(out, noise...)
		last = o
	}
	out = append(out, src[last:]...)
	out = append(out, "\n\n// trailing noise\n"...)
	return out, true
}

// withRenamedClient renames every LD client variable (or struct field) that
// an SDK call is made on, consistently, to a name no fixture uses. ok is false
// when the file has no such variable.
func withRenamedClient(t *testing.T, src []byte, caseDir string) ([]byte, bool) {
	t.Helper()
	pkg := newWorkspace(t, "input.go", src, caseDir).load(t)
	info := pkg.TypesInfo
	names := map[types.Object]string{}
	for _, f := range pkg.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if _, ok := detect.Method(info, call); !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			var id *ast.Ident
			switch x := ast.Unparen(sel.X).(type) {
			case *ast.Ident:
				id = x
			case *ast.SelectorExpr:
				id = x.Sel
			}
			if id == nil {
				return true
			}
			if v, ok := info.Uses[id].(*types.Var); ok && !v.Embedded() && v.Pkg() == pkg.Types {
				if _, seen := names[v]; !seen {
					names[v] = fmt.Sprintf("ldRenamed%d", len(names))
				}
			}
			return true
		})
	}
	if len(names) == 0 {
		return nil, false
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	collect := func(m map[*ast.Ident]types.Object) {
		for id, obj := range m {
			if name, ok := names[obj]; ok {
				off := pkg.Fset.Position(id.Pos()).Offset
				edits = append(edits, edit{off, off + len(id.Name), name})
			}
		}
	}
	collect(info.Defs)
	collect(info.Uses)
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := slices.Clone(src)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	return out, true
}

func TestMetamorphic(t *testing.T) {
	t.Parallel()
	specJSON, fixtures, err := locate(*specRoot, specExplicit())
	if err != nil {
		t.Fatal(err)
	}
	if specJSON == stubSpecPath {
		t.Skip("stub spec: the M0 stub fixtures are not backed by the real engine")
	}
	sp, err := spec.LoadSpec(specJSON)
	if err != nil {
		t.Fatal(err)
	}
	transforms := []struct {
		name string
		fn   func(*testing.T, []byte, string) ([]byte, bool)
	}{
		{"blank lines and comments elsewhere", withNoise},
		{"renamed LD client variable", withRenamedClient},
	}
	for _, lc := range sp.Cases {
		want, ok := lc.Case.Expect["go"]
		if !ok {
			continue
		}
		dir := lc.Dir(fixtures)
		tolerate := slices.ContainsFunc(want.Diagnostics, func(d spec.Diagnostic) bool { return d.Skip == "default-type-unproven" })
		t.Run(lc.Path, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(filepath.Join(dir, "go", "input.go"))
			if err != nil {
				t.Fatal(err)
			}
			wantSet, wantFP := summary(identities(t, src, dir, tolerate))
			for _, tr := range transforms {
				t.Run(tr.name, func(t *testing.T) {
					t.Parallel()
					changed, ok := tr.fn(t, src, dir)
					if !ok {
						t.Skip("transformation does not apply to this fixture")
					}
					if bytes.Equal(changed, src) {
						t.Fatal("transformation changed nothing")
					}
					gotSet, gotFP := summary(identities(t, changed, dir, tolerate))
					if !slices.Equal(gotSet, wantSet) {
						t.Errorf("(ruleId, skip, key) multiset changed\n got: %s\nwant: %s\n--- transformed source ---\n%s",
							strings.Join(gotSet, "\n      "), strings.Join(wantSet, "\n      "), changed)
					}
					if !slices.Equal(gotFP, wantFP) {
						t.Errorf("fingerprints changed\n got: %v\nwant: %v", gotFP, wantFP)
					}
				})
			}
		})
	}
}
