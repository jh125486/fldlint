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
			// TODO(M2): replace with analysistest.RunWithSuggestedFixes once the
			// real engine lands; until then the case is only counted, not passed.
			t.Skip("pending: Go engine lands in M2 (analysistest.RunWithSuggestedFixes)")
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
	const jsOnly = "id: x\nrule: bool-variation\ntags: []\ndescription: d\nexpect:\n  js: {diagnostics: [], output: false}\n"
	tests := []struct {
		name    string
		dir     string
		yaml    string
		files   []string
		wantErr string
	}{
		{name: "ok no output", yaml: fmt.Sprintf(base, false), files: []string{"go/input.go"}},
		{name: "ok with output", yaml: fmt.Sprintf(base, true), files: []string{"go/input.go", "go/output.go.golden"}},
		{name: "unknown rule", yaml: strings.Replace(fmt.Sprintf(base, false), "rule: bool-variation", "rule: nope", 1), files: []string{"go/input.go"}, wantErr: "unknown rule"},
		{name: "unknown diagnostic ruleId", yaml: strings.Replace(fmt.Sprintf(base, false), "ruleId: bool-variation", "ruleId: nope", 1), files: []string{"go/input.go"}, wantErr: "unknown ruleId"},
		{name: "rule differs from directory", dir: "bool-variation-detail", yaml: fmt.Sprintf(base, false), files: []string{"go/input.go"}, wantErr: `does not match its directory name "bool-variation-detail"`},
		{name: "stray go golden without expect.go", yaml: jsOnly, files: []string{"js/input.ts", "go/output.go.golden"}, wantErr: "go/output.go.golden exists but case.yaml has no expect.go"},
		{name: "stray js output without expect.js", yaml: fmt.Sprintf(base, false), files: []string{"go/input.go", "js/output.ts"}, wantErr: "js/output.ts exists but case.yaml has no expect.js"},
		{name: "stray java output without expect.java", yaml: fmt.Sprintf(base, false), files: []string{"go/input.go", "java/After.java"}, wantErr: "java/After.java exists but case.yaml has no expect.java"},
		{name: "js expectation needs js input", yaml: jsOnly, wantErr: "js/input.ts, which does not exist"},
		{name: "malformed yaml", yaml: "id: [", wantErr: "parse case"},
		{name: "invalid case", yaml: "id: x\n", wantErr: "missing required key"},
		{name: "missing input", yaml: fmt.Sprintf(base, false), wantErr: "go/input.go, which does not exist"},
		{name: "missing golden", yaml: fmt.Sprintf(base, true), files: []string{"go/input.go"}, wantErr: "go/output.go.golden does not exist"},
		{name: "unexpected golden", yaml: fmt.Sprintf(base, false), files: []string{"go/input.go", "go/output.go.golden"}, wantErr: "go/output.go.golden exists"},
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
