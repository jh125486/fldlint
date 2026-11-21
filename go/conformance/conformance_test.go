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

var specRoot = flag.String("spec", "../../spec", "spec root; falls back to the stub spec under testdata when rules.yaml or fixtures are missing")

const (
	stubRulesPath = "../internal/spec/testdata/rules.yaml"
	stubFixtures  = "testdata/fixtures"
)

// locate returns the rules path and fixtures dir, preferring the real spec.
func locate() (rules, fixtures string) {
	r := filepath.Join(*specRoot, "rules.yaml")
	f := filepath.Join(*specRoot, "fixtures")
	_, errR := os.Stat(r)
	_, errF := os.Stat(f)
	if errR != nil || errF != nil {
		return stubRulesPath, stubFixtures
	}
	return r, f
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
	if want, ok := c.Expect["go"]; ok {
		dir := filepath.Join(filepath.Dir(path), "go")
		if _, err := os.Stat(filepath.Join(dir, "input.go")); err != nil {
			return nil, fmt.Errorf("case %q: go/input.go: %w", c.ID, err)
		}
		_, err := os.Stat(filepath.Join(dir, "output.go.golden"))
		if want.Output && err != nil {
			return nil, fmt.Errorf("case %q: output: true but go/output.go.golden missing", c.ID)
		}
		if !want.Output && err == nil {
			return nil, fmt.Errorf("case %q: output: false but go/output.go.golden present", c.ID)
		}
	}
	return c, nil
}

func TestConformance(t *testing.T) {
	t.Parallel()
	rulesPath, fixtures := locate()
	rf, err := spec.LoadRules(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(fixtures, "*", "*", "case.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fixture cases discovered under %s", fixtures)
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

func TestCheckCase(t *testing.T) {
	t.Parallel()
	rf, err := spec.LoadRules(stubRulesPath)
	if err != nil {
		t.Fatal(err)
	}
	write := func(t *testing.T, caseYAML string, files ...string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "go"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "case.yaml"), []byte(caseYAML), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, "go", f), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(dir, "case.yaml")
	}
	const base = "id: x\nrule: bool-variation\ndescription: d\nexpect:\n  go: {diagnostics: [{line: 1, col: 1, ruleId: bool-variation}], output: %v}\n"
	tests := []struct {
		name    string
		yaml    string
		files   []string
		wantErr bool
	}{
		{name: "ok no output", yaml: fmt.Sprintf(base, false), files: []string{"input.go"}},
		{name: "ok with output", yaml: fmt.Sprintf(base, true), files: []string{"input.go", "output.go.golden"}},
		{name: "malformed yaml", yaml: "id: [", wantErr: true},
		{name: "invalid case", yaml: "id: x\n", wantErr: true},
		{name: "missing input", yaml: fmt.Sprintf(base, false), wantErr: true},
		{name: "missing golden", yaml: fmt.Sprintf(base, true), files: []string{"input.go"}, wantErr: true},
		{name: "unexpected golden", yaml: fmt.Sprintf(base, false), files: []string{"input.go", "output.go.golden"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := checkCase(write(t, tt.yaml, tt.files...), rf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
