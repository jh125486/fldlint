package analyzer_test

import (
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
