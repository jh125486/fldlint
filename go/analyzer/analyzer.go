// Package analyzer provides the fldlint go/analysis analyzer.
package analyzer

import "golang.org/x/tools/go/analysis"

// Analyzer is the fldlint analyzer. M0 stub: reports nothing.
var Analyzer = &analysis.Analyzer{
	Name: "fldlint",
	Doc:  "reports LaunchDarkly SDK usage that can migrate to OpenFeature (stub in M0)",
	Run: func(*analysis.Pass) (any, error) {
		return nil, nil
	},
}
