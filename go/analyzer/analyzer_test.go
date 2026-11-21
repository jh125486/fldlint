package analyzer_test

import (
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/jh125486/fldlint/go/analyzer"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  func() any
		want any
	}{
		{name: "name", got: func() any { return analyzer.Analyzer.Name }, want: "fldlint"},
		{name: "valid", got: func() any { return analysis.Validate([]*analysis.Analyzer{analyzer.Analyzer}) }, want: error(nil)},
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
