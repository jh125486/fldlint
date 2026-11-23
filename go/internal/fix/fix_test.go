package fix_test

import (
	"testing"

	"github.com/jh125486/fldlint/go/internal/fix"
)

func TestExpand(t *testing.T) {
	t.Parallel()
	vals := map[string]string{"client": "c", "key": `"k"`, "empty": ""}
	tests := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{name: "no placeholders", tmpl: "plain()", want: "plain()"},
		{name: "several", tmpl: "{{client}}.V({{key}}, {{client}})", want: `c.V("k", c)`},
		{name: "empty value", tmpl: "a{{empty}}b", want: "ab"},
		{name: "value is not re-expanded", tmpl: "{{key}}", want: `"k"`},
		{name: "unknown placeholder", tmpl: "{{nope}}", wantErr: true},
		{name: "unterminated", tmpl: "{{client", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := fix.Expand(tt.tmpl, vals)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
