package spec_test

import (
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const okRules = `{
  "version": 1,
  "skipCategories": {"detail": {"severity": "warning", "message": "d"}},
  "rules": [
    {"id": "a", "valueType": "boolean", "risk": "low", "keyArg": 0, "defaultArg": 2, "contextArg": 1,
     "message": "m",
     "languages": {
       "js": {"methods": ["aJS"], "async": true, "template": "t"},
       "go": {"methods": ["A", "ACtx"], "ctxArg": {"ACtx": 0}, "requires": ["detail"], "template": "t"}}},
    {"id": "b", "risk": "high", "skip": "detail", "languages": {"go": {"methods": ["B"]}}}
  ]
}`

func TestParseRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		old, new string // replace old with new in okRules
		suffix   string // appended to the result
		wantErr  string
	}{
		{name: "ok"},
		{name: "bad version", old: `"version": 1`, new: `"version": 2`, wantErr: "unsupported version"},
		{name: "unknown top key", old: `"version": 1,`, new: `"version": 1, "extra": 1,`, wantErr: `unknown field "extra"`},
		{name: "unknown rule key", old: `"keyArg": 0`, new: `"keyArg": 0, "bogus": 1`, wantErr: `unknown field "bogus"`},
		{name: "unknown binding key", old: `"async": true`, new: `"async": true, "temlate": "x"`, wantErr: `unknown field "temlate"`},
		{name: "wrong type", old: `"keyArg": 0`, new: `"keyArg": "0"`, wantErr: "parse rules"},
		{name: "trailing object", suffix: " {}", wantErr: "after the JSON value"},
		{name: "trailing close bracket", suffix: "]", wantErr: "after the JSON value"},
		{name: "trailing close brace", suffix: "}", wantErr: "after the JSON value"},
		{name: "trailing garbage", suffix: " garbage", wantErr: "after the JSON value"},
		{name: "rewriting rule missing keyArg", old: `"keyArg": 0, `, new: "", wantErr: "requires keyArg"},
		{name: "rewriting rule missing defaultArg", old: `"defaultArg": 2, `, new: "", wantErr: "requires defaultArg"},
		{name: "negative arg", old: `"keyArg": 0`, new: `"keyArg": -1`, wantErr: "must be >= 0"},
		{name: "null language", old: `"go": {"methods": ["B"]}`, new: `"go": null`, wantErr: "must not be null"},
		{name: "duplicate go method", old: `"methods": ["B"]`, new: `"methods": ["A"]`, wantErr: "bound by rules"},
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
			rf, err := spec.ParseRules([]byte(src + tt.suffix))
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
			if len(rf.Rules) != 2 || a.KeyArg != 0 || a.DefaultArg != 2 || a.Message == "" || len(a.Langs) != 2 || a.Go == nil || !a.Langs["js"].Async {
				t.Fatalf("unexpected rules: %+v", rf.Rules)
			}
			if b := rf.Rules[1]; b.KeyArg != -1 || b.DefaultArg != -1 || b.ContextArg != -1 || b.Skip != "detail" {
				t.Fatalf("unexpected skip rule: %+v", b)
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
