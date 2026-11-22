package analyzer_test

import (
	"bytes"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/jh125486/fldlint/go/analyzer"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()
	// withErrors runs the analyzer on a package that deliberately does not
	// type-check against the real SDK signatures.
	withErrors := *analyzer.Analyzer
	withErrors.RunDespiteErrors = true

	tests := []struct {
		name string
		a    *analysis.Analyzer
		pkg  string
		// fixes reports whether the package has .golden files to compare.
		fixes bool
	}{
		{name: "add context import and remove ldcontext", a: analyzer.Analyzer, pkg: "addctx", fixes: true},
		{name: "keep ldcontext when still used", a: analyzer.Analyzer, pkg: "keepldctx", fixes: true},
		{name: "two rewrites share one import edit set", a: analyzer.Analyzer, pkg: "twocalls", fixes: true},
		{name: "goctx resolution", a: analyzer.Analyzer, pkg: "goctx", fixes: true},
		{name: "openfeature import added from another file's client", a: analyzer.Analyzer, pkg: "twofiles", fixes: true},
		{name: "import aliases", a: analyzer.Analyzer, pkg: "aliases", fixes: true},
		{name: "ldcontext alone in its group, openfeature client in another file", a: analyzer.Analyzer, pkg: "overlap", fixes: true},
		{name: "lone ldcontext import decl", a: analyzer.Analyzer, pkg: "loneimport", fixes: true},
		{name: "parenthesized decl whose only spec is ldcontext", a: analyzer.Analyzer, pkg: "parenonly", fixes: true},
		{name: "receiver stranding and purity", a: analyzer.Analyzer, pkg: "stranded"},
		{name: "shadowed openfeature name", a: analyzer.Analyzer, pkg: "shadowedof"},
		{name: "skip categories", a: analyzer.Analyzer, pkg: "skips"},
		{name: "client identity and binding", a: analyzer.Analyzer, pkg: "clients"},
		{name: "default type unproven", a: &withErrors, pkg: "defaulttype"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.fixes {
				analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), tt.a, tt.pkg)
				return
			}
			analysistest.Run(t, analysistest.TestData(), tt.a, tt.pkg)
		})
	}
}

func TestAnalyzerMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  func() any
		want any
	}{
		{name: "name", got: func() any { return analyzer.Analyzer.Name }, want: "fldlint"},
		{name: "valid", got: func() any { return analysis.Validate([]*analysis.Analyzer{analyzer.Analyzer}) }, want: error(nil)},
		{name: "requires inspect", got: func() any { return len(analyzer.Analyzer.Requires) }, want: 1},
		{name: "documented", got: func() any { return analyzer.Analyzer.Doc != "" }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.got(); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAnalyzer_SuggestedFixes checks the raw edits of each fix, which
// RunWithSuggestedFixes cannot: it formats the result with the driver's
// remove-unused-imports pass, so an import removal that is missing (or
// present when it must not be) cannot fail a golden.
func TestAnalyzer_SuggestedFixes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pkg  string
		// removesLDContext has one entry per diagnostic that carries a fix, in
		// report order: does that fix delete the ldcontext import?
		removesLDContext []bool
		// noFix lists the count of diagnostics expected to carry no fix.
		noFix int
	}{
		{name: "a fix that consumes the last use removes the import", pkg: "addctx", removesLDContext: []bool{true}},
		{name: "fixes that share the import leave it to the driver", pkg: "twocalls", removesLDContext: []bool{false, false}},
		{name: "a remaining use keeps the import", pkg: "keepldctx", removesLDContext: []bool{false}},
		{name: "a lone import decl is removed", pkg: "loneimport", removesLDContext: []bool{true}},
		{name: "a parenthesized decl with one spec is removed", pkg: "parenonly", removesLDContext: []bool{true}},
		{name: "ldcontext alone in its group next to an added import", pkg: "overlap", removesLDContext: []bool{true}},
		{name: "a //line-adjusted position gets no fix", pkg: "lines", noFix: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, tt.pkg)
			var got []bool
			noFix := 0
			for _, res := range results {
				for _, d := range res.Diagnostics {
					if len(d.SuggestedFixes) == 0 {
						noFix++
						continue
					}
					fx := d.SuggestedFixes[0]
					removes := false
					edits := slices.Clone(fx.TextEdits)
					slices.SortFunc(edits, func(a, b analysis.TextEdit) int { return int(a.Pos) - int(b.Pos) })
					for i, e := range edits {
						if i > 0 && edits[i-1].End > e.Pos {
							t.Errorf("overlapping edits [%d,%d) and [%d,%d)", edits[i-1].Pos, edits[i-1].End, e.Pos, e.End)
						}
						name := res.Pass.Fset.File(e.Pos).Name()
						src, err := res.Pass.ReadFile(name)
						if err != nil {
							t.Fatal(err)
						}
						tf := res.Pass.Fset.File(e.Pos)
						old := src[tf.Offset(e.Pos):tf.Offset(e.End)]
						if bytes.Contains(old, []byte("go-sdk-common/v3/ldcontext\"")) && !bytes.Contains(e.NewText, []byte("ldcontext\"")) {
							removes = true
						}
					}
					got = append(got, removes)
				}
			}
			if !slices.Equal(got, tt.removesLDContext) || noFix != tt.noFix {
				t.Fatalf("removal per fix = %v (no-fix diagnostics %d), want %v (%d)", got, noFix, tt.removesLDContext, tt.noFix)
			}
		})
	}
}
