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
    {"id": "b", "risk": "high", "skip": "detail", "languages": {"go": {"methods": ["B"], "argCount": {"min": 2, "max": 3}}}}
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
		{name: "unknown argCount key", old: `"min": 2`, new: `"min": 2, "mx": 1`, wantErr: `unknown field "mx"`},
		{name: "argCount wrong type", old: `"min": 2`, new: `"min": "2"`, wantErr: "parse rules"},
		{name: "unknown binding key", old: `"async": true`, new: `"async": true, "temlate": "x"`, wantErr: `unknown field "temlate"`},
		{name: "wrong type", old: `"keyArg": 0`, new: `"keyArg": "0"`, wantErr: "parse rules"},
		{name: "trailing object", suffix: " {}", wantErr: "after the JSON value"},
		{name: "trailing close bracket", suffix: "]", wantErr: "after the JSON value"},
		{name: "trailing close brace", suffix: "}", wantErr: "after the JSON value"},
		{name: "trailing garbage", suffix: " garbage", wantErr: "after the JSON value"},
		{name: "rewriting rule missing keyArg", old: `"keyArg": 0, `, new: "", wantErr: "requires keyArg"},
		{name: "rewriting rule missing defaultArg", old: `"defaultArg": 2, `, new: "", wantErr: "requires defaultArg"},
		{name: "negative arg", old: `"keyArg": 0`, new: `"keyArg": -1`, wantErr: "must be >= 0"},
		{name: "null language", old: `"go": {"methods": ["B"], "argCount": {"min": 2, "max": 3}}`, new: `"go": null`, wantErr: "must not be null"},
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
			if got := rf.Rules[1].Go.ArgCount; got == nil || *got != (spec.ArgCount{Min: 2, Max: 3}) {
				t.Fatalf("argCount = %+v, want {2 3}", got)
			}
			if got := rf.Rules[0].Go.ArgCount; got != nil {
				t.Fatalf("rewriting rule argCount = %+v, want nil", got)
			}
		})
	}
}

func TestArgCount_Accepts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		ac    spec.ArgCount
		n     int
		shift int
		want  bool
	}{
		{name: "exact", ac: spec.ArgCount{Min: 3, Max: 3}, n: 3, want: true},
		{name: "below min", ac: spec.ArgCount{Min: 3, Max: 3}, n: 2},
		{name: "above max", ac: spec.ArgCount{Min: 3, Max: 3}, n: 4},
		{name: "range low end", ac: spec.ArgCount{Min: 3, Max: 4}, n: 3, want: true},
		{name: "range high end", ac: spec.ArgCount{Min: 3, Max: 4}, n: 4, want: true},
		{name: "range above", ac: spec.ArgCount{Min: 3, Max: 4}, n: 5},
		{name: "ctx shift accepts shifted count", ac: spec.ArgCount{Min: 3, Max: 3}, n: 4, shift: 1, want: true},
		{name: "ctx shift rejects unshifted count", ac: spec.ArgCount{Min: 3, Max: 3}, n: 3, shift: 1},
		{name: "ctx shift rejects one too many", ac: spec.ArgCount{Min: 3, Max: 3}, n: 5, shift: 1},
		{name: "variadic minimum", ac: spec.ArgCount{Min: 1}, n: 1, want: true},
		{name: "variadic many", ac: spec.ArgCount{Min: 1}, n: 50, want: true},
		{name: "variadic none", ac: spec.ArgCount{Min: 1}, n: 0},
		{name: "variadic ctx shift below", ac: spec.ArgCount{Min: 1}, n: 1, shift: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.ac.Accepts(tt.n, tt.shift); got != tt.want {
				t.Fatalf("%+v.Accepts(%d, %d) = %v, want %v", tt.ac, tt.n, tt.shift, got, tt.want)
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
	// Every Go binding of a report-only rule carries argCount; rewriting
	// rules carry none (their arity comes from keyArg/defaultArg/contextArg).
	for _, r := range spec.Rules() {
		if r.Go == nil {
			continue
		}
		if has := r.Go.ArgCount != nil; has != (r.Skip != "") {
			t.Errorf("rule %q: argCount present = %v, report-only = %v", r.ID, has, r.Skip != "")
		}
	}
}

func TestSkipCategories(t *testing.T) {
	t.Parallel()
	if _, ok := spec.SkipCategories()["detail"]; !ok {
		t.Fatal("detail category missing from generated table")
	}
}
