// Command gen regenerates internal/spec/rules_gen.go and the Java RulesGen.java from spec/generated/spec.json.
// Run from the go/ directory: go run ./internal/spec/gen
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const stubSpec = "internal/spec/testdata/spec.json"

// usageError marks a usage or configuration error (bad flag, missing explicit
// -spec). It exits 2, like the JS tools and ESLint (ADR 004); other failures exit 1.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

func exitCode(err error) int {
	var u *usageError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &u):
		return 2
	default:
		return 1
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(exitCode(err))
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	root := fs.String("spec", "../spec", "spec root containing generated/spec.json; an explicit -spec that lacks it is an error (the stub is used only when -spec is not given and the default root is missing)")
	out := fs.String("out", "internal/spec/rules_gen.go", "Go output file")
	javaOut := fs.String("java-out", "../java/src/main/java/io/github/jh125486/fldlint/spec/RulesGen.java", "Java output file; empty skips it (it is also skipped when the stub spec is used)")
	if err := fs.Parse(args); err != nil {
		return &usageError{err}
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "spec" {
			explicit = true
		}
	})
	path := filepath.Join(*root, "generated", "spec.json")
	usingStub := false
	if _, err := os.Stat(path); err != nil {
		if explicit {
			return &usageError{fmt.Errorf("-spec %q: %w", *root, err)}
		}
		fmt.Fprintf(os.Stderr, "gen: %s not found, using stub %s\n", path, stubSpec)
		path = stubSpec
		usingStub = true
	}
	sp, err := spec.LoadSpec(path)
	if err != nil {
		return err
	}
	src, err := spec.Generate(sp.Rules)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil { //nolint:gosec // G306: generated source is committed and must be world-readable
		return err
	}
	if *javaOut == "" || usingStub {
		return nil
	}
	return os.WriteFile(*javaOut, spec.GenerateJava(sp.Rules), 0o644) //nolint:gosec // G306: generated source is committed and must be world-readable
}
