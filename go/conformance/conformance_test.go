package conformance_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

var specRoot = flag.String("spec", defaultSpecRoot, "spec root; an explicit value must contain generated/spec.json and fixtures/ (the stub spec under testdata is used only when -spec is not given and the default root is missing)")

const (
	defaultSpecRoot = "../../spec"
	stubSpecPath    = "../internal/spec/testdata/spec.json"
	stubFixtures    = "testdata/fixtures"
)

// specExplicit reports whether -spec was given on the command line.
func specExplicit() bool {
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "spec" {
			explicit = true
		}
	})
	return explicit
}

// locate returns the spec.json path and fixtures dir. An explicit root must be
// complete, otherwise it is an error; the stub is used only when no root was
// given and the default root is incomplete.
func locate(root string, explicit bool) (specJSON, fixtures string, err error) {
	j := filepath.Join(root, "generated", "spec.json")
	f := filepath.Join(root, "fixtures")
	_, errJ := os.Stat(j)
	fi, errF := os.Stat(f)
	complete := errJ == nil && errF == nil && fi.IsDir()
	switch {
	case complete:
		return j, f, nil
	case explicit:
		return "", "", fmt.Errorf("-spec %q must contain generated/spec.json and a fixtures/ directory", root)
	default:
		return stubSpecPath, stubFixtures, nil
	}
}

// TestConformance runs every case of spec.json through the engine. The cases
// were validated (YAML, schema, semantics, fixture files) by the JS loader
// before spec.json was emitted, so only the engine behaviour is checked here.
func TestConformance(t *testing.T) {
	t.Parallel()
	specJSON, fixtures, err := locate(*specRoot, specExplicit())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := spec.LoadSpec(specJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(sp.Cases) == 0 {
		t.Fatalf("no cases in %s", specJSON)
	}
	t.Logf("%d cases from %s (fixtures: %s)", len(sp.Cases), specJSON, fixtures)
	for _, lc := range sp.Cases {
		t.Run(lc.Path, func(t *testing.T) {
			t.Parallel()
			c := lc.Case
			if _, ok := c.Expect["go"]; !ok {
				t.Skip("not applicable: no go expectation")
			}
			if specJSON == stubSpecPath {
				t.Skip("stub spec: the M0 stub fixtures are not backed by the real engine")
			}
			checkEngine(t, c, lc.Dir(fixtures))
		})
	}
}

// TestLocalCases runs harness-local cases that the shared spec fixtures do not
// carry (they need a helper package or several engine runs). Same checks as
// the fixtures: findings, each fix alone, the union, the golden.
func TestLocalCases(t *testing.T) {
	t.Parallel()
	rw := func(line, col int) spec.Diagnostic {
		return spec.Diagnostic{Line: line, Col: col, RuleID: "bool-variation"}
	}
	tests := []struct {
		name string
		want []spec.Diagnostic
	}{
		// A package-qualified receiver: the fix removes the import of the
		// package when it is the last use, and leaves it for the driver when
		// another fix still needs it.
		{name: "pkg-receiver", want: []spec.Diagnostic{rw(13, 11)}},
		{name: "pkg-receiver-shared", want: []spec.Diagnostic{rw(13, 11), rw(18, 11)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &spec.Case{ID: tt.name, Rule: "bool-variation", Expect: map[string]spec.LangExpect{"go": {Diagnostics: tt.want, Output: true}}}
			checkEngine(t, c, filepath.Join("testdata", "local", tt.name))
		})
	}
}

// TestNestedConvergence checks that nested rewritable calls converge under
// `go fix`-style driving: only the inner call carries a fix in the first
// run, the outer one in the next, and the result compiles with nothing left
// to fix.
func TestNestedConvergence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nested-default", "nested-in-context"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join("testdata", "local", name)
			src, err := os.ReadFile(filepath.Join(dir, "go", "input.go"))
			if err != nil {
				t.Fatal(err)
			}
			first, _ := runSource(t, src, dir, false)
			fixes := 0
			for _, f := range first {
				if f.hasFix {
					fixes++
				}
			}
			if len(first) != 2 || fixes != 1 {
				t.Fatalf("first run: %d diagnostics with %d fixes, want 2 with 1 (the inner call)", len(first), fixes)
			}
			final := converge(t, src, dir)
			if left, _ := runSource(t, final, dir, false); len(left) != 0 {
				t.Fatalf("diagnostics left after converging: %+v\n%s", left, final)
			}
		})
	}
}

func TestLocate(t *testing.T) {
	t.Parallel()
	full, noFixtures := t.TempDir(), t.TempDir()
	for _, root := range []string{full, noFixtures} {
		if err := os.MkdirAll(filepath.Join(root, "generated"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "generated", "spec.json"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(full, "fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "nope")
	tests := []struct {
		name     string
		root     string
		explicit bool
		wantStub bool
		wantErr  bool
	}{
		{name: "complete explicit", root: full, explicit: true},
		{name: "complete default", root: full},
		{name: "missing explicit", root: missing, explicit: true, wantErr: true},
		{name: "no fixtures explicit", root: noFixtures, explicit: true, wantErr: true},
		{name: "missing default falls back", root: missing, wantStub: true},
		{name: "no fixtures default falls back", root: noFixtures, wantStub: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j, f, err := locate(tt.root, tt.explicit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if stub := j == stubSpecPath && f == stubFixtures; stub != tt.wantStub {
				t.Fatalf("got spec=%q fixtures=%q, wantStub %v", j, f, tt.wantStub)
			}
		})
	}
}
