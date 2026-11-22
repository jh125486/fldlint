package conformance_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

var specRoot = flag.String("spec", defaultSpecRoot, "spec root; an explicit value must contain rules.yaml and fixtures/ (the stub spec under testdata is used only when -spec is not given and the default root is missing)")

const (
	defaultSpecRoot = "../../spec"
	stubRulesPath   = "../internal/spec/testdata/rules.yaml"
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

// locate returns the rules path and fixtures dir. An explicit root must be
// complete, otherwise it is an error; the stub is used only when no root was
// given and the default root is incomplete.
func locate(root string, explicit bool) (rules, fixtures string, err error) {
	r := filepath.Join(root, "rules.yaml")
	f := filepath.Join(root, "fixtures")
	_, errR := os.Stat(r)
	fi, errF := os.Stat(f)
	complete := errR == nil && errF == nil && fi.IsDir()
	switch {
	case complete:
		return r, f, nil
	case explicit:
		return "", "", fmt.Errorf("-spec %q must contain rules.yaml and a fixtures/ directory", root)
	default:
		return stubRulesPath, stubFixtures, nil
	}
}

// checkCase loads and validates one case.yaml and verifies that the files its
// Go expectation promises are present.
func checkCase(path string, rf *spec.RulesFile) (*spec.Case, error) {
	c, err := spec.LoadCase(path)
	if err != nil {
		return nil, err
	}
	if err := c.Validate(rf); err != nil {
		return nil, err
	}
	if err := c.CheckFixtureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return c, nil
}

func TestConformance(t *testing.T) {
	t.Parallel()
	rulesPath, fixtures, err := locate(*specRoot, specExplicit())
	if err != nil {
		t.Fatal(err)
	}
	rf, err := spec.LoadRules(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := spec.DiscoverFixtures(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fixture cases discovered under %s", fixtures)
	}
	loaded := map[string]*spec.Case{}
	for _, p := range paths {
		if c, err := spec.LoadCase(p); err == nil {
			loaded[p] = c
		}
	}
	if err := spec.CheckUniqueIDs(loaded); err != nil {
		t.Error(err)
	}
	t.Logf("discovered %d cases under %s (rules: %s)", len(paths), fixtures, rulesPath)
	for _, p := range paths {
		name := filepath.ToSlash(strings.TrimSuffix(strings.TrimPrefix(p, fixtures+string(filepath.Separator)), string(filepath.Separator)+"case.yaml"))
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := checkCase(p, rf)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := c.Expect["go"]; !ok {
				t.Skip("not applicable: no go expectation")
			}
			if rulesPath == stubRulesPath {
				t.Skip("stub spec: the M0 stub fixtures are not backed by the real engine")
			}
			checkEngine(t, c, filepath.Dir(p))
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
	full := t.TempDir()
	if err := os.MkdirAll(filepath.Join(full, "fixtures"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "rules.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	noFixtures := t.TempDir()
	if err := os.WriteFile(filepath.Join(noFixtures, "rules.yaml"), nil, 0o644); err != nil {
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
			r, f, err := locate(tt.root, tt.explicit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if stub := r == stubRulesPath && f == stubFixtures; stub != tt.wantStub {
				t.Fatalf("got rules=%q fixtures=%q, wantStub %v", r, f, tt.wantStub)
			}
		})
	}
}

func TestCheckCase(t *testing.T) {
	t.Parallel()
	rf, err := spec.LoadRules(stubRulesPath)
	if err != nil {
		t.Fatal(err)
	}
	// write lays out <tmp>/<ruleDir>/x/case.yaml plus the given files.
	write := func(t *testing.T, ruleDir, caseYAML string, files ...string) string {
		t.Helper()
		dir := filepath.Join(t.TempDir(), ruleDir, "x")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "case.yaml"), []byte(caseYAML), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(f)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(dir, "case.yaml")
	}
	const base = "id: x\nrule: bool-variation\ntags: []\ndescription: d\nexpect:\n  go: {diagnostics: [{line: 1, col: 1, ruleId: bool-variation}], output: %v}\n"
	// output false on a rewriting rule needs a call-site skip on every diagnostic.
	baseNoOutput := strings.Replace(fmt.Sprintf(base, false), "ruleId: bool-variation}", "ruleId: bool-variation, skip: dynamic-key}", 1)
	const jsOnly = "id: x\nrule: bool-variation\ntags: []\ndescription: d\nexpect:\n  js: {diagnostics: [], output: false}\n"
	tests := []struct {
		name    string
		dir     string
		yaml    string
		files   []string
		wantErr string
	}{
		{name: "ok no output", yaml: baseNoOutput, files: []string{"go/input.go"}},
		{name: "ok with output", yaml: fmt.Sprintf(base, true), files: []string{"go/input.go", "go/output.go.golden"}},
		{name: "unknown rule", yaml: strings.Replace(baseNoOutput, "rule: bool-variation", "rule: nope", 1), files: []string{"go/input.go"}, wantErr: "unknown rule"},
		{name: "unknown diagnostic ruleId", yaml: strings.Replace(baseNoOutput, "ruleId: bool-variation", "ruleId: nope", 1), files: []string{"go/input.go"}, wantErr: "unknown ruleId"},
		{name: "rule differs from directory", dir: "bool-variation-detail", yaml: baseNoOutput, files: []string{"go/input.go"}, wantErr: `does not match its directory name "bool-variation-detail"`},
		{name: "stray go golden without expect.go", yaml: jsOnly, files: []string{"js/input.ts", "go/output.go.golden"}, wantErr: "go/output.go.golden exists but case.yaml has no expect.go"},
		{name: "stray js output without expect.js", yaml: baseNoOutput, files: []string{"go/input.go", "js/output.ts"}, wantErr: "js/output.ts exists but case.yaml has no expect.js"},
		{name: "stray java output without expect.java", yaml: baseNoOutput, files: []string{"go/input.go", "java/After.java"}, wantErr: "java/After.java exists but case.yaml has no expect.java"},
		{name: "js expectation needs js input", yaml: jsOnly, wantErr: "js/input.ts, which does not exist"},
		{name: "malformed yaml", yaml: "id: [", wantErr: "parse case"},
		{name: "invalid case", yaml: "id: x\n", wantErr: "missing required key"},
		{name: "missing input", yaml: baseNoOutput, wantErr: "go/input.go, which does not exist"},
		{name: "missing golden", yaml: fmt.Sprintf(base, true), files: []string{"go/input.go"}, wantErr: "go/output.go.golden does not exist"},
		{name: "unexpected golden", yaml: baseNoOutput, files: []string{"go/input.go", "go/output.go.golden"}, wantErr: "go/output.go.golden exists"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.dir
			if dir == "" {
				dir = "bool-variation"
			}
			_, err := checkCase(write(t, dir, tt.yaml, tt.files...), rf)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
