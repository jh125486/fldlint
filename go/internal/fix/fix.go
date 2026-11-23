// Package fix builds the text edits for a planned rewrite: template
// expansion and import changes. Argument text is always copied verbatim from
// the source; nothing is re-printed through go/printer.
package fix

import (
	"fmt"
	"strings"
)

// Expand substitutes every {{name}} placeholder in tmpl with vals[name].
// An unknown placeholder, or one with no value, is an error.
func Expand(tmpl string, vals map[string]string) (string, error) {
	var b strings.Builder
	for {
		i := strings.Index(tmpl, "{{")
		if i < 0 {
			b.WriteString(tmpl)
			return b.String(), nil
		}
		j := strings.Index(tmpl[i:], "}}")
		if j < 0 {
			return "", fmt.Errorf("unterminated placeholder in template %q", tmpl)
		}
		name := tmpl[i+2 : i+j]
		v, ok := vals[name]
		if !ok {
			return "", fmt.Errorf("placeholder {{%s}} has no value", name)
		}
		b.WriteString(tmpl[:i])
		b.WriteString(v)
		tmpl = tmpl[i+j+2:]
	}
}
