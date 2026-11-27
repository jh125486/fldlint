package sarif_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/fingerprint"
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

// resultDoc is the part of the SARIF output that TestConvert asserts on.
type resultDoc struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []struct {
		ColumnKind string `json:"columnKind"`
		Tool       struct {
			Driver struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Rules   []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []struct {
			RuleID  string `json:"ruleId"`
			Level   string `json:"level"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI       string `json:"uri"`
						URIBaseID string `json:"uriBaseId"`
					} `json:"artifactLocation"`
				} `json:"physicalLocation"`
			} `json:"locations"`
			PartialFingerprints map[string]string `json:"partialFingerprints"`
			BaselineState       *string           `json:"baselineState"`
			Properties          map[string]string `json:"properties"`
		} `json:"results"`
	} `json:"runs"`
}

func fp(rule, path string, key *string, occ int) string {
	return fingerprint.Fingerprint(fingerprint.Input{RuleID: rule, Path: path, Key: key, Occ: occ})
}

func strp(s string) *string { return &s }

type convertCase struct {
	name    string
	files   map[string]string
	in      func(root string) string
	opts    func(t *testing.T, root string) sarif.Options
	wantErr string // substring of an ErrInput; empty means success
	check   func(t *testing.T, d resultDoc)
}

// TestConvert covers the contract's document shape, the callee and key forms
// the analyzer accepts, input forms and every failure.
func TestConvert(t *testing.T) {
	t.Parallel()
	const two = "package p\n\nfunc f() {\n\t_ = c.StringVariation(\"k\", c, \"\")\n\t_ = c.BoolVariation(\"k\", c, false)\n}\n"
	// posn returns "<root>/<file><at>" as fldlint prints it.
	posn := func(root, file, at string) string { return filepath.ToSlash(filepath.Join(root, file)) + at }
	wantFP := func(rule, path string, key *string) func(t *testing.T, d resultDoc) {
		return func(t *testing.T, d resultDoc) {
			t.Helper()
			if got, want := d.Runs[0].Results[0].PartialFingerprints["fldlint/fingerprint"], fp(rule, path, key, 0); got != want {
				t.Errorf("fingerprint = %s, want %s", got, want)
			}
		}
	}

	tests := []convertCase{
		{
			name:  "document shape, order, levels and properties",
			files: map[string]string{"a.go": two, "b.go": two},
			in: func(r string) string {
				return `{"p2": {"fldlint": [{"category": "bool-variation", "posn": "` + posn(r, "b.go", ":5:6") + `", "message": "Migrate boolean flag \"k\" to OpenFeature [no-fix: comment]"}]},
				"p1": {"fldlint": [
				  {"category": "string-variation", "posn": "` + posn(r, "a.go", ":4:6") + `", "message": "Migrate string flag \"k\" to OpenFeature"},
				  {"category": "bool-variation", "posn": "` + posn(r, "a.go", ":5:6") + `", "message": "Cannot prove LD context -> EvaluationContext conversion [skip: context-unproven]"}],
				  "other": [{"category": "x", "posn": "nope", "message": "ignored"}]}}`
			},
			opts: func(_ *testing.T, r string) sarif.Options {
				return sarif.Options{Root: r, Level: map[string]string{"": "note", "string-variation": "error"}}
			},
			check: func(t *testing.T, d resultDoc) {
				run := d.Runs[0]
				if d.Schema != sarif.Schema || d.Version != "2.1.0" || run.ColumnKind != "utf16CodeUnits" ||
					run.Tool.Driver.Name != "fldlint" || run.Tool.Driver.Version != "0.0.0" {
					t.Errorf("header = %+v", d)
				}
				var ids []string
				for _, r := range run.Tool.Driver.Rules {
					ids = append(ids, r.ID)
				}
				if want := []string{"bool-variation", "string-variation"}; !slices.Equal(ids, want) {
					t.Errorf("rules = %v, want %v", ids, want)
				}
				type want struct {
					uri, rule, level, text string
					props                  map[string]string
				}
				wants := []want{
					{"a.go", "string-variation", "error", `Migrate string flag "k" to OpenFeature`, map[string]string{"risk": "low"}},
					{"a.go", "bool-variation", "note", "Cannot prove LD context -> EvaluationContext conversion", map[string]string{"risk": "low", "skip": "context-unproven"}},
					{"b.go", "bool-variation", "note", `Migrate boolean flag "k" to OpenFeature`, map[string]string{"risk": "low", "noFix": "comment"}},
				}
				if len(run.Results) != len(wants) {
					t.Fatalf("%d results, want %d", len(run.Results), len(wants))
				}
				for i, w := range wants {
					r := run.Results[i]
					loc := r.Locations[0].PhysicalLocation.ArtifactLocation
					if loc.URI != w.uri || loc.URIBaseID != "%SRCROOT%" || r.RuleID != w.rule || r.Level != w.level ||
						r.Message.Text != w.text || !maps.Equal(r.Properties, w.props) || r.BaselineState != nil ||
						len(r.PartialFingerprints) != 1 || len(r.PartialFingerprints["fldlint/fingerprint"]) != 32 {
						t.Errorf("result %d = %+v", i, r)
					}
				}
			},
		},
		{
			name:  "parenthesized callee keeps its key",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = (c.BoolVariation)(\"paren-fun\", c, false)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("bool-variation", "a.go", strp("paren-fun")),
		},
		{
			name:  "Ctx method shifts the key argument",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = c.BoolVariationCtx(ctx, \"ctx-key\", c, false)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("bool-variation", "a.go", strp("ctx-key")),
		},
		{
			name:  "parenthesized key literal is static",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = c.BoolVariation((\"paren-key\"), c, false)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("bool-variation", "a.go", strp("paren-key")),
		},
		{
			name:  "method expression skips the receiver argument",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = (*ldclient.LDClient).BoolVariation(c, \"mexpr-key\", u, false)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("bool-variation", "a.go", strp("mexpr-key")),
		},
		{
			name:  "Ctx method expression skips receiver and context",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = (*ldclient.LDClient).BoolVariationCtx(c, ctx, \"mexpr-ctx\", u, false)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("bool-variation", "a.go", strp("mexpr-ctx")),
		},
		{
			name:  "method expression of a report-only rule",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = (*ldclient.LDClient).IntVariation(c, \"iv-key\", u, 1)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"int-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("int-variation", "a.go", strp("iv-key")),
		},
		{
			name:  "Ctx method expression of a report-only rule",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = (*ldclient.LDClient).IntVariationCtx(c, ctx, \"ivc-key\", u, 1)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"int-variation", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("int-variation", "a.go", strp("ivc-key")),
		},
		{
			name:  "method expression of a detail rule",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = (*ldclient.LDClient).BoolVariationDetail(c, \"bd-key\", u, false)\n}\n"},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation-detail", posn(r, "a.go", ":4:6"), "m"}) },
			check: wantFP("bool-variation-detail", "a.go", strp("bd-key")),
		},
		{
			name:  "multi-line raw key with a no-fix suffix",
			files: map[string]string{"a.go": "package p\n\nfunc f() {\n\t_ = c.BoolVariation(`raw\nmulti`, ldcontext.New(/* c */ \"user\"), false)\n}\n"},
			in: func(r string) string {
				return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":4:6"), "Migrate boolean flag `raw\nmulti` to OpenFeature [no-fix: comment]"})
			},
			check: func(t *testing.T, d resultDoc) {
				r := d.Runs[0].Results[0]
				if r.Message.Text != "Migrate boolean flag `raw\nmulti` to OpenFeature" || r.Properties["noFix"] != "comment" {
					t.Errorf("message %q, properties %v", r.Message.Text, r.Properties)
				}
				wantFP("bool-variation", "a.go", strp("raw\nmulti"))(t, d)
			},
		},
		{
			name:  "go vet output: one object per package",
			files: map[string]string{"a.go": two, "b.go": two},
			in: func(r string) string {
				first := strings.Replace(diagJSON([3]string{"string-variation", posn(r, "a.go", ":4:6"), "m"}), `{"p"`, `{"p1"`, 1)
				second := strings.Replace(diagJSON([3]string{"bool-variation", posn(r, "b.go", ":5:6"), "m"}), `{"p"`, `{"p2"`, 1)
				return first + "\n" + second + "\n"
			},
			check: func(t *testing.T, d resultDoc) {
				if n := len(d.Runs[0].Results); n != 2 {
					t.Errorf("%d results, want 2", n)
				}
			},
		},
		{
			name:  "same finding from two package variants is one result",
			files: map[string]string{"a.go": two},
			in: func(r string) string {
				plain := `{"fldlint": [{"category": "bool-variation", "posn": "` + posn(r, "a.go", ":5:6") + `", "message": "m"}]}`
				withEnd := `{"fldlint": [{"category": "bool-variation", "posn": "` + posn(r, "a.go", ":5:6") + `", "end": "` + posn(r, "a.go", ":5:30") + `", "message": "m"}]}`
				return `{"p": ` + plain + `, "p [p.test]": ` + withEnd + `}`
			},
			check: func(t *testing.T, d resultDoc) {
				if n := len(d.Runs[0].Results); n != 1 {
					t.Errorf("%d results, want 1", n)
				}
			},
		},
		{
			name: "empty run",
			in:   func(string) string { return "{}" },
			check: func(t *testing.T, d resultDoc) {
				if len(d.Runs[0].Results) != 0 || len(d.Runs[0].Tool.Driver.Rules) != 0 {
					t.Errorf("run = %+v", d.Runs[0])
				}
			},
		},
		{
			name:  "file symlink escaping the root",
			files: map[string]string{"a.go": two},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "link.go", ":5:6"), "m"}) },
			opts: func(t *testing.T, r string) sarif.Options {
				outside := filepath.Join(t.TempDir(), "outside.go")
				if err := os.WriteFile(outside, []byte(two), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(r, "link.go")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return sarif.Options{Root: r}
			},
			wantErr: "outside the scan root",
		},
		{
			name:  "root reached through a symlink",
			files: map[string]string{"a.go": two},
			in:    func(r string) string { return diagJSON([3]string{"bool-variation", posn(r, "a.go", ":5:6"), "m"}) },
			opts: func(t *testing.T, r string) sarif.Options {
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(r, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return sarif.Options{Root: link}
			},
			check: func(t *testing.T, d resultDoc) {
				if uri := d.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "a.go" {
					t.Errorf("uri = %q, want a.go", uri)
				}
			},
		},
	}

	failures := []struct{ name, in, want string }{
		{"empty input", ``, "reading fldlint -json"},
		{"not json", `nope`, "reading fldlint -json"},
		{"not an object", `[]`, "reading fldlint -json"},
		{"null", `null`, "expected a JSON object"},
		{"trailing garbage", `{} nope`, "reading fldlint -json"},
		{"analysis error", `{"flags": {"fldlint": {"error": "analysis skipped due to errors in package"}}}`, "analysis skipped due to errors in package"},
		{"diagnostics not an array", `{"p": {"fldlint": "x"}}`, "package p"},
		{"unknown rule", `{"p": {"fldlint": [{"category": "no-such-rule", "posn": "{{a}}:4:6", "message": "m"}]}}`, "unknown rule id"},
		{"malformed posn", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "a.go", "message": "m"}]}}`, "malformed position"},
		{"zero column", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "{{a}}:4:0", "message": "m"}]}}`, "malformed position"},
		{"outside root", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "/elsewhere/x.go:4:6", "message": "m"}]}}`, "outside the scan root"},
		{"missing file", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "{{r}}/gone.go:4:6", "message": "m"}]}}`, "reading source"},
		{"line past end", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "{{a}}:99:1", "message": "m"}]}}`, "outside the file"},
		{"column past end of line", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "{{a}}:5:200", "message": "m"}]}}`, "outside the file"},
		{"column past end of file", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "{{a}}:7:999", "message": "m"}]}}`, "outside the file"},
		{"source does not parse", `{"p": {"fldlint": [{"category": "bool-variation", "posn": "{{bad}}:1:1", "message": "m"}]}}`, "parsing"},
	}
	for _, f := range failures {
		tests = append(tests, convertCase{
			name:  "error: " + f.name,
			files: map[string]string{"a.go": two, "bad.go": "not go"},
			in: func(r string) string {
				return strings.NewReplacer("{{a}}", posn(r, "a.go", ""), "{{bad}}", posn(r, "bad.go", ""), "{{r}}", filepath.ToSlash(r)).Replace(f.in)
			},
			wantErr: f.want,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			opts := sarif.Options{Root: root}
			if tt.opts != nil {
				opts = tt.opts(t, root)
			}
			out, err := sarif.Convert(strings.NewReader(tt.in(root)), opts)
			if tt.wantErr != "" {
				if !errors.Is(err, sarif.ErrInput) || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want ErrInput containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var d resultDoc
			if err := json.Unmarshal(out, &d); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			tt.check(t, d)
		})
	}
}

// TestConvert_baseline checks baselineState on every result.
func TestConvert_baseline(t *testing.T) {
	t.Parallel()
	const src = "package p\n\nfunc f() {\n\t_ = c.BoolVariation(\"one\", c, false)\n\t_ = c.BoolVariation(\"two\", c, false)\n}\n"
	one := fp("bool-variation", "a.go", strp("one"), 0)
	tests := []struct {
		name     string
		baseline map[string]struct{}
		want     []string // "" means baselineState is omitted
	}{
		{"no baseline", nil, []string{"", ""}},
		{"empty baseline", map[string]struct{}{}, []string{"new", "new"}},
		{"one baselined", map[string]struct{}{one: {}}, []string{"unchanged", "new"}},
		{"unrelated entry", map[string]struct{}{"ffffffffffffffffffffffffffffffff": {}}, []string{"new", "new"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "a.go"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			a := filepath.ToSlash(filepath.Join(root, "a.go"))
			in := diagJSON([3]string{"bool-variation", a + ":4:6", "m"}, [3]string{"bool-variation", a + ":5:6", "m"})
			out, err := sarif.Convert(strings.NewReader(in), sarif.Options{Root: root, Baseline: tt.baseline})
			if err != nil {
				t.Fatal(err)
			}
			var d resultDoc
			if err := json.Unmarshal(out, &d); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range d.Runs[0].Results {
				s := ""
				if r.BaselineState != nil {
					s = *r.BaselineState
				}
				got = append(got, s)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("states = %v, want %v", got, tt.want)
			}
		})
	}
}
