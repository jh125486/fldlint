package fix

import (
	"fmt"
	"regexp"
	"strings"
)

// Comment is one source comment, verbatim (Text includes the // or /* */
// delimiters).
type Comment struct {
	Text string
	// Line marks a // comment: it must be followed by a line terminator, EOL.
	Line bool
	// EOL is the line terminator that ended a // comment in the source ("\n"
	// or "\r\n"); empty for a block comment.
	EOL string
}

var directive = regexp.MustCompile(`@ts-(?:expect-error|ignore|nocheck)|eslint-(?:disable|enable)|\bnolint\b|^//go:|lint:ignore|istanbul ignore`)

// IsDirective reports whether a comment governs a line or argument (type
// check, lint, coverage or build directive): moving it would change what it
// governs, so it has no home outside its own argument text.
func IsDirective(text string) bool { return directive.MatchString(text) }

// Breaks reports whether more code may not follow the comment on its line: a
// // comment, or a block comment containing a line break (Go inserts a
// semicolon after it just as after a newline).
func (c Comment) Breaks() bool {
	return c.Line || strings.ContainsAny(c.Text, "\n\r")
}

// ArgNotes are the comments that travel with one argument: Lead sits before
// it, Trail after it.
type ArgNotes struct {
	Lead, Trail []Comment
}

// Notes is where a call's comments go in its rewrite (issue #31). Args is
// keyed by template placeholder name. Head comments sit between the call
// start and its "(" and go before the new call; Tail comments follow the
// last argument on their own line and go before the closing ")". Unhomed
// means some comment has no home (inside the converted context, or a line
// or multi-line comment in Head): the call gets no fix.
type Notes struct {
	Args    map[string]*ArgNotes
	Head    []Comment
	Tail    []Comment
	Unhomed bool
}

func deferred(cs []Comment) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(" " + c.Text)
		if c.Line {
			b.WriteString(c.EOL)
		}
	}
	return b.String()
}

type tmplPart struct {
	name string // placeholder name, or "" for literal text
	lit  string
}

func tokenize(tmpl string) ([]tmplPart, error) {
	var toks []tmplPart
	for {
		i := strings.Index(tmpl, "{{")
		if i < 0 {
			if tmpl != "" {
				toks = append(toks, tmplPart{lit: tmpl})
			}
			return toks, nil
		}
		j := strings.Index(tmpl[i:], "}}")
		if j < 0 {
			return nil, fmt.Errorf("unterminated placeholder in template %q", tmpl)
		}
		if i > 0 {
			toks = append(toks, tmplPart{lit: tmpl[:i]})
		}
		toks = append(toks, tmplPart{name: tmpl[i+2 : i+j]})
		tmpl = tmpl[i+j+2:]
	}
}

// ExpandNotes is Expand plus comment placement. It reports ok=false when a
// comment has no home. The JS engine implements the same algorithm
// (js/packages/core/src/expand.ts); the shared fixtures keep them equal.
//
//   - An argument's Lead comments go right before its text (a // comment is
//     followed by its line terminator, a block comment by one space).
//   - Its Trail comments that fit on the line go right after it. A // or
//     multi-line one is moved behind the next "," of the template (or, for
//     the last argument, behind a comma added before the closing ")"), so a
//     line break never separates an argument from its comma: Go inserts a
//     semicolon there and rejects the call.
//   - Head comments go before the new call; Tail comments before the final ")".
//
// Decorations apply to a placeholder's first occurrence only.
func ExpandNotes(tmpl string, vals map[string]string, notes *Notes) (text string, ok bool, err error) {
	toks, err := tokenize(tmpl)
	if err != nil {
		return "", false, err
	}
	last := -1
	for i, t := range toks {
		if t.name != "" {
			if _, known := vals[t.name]; !known {
				return "", false, fmt.Errorf("placeholder {{%s}} has no value", t.name)
			}
			last = i
		}
	}
	if notes == nil {
		notes = &Notes{}
	}
	var b strings.Builder
	for _, c := range notes.Head {
		if c.Breaks() {
			return "", false, nil
		}
		b.WriteString(c.Text + " ")
	}
	var pending []Comment
	used := map[string]bool{}
	for i, t := range toks {
		if t.name == "" {
			lit := t.lit
			if len(pending) > 0 {
				comma, closeAt := strings.Index(lit, ","), strings.LastIndex(lit, ")")
				switch {
				case comma >= 0:
					lit = lit[:comma+1] + deferred(pending) + lit[comma+1:]
					pending = nil
				case i == len(toks)-1 && closeAt >= 0:
					lit = lit[:closeAt] + "," + deferred(pending) + lit[closeAt:]
					pending = nil
				}
			}
			b.WriteString(lit)
			continue
		}
		var a ArgNotes
		if n := notes.Args[t.name]; n != nil && !used[t.name] {
			a = *n
		}
		used[t.name] = true
		for _, c := range a.Lead {
			b.WriteString(c.Text)
			if c.Line {
				b.WriteString(c.EOL)
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(vals[t.name])
		for _, c := range a.Trail {
			if c.Breaks() {
				pending = append(pending, c)
			} else {
				b.WriteString(" " + c.Text)
			}
		}
		if i == last {
			pending = append(pending, notes.Tail...)
		}
	}
	if len(pending) > 0 || (len(notes.Tail) > 0 && last < 0) {
		return "", false, nil
	}
	// A comment bound to an argument the template never uses has no home.
	for name, a := range notes.Args {
		if !used[name] && (len(a.Lead) > 0 || len(a.Trail) > 0) {
			return "", false, nil
		}
	}
	return b.String(), true, nil
}
