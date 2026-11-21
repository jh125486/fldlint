package spec_test

import (
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const okRules = `
version: 1
skipCategories:
  detail: {severity: warning, message: m}
rules:
  - id: a
    keyArg: 0
    languages:
      go: {methods: [A, ACtx], ctxArg: {ACtx: 0}, template: "{{client}}.X({{key}})"}
  - id: b
    skip: detail
    languages:
      go: {methods: [B]}
`

func TestParseRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "ok", yaml: okRules},
		{name: "bad yaml", yaml: "version: [", wantErr: "parse rules"},
		{name: "bad version", yaml: "version: 2", wantErr: "unsupported version"},
		{name: "duplicate id", yaml: strings.Replace(okRules, "id: b", "id: a", 1), wantErr: "duplicate rule id"},
		{name: "duplicate method", yaml: strings.Replace(okRules, "methods: [B]", "methods: [A]", 1), wantErr: "already bound"},
		{name: "unknown skip", yaml: strings.Replace(okRules, "skip: detail", "skip: nope", 1), wantErr: "unknown skip category"},
		{name: "unknown placeholder", yaml: strings.Replace(okRules, "{{key}}", "{{bogus}}", 1), wantErr: "unknown placeholder"},
		{name: "skip and template", yaml: strings.Replace(okRules, "id: a\n", "id: a\n    skip: detail\n", 1), wantErr: "mutually exclusive"},
		{name: "missing template", yaml: strings.Replace(okRules, `, template: "{{client}}.X({{key}})"`, "", 1), wantErr: "needs a template"},
		{name: "ctxArg unlisted", yaml: strings.Replace(okRules, "ACtx: 0", "Zed: 0", 1), wantErr: "unlisted method"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rf, err := spec.ParseRules([]byte(tt.yaml))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(rf.Rules) != 2 || rf.Rules[0].KeyArg != 0 || rf.Rules[0].DefaultArg != -1 {
				t.Fatalf("unexpected rules: %+v", rf.Rules)
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
