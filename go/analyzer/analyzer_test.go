package analyzer_test

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/jh125486/fldlint/go/analyzer"
)

const ldctx = "github.com/launchdarkly/go-sdk-common/v3/ldcontext"

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
		// dir is the testdata root; empty means analysistest.TestData().
		dir string
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
		{name: "package-qualified receiver import is removed with its last use", a: analyzer.Analyzer, pkg: "pkgrecv", fixes: true},
		{name: "package-qualified receiver shared by two fixes", a: analyzer.Analyzer, pkg: "pkgrecvtwo", fixes: true},
		{name: "nested rewrites fix only the inner call", a: analyzer.Analyzer, pkg: "nested", fixes: true},
		{name: "a changed SDK signature is not a finding", a: analyzer.Analyzer, pkg: "forkuse", dir: "testdata/fork"},
		{name: "blank and dot openfeature imports are unbound", a: analyzer.Analyzer, pkg: "badof"},
		{name: "a shadowed or unusable context import is context-unproven", a: analyzer.Analyzer, pkg: "badctx"},
		{name: "skip categories", a: analyzer.Analyzer, pkg: "skips"},
		{name: "client identity and binding", a: analyzer.Analyzer, pkg: "clients"},
		{name: "default type unproven", a: &withErrors, pkg: "defaulttype"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := analysistest.TestData()
			if tt.dir != "" {
				abs, err := filepath.Abs(tt.dir)
				if err != nil {
					t.Fatal(err)
				}
				dir = abs
			}
			if tt.fixes {
				analysistest.RunWithSuggestedFixes(t, dir, tt.a, tt.pkg)
				return
			}
			analysistest.Run(t, dir, tt.a, tt.pkg)
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
//
// Rule 7 exception: analyzer.Analyzer is one exported value, not a function,
// and the three tests of this file check structurally different things
// (golden output, metadata, raw edits), so they stay separate.
func TestAnalyzer_SuggestedFixes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pkg  string
		// removes has one entry per diagnostic that carries a fix, in report
		// order: the comma-joined import paths that fix deletes ("" for none).
		removes []string
		// noFix is the number of diagnostics expected to carry no fix.
		noFix int
	}{
		{name: "a fix that consumes the last use removes the import", pkg: "addctx", removes: []string{ldctx}},
		{name: "fixes that share the import leave it to the driver", pkg: "twocalls", removes: []string{"", ""}},
		{name: "a remaining use keeps the import", pkg: "keepldctx", removes: []string{""}},
		{name: "a lone import decl is removed", pkg: "loneimport", removes: []string{ldctx}},
		{name: "a parenthesized decl with one spec is removed", pkg: "parenonly", removes: []string{ldctx}},
		{name: "ldcontext alone in its group next to an added import", pkg: "overlap", removes: []string{ldctx}},
		{name: "a //line-adjusted position gets no fix", pkg: "lines", noFix: 1},
		{name: "a generated file gets no fix", pkg: "generated", noFix: 1},
		{name: "a package-qualified receiver import goes with its last use", pkg: "pkgrecv", removes: []string{ldctx + ",example.com/pkgrecv/clients"}},
		{name: "a package-qualified receiver import shared by two fixes stays", pkg: "pkgrecvtwo", removes: []string{"", ""}},
		{name: "the outer of nested rewrites has no fix", pkg: "nested", removes: []string{""}, noFix: 1},
	}
	candidates := []string{ldctx, "example.com/pkgrecv/clients", "example.com/pkgrecvtwo/clients"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, tt.pkg)
			var got []string
			noFix := 0
			for _, res := range results {
				for _, d := range res.Diagnostics {
					if len(d.SuggestedFixes) == 0 {
						noFix++
						continue
					}
					edits := slices.Clone(d.SuggestedFixes[0].TextEdits)
					slices.SortFunc(edits, func(a, b analysis.TextEdit) int { return int(a.Pos) - int(b.Pos) })
					var removed []string
					for i, e := range edits {
						if i > 0 && edits[i-1].End > e.Pos {
							t.Errorf("overlapping edits [%d,%d) and [%d,%d)", edits[i-1].Pos, edits[i-1].End, e.Pos, e.End)
						}
						tf := res.Pass.Fset.File(e.Pos)
						src, err := res.Pass.ReadFile(tf.Name())
						if err != nil {
							t.Fatal(err)
						}
						old := src[tf.Offset(e.Pos):tf.Offset(e.End)]
						for _, c := range candidates {
							q := []byte(`"` + c + `"`)
							if bytes.Contains(old, q) && !bytes.Contains(e.NewText, q) && !slices.Contains(removed, c) {
								removed = append(removed, c)
							}
						}
					}
					got = append(got, strings.Join(removed, ","))
				}
			}
			if !slices.Equal(got, tt.removes) || noFix != tt.noFix {
				t.Fatalf("removal per fix = %q (no-fix diagnostics %d), want %q (%d)", got, noFix, tt.removes, tt.noFix)
			}
		})
	}
}
