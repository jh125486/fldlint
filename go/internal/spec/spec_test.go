package spec_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const okSpecRules = `"rules": {"version": 1, "skipCategories": {}, "rules": [` +
	`{"id": "r", "risk": "low", "skip": "detail", "languages": {"go": {"methods": ["M"]}}}]}`

const okCase = `{"id": "c", "rule": "r", "tags": [], "description": "d", "expect": {}}`

func TestParseSpec(t *testing.T) {
	t.Parallel()
	mk := func(cases string) string { return "{" + okSpecRules + `, "cases": [` + cases + "]}" }
	entry := func(path string) string { return `{"path": "` + path + `", "case": ` + okCase + `}` }
	tests := []struct {
		name    string
		src     string
		want    []string
		wantErr string
	}{
		{name: "no cases", src: mk(""), want: nil},
		{name: "two cases", src: mk(entry("r/a") + "," + entry("r/b")), want: []string{"r/a", "r/b"}},
		{name: "missing rules", src: `{"cases": []}`, wantErr: `missing "rules"`},
		{name: "unknown top key", src: strings.Replace(mk(""), `"cases"`, `"extra": 1, "cases"`, 1), wantErr: `unknown field "extra"`},
		{name: "path not rule/case", src: mk(entry("r")), wantErr: "not <rule>/<case>"},
		{name: "path too deep", src: mk(entry("r/a/b")), wantErr: "not <rule>/<case>"},
		{name: "duplicate path", src: mk(entry("r/a") + "," + entry("r/a")), wantErr: "duplicate case path"},
		{name: "directory differs from rule", src: mk(entry("other/a")), wantErr: "must be a known rule"},
		{name: "unknown rule", src: strings.Replace(mk(entry("q/a")), `"rule": "r"`, `"rule": "q"`, 1), wantErr: "must be a known rule"},
		{name: "bad case", src: strings.Replace(mk(entry("r/a")), `"tags"`, `"tgs"`, 1), wantErr: "unknown field"},
		{name: "bad rules", src: strings.Replace(mk(""), `"version": 1`, `"version": 9`, 1), wantErr: "unsupported version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := spec.ParseSpec([]byte(tt.src))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, c := range s.Cases {
				got = append(got, c.Path)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("cases = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		path      string
		wantErr   bool
		wantCases bool
	}{
		{name: "stub spec", path: "testdata/spec.json", wantCases: true},
		{name: "repo spec", path: "../../../spec/generated/spec.json", wantCases: true},
		{name: "missing", path: "testdata/nope.json", wantErr: true},
		{name: "not a spec", path: "gen/main.go", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := spec.LoadSpec(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (len(s.Rules.Rules) == 0 || (tt.wantCases && len(s.Cases) == 0)) {
				t.Fatal("nothing loaded")
			}
		})
	}
}

func TestLoadedCase_Dir(t *testing.T) {
	t.Parallel()
	lc := spec.LoadedCase{Path: "r/c"}
	if got, want := lc.Dir("fx"), filepath.Join("fx", "r", "c"); got != want {
		t.Fatalf("Dir = %q, want %q", got, want)
	}
}
