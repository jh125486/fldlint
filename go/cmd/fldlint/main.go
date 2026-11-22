// Command fldlint runs the fldlint analyzer standalone.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/jh125486/fldlint/go/analyzer"
)

func main() { singlechecker.Main(analyzer.Analyzer) }
