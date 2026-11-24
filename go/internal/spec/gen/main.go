// Command gen regenerates internal/spec/rules_gen.go from spec/generated/spec.json.
// Run from the go/ directory: go run ./internal/spec/gen
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const stubSpec = "internal/spec/testdata/spec.json"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	root := fs.String("spec", "../spec", "spec root containing generated/spec.json; an explicit -spec that lacks it is an error (the stub is used only when -spec is not given and the default root is missing)")
	out := fs.String("out", "internal/spec/rules_gen.go", "output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "spec" {
			explicit = true
		}
	})
	path := filepath.Join(*root, "generated", "spec.json")
	if _, err := os.Stat(path); err != nil {
		if explicit {
			return fmt.Errorf("-spec %q: %w", *root, err)
		}
		fmt.Fprintf(os.Stderr, "gen: %s not found, using stub %s\n", path, stubSpec)
		path = stubSpec
	}
	sp, err := spec.LoadSpec(path)
	if err != nil {
		return err
	}
	src, err := spec.Generate(sp.Rules)
	if err != nil {
		return err
	}
	return os.WriteFile(*out, src, 0o644) //nolint:gosec // G306: generated source is committed and must be world-readable
}
