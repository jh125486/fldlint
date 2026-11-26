package sarif_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/sarif"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const (
	fixtures  = "../../../spec/fixtures"
	contracts = "../../../spec/contracts"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// memFS serves in-memory sources under a root, for Options.ReadFile.
func memFS(files map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if s, ok := files[p]; ok {
			return []byte(s), nil
		}
		return nil, os.ErrNotExist
	}
}

// diagJSON builds a `fldlint -json` document with one package and the given
// diagnostics (category, posn, message).
func diagJSON(diags ...[3]string) string {
	var parts []string
	for _, d := range diags {
		parts = append(parts, fmt.Sprintf(`{"category": %q, "posn": %q, "message": %q}`, d[0], d[1], d[2]))
	}
	return `{"p": {"fldlint": [` + strings.Join(parts, ",") + `]}}`
}

// TestConvert_golden converts a real `fldlint -json` capture (testdata/fldlint.json)
// over two shared fixtures. The first has a non-ASCII prefix before the call, so
// the byte column (35) and the contract's UTF-16 column (32) differ; the second
// is a skipped call with a dynamic key.
func TestConvert_golden(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for dst, src := range map[string]string{
		"flags/input.go": "bool-variation/non-ascii-prefix/go/input.go",
		"dyn/dyn.go":     "bool-variation/dynamic-key/go/input.go",
	} {
		p := filepath.Join(root, filepath.FromSlash(dst))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, readFile(t, filepath.Join(fixtures, filepath.FromSlash(src))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := strings.ReplaceAll(string(readFile(t, "testdata/fldlint.json")), "{{ROOT}}", filepath.ToSlash(root))

	tests := []struct {
		name   string
		opts   sarif.Options
		golden string
	}{
		{name: "default", opts: sarif.Options{Root: root}, golden: "testdata/default.sarif.golden"},
		{
			name: "severity and baseline",
			opts: sarif.Options{
				Root:     root,
				Version:  "1.2.3",
				Level:    map[string]string{"": "error"},
				Baseline: map[string]struct{}{"87978fb58d6fc554876accf055a31851": {}},
			},
			golden: "testdata/baseline.sarif.golden",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := sarif.Convert(strings.NewReader(in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.WriteFile(tt.golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if want := readFile(t, tt.golden); !bytes.Equal(got, want) {
				t.Fatalf("output differs from %s (run with -update to refresh):\n%s", tt.golden, got)
			}
		})
	}
}

// vector is one row of the shared fingerprint-vectors.json.
type vector struct {
	RuleID      string  `json:"ruleId"`
	Path        string  `json:"path"`
	Key         *string `json:"key"`
	Occ         int     `json:"occ"`
	Fingerprint string  `json:"fingerprint"`
}

// TestConvert_fingerprintVectors emits Go sources whose calls have the same
// (rule, path, key, occ) as rows of the shared vectors, and checks the SARIF
// partialFingerprints equal those rows. The expected values are read from
// spec/contracts, never copied here.
func TestConvert_fingerprintVectors(t *testing.T) {
	t.Parallel()
	var vectors []vector
	if err := json.Unmarshal(readFile(t, filepath.Join(contracts, "fingerprint-vectors.json")), &vectors); err != nil {
		t.Fatal(err)
	}
	find := func(rule, path string, key *string, occ int) string {
		for _, v := range vectors {
			if v.RuleID == rule && v.Path == path && v.Occ == occ && (v.Key == nil) == (key == nil) && (key == nil || *v.Key == *key) {
				return v.Fingerprint
			}
		}
		t.Fatalf("no vector for %s %s %v occ %d", rule, path, key, occ)
		return ""
	}
	s := func(v string) *string { return &v }

	type call struct {
		method, args string // method name and its argument list source
		rule         string
		key          *string // vector key
		occ          int
	}
	tests := []struct {
		name  string
		path  string // vector path, also the file name
		calls []call
	}{
		{
			name: "same key twice then a dynamic key twice",
			path: "src/checkout/service.ts",
			calls: []call{
				{"BoolVariation", `"checkout-v2", c, false`, "bool-variation", s("checkout-v2"), 0},
				{"BoolVariation", `name, c, false`, "bool-variation", nil, 0},
				{"BoolVariation", `"checkout-v2", c, true`, "bool-variation", s("checkout-v2"), 1},
				{"BoolVariation", `name, c, true`, "bool-variation", nil, 1},
				{"StringVariation", `"checkout-v2", c, ""`, "string-variation", s("checkout-v2"), 0},
			},
		},
		{
			name:  "go file",
			path:  "internal/flags/flags.go",
			calls: []call{{"BoolVariation", `"new-ui", c, false`, "bool-variation", s("new-ui"), 0}, {"IntVariation", `"flag-a", c, 0`, "int-variation", s("flag-a"), 0}, {"IntVariation", `"flag-b", c, 0`, "int-variation", s("flag-b"), 0}},
		},
		{
			name:  "bulk rule has no key",
			path:  "svc/main.go",
			calls: []call{{"AllFlagsState", `c`, "all-flags-state", nil, 0}},
		},
		{
			name:  "escaped key decodes",
			path:  "a.ts", // the vectors' path; only the file name matters to Convert
			calls: []call{{"BoolVariation", `"a\x41", c, false`, "bool-variation", s("aA"), 0}},
		},
		{
			name:  "raw string key decodes",
			path:  "a.ts",
			calls: []call{{"BoolVariation", "`aA`, c, false", "bool-variation", s("aA"), 0}},
		},
		{
			name:  "astral key and empty key",
			path:  "a.ts",
			calls: []call{{"BoolVariation", `"😀", c, false`, "bool-variation", s("😀"), 0}, {"BoolVariation", `"", c, false`, "bool-variation", s(""), 0}},
		},
		{
			name:  "non-ASCII path is not normalized",
			path:  "src/é.ts",
			calls: []call{{"BoolVariation", `"flag", c, false`, "bool-variation", s("flag"), 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			abs := filepath.Join(root, filepath.FromSlash(tt.path))
			var src strings.Builder
			src.WriteString("package p\n\nfunc f() {\n")
			var diags [][3]string
			var want []string
			for i, c := range tt.calls {
				fmt.Fprintf(&src, "\t_ = c.%s(%s)\n", c.method, c.args)
				diags = append(diags, [3]string{c.rule, fmt.Sprintf("%s:%d:6", filepath.ToSlash(abs), 4+i), "msg"})
				want = append(want, find(c.rule, tt.path, c.key, c.occ))
			}
			src.WriteString("}\n")
			out, err := sarif.Convert(strings.NewReader(diagJSON(diags...)), sarif.Options{
				Root:     root,
				ReadFile: memFS(map[string]string{abs: src.String()}),
			})
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Runs []struct {
					Results []struct {
						PartialFingerprints map[string]string `json:"partialFingerprints"`
					} `json:"results"`
				} `json:"runs"`
			}
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range doc.Runs[0].Results {
				got = append(got, r.PartialFingerprints["fldlint/fingerprint"])
			}
			if !slices.Equal(got, want) {
				t.Fatalf("fingerprints = %v\nwant           %v\n%s", got, want, src.String())
			}
		})
	}
}

// TestConvert_illformedKey checks that a key that is not valid UTF-8 counts as
// absent, per fingerprint-illformed-vectors.json.
func TestConvert_illformedKey(t *testing.T) {
	t.Parallel()
	var vectors []struct {
		RuleID      string `json:"ruleId"`
		Path        string `json:"path"`
		KeyUtf8Hex  string `json:"keyUtf8Hex"`
		Occ         int    `json:"occ"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(readFile(t, filepath.Join(contracts, "fingerprint-illformed-vectors.json")), &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) == 0 {
		t.Fatal("no ill-formed vectors")
	}
	for i, v := range vectors {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			if v.Occ != 0 || v.RuleID != "bool-variation" {
				t.Skip("only occ 0 bool-variation rows are replayed")
			}
			var lit strings.Builder
			for j := 0; j+1 < len(v.KeyUtf8Hex); j += 2 {
				lit.WriteString(`\x` + v.KeyUtf8Hex[j:j+2])
			}
			root := t.TempDir()
			abs := filepath.Join(root, filepath.FromSlash(v.Path))
			src := "package p\n\nfunc f() {\n\t_ = c.BoolVariation(\"" + lit.String() + "\", c, false)\n}\n"
			out, err := sarif.Convert(strings.NewReader(diagJSON([3]string{"bool-variation", filepath.ToSlash(abs) + ":4:6", "m"})),
				sarif.Options{Root: root, ReadFile: memFS(map[string]string{abs: src})})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), `"fldlint/fingerprint": "`+v.Fingerprint+`"`) {
				t.Fatalf("want fingerprint %s in\n%s", v.Fingerprint, out)
			}
		})
	}
}

// TestConvert_structure asserts the contract's document shape on a run with
// several rules, skips and an unfixable result, without a baseline.
func TestConvert_structure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, a := filepath.Join(root, "b.go"), filepath.Join(root, "a.go")
	const body = "package p\n\nfunc f() {\n\t_ = c.StringVariation(\"k\", c, \"\")\n\t_ = c.BoolVariation(\"k\", c, false)\n}\n"
	in := `{"p2": {"fldlint": [{"category": "bool-variation", "posn": "` + filepath.ToSlash(b) + `:5:6", "message": "Migrate boolean flag \"k\" to OpenFeature [no-fix: comment]"}]},
	"p1": {"fldlint": [
	  {"category": "string-variation", "posn": "` + filepath.ToSlash(a) + `:4:6", "message": "Migrate string flag \"k\" to OpenFeature"},
	  {"category": "bool-variation", "posn": "` + filepath.ToSlash(a) + `:5:6", "message": "Cannot prove LD context -> EvaluationContext conversion [skip: context-unproven]"}],
	  "other": [{"category": "x", "posn": "nope", "message": "ignored"}]}}`
	out, err := sarif.Convert(strings.NewReader(in), sarif.Options{
		Root:     root,
		ReadFile: memFS(map[string]string{a: body, b: body}),
		Level:    map[string]string{"": "note", "string-variation": "error"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != sarif.Schema || doc["version"] != "2.1.0" {
		t.Fatalf("header: %v %v", doc["$schema"], doc["version"])
	}
	run := doc["runs"].([]any)[0].(map[string]any)
	if run["columnKind"] != "utf16CodeUnits" {
		t.Fatalf("columnKind = %v", run["columnKind"])
	}
	driver := run["tool"].(map[string]any)["driver"].(map[string]any)
	if driver["name"] != "fldlint" || driver["version"] != "0.0.0" {
		t.Fatalf("driver = %v", driver)
	}
	var ruleIDs []string
	for _, r := range driver["rules"].([]any) {
		ruleIDs = append(ruleIDs, r.(map[string]any)["id"].(string))
	}
	if want := []string{"bool-variation", "string-variation"}; !slices.Equal(ruleIDs, want) {
		t.Fatalf("rules = %v, want %v", ruleIDs, want)
	}

	type want struct {
		uri, rule, level, text string
		props                  map[string]any
	}
	wants := []want{
		{"a.go", "string-variation", "error", `Migrate string flag "k" to OpenFeature`, map[string]any{"risk": "low"}},
		{"a.go", "bool-variation", "note", "Cannot prove LD context -> EvaluationContext conversion", map[string]any{"risk": "low", "skip": "context-unproven"}},
		{"b.go", "bool-variation", "note", `Migrate boolean flag "k" to OpenFeature`, map[string]any{"risk": "low", "noFix": "comment"}},
	}
	results := run["results"].([]any)
	if len(results) != len(wants) {
		t.Fatalf("%d results, want %d:\n%s", len(results), len(wants), out)
	}
	for i, w := range wants {
		r := results[i].(map[string]any)
		loc := r["locations"].([]any)[0].(map[string]any)["physicalLocation"].(map[string]any)
		art := loc["artifactLocation"].(map[string]any)
		if art["uri"] != w.uri || art["uriBaseId"] != "%SRCROOT%" {
			t.Errorf("result %d location = %v", i, art)
		}
		if r["ruleId"] != w.rule || r["level"] != w.level || r["message"].(map[string]any)["text"] != w.text {
			t.Errorf("result %d = %v", i, r)
		}
		if _, ok := r["baselineState"]; ok {
			t.Errorf("result %d has baselineState without a baseline", i)
		}
		fps := r["partialFingerprints"].(map[string]any)
		if len(fps) != 1 || len(fps["fldlint/fingerprint"].(string)) != 32 {
			t.Errorf("result %d fingerprints = %v", i, fps)
		}
		if got, _ := json.Marshal(r["properties"]); !jsonEqual(t, got, w.props) {
			t.Errorf("result %d properties = %s, want %v", i, got, w.props)
		}
	}
}

func jsonEqual(t *testing.T, got []byte, want map[string]any) bool {
	t.Helper()
	w, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	return string(got) == string(w)
}

// TestConvert_baseline checks baselineState and that it is set on every result.
func TestConvert_baseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	abs := filepath.Join(root, "a.go")
	src := "package p\n\nfunc f() {\n\t_ = c.BoolVariation(\"one\", c, false)\n\t_ = c.BoolVariation(\"two\", c, false)\n}\n"
	in := diagJSON(
		[3]string{"bool-variation", filepath.ToSlash(abs) + ":4:6", "m"},
		[3]string{"bool-variation", filepath.ToSlash(abs) + ":5:6", "m"},
	)
	first := func(base map[string]struct{}) []byte {
		out, err := sarif.Convert(strings.NewReader(in), sarif.Options{Root: root, ReadFile: memFS(map[string]string{abs: src}), Baseline: base})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	// Learn the fingerprint of "one" from an unbaselined run.
	var doc struct {
		Runs []struct {
			Results []struct {
				PartialFingerprints map[string]string `json:"partialFingerprints"`
				BaselineState       string            `json:"baselineState"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(first(nil), &doc); err != nil {
		t.Fatal(err)
	}
	one := doc.Runs[0].Results[0].PartialFingerprints["fldlint/fingerprint"]

	for name, base := range map[string]map[string]struct{}{
		"empty baseline":   {},
		"one baselined":    {one: {}},
		"unrelated stored": {"ffffffffffffffffffffffffffffffff": {}},
	} {
		if err := json.Unmarshal(first(base), &doc); err != nil {
			t.Fatal(err)
		}
		var states []string
		for _, r := range doc.Runs[0].Results {
			states = append(states, r.BaselineState)
		}
		want := []string{"new", "new"}
		if name == "one baselined" {
			want = []string{"unchanged", "new"}
		}
		if !slices.Equal(states, want) {
			t.Errorf("%s: states = %v, want %v", name, states, want)
		}
	}
}

// TestConvert_duplicatesAndEmpty covers the dedupe of repeated diagnostics (one
// per package variant) and the empty document.
func TestConvert_duplicatesAndEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	abs := filepath.Join(root, "a.go")
	src := "package p\n\nfunc f() {\n\t_ = c.BoolVariation(\"k\", c, false)\n}\n"
	d := [3]string{"bool-variation", filepath.ToSlash(abs) + ":4:6", "m"}
	two := `{"p": ` + diagBody(d) + `, "p [p.test]": ` + diagBody(d) + `}`
	out, err := sarif.Convert(strings.NewReader(two), sarif.Options{Root: root, ReadFile: memFS(map[string]string{abs: src})})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(out), `"ruleId"`); n != 1 {
		t.Fatalf("%d results, want 1 after dedupe:\n%s", n, out)
	}

	empty, err := sarif.Convert(strings.NewReader("{}"), sarif.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), `"results": []`) || !strings.Contains(string(empty), `"rules": []`) {
		t.Fatalf("empty run:\n%s", empty)
	}
}

func diagBody(d [3]string) string {
	return `{"fldlint": [` + fmt.Sprintf(`{"category": %q, "posn": %q, "message": %q}`, d[0], d[1], d[2]) + `]}`
}

// TestConvert_errors checks that every input failure wraps ErrInput.
func TestConvert_errors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	abs := filepath.Join(root, "a.go")
	src := "package p\n\nfunc f() {\n\t_ = c.BoolVariation(\"k\", c, false)\n}\n"
	posn := func(s string) string { return filepath.ToSlash(abs) + s }
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty input", ``, "reading fldlint -json"},
		{"not json", `nope`, "reading fldlint -json"},
		{"not an object", `[]`, "reading fldlint -json"},
		{"null", `null`, "expected a JSON object"},
		{"trailing data", `{} {}`, "trailing data"},
		{"analysis error", `{"flags": {"fldlint": {"error": "analysis skipped due to errors in package"}}}`, "analysis skipped due to errors in package"},
		{"bad diagnostics", `{"p": {"fldlint": {"x": 1}, "y": 1}}`, "analysis of p failed"},
		{"diagnostics not an array", `{"p": {"fldlint": "x"}}`, "package p"},
		{"unknown rule", diagJSON([3]string{"no-such-rule", posn(":4:6"), "m"}), "unknown rule id"},
		{"malformed posn", diagJSON([3]string{"bool-variation", "a.go", "m"}), "malformed position"},
		{"zero column", diagJSON([3]string{"bool-variation", posn(":4:0"), "m"}), "malformed position"},
		{"outside root", diagJSON([3]string{"bool-variation", filepath.ToSlash(filepath.Join(filepath.Dir(root), "x.go")) + ":4:6", "m"}), "outside the scan root"},
		{"missing file", diagJSON([3]string{"bool-variation", filepath.ToSlash(filepath.Join(root, "gone.go")) + ":4:6", "m"}), "reading source"},
		{"line past end", diagJSON([3]string{"bool-variation", posn(":99:1"), "m"}), "outside the file"},
		{"column past end", diagJSON([3]string{"bool-variation", posn(":4:999"), "m"}), "outside the file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := sarif.Convert(strings.NewReader(tt.in), sarif.Options{Root: root, ReadFile: memFS(map[string]string{abs: src})})
			if !errors.Is(err, sarif.ErrInput) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want ErrInput containing %q", err, tt.want)
			}
		})
	}

	t.Run("source does not parse", func(t *testing.T) {
		t.Parallel()
		_, err := sarif.Convert(strings.NewReader(diagJSON([3]string{"bool-variation", posn(":1:1"), "m"})),
			sarif.Options{Root: root, ReadFile: memFS(map[string]string{abs: "not go"})})
		if !errors.Is(err, sarif.ErrInput) || !strings.Contains(err.Error(), "parsing") {
			t.Fatalf("err = %v", err)
		}
	})
}
