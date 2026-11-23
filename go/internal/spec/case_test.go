package spec_test

import (
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

func TestParseCase(t *testing.T) {
	t.Parallel()
	const ok = `{"id": "x", "rule": "r", "tags": ["t"], "description": "d",
	  "expect": {"go": {"diagnostics": [{"line": 3, "col": 4, "ruleId": "r", "skip": "dynamic-key"}], "output": false}}}`
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{name: "ok", src: ok},
		{name: "unknown field", src: strings.Replace(ok, `"tags"`, `"extra": 1, "tags"`, 1), wantErr: `unknown field "extra"`},
		{name: "unknown nested field", src: strings.Replace(ok, `"output"`, `"outptu": 1, "output"`, 1), wantErr: `unknown field "outptu"`},
		{name: "wrong type", src: strings.Replace(ok, `"line": 3`, `"line": "3"`, 1), wantErr: "parse case"},
		{name: "not json", src: "id: x", wantErr: "parse case"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := spec.ParseCase([]byte(tt.src))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			d := c.Expect["go"].Diagnostics
			if c.ID != "x" || c.Rule != "r" || len(d) != 1 || d[0].Line != 3 || d[0].Col != 4 || d[0].Skip != "dynamic-key" {
				t.Fatalf("unexpected case: %+v", c)
			}
		})
	}
}
