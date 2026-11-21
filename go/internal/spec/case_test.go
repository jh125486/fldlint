package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const okCase = `
id: c1
rule: bool-variation
tags: [quote-double]
description: d
expect:
  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], output: false}
`

func stubRules(t *testing.T) *spec.RulesFile {
	t.Helper()
	rf, err := spec.LoadRules("testdata/rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return rf
}

func TestParseCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		old     string
		new     string
		wantErr string
	}{
		{name: "ok"},
		{name: "empty", old: okCase, new: "", wantErr: "empty document"},
		{name: "bad yaml", old: "id: c1", new: "id: [", wantErr: "parse case"},
		{name: "unknown field", old: "description: d", new: "description: d\nbogus: 1", wantErr: `unknown key "bogus"`},
		{name: "unknown expect field", old: "output: false}", new: "output: false, extra: 1}", wantErr: `unknown key "extra"`},
		{name: "unknown diagnostic field", old: "col: 12,", new: "col: 12, extra: 1,", wantErr: `unknown key "extra"`},
		{name: "duplicate key", old: "description: d", new: "description: d\ndescription: e", wantErr: "duplicate key"},
		{name: "missing id", old: "id: c1\n", new: "", wantErr: `missing required key "id"`},
		{name: "missing rule", old: "rule: bool-variation\n", new: "", wantErr: `missing required key "rule"`},
		{name: "missing tags", old: "tags: [quote-double]\n", new: "", wantErr: `missing required key "tags"`},
		{name: "missing description", old: "description: d\n", new: "", wantErr: `missing required key "description"`},
		{name: "missing expect", old: "expect:\n  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], output: false}\n", new: "", wantErr: `missing required key "expect"`},
		{name: "empty description", old: "description: d", new: `description: ""`, wantErr: "must not be empty"},
		{name: "bad id pattern", old: "id: c1", new: "id: Bad_ID", wantErr: "case id pattern"},
		{name: "numeric id", old: "id: c1", new: "id: 5", wantErr: "must be a string"},
		{name: "bad rule pattern", old: "rule: bool-variation", new: "rule: Bool", wantErr: "rule id pattern"},
		{name: "bad tag pattern", old: "[quote-double]", new: "[Q Q]", wantErr: "tag pattern"},
		{name: "empty tag", old: "[quote-double]", new: `[""]`, wantErr: "tag pattern"},
		{name: "duplicate tag", old: "[quote-double]", new: "[a, a]", wantErr: "duplicate item"},
		{name: "tags not a list", old: "[quote-double]", new: "quote-double", wantErr: "must be a sequence"},
		{name: "empty expect", old: "  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], output: false}", new: "  {}", wantErr: "at least 1 language"},
		{name: "unknown language", old: "  go:", new: "  rust:", wantErr: `unknown key "rust"`},
		{name: "null language", old: "  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], output: false}", new: "  go:", wantErr: "must not be null"},
		{name: "missing diagnostics", old: "diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], ", new: "", wantErr: `missing required key "diagnostics"`},
		{name: "missing output", old: ", output: false", new: "", wantErr: `missing required key "output"`},
		{name: "output yes", old: "output: false", new: "output: yes", wantErr: "must be a boolean"},
		{name: "output string", old: "output: false", new: `output: "true"`, wantErr: "must be a boolean"},
		{name: "zero line", old: "line: 9", new: "line: 0", wantErr: "must be >= 1"},
		{name: "float line", old: "line: 9", new: "line: 3.5", wantErr: "must be an integer"},
		{name: "string col", old: "col: 12", new: `col: "12"`, wantErr: "must be an integer"},
		{name: "missing ruleId", old: "ruleId: bool-variation, ", new: "", wantErr: `missing required key "ruleId"`},
		{name: "bad ruleId pattern", old: "ruleId: bool-variation", new: "ruleId: NOPE", wantErr: "rule id pattern"},
		{name: "bad skip", old: "context-unproven", new: "nope", wantErr: "is not one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := okCase
			if tt.old != "" {
				if !strings.Contains(src, tt.old) {
					t.Fatalf("test bug: %q not in base case", tt.old)
				}
				src = strings.Replace(src, tt.old, tt.new, 1)
			}
			c, err := spec.ParseCase([]byte(src))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Expect["go"].Diagnostics[0].Col != 12 || c.Expect["go"].Output {
				t.Fatalf("unexpected parse: %+v", c)
			}
		})
	}
}

func TestLoadCase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "case.yaml")
	if err := os.WriteFile(good, []byte(okCase), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "ok", path: good},
		{name: "missing", path: filepath.Join(dir, "nope.yaml"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadCase(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCase_Validate(t *testing.T) {
	t.Parallel()
	rf := stubRules(t)
	tests := []struct {
		name    string
		old     string
		new     string
		wantErr string
	}{
		{name: "ok"},
		{name: "unknown rule", old: "rule: bool-variation", new: "rule: zzz", wantErr: "unknown rule"},
		{name: "unknown diagnostic ruleId", old: "ruleId: bool-variation", new: "ruleId: zzz", wantErr: "unknown ruleId"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := okCase
			if tt.old != "" {
				src = strings.Replace(src, tt.old, tt.new, 1)
			}
			c, err := spec.ParseCase([]byte(src))
			if err != nil {
				t.Fatal(err)
			}
			err = c.Validate(rf)
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

func TestCase_Validate_probes(t *testing.T) {
	t.Parallel()
	rf := stubRules(t)
	paths, err := filepath.Glob("testdata/probes/c_*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no probes: %v", err)
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			t.Parallel()
			c, err := spec.LoadCase(p)
			if err == nil {
				err = c.Validate(rf)
			}
			if err == nil {
				t.Fatal("expected the probe to be rejected")
			}
		})
	}
}

func TestCase_CheckFixtureDir(t *testing.T) {
	t.Parallel()
	const withGo = `
id: c1
rule: bool-variation
tags: []
description: d
expect:
  go: {diagnostics: [], output: %s}
`
	tests := []struct {
		name    string
		ruleDir string
		output  string
		files   []string
		wantErr string
	}{
		{name: "ok", ruleDir: "bool-variation", output: "false", files: []string{"go/input.go"}},
		{name: "ok with output", ruleDir: "bool-variation", output: "true", files: []string{"go/input.go", "go/output.go.golden"}},
		{name: "directory mismatch", ruleDir: "string-variation", output: "false", files: []string{"go/input.go"}, wantErr: `rule "bool-variation" does not match its directory name "string-variation"`},
		{name: "missing input", ruleDir: "bool-variation", output: "false", wantErr: "go/input.go, which does not exist"},
		{name: "missing output", ruleDir: "bool-variation", output: "true", files: []string{"go/input.go"}, wantErr: "output is true but go/output.go.golden does not exist"},
		{name: "unexpected output", ruleDir: "bool-variation", output: "false", files: []string{"go/input.go", "go/output.go.golden"}, wantErr: "output is false but go/output.go.golden exists"},
		{name: "stray js output", ruleDir: "bool-variation", output: "false", files: []string{"go/input.go", "js/output.ts"}, wantErr: "js/output.ts exists but case.yaml has no expect.js"},
		{name: "stray java output", ruleDir: "bool-variation", output: "false", files: []string{"go/input.go", "java/After.java"}, wantErr: "java/After.java exists but case.yaml has no expect.java"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), tt.ruleDir, "c1")
			for _, f := range tt.files {
				if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(f)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			c, err := spec.ParseCase([]byte(strings.Replace(withGo, "%s", tt.output, 1)))
			if err != nil {
				t.Fatal(err)
			}
			err = c.CheckFixtureDir(dir)
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
