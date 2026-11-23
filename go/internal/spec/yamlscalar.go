package spec

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// maxSafeInt is 2^53 - 1, the largest integer JavaScript represents exactly;
// integers are accepted in [-maxSafeInt, maxSafeInt].
const maxSafeInt = "9007199254740991"

// Scalar typing rules R1 to R4f of the contract, copied verbatim. Only ^, $
// and [0-9] are used, so they match the same texts as in JavaScript.
var (
	r1  = regexp.MustCompile(`^(true|false)$`)
	r2  = regexp.MustCompile(`^(0|-?[1-9][0-9]*)$`)
	r3  = regexp.MustCompile(`^(|~|null|Null|NULL)$`)
	r4a = regexp.MustCompile(`(?i)^(true|false|null|y|n|yes|no|on|off|=|<<)$`)
	r4b = regexp.MustCompile(`^[-+]?([0-9][0-9_,]*(\.[0-9_]*)?|\.[0-9][0-9_]*)([eE][-+]?[0-9]+)?$`)
	r4c = regexp.MustCompile(`(?i)^[-+]?\.(inf|nan)$`)
	r4d = regexp.MustCompile(`(?i)^[-+]?0[xob][0-9a-f_]+$`)
	r4e = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9])+(\.[0-9_]*)?$`)
	r4f = regexp.MustCompile(`^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}([Tt ]|$)`)
)

type scalarKind int

const (
	kindString scalarKind = iota
	kindBool
	kindInt
	kindNull
	kindAmbiguous // contract rule R4
	kindOutOfRange
)

// scalar is the typed result of classifying a scalar.
type scalar struct {
	kind scalarKind
	text string // the scalar text
}

// tag is the YAML core tag matching the kind of an accepted scalar.
func (s scalar) tag() string {
	switch s.kind {
	case kindBool:
		return "!!bool"
	case kindInt:
		return "!!int"
	case kindNull:
		return "!!null"
	case kindString, kindAmbiguous, kindOutOfRange:
	}
	return "!!str"
}

// classify types the text of a plain scalar: first match of R1 to R5 wins.
func classify(text string) scalar {
	switch {
	case r1.MatchString(text):
		return scalar{kind: kindBool, text: text}
	case r2.MatchString(text):
		return classifyInt(text)
	case r3.MatchString(text):
		return scalar{kind: kindNull, text: text}
	case isAmbiguous(text):
		return scalar{kind: kindAmbiguous, text: text}
	}
	return scalar{kind: kindString, text: text}
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// isAmbiguous reports whether any rule of R4 matches. The case-insensitive
// rules (R4a, R4c, R4d) fold ASCII letters only, as in JavaScript, whereas
// Go's (?i) also folds U+017F to s and U+212A to k. Their literals are all
// ASCII, so a non-ASCII text cannot match them and they are skipped for it;
// the other rules are case-sensitive and ASCII-only except for the prefix
// rule R4f, which must see the whole text.
func isAmbiguous(text string) bool {
	if isASCII(text) && (r4a.MatchString(text) || r4c.MatchString(text) || r4d.MatchString(text)) {
		return true
	}
	return r4b.MatchString(text) || r4e.MatchString(text) || r4f.MatchString(text)
}

// classifyInt types text, which matches R2, enforcing the safe range on the
// digits rather than through a conversion.
func classifyInt(text string) scalar {
	digits := strings.TrimPrefix(text, "-")
	if len(digits) > len(maxSafeInt) || len(digits) == len(maxSafeInt) && digits > maxSafeInt {
		return scalar{kind: kindOutOfRange, text: text}
	}
	return scalar{kind: kindInt, text: text}
}

// plainMask selects the Style bits of the non-plain scalar styles.
const plainMask = yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle

// scalarOf types a scalar node: quoted and block scalars are always strings.
func scalarOf(n *yaml.Node) scalar {
	if n.Style&plainMask != 0 {
		return scalar{kind: kindString, text: n.Value}
	}
	return classify(n.Value)
}
