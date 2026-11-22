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
rule: bool-variation-detail
tags: [quote-double]
description: d
expect:
  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation-detail, skip: detail}], output: false}
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
		{name: "default-type-unproven skip accepted", old: "skip: detail", new: "skip: default-type-unproven"},
		{name: "empty", old: okCase, new: "", wantErr: "empty document"},
		{name: "bad yaml", old: "id: c1", new: "id: [", wantErr: "parse case"},
		{name: "unknown field", old: "description: d", new: "description: d\nbogus: 1", wantErr: `unknown key "bogus"`},
		{name: "unknown expect field", old: "output: false}", new: "output: false, extra: 1}", wantErr: `unknown key "extra"`},
		{name: "unknown diagnostic field", old: "col: 12,", new: "col: 12, extra: 1,", wantErr: `unknown key "extra"`},
		{name: "duplicate key", old: "description: d", new: "description: d\ndescription: e", wantErr: "duplicate key"},
		{name: "missing id", old: "id: c1\n", new: "", wantErr: `missing required key "id"`},
		{name: "missing rule", old: "rule: bool-variation-detail\n", new: "", wantErr: `missing required key "rule"`},
		{name: "missing tags", old: "tags: [quote-double]\n", new: "", wantErr: `missing required key "tags"`},
		{name: "missing description", old: "description: d\n", new: "", wantErr: `missing required key "description"`},
		{name: "missing expect", old: "expect:\n  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation-detail, skip: detail}], output: false}\n", new: "", wantErr: `missing required key "expect"`},
		{name: "empty description", old: "description: d", new: `description: ""`, wantErr: "must not be empty"},
		{name: "bad id pattern", old: "id: c1", new: "id: Bad_ID", wantErr: "case id pattern"},
		{name: "numeric id", old: "id: c1", new: "id: 5", wantErr: "must be a string"},
		{name: "bad rule pattern", old: "rule: bool-variation-detail", new: "rule: Bool", wantErr: "rule id pattern"},
		{name: "bad tag pattern", old: "[quote-double]", new: "[Q Q]", wantErr: "tag pattern"},
		{name: "empty tag", old: "[quote-double]", new: `[""]`, wantErr: "tag pattern"},
		{name: "duplicate tag", old: "[quote-double]", new: "[a, a]", wantErr: "duplicate item"},
		{name: "tags not a list", old: "[quote-double]", new: "quote-double", wantErr: "must be a sequence"},
		{name: "empty expect", old: "  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation-detail, skip: detail}], output: false}", new: "  {}", wantErr: "at least 1 language"},
		{name: "unknown language", old: "  go:", new: "  rust:", wantErr: `unknown key "rust"`},
		{name: "null language", old: "  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation-detail, skip: detail}], output: false}", new: "  go:", wantErr: "must not be null"},
		{name: "missing diagnostics", old: "diagnostics: [{line: 9, col: 12, ruleId: bool-variation-detail, skip: detail}], ", new: "", wantErr: `missing required key "diagnostics"`},
		{name: "missing output", old: ", output: false", new: "", wantErr: `missing required key "output"`},
		{name: "output yes", old: "output: false", new: "output: yes", wantErr: "must be a boolean"},
		{name: "output string", old: "output: false", new: `output: "true"`, wantErr: "must be a boolean"},
		{name: "zero line", old: "line: 9", new: "line: 0", wantErr: "must be >= 1"},
		{name: "float line", old: "line: 9", new: "line: 3.5", wantErr: "must be an integer"},
		{name: "string col", old: "col: 12", new: `col: "12"`, wantErr: "must be an integer"},
		{name: "missing ruleId", old: "ruleId: bool-variation-detail, ", new: "", wantErr: `missing required key "ruleId"`},
		{name: "bad ruleId pattern", old: "ruleId: bool-variation-detail", new: "ruleId: NOPE", wantErr: "rule id pattern"},
		{name: "bad skip", old: "skip: detail", new: "skip: nope", wantErr: "is not one of"},
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
	const rewrite = "id: c1\nrule: bool-variation\ntags: []\ndescription: d\nexpect:\n  go: {diagnostics: [{line: 1, col: 1, ruleId: bool-variation}], output: true}\n"
	tests := []struct {
		name    string
		src     string // full case; defaults to okCase with old replaced by new
		old     string
		new     string
		wantErr string
	}{
		{name: "ok"},
		{name: "ok rewrite", src: rewrite},
		{name: "unknown rule", old: "rule: bool-variation-detail", new: "rule: zzz", wantErr: "unknown rule"},
		{name: "unknown diagnostic ruleId", old: "ruleId: bool-variation-detail", new: "ruleId: zzz", wantErr: "unknown ruleId"},
		{name: "diagnostic ruleId differs from rule", old: "ruleId: bool-variation-detail", new: "ruleId: bool-variation", wantErr: "must equal the case rule"},
		{name: "skip differs from rule skip", old: "skip: detail", new: "skip: dynamic-key", wantErr: "must equal the rule's skip"},
		{name: "skip rule diagnostic without skip", old: ", skip: detail", new: "", wantErr: "must equal the rule's skip"},
		{name: "rewriting rule with skip detail", src: strings.Replace(rewrite, "ruleId: bool-variation}", "ruleId: bool-variation, skip: detail}", 1), wantErr: "not a call-site skip"},
		{name: "rewriting rule with call-site skip", src: strings.Replace(strings.Replace(rewrite, "ruleId: bool-variation}", "ruleId: bool-variation, skip: dynamic-key}", 1), "output: true", "output: false", 1)},
		{name: "rewriting rule with default-type-unproven skip", src: strings.Replace(strings.Replace(rewrite, "ruleId: bool-variation}", "ruleId: bool-variation, skip: default-type-unproven}", 1), "output: true", "output: false", 1)},
		{name: "rewriting rule call-site skip with output true", src: strings.Replace(rewrite, "ruleId: bool-variation}", "ruleId: bool-variation, skip: context-unproven}", 1), wantErr: "output must be false"},
		{name: "skip rule with output true", old: "output: false", new: "output: true", wantErr: "output must be false"},
		{name: "rewriting rule no skip with output false", src: strings.Replace(rewrite, "output: true", "output: false", 1), wantErr: "has no skip on rewriting rule"},
		{name: "output true without diagnostics", src: strings.Replace(rewrite, "[{line: 1, col: 1, ruleId: bool-variation}]", "[]", 1), wantErr: "no diagnostics"},
		{name: "language not bound", src: strings.Replace(rewrite, "  go:", "  java:", 1), wantErr: "has no java binding"},
		{name: "rule bound to go and js", src: strings.Replace(rewrite, "  go:", "  js:", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := tt.src
			if src == "" {
				src = okCase
				if tt.old != "" {
					if !strings.Contains(src, tt.old) {
						t.Fatalf("test bug: %q not in base case", tt.old)
					}
					src = strings.Replace(src, tt.old, tt.new, 1)
				}
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
	// Probe files from the reviewers must be rejected by load or Validate.
	paths, err := filepath.Glob("testdata/probes/c_*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no probes: %v", err)
	}
	for _, p := range paths {
		t.Run("probe "+filepath.Base(p), func(t *testing.T) {
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
rule: bool-variation-detail
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
		same    bool
		wantErr string
	}{
		{name: "identical output", ruleDir: "bool-variation-detail", output: "true", files: []string{"go/input.go", "go/output.go.golden"}, same: true, wantErr: "is identical to go/input.go"},
		{name: "ok", ruleDir: "bool-variation-detail", output: "false", files: []string{"go/input.go"}},
		{name: "ok with output", ruleDir: "bool-variation-detail", output: "true", files: []string{"go/input.go", "go/output.go.golden"}},
		{name: "directory mismatch", ruleDir: "string-variation", output: "false", files: []string{"go/input.go"}, wantErr: `rule "bool-variation-detail" does not match its directory name "string-variation"`},
		{name: "missing input", ruleDir: "bool-variation-detail", output: "false", wantErr: "go/input.go, which does not exist"},
		{name: "missing output", ruleDir: "bool-variation-detail", output: "true", files: []string{"go/input.go"}, wantErr: "output is true but go/output.go.golden does not exist"},
		{name: "unexpected output", ruleDir: "bool-variation-detail", output: "false", files: []string{"go/input.go", "go/output.go.golden"}, wantErr: "output is false but go/output.go.golden exists"},
		{name: "stray js output", ruleDir: "bool-variation-detail", output: "false", files: []string{"go/input.go", "js/output.ts"}, wantErr: "js/output.ts exists but case.yaml has no expect.js"},
		{name: "stray js input", ruleDir: "bool-variation-detail", output: "false", files: []string{"go/input.go", "js/input.ts"}, wantErr: "js/input.ts exists but case.yaml has no expect.js"},
		{name: "stray java input", ruleDir: "bool-variation-detail", output: "false", files: []string{"go/input.go", "java/Before.java"}, wantErr: "java/Before.java exists but case.yaml has no expect.java"},
		{name: "stray java output", ruleDir: "bool-variation-detail", output: "false", files: []string{"go/input.go", "java/After.java"}, wantErr: "java/After.java exists but case.yaml has no expect.java"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), tt.ruleDir, "c1")
			for _, f := range tt.files {
				if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(f)), 0o755); err != nil {
					t.Fatal(err)
				}
				content := []byte(f)
				if tt.same {
					content = []byte("same")
				}
				if err := os.WriteFile(filepath.Join(dir, f), content, 0o644); err != nil {
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

func TestDiscoverFixtures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   []string
		dirs    []string
		want    int
		wantErr string
	}{
		{name: "ok", files: []string{"r/a/case.yaml", "r/b/case.yaml", "r/b/go/input.go", "r/b/go/output.go.golden", "r/.DS_Store", "r/a/.gitkeep"}, want: 2},
		{name: "stray file at root", files: []string{"r/a/case.yaml", "README"}, wantErr: "README: stray file at fixtures/ level"},
		{name: "stray file at rule level", files: []string{"r/a/case.yaml", "r/README"}, wantErr: "r/README: stray file at fixtures/<rule>/ level"},
		{name: "stray file in case", files: []string{"r/a/case.yaml", "r/a/notes.txt"}, wantErr: "r/a/notes.txt: unexpected file in case directory"},
		{name: "unknown dir in case", files: []string{"r/a/case.yaml", "r/a/rust/input.rs"}, wantErr: "r/a/rust: unexpected directory in case directory"},
		{name: "unknown file in language dir", files: []string{"r/a/case.yaml", "r/a/go/output.go"}, wantErr: "unexpected entry in go/"},
		{name: "dir without case", files: []string{"r/a/case.yaml"}, dirs: []string{"r/b"}, wantErr: "r/b has no case.yaml"},
		{name: "case.yml", files: []string{"r/a/case.yaml", "r/b/case.yaml", "r/b/case.yml"}, wantErr: "case.yml is not allowed"},
		{name: "case.yml only", files: []string{"r/b/case.yml"}, wantErr: "r/b has no case.yaml"},
		{name: "too deep", files: []string{"r/a/case.yaml", "r/a/x/case.yaml"}, wantErr: "unexpected directory in case directory"},
		{name: "too shallow", files: []string{"r/case.yaml"}, wantErr: "unexpected depth"},
		{name: "at root", files: []string{"case.yaml"}, wantErr: "unexpected depth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for _, d := range tt.dirs {
				if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range tt.files {
				p := filepath.Join(root, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := spec.DiscoverFixtures(root)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || len(got) != tt.want {
				t.Fatalf("got %v, %v; want %d paths", got, err, tt.want)
			}
		})
	}
}

func TestCheckUniqueIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ids     map[string]string
		wantErr bool
	}{
		{name: "unique", ids: map[string]string{"a": "x", "b": "y"}},
		{name: "empty"},
		{name: "duplicate across rules", ids: map[string]string{"r1/a": "x", "r2/a": "x"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cases := map[string]*spec.Case{}
			for p, id := range tt.ids {
				cases[p] = &spec.Case{ID: id}
			}
			if err := spec.CheckUniqueIDs(cases); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
