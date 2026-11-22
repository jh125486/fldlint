// Command fldlint-fix is the unitchecker entry point (go vet -vettool).
package main

import (
	"golang.org/x/tools/go/analysis/unitchecker"

	"github.com/jh125486/fldlint/go/analyzer"
)

func main() { unitchecker.Main(analyzer.Analyzer) }
