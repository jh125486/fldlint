package spec_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const okRules = `version: 1
skipCategories:
  detail: {severity: warning, message: d}
  bulk: {severity: warning, message: b}
  dynamic-key: {severity: warning, message: k}
  unbound-client: {severity: warning, message: u}
  type-mismatch: {severity: warning, message: t}
  context-unproven: {severity: warning, message: c}
rules:
  - id: a
    valueType: boolean
    risk: low
    keyArg: 0
    defaultArg: 2
    contextArg: 1
    message: "flag {{key}} migrates"
    languages:
      js: {methods: [aJS], async: true, template: "{{client}}.get({{key}}, {{default}}, {{context}})"}
      go: {methods: [A, ACtx], ctxArg: {ACtx: 0}, requires: [context-unproven], template: "{{client}}.X({{goctx}}, {{key}}, {{default}}, {{evalctx}})"}
      java: {methods: [aJava], template: "#{any(dev.openfeature.sdk.Client)}.get(#{key}, #{default}, #{context})"}
  - id: b
    risk: high
    skip: detail
    languages:
      go: {methods: [B]}
`

func TestParseRules(t *testing.T) {
	t.Parallel()
	// each case replaces old with new in okRules and expects the error text.
	tests := []struct {
		name     string
		old, new string
		wantErr  string
	}{
		{name: "ok"},
		{name: "bad yaml", old: "version: 1", new: "version: [", wantErr: "parse rules"},
		{name: "bad version", old: "version: 1", new: "version: 2", wantErr: "unsupported version"},
		{name: "string version", old: "version: 1", new: `version: "1"`, wantErr: "must be an integer"},
		{name: "unknown top key", old: "version: 1", new: "version: 1\nextra: 1", wantErr: `unknown key "extra"`},
		{name: "duplicate top key", old: "version: 1", new: "version: 1\nversion: 1", wantErr: "duplicate key"},
		{name: "missing skip category", old: "  bulk: {severity: warning, message: b}\n", new: "", wantErr: `missing required key "bulk"`},
		{name: "extra skip category", old: "  bulk:", new: "  other: {severity: warning, message: o}\n  bulk:", wantErr: `unknown key "other"`},
		{name: "bad severity", old: "severity: warning, message: d", new: "severity: fatal, message: d", wantErr: "is not one of"},
		{name: "empty skip message", old: "message: d}", new: `message: ""}`, wantErr: "must not be empty"},
		{name: "duplicate id", old: "id: b", new: "id: a", wantErr: "duplicate rule id"},
		{name: "bad id pattern", old: "id: b", new: "id: Bad_ID", wantErr: "rule id pattern"},
		{name: "numeric id", old: "id: b", new: "id: 5", wantErr: "must be a string"},
		{name: "bad valueType", old: "valueType: boolean", new: "valueType: bool", wantErr: "is not one of"},
		{name: "bad risk", old: "risk: low", new: "risk: extreme", wantErr: "is not one of"},
		{name: "missing risk", old: "    risk: low\n", new: "", wantErr: `missing required key "risk"`},
		{name: "unknown rule key", old: "keyArg: 0", new: "keyArg: 0\n    bogus: 1", wantErr: `unknown key "bogus"`},
		{name: "float arg", old: "keyArg: 0", new: "keyArg: 0.5", wantErr: "must be an integer"},
		{name: "negative arg", old: "keyArg: 0", new: "keyArg: -1", wantErr: "must be >= 0"},
		{name: "missing keyArg", old: "    keyArg: 0\n", new: "", wantErr: "requires keyArg"},
		{name: "missing defaultArg", old: "    defaultArg: 2\n", new: "", wantErr: "requires defaultArg"},
		{name: "missing contextArg", old: "    contextArg: 1\n", new: "", wantErr: "requires contextArg"},
		{name: "missing valueType", old: "    valueType: boolean\n", new: "", wantErr: "requires valueType"},
		{name: "missing message", old: "    message: \"flag {{key}} migrates\"\n", new: "", wantErr: "requires message"},
		{name: "message bad placeholder", old: "flag {{key}} migrates", new: "flag {{client}} migrates", wantErr: "{{key}} placeholder"},
		{name: "message lone close brace", old: "flag {{key}} migrates", new: "flag } migrates", wantErr: "{{key}} placeholder"},
		{name: "message stray brace", old: "flag {{key}} migrates", new: "flag { migrates", wantErr: "{{key}} placeholder"},
		{name: "empty message", old: `"flag {{key}} migrates"`, new: `""`, wantErr: "non-empty"},
		{name: "numeric message", old: `message: "flag {{key}} migrates"`, new: "message: 5", wantErr: "must be a string"},
		{name: "skip rule with message", old: "    skip: detail\n", new: "    skip: detail\n    message: \"m\"\n", wantErr: "must not carry a message"},
		{name: "bad skip name", old: "skip: detail", new: "skip: nope", wantErr: "is not one of"},
		{name: "duplicate go method", old: "methods: [B]", new: "methods: [A]", wantErr: "already bound"},
		{name: "duplicate method in one rule", old: "methods: [B]", new: "methods: [B, B]", wantErr: "duplicate item"},
		{name: "same js method in two rules", old: "go: {methods: [B]}", new: "js: {methods: [aJS]}", wantErr: "js method"},
		{name: "same java method in two rules", old: "go: {methods: [B]}", new: "java: {methods: [aJava]}", wantErr: "java method"},
		{name: "empty methods", old: "methods: [B]", new: "methods: []", wantErr: "at least 1"},
		{name: "empty method name", old: "methods: [B]", new: `methods: [""]`, wantErr: "must not be empty"},
		{name: "unknown placeholder", old: "{{goctx}}", new: "{{bogus}}", wantErr: "template must be non-empty"},
		{name: "spaced placeholder", old: "{{goctx}}", new: "{{ goctx }}", wantErr: "template must be non-empty"},
		{name: "stray brace", old: "{{client}}.X(", new: "{{client}}.X({", wantErr: "template must be non-empty"},
		{name: "empty template", old: `template: "{{client}}.get({{key}}, {{default}}, {{context}})"`, new: `template: ""`, wantErr: "template must be non-empty"},
		{name: "java double-brace placeholder", old: "#{key}", new: "{{key}}", wantErr: "template must be non-empty"},
		{name: "java unknown placeholder", old: "#{default}", new: "#{nope}", wantErr: "template must be non-empty"},
		{name: "java stray hash", old: "#{key}", new: "#key", wantErr: "template must be non-empty"},
		{name: "ctxArg unlisted", old: "ACtx: 0", new: "Zed: 0", wantErr: "unlisted method"},
		{name: "ctxArg negative", old: "ACtx: 0", new: "ACtx: -1", wantErr: "must be >= 0"},
		{name: "requires unknown", old: "requires: [context-unproven]", new: "requires: [nope]", wantErr: "not a skip category"},
		{name: "requires duplicate", old: "requires: [context-unproven]", new: "requires: [bulk, bulk]", wantErr: "duplicate item"},
		{name: "async not bool", old: "async: true", new: "async: yes", wantErr: "must be a boolean"},
		{name: "unknown binding key", old: "async: true", new: "async: true, temlate: x", wantErr: `unknown key "temlate"`},
		{name: "unknown language", old: "go: {methods: [B]}", new: "rust: {methods: [B]}", wantErr: `unknown key "rust"`},
		{name: "null language", old: "go: {methods: [B]}", new: "go:", wantErr: "must not be null"},
		{name: "no languages", old: "languages:\n      go: {methods: [B]}", new: "languages: {}", wantErr: "at least 1 language"},
		{name: "skip and template", old: "go: {methods: [B]}", new: `go: {methods: [B], template: "x"}`, wantErr: "mutually exclusive"},
		{name: "missing template", old: `, template: "{{client}}.X({{goctx}}, {{key}}, {{default}}, {{evalctx}})"`, new: "", wantErr: "needs a template"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			src := okRules
			if tt.old != "" {
				if !strings.Contains(src, tt.old) {
					t.Fatalf("test bug: %q not in base rules", tt.old)
				}
				src = strings.Replace(src, tt.old, tt.new, 1)
			}
			rf, err := spec.ParseRules([]byte(src))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			a := rf.Rules[0]
			if len(rf.Rules) != 2 || a.KeyArg != 0 || a.DefaultArg != 2 || a.Message == "" || len(a.Langs) != 3 || a.Go == nil || !a.Langs["js"].Async {
				t.Fatalf("unexpected rules: %+v", rf.Rules)
			}
			if b := rf.Rules[1]; b.DefaultArg != -1 || b.Skip != "detail" {
				t.Fatalf("unexpected skip rule: %+v", b)
			}
		})
	}
	// Probe files from the reviewers must be rejected.
	paths, err := filepath.Glob("testdata/probes/r_*.yaml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no probes: %v", err)
	}
	for _, p := range paths {
		t.Run("probe "+filepath.Base(p), func(t *testing.T) {
			t.Parallel()
			if _, err := spec.LoadRules(p); err == nil {
				t.Fatal("expected the probe to be rejected")
			}
		})
	}
}

func TestLoadRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "stub spec", path: "testdata/rules.yaml"},
		{name: "missing", path: "testdata/nope.yaml", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rf, err := spec.LoadRules(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(rf.Rules) == 0 {
				t.Fatal("no rules loaded")
			}
		})
	}
}

func TestRulesFile_Validate(t *testing.T) {
	t.Parallel()
	cats := map[string]spec.SkipCategory{"detail": {}, "bulk": {}}
	bind := func(tmpl string, methods ...string) map[string]*spec.Binding {
		return map[string]*spec.Binding{"go": {Methods: methods, Template: tmpl}}
	}
	rewrite := func(id string, langs map[string]*spec.Binding) spec.Rule {
		return spec.Rule{ID: id, ValueType: "boolean", Message: "m", KeyArg: 0, DefaultArg: 2, ContextArg: 1, Langs: langs}
	}
	skip := func(id string, langs map[string]*spec.Binding) spec.Rule {
		return spec.Rule{ID: id, Skip: "detail", KeyArg: -1, DefaultArg: -1, ContextArg: -1, Langs: langs}
	}
	tests := []struct {
		name    string
		version int
		rules   []spec.Rule
		wantErr string
	}{
		{name: "ok", version: 1, rules: []spec.Rule{rewrite("a", bind("t", "A")), skip("b", bind("", "B"))}},
		{name: "bad version", version: 2, rules: []spec.Rule{skip("b", bind("", "B"))}, wantErr: "unsupported version"},
		{name: "empty id", version: 1, rules: []spec.Rule{skip("", bind("", "B"))}, wantErr: "empty id"},
		{name: "duplicate id", version: 1, rules: []spec.Rule{skip("b", bind("", "B")), skip("b", bind("", "C"))}, wantErr: "duplicate rule id"},
		{name: "unknown skip", version: 1, rules: []spec.Rule{{ID: "b", Skip: "zzz", Langs: bind("", "B")}}, wantErr: "unknown skip category"},
		{name: "skip with message", version: 1, rules: []spec.Rule{{ID: "b", Skip: "bulk", Message: "m", Langs: bind("", "B")}}, wantErr: "must not carry a message"},
		{name: "rewrite missing keyArg", version: 1, rules: []spec.Rule{{ID: "a", ValueType: "boolean", Message: "m", KeyArg: -1, Langs: bind("t", "A")}}, wantErr: "requires"},
		{name: "rewrite missing message", version: 1, rules: []spec.Rule{{ID: "a", ValueType: "boolean", KeyArg: 0, DefaultArg: 2, ContextArg: 1, Langs: bind("t", "A")}}, wantErr: "requires message"},
		{name: "no methods", version: 1, rules: []spec.Rule{skip("b", bind(""))}, wantErr: "no methods"},
		{name: "method bound twice", version: 1, rules: []spec.Rule{skip("b", bind("", "B")), skip("c", bind("", "B"))}, wantErr: "already bound"},
		{name: "ctxArg unlisted", version: 1, rules: []spec.Rule{skip("b", map[string]*spec.Binding{"go": {Methods: []string{"B"}, CtxArg: map[string]int{"Z": 0}}})}, wantErr: "unlisted method"},
		{name: "requires unknown", version: 1, rules: []spec.Rule{skip("b", map[string]*spec.Binding{"go": {Methods: []string{"B"}, Requires: []string{"zzz"}}})}, wantErr: "unknown category"},
		{name: "skip with template", version: 1, rules: []spec.Rule{skip("b", bind("t", "B"))}, wantErr: "mutually exclusive"},
		{name: "rewrite without template", version: 1, rules: []spec.Rule{rewrite("a", bind("", "A"))}, wantErr: "needs a template"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rf := &spec.RulesFile{Version: tt.version, SkipCategories: cats, Rules: tt.rules}
			err := rf.Validate()
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

func TestRulesFile_SkipCategoryNames(t *testing.T) {
	t.Parallel()
	rf := &spec.RulesFile{SkipCategories: map[string]spec.SkipCategory{"b": {}, "a": {}, "c": {}}}
	got := strings.Join(rf.SkipCategoryNames(), ",")
	if got != "a,b,c" {
		t.Fatalf("got %q", got)
	}
}

func TestLookupGoMethod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method string
		want   string
		ok     bool
	}{
		{method: "BoolVariation", want: "bool-variation", ok: true},
		{method: "BoolVariationCtx", want: "bool-variation", ok: true},
		{method: "BoolVariationDetail", want: "bool-variation-detail", ok: true},
		{method: "Nope"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()
			r, ok := spec.LookupGoMethod(tt.method)
			if ok != tt.ok || r.ID != tt.want {
				t.Fatalf("got (%q,%v), want (%q,%v)", r.ID, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestRules(t *testing.T) {
	t.Parallel()
	if len(spec.Rules()) == 0 {
		t.Fatal("generated rules table is empty")
	}
}

func TestSkipCategories(t *testing.T) {
	t.Parallel()
	if _, ok := spec.SkipCategories()["detail"]; !ok {
		t.Fatal("detail category missing from generated table")
	}
}
