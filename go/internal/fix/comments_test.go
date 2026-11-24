package fix_test

import (
	"testing"

	"github.com/jh125486/fldlint/go/internal/fix"
)

func block(text string) fix.Comment { return fix.Comment{Text: text} }
func line(text string) fix.Comment  { return fix.Comment{Text: text, Line: true, EOL: "\n"} }

func TestExpandNotes(t *testing.T) {
	t.Parallel()
	vals := map[string]string{"client": "c", "key": `"k"`, "default": "false", "ctx": "{ x }"}
	const tmpl = "{{client}}.f({{key}}, {{default}}, {{ctx}})"
	args := func(m map[string]*fix.ArgNotes) *fix.Notes { return &fix.Notes{Args: m} }
	tests := []struct {
		name   string
		tmpl   string
		notes  *fix.Notes
		want   string
		wantOK bool
		// wantErr is a malformed template or a placeholder with no value.
		wantErr bool
	}{
		{name: "unknown placeholder is an error", tmpl: "{{nope}}", wantErr: true},
		{name: "unterminated placeholder is an error", tmpl: "{{key", wantErr: true},
		{name: "nil notes equal plain substitution", tmpl: tmpl, want: `c.f("k", false, { x })`, wantOK: true},
		{name: "empty notes equal plain substitution", tmpl: tmpl, notes: &fix.Notes{}, want: `c.f("k", false, { x })`, wantOK: true},
		{
			name:   "block comment leads an argument",
			tmpl:   tmpl,
			notes:  args(map[string]*fix.ArgNotes{"ctx": {Lead: []fix.Comment{block("/* who */")}}}),
			want:   `c.f("k", false, /* who */ { x })`,
			wantOK: true,
		},
		{
			name:   "line comment leads an argument and keeps its terminator",
			tmpl:   tmpl,
			notes:  args(map[string]*fix.ArgNotes{"key": {Lead: []fix.Comment{{Text: "// k", Line: true, EOL: "\r\n"}}}}),
			want:   "c.f(// k\r\n\"k\", false, { x })",
			wantOK: true,
		},
		{
			name:   "single-line block comment trails inline",
			tmpl:   tmpl,
			notes:  args(map[string]*fix.ArgNotes{"default": {Trail: []fix.Comment{block("/* off */")}}}),
			want:   `c.f("k", false /* off */, { x })`,
			wantOK: true,
		},
		{
			name:   "line comment trailing a middle argument moves behind the next comma",
			tmpl:   tmpl,
			notes:  args(map[string]*fix.ArgNotes{"key": {Trail: []fix.Comment{line("// the flag")}}}),
			want:   "c.f(\"k\", // the flag\n false, { x })",
			wantOK: true,
		},
		{
			name:   "line comment trailing the last argument gets a comma before the paren",
			tmpl:   tmpl,
			notes:  args(map[string]*fix.ArgNotes{"ctx": {Trail: []fix.Comment{line("// who")}}}),
			want:   "c.f(\"k\", false, { x }, // who\n)",
			wantOK: true,
		},
		{
			name:   "multi-line block comment trailing an argument is deferred like a line comment",
			tmpl:   tmpl,
			notes:  args(map[string]*fix.ArgNotes{"key": {Trail: []fix.Comment{block("/* a\n b */")}}}),
			want:   "c.f(\"k\", /* a\n b */ false, { x })",
			wantOK: true,
		},
		{
			name:   "tail comments go before the closing paren",
			tmpl:   tmpl,
			notes:  &fix.Notes{Tail: []fix.Comment{line("// end")}},
			want:   "c.f(\"k\", false, { x }, // end\n)",
			wantOK: true,
		},
		{
			name:   "head block comments go before the call",
			tmpl:   tmpl,
			notes:  &fix.Notes{Head: []fix.Comment{block("/* a */"), block("/* b */")}},
			want:   `/* a */ /* b */ c.f("k", false, { x })`,
			wantOK: true,
		},
		{name: "a line comment in the head has no home", tmpl: tmpl, notes: &fix.Notes{Head: []fix.Comment{line("// a")}}},
		{name: "a multi-line block comment in the head has no home", tmpl: tmpl, notes: &fix.Notes{Head: []fix.Comment{block("/* a\n*/")}}},
		{
			name:  "a comment bound to an argument the template never uses has no home",
			tmpl:  "{{client}}.f({{key}})",
			notes: args(map[string]*fix.ArgNotes{"ctx": {Lead: []fix.Comment{block("/* a */")}}}),
		},
		{name: "a tail comment with no placeholder has no home", tmpl: "f()", notes: &fix.Notes{Tail: []fix.Comment{block("/* t */")}}},
		{
			name:  "a deferred comment with no closing paren has no home",
			tmpl:  "{{key}}",
			notes: args(map[string]*fix.ArgNotes{"key": {Trail: []fix.Comment{line("// a")}}}),
		},
		{
			name:   "decorations apply to the first occurrence only",
			tmpl:   "{{key}} {{key}}",
			notes:  args(map[string]*fix.ArgNotes{"key": {Lead: []fix.Comment{block("/* a */")}}}),
			want:   `/* a */ "k" "k"`,
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok, err := fix.ExpandNotes(tt.tmpl, vals, tt.notes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestComment_Breaks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    fix.Comment
		want bool
	}{
		{name: "line comment", c: line("// a"), want: true},
		{name: "single-line block comment", c: block("/* a */"), want: false},
		{name: "block comment with LF", c: block("/* a\n b */"), want: true},
		{name: "block comment with CR", c: block("/* a\r b */"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.c.Breaks(); got != tt.want {
				t.Fatalf("Breaks() = %v, want %v", got, tt.want)
			}
		})
	}
}
