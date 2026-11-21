// Command gen regenerates internal/spec/rules_gen.go from rules.yaml.
// Run from the go/ directory: go run ./internal/spec/gen
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const stubRules = "internal/spec/testdata/rules.yaml"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	root := fs.String("spec", "../spec", "spec root containing rules.yaml (falls back to the stub spec when missing)")
	out := fs.String("out", "internal/spec/rules_gen.go", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := filepath.Join(*root, "rules.yaml")
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %s not found, using stub %s\n", path, stubRules)
		path = stubRules
	}
	rf, err := spec.LoadRules(path)
	if err != nil {
		return err
	}
	src, err := spec.Generate(rf)
	if err != nil {
		return err
	}
	return os.WriteFile(*out, src, 0o644)
}
