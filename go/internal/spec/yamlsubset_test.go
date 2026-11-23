package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const (
	probeDir    = "../../../spec/probes/yaml"
	minReject   = 102 // keep in sync with spec/probes/README.md
	minAccept   = 62
	maxSafeText = "9007199254740991"
	overSafe    = "9007199254740992"
)

type scalarCase struct {
	name    string
	value   string // the text after "a: "
	want    any
	wantErr error // nil: accepted as want
}

// scalarCases lists matching and near-miss inputs for each contract rule.
func scalarCases() []scalarCase {
	return slices.Concat(scalarCasesR1toR3(), scalarCasesR4ab(), scalarCasesR4cToR5())
}

func scalarCasesR1toR3() []scalarCase {
	longDigits := strings.Repeat("9", 400)
	return []scalarCase{
		// R1
		{name: "R1 true", value: "true", want: true},
		{name: "R1 false", value: "false", want: false},
		{name: "R1 near miss True", value: "True", wantErr: spec.ErrAmbiguousScalar},
		{name: "R1 near miss FALSE", value: "FALSE", wantErr: spec.ErrAmbiguousScalar},
		{name: "R1 near miss tRuE", value: "tRuE", wantErr: spec.ErrAmbiguousScalar},
		// R2
		{name: "R2 zero", value: "0", want: int64(0)},
		{name: "R2 positive", value: "42", want: int64(42)},
		{name: "R2 negative", value: "-42", want: int64(-42)},
		{name: "R2 max safe", value: maxSafeText, want: int64(9007199254740991)},
		{name: "R2 min safe", value: "-" + maxSafeText, want: int64(-9007199254740991)},
		{name: "R2 above safe", value: overSafe, wantErr: spec.ErrIntRange},
		{name: "R2 below safe", value: "-" + overSafe, wantErr: spec.ErrIntRange},
		{name: "R2 seventeen digits", value: "10000000000000000", wantErr: spec.ErrIntRange},
		{name: "R2 beyond int64", value: "9223372036854775808", wantErr: spec.ErrIntRange},
		{name: "R2 very long digit string", value: longDigits, wantErr: spec.ErrIntRange},
		{name: "R2 very long negative digit string", value: "-" + longDigits, wantErr: spec.ErrIntRange},
		{name: "R2 negative zero", value: "-0", wantErr: spec.ErrAmbiguousScalar},
		{name: "R2 leading zero", value: "017", wantErr: spec.ErrAmbiguousScalar},
		{name: "R2 double zero", value: "00", wantErr: spec.ErrAmbiguousScalar},
		{name: "R2 plus sign", value: "+5", wantErr: spec.ErrAmbiguousScalar},
		{name: "R2 underscore", value: "1_0", wantErr: spec.ErrAmbiguousScalar},
		{name: "R2 comma", value: "1,000", wantErr: spec.ErrAmbiguousScalar},
		// R3
		{name: "R3 empty", value: "", want: nil},
		{name: "R3 tilde", value: "~", want: nil},
		{name: "R3 null", value: "null", want: nil},
		{name: "R3 Null", value: "Null", want: nil},
		{name: "R3 NULL", value: "NULL", want: nil},
		{name: "R3 near miss nULL", value: "nULL", wantErr: spec.ErrAmbiguousScalar},
		{name: "R3 near miss tilde word", value: "~x", want: "~x"},
	}
}

func scalarCasesR4ab() []scalarCase {
	return []scalarCase{
		// R4a
		{name: "R4a y", value: "y", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a Y", value: "Y", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a n", value: "n", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a yes", value: "yes", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a Yes", value: "Yes", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a YES", value: "YES", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a no", value: "No", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a on", value: "ON", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a off", value: "Off", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a value indicator", value: "=", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a merge indicator", value: "<<", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4a near miss yesterday", value: "yesterday", want: "yesterday"},
		{name: "R4a near miss nope", value: "nope", want: "nope"},
		{name: "R4a near miss online", value: "online", want: "online"},
		{name: "R4a near miss maybe", value: "maybe", want: "maybe"},
		{name: "R4a long s is not folded to s", value: "yeſ", want: "yeſ"},
		{name: "R4a long s in false", value: "falſe", want: "falſe"},
		{name: "R4a fullwidth letters", value: "ＴＲＵＥ", want: "ＴＲＵＥ"},
		{name: "R4c dotless i is not folded to i", value: ".ınf", want: ".ınf"},
		{name: "R4d Kelvin sign is not folded to k", value: "0x\u212A", want: "0x\u212A"},
		{name: "R4f prefix with non-ASCII tail", value: "2001-12-14 café", wantErr: spec.ErrAmbiguousScalar},
		{name: "R5 yaml.v3 number look-alike 0e0_", value: "0e0_", want: "0e0_"},
		{name: "R5 yaml.v3 number look-alike 1e_0", value: "1e_0", want: "1e_0"},
		{name: "R5 yaml.v3 number look-alike 0o-1", value: "0o-1", want: "0o-1"},
		{name: "R5 yaml.v3 number look-alike 0b+1", value: "0b+1", want: "0b+1"},
		{name: "R5 yaml.v3 number look-alike 0_x1", value: "0_x1", want: "0_x1"},
		// R4b
		{name: "R4b float", value: "1.5", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b float zero", value: "3.0", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b trailing dot", value: "5.", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b leading dot", value: ".5", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b exponent", value: "1e3", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b exponent upper", value: "1E+3", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b signed float", value: "-1.5e-3", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b big exponent", value: "1e300", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4b near miss dotted quad", value: "10.0.0.1", want: "10.0.0.1"},
		{name: "R4b near miss version", value: "1.2.3", want: "1.2.3"},
		{name: "R4b near miss digit prefix", value: "1st", want: "1st"},
		{name: "R4b near miss words", value: "3 flags", want: "3 flags"},
		{name: "R4b near miss dot alone", value: ".", want: "."},
		{name: "R4b near miss exponent without digits", value: "1e", want: "1e"},
	}
}

func scalarCasesR4cToR5() []scalarCase {
	return []scalarCase{
		// R4c
		{name: "R4c inf", value: ".inf", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4c negative Inf", value: "-.Inf", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4c signed NaN", value: "+.NAN", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4c nan", value: ".nan", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4c near miss", value: ".infinity", want: ".infinity"},
		{name: "R4c near miss no dot", value: "inf", want: "inf"},
		// R4d
		{name: "R4d hex", value: "0x1F", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4d octal", value: "0o17", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4d binary", value: "0b101", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4d signed hex", value: "-0xff", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4d prefix alone", value: "0x", want: "0x"},
		{name: "R4d near miss non-hex digit", value: "0xg", want: "0xg"},
		{name: "R4d near miss no leading zero", value: "x10", want: "x10"},
		// R4e
		{name: "R4e sexagesimal", value: "1:30", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4e sexagesimal fraction", value: "190:20:30.15", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4e near miss minutes above 59", value: "1:60", want: "1:60"},
		{name: "R4e near miss text", value: "a:b", want: "a:b"},
		// R4f
		{name: "R4f date", value: "2001-12-14", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4f short date", value: "2001-1-1", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4f timestamp T", value: "2001-12-14t21:59:43.10-05:00", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4f timestamp space", value: "2001-12-14 21:59:43", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4f prefix", value: "2001-12-14 is a date", wantErr: spec.ErrAmbiguousScalar},
		{name: "R4f near miss extra digit", value: "2001-12-141", want: "2001-12-141"},
		{name: "R4f near miss day first", value: "14-12-2001", want: "14-12-2001"},
		{name: "R4f near miss no zero padding of year", value: "201-12-14", want: "201-12-14"},
		// R5
		{name: "R5 word", value: "hello", want: "hello"},
		{name: "R5 non-ASCII", value: "héllo", want: "héllo"},
		// not plain
		{name: "quoted true", value: `"true"`, want: "true"},
		{name: "quoted int", value: `'017'`, want: "017"},
		{name: "quoted null", value: `"null"`, want: "null"},
		{name: "quoted empty", value: `""`, want: ""},
		{name: "quoted over safe", value: `"` + overSafe + `"`, want: overSafe},
		{name: "literal block", value: "|-\n  yes", want: "yes"},
		{name: "folded block", value: ">-\n  1.5", want: "1.5"},
	}
}

func TestParseYAMLScalars(t *testing.T) {
	t.Parallel()
	tests := scalarCases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := spec.ParseYAML([]byte("a: " + tt.value + "\n"))
			if tt.wantErr != nil {
				assertSubsetError(t, err, tt.wantErr, 1, 4)
				return
			}
			if err != nil {
				t.Fatalf("ParseYAML() error = %v", err)
			}
			if want := map[string]any{"a": tt.want}; !reflect.DeepEqual(got, want) {
				t.Errorf("ParseYAML() = %#v, want %#v", got, want)
			}
		})
	}
}

// assertSubsetError checks err is a *SubsetError wrapping ErrSubset and
// reason at the given position (a zero column is not checked).
func assertSubsetError(t *testing.T, err, reason error, line, col int) {
	t.Helper()
	if !errors.Is(err, spec.ErrSubset) {
		t.Fatalf("error = %v, want it to wrap ErrSubset", err)
	}
	if !errors.Is(err, reason) {
		t.Fatalf("error = %v, want it to wrap %v", err, reason)
	}
	var se *spec.SubsetError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want a *SubsetError", err)
	}
	if se.Line != line {
		t.Errorf("line = %d, want %d (%v)", se.Line, line, err)
	}
	if col != 0 && se.Column != col {
		t.Errorf("column = %d, want %d (%v)", se.Column, col, err)
	}
	if want := fmt.Sprintf("line %d", line); !strings.Contains(err.Error(), want) {
		t.Errorf("message %q does not mention %q", err.Error(), want)
	}
}

type rejectionCase struct {
	name     string
	doc      string
	reason   error
	line     int
	col      int // 0: not checked
	contains string
}

// rejectionCases lists one or more inputs per rejection reason.
func rejectionCases() []rejectionCase {
	return slices.Concat(rejectionCases1(), rejectionCases2(), rejectionCases3(), rejectionCases4())
}

func rejectionCases1() []rejectionCase {
	return []rejectionCase{
		{name: "invalid UTF-8", doc: "a: \"x\xffy\"\n", reason: spec.ErrEncoding, line: 1, col: 1},
		{name: "UTF-8 encoded surrogate", doc: "a: \"x\xed\xa0\x80y\"\n", reason: spec.ErrEncoding, line: 1, col: 1},
		{name: "truncated UTF-8", doc: "a: \xe2\x82", reason: spec.ErrEncoding, line: 1, col: 1},
		{name: "empty input", doc: "", reason: spec.ErrEmptyDocument, line: 1, col: 1},
		{name: "only blank lines", doc: "\n  \n\n", reason: spec.ErrEmptyDocument, line: 1, col: 1},
		{name: "only comments", doc: "# one\n  # two\n", reason: spec.ErrEmptyDocument, line: 1, col: 1},
		{name: "only a BOM", doc: "\xef\xbb\xbf", reason: spec.ErrEmptyDocument, line: 1, col: 1},
		{name: "only a BOM and a comment with CRLF", doc: "\xef\xbb\xbf# c\r\n", reason: spec.ErrEmptyDocument, line: 1, col: 1},
		{name: "lone CR", doc: "a: 1\rb: 2\n", reason: spec.ErrCharacter, line: 1, col: 5, contains: "U+000D"},
		{name: "lone CR at the end", doc: "a: 1\r", reason: spec.ErrCharacter, line: 1, col: 5},
		{name: "directive hidden behind lone CR", doc: "# c\r%YAML 1.1\na: 1\n", reason: spec.ErrCharacter, line: 1, col: 4},
		{name: "NEL", doc: "# c\u0085%YAML 1.1\na: 1\n", reason: spec.ErrCharacter, line: 1, col: 4, contains: "U+0085"},
		{name: "line separator", doc: "a: 1\u2028b: 2\n", reason: spec.ErrCharacter, line: 1, col: 5, contains: "U+2028"},
		{name: "paragraph separator in a string", doc: "a: \"x\u2029y\"\n", reason: spec.ErrCharacter, line: 1, col: 6},
		{name: "form feed", doc: "a: 1\n\f\nb: 2\n", reason: spec.ErrCharacter, line: 2, col: 1},
		{name: "NUL", doc: "a: \"x\x00y\"\n", reason: spec.ErrCharacter, line: 1, col: 6},
		{name: "DEL", doc: "a: \"x\x7fy\"\n", reason: spec.ErrCharacter, line: 1, col: 6},
		{name: "C1 control", doc: "a: \"x\u0080y\"\n", reason: spec.ErrCharacter, line: 1, col: 6},
		{name: "BOM inside the text", doc: "a: \"x\ufeffy\"\n", reason: spec.ErrCharacter, line: 1, col: 6},
		{name: "BOM twice", doc: "\xef\xbb\xbf\xef\xbb\xbfa: 1\n", reason: spec.ErrCharacter, line: 1, col: 1},
		{name: "noncharacter", doc: "a: \"x\ufffey\"\n", reason: spec.ErrCharacter, line: 1, col: 6},
		{name: "position counts runes", doc: "k: \U0001F600\nb: \"\x01\"\n", reason: spec.ErrCharacter, line: 2, col: 5},
		{name: "tab-indented comment", doc: "\t# c\na: 1\n", reason: spec.ErrTabIndent, line: 1, col: 1},
		{name: "tab-indented blank line", doc: "a: 1\n  \t\nb: 2\n", reason: spec.ErrTabIndent, line: 2, col: 3},
		{name: "tab-indented block", doc: "a:\n\tb: 1\n", reason: spec.ErrTabIndent, line: 2, col: 1},
		{name: "tab after a dash", doc: "- \tx\n", reason: spec.ErrTabIndent, line: 1, col: 3},
		{name: "tab after nested dashes", doc: "- - \tx\n", reason: spec.ErrTabIndent, line: 1, col: 5},
		{name: "tab inside a quoted continuation line", doc: "a: \"x\n\ty\"\n", reason: spec.ErrTabIndent, line: 2, col: 1},
		{name: "flow scalar: trailing colon", doc: "a: [b:]\n", reason: spec.ErrFlowScalar, line: 1, col: 5},
		{name: "flow scalar: trailing colon in a flow mapping key", doc: "{b:: 1}\n", reason: spec.ErrFlowScalar, line: 1, col: 2},
		{name: "flow scalar: trailing colon in a nested flow mapping", doc: "- {a: [x, z:]}\n", reason: spec.ErrFlowScalar, line: 1, col: 11},
		{name: "nesting too deep, flow", doc: "a: " + strings.Repeat("[", spec.MaxDepth) + strings.Repeat("]", spec.MaxDepth) + "\n", reason: spec.ErrTooDeep, line: 1, col: 3 + spec.MaxDepth},
		{name: "nesting too deep, block", doc: blockMappings(spec.MaxDepth + 1), reason: spec.ErrTooDeep, line: spec.MaxDepth + 1, col: spec.MaxDepth + 1},
	}
}

func rejectionCases2() []rejectionCase {
	return []rejectionCase{
		{name: "block scalar indentation indicator", doc: "a: |2\n   x\n", reason: spec.ErrBlockIndicator, line: 1, col: 4},
		{name: "folded block scalar indentation indicator with chomping", doc: "a: >-1\n  x\n", reason: spec.ErrBlockIndicator, line: 1, col: 4},
		{name: "block scalar chomping after the indentation indicator", doc: "a: |1+\n  x\n", reason: spec.ErrBlockIndicator, line: 1, col: 4},
		{name: "tag on key in flow mapping", doc: "{! a: 1}\n", reason: spec.ErrTag, line: 1, col: 2},
		{name: "custom tag on key in flow mapping", doc: "{!!str a: 1}\n", reason: spec.ErrTag, line: 1, col: 2},
		{name: "anchor on key", doc: "{&x a: 1}\n", reason: spec.ErrAnchor, line: 1, col: 2},
		{name: "directive YAML 1.1", doc: "%YAML 1.1\n---\na: 1\n", reason: spec.ErrDirective, line: 1, col: 1},
		{name: "directive YAML 1.2", doc: "%YAML 1.2\n---\na: 1\n", reason: spec.ErrDirective, line: 1, col: 1},
		{name: "directive TAG", doc: "%TAG ! tag:example.com,2000:\n---\na: 1\n", reason: spec.ErrDirective, line: 1, col: 1},
		{name: "directive after comment and blank lines", doc: "# c\n\n  \n   # c2\n%YAML 1.1\n---\na: 1\n", reason: spec.ErrDirective, line: 5, col: 1},
		{name: "directive after BOM", doc: "\xef\xbb\xbf%YAML 1.1\n---\na: 1\n", reason: spec.ErrDirective, line: 1, col: 1},
		{name: "directive with CRLF", doc: "# c\r\n%YAML 1.1\r\n---\r\na: 1\r\n", reason: spec.ErrDirective, line: 2, col: 1},
	}
}

func rejectionCases3() []rejectionCase {
	return []rejectionCase{
		{name: "tag !!str", doc: "a: !!str 5\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag !!int", doc: "a: !!int 5\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag !!null", doc: "a: !!null\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag custom", doc: "a: !custom 5\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag verbatim", doc: "a: !<tag:yaml.org,2002:str> 5\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on plain", doc: "a: ! 5\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on empty", doc: "a: !\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on quoted", doc: "a: ! \"x\"\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on literal", doc: "a: ! |\n  x\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on flow mapping", doc: "a: ! {}\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on block mapping", doc: "a: !\n  b: 1\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on block sequence", doc: "a: !\n  - 1\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "tag non-specific on root scalar", doc: "--- ! 5\n", reason: spec.ErrTag, line: 1, col: 5},
		{name: "tag non-specific on root mapping", doc: "--- !\na: 1\n", reason: spec.ErrTag, line: 1, col: 5},
		{name: "tag non-specific on key", doc: "! a: 1\n", reason: spec.ErrTag, line: 1, col: 1},
		{name: "tag custom on key", doc: "!x a: 1\n", reason: spec.ErrTag, line: 1, col: 1},
		{name: "tag on sequence item", doc: "- ! x\n", reason: spec.ErrTag, line: 1, col: 3},
		{name: "tag after one non-ASCII rune", doc: "{a: \"é\", b: ! 5}\n", reason: spec.ErrTag, line: 1, col: 13},
		{name: "tag after two non-ASCII runes", doc: "{a: \"éé\", b: ! 5}\n", reason: spec.ErrTag, line: 1, col: 14},
		{name: "tag after wide runes on CRLF line", doc: "[\r\n{a: \"日本語\", b: ! 5}\r\n]\r\n", reason: spec.ErrTag, line: 2, col: 15},
		{name: "tag on the second of two lines", doc: "a: 1\nb: ! 2\n", reason: spec.ErrTag, line: 2, col: 4},
		{name: "tag after BOM", doc: "\xef\xbb\xbfa: ! 5\n", reason: spec.ErrTag, line: 1, col: 4},
		{name: "anchor on scalar", doc: "a: &x 1\n", reason: spec.ErrAnchor, line: 1, col: 4},
		{name: "anchor on mapping", doc: "a: &x\n  b: 1\n", reason: spec.ErrAnchor, line: 1, col: 4},
		{name: "alias is reported at its anchor", doc: "a: &x\n  b: 1\nc: *x\n", reason: spec.ErrAnchor, line: 1, col: 4},
		{name: "second document", doc: "a: 1\n---\nb: 2\n", reason: spec.ErrMultipleDocuments, line: 2},
		{name: "trailing document marker", doc: "a: 1\n---\n", reason: spec.ErrMultipleDocuments, line: 2},
		{name: "key is a mapping", doc: "{{a: b}: 1}\n", reason: spec.ErrKeyNotScalar, line: 1, col: 2},
		{name: "key is a sequence", doc: "{[a, b]: 1}\n", reason: spec.ErrKeyNotScalar, line: 1, col: 2},
		{name: "key is true", doc: "true: x\n", reason: spec.ErrKeyNotString, line: 1, col: 1},
		{name: "key is an integer", doc: "1: x\n", reason: spec.ErrKeyNotString, line: 1, col: 1},
		{name: "key is tilde", doc: "~: x\n", reason: spec.ErrKeyNotString, line: 1, col: 1},
		{name: "key is null", doc: "null: x\n", reason: spec.ErrKeyNotString, line: 1, col: 1},
		{name: "key is leading-zero integer", doc: "017: x\n", reason: spec.ErrAmbiguousScalar, line: 1, col: 1},
		{name: "key is merge", doc: "<<: {a: 1}\n", reason: spec.ErrAmbiguousScalar, line: 1, col: 1},
	}
}

func rejectionCases4() []rejectionCase {
	return []rejectionCase{
		{name: "duplicate key", doc: "a: 1\nb: 2\na: 3\n", reason: spec.ErrDuplicateKey, line: 3, col: 1, contains: `duplicate key: "a"`},
		{name: "duplicate key quoted", doc: "a: 1\n\"a\": 2\n", reason: spec.ErrDuplicateKey, line: 2, col: 1},
		{name: "duplicate key nested", doc: "x:\n  a: 1\n  a: 2\n", reason: spec.ErrDuplicateKey, line: 3, col: 3},
		{name: "duplicate key in flow mapping in sequence", doc: "- {a: 1, a: 2}\n", reason: spec.ErrDuplicateKey, line: 1, col: 10},
		{name: "same key in sibling mappings is not a duplicate", doc: "- {a: 1}\n- {a: 2, b: yes}\n", reason: spec.ErrAmbiguousScalar, line: 2, col: 13},
		{name: "ambiguous in sequence", doc: "- 1\n- yes\n", reason: spec.ErrAmbiguousScalar, line: 2, col: 3, contains: `"yes"`},
		{name: "ambiguous root scalar", doc: "yes\n", reason: spec.ErrAmbiguousScalar, line: 1, col: 1},
		{name: "ambiguous multi-line plain scalar", doc: "a: 2001-12-14\n  rest\n", reason: spec.ErrAmbiguousScalar, line: 1, col: 4},
		{name: "integer out of range", doc: "a: " + overSafe + "\n", reason: spec.ErrIntRange, line: 1, col: 4, contains: overSafe},
		{name: "integer out of range, second line", doc: "a: 1\nb: -" + overSafe + "\n", reason: spec.ErrIntRange, line: 2, col: 4},
	}
}

func TestParseYAMLRejections(t *testing.T) {
	t.Parallel()
	tests := rejectionCases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, parse := range subsetLoaders() {
				_, err := parse.fn([]byte(tt.doc))
				if err == nil {
					t.Fatalf("%s: accepted, want rejection", parse.name)
				}
				assertSubsetError(t, err, tt.reason, tt.line, tt.col)
				if tt.contains != "" && !strings.Contains(err.Error(), tt.contains) {
					t.Errorf("%s: error %q does not contain %q", parse.name, err, tt.contains)
				}
			}
		})
	}
}

type loader struct {
	name string
	fn   func([]byte) (any, error)
}

// subsetLoaders are the entry points that must all enforce the subset.
func subsetLoaders() []loader {
	return []loader{
		{"ParseYAML", spec.ParseYAML},
		{"ParseRules", func(b []byte) (any, error) { return spec.ParseRules(b) }},
		{"ParseCase", func(b []byte) (any, error) { return spec.ParseCase(b) }},
	}
}

func TestParseYAMLSyntaxErrorsAreNotSubsetViolations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		doc  string
	}{
		{name: "unclosed flow sequence", doc: "a: [\n"},
		{name: "percent line after the first line", doc: "a: 1\n%YAML 1.1\n"},
		{name: "indented percent line", doc: "  %YAML 1.1\na: 1\n"},
		{name: "unknown anchor", doc: "- *x\n"},
		{name: "document end marker only", doc: "...\n"},
		{name: "anchor after a quoted key", doc: "a: { 'q'\n  &a}\n"},
		{name: "syntax error in a second document", doc: "a: 1\n---\n[\n"},
		{name: "directive after the start marker", doc: "---\n%YAML 1.2\na: 1\n"},
		{name: "unclosed quote", doc: "a: \"x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.ParseYAML([]byte(tt.doc))
			if err == nil || errors.Is(err, spec.ErrSubset) {
				t.Fatalf("ParseYAML() error = %v, want a non-subset error", err)
			}
		})
	}
}

type valueCase struct {
	name string
	doc  []byte
	want any
}

func valueCases() []valueCase {
	return slices.Concat(valueCases1(), valueCases2())
}

func valueCases1() []valueCase {
	return []valueCase{
		{name: "mapping", doc: []byte("a: 1\nb: [x, true, null, \"2\"]\nc: {d: ~}\n"), want: map[string]any{
			"a": int64(1), "b": []any{"x", true, nil, "2"}, "c": map[string]any{"d": nil},
		}},
		{name: "top-level sequence", doc: []byte("- 1\n- two\n"), want: []any{int64(1), "two"}},
		{name: "top-level scalar", doc: []byte("hello\n"), want: "hello"},
		{name: "empty flow collections", doc: []byte("a: []\nb: {}\n"), want: map[string]any{"a": []any{}, "b": map[string]any{}}},
		{name: "explicit start marker", doc: []byte("---\na: 1\n"), want: map[string]any{"a": int64(1)}},
		{name: "start marker only is a null", doc: []byte("---\n"), want: nil},
		{name: "start marker without a line break", doc: []byte("---"), want: nil},
		{name: "start marker with a comment", doc: []byte("--- # c\n"), want: nil},
		{name: "start marker and document end marker", doc: []byte("---\n...\n"), want: nil},
		{name: "BOM and start marker", doc: []byte("\xef\xbb\xbf---\n"), want: nil},
		{name: "missing final line break in a folded scalar", doc: []byte("a: >\n  x\n  y"), want: map[string]any{"a": "x y\n"}},
		{name: "missing final line break in a literal scalar", doc: []byte("a: |\n  x"), want: map[string]any{"a": "x\n"}},
		{name: "missing final line break", doc: []byte("a: 1"), want: map[string]any{"a": int64(1)}},
		{name: "keys without values", doc: []byte("{a, b: }\n"), want: map[string]any{"a": nil, "b": nil}},
		{name: "quoted scalars may end with a colon in a flow collection", doc: []byte("a: [\"x:\", 'y:', {\"k\": 1}]\nb: {\"k\":1}\n"), want: map[string]any{"a": []any{"x:", "y:", map[string]any{"k": int64(1)}}, "b": map[string]any{"k": int64(1)}}},
		{name: "a plain scalar may contain a colon in a flow collection", doc: []byte("a: [x:y, {p: q:r}]\n"), want: map[string]any{"a": []any{"x:y", map[string]any{"p": "q:r"}}}},
		{name: "escapes both parsers read alike", doc: []byte("a: \"x\\\\\n  y\"\nb: \"\\n\\t\\u00e9\\x41\\N\\_\\ \\0\\e\\L\"\nc: 'x\\'\nd: x\\\n"), want: map[string]any{
			"a": "x\\ y", "b": "\n\t\u00e9A\u0085\u00a0 \x00\x1b\u2028", "c": "x\\", "d": "x\\",
		}},
	}
}

func valueCases2() []valueCase {
	return []valueCase{
		{name: "literal block with lines that look like entries", doc: []byte("a: |\n  - x\n  k: v\n  # c\nb: 1\n"), want: map[string]any{"a": "- x\nk: v\n# c\n", "b": int64(1)}},
		{name: "plain scalar continued on a line that looks like a comment-free entry", doc: []byte("a: x\n  - y\n"), want: map[string]any{"a": "x - y"}},
		{name: "comments inside a flow collection hold brackets and quotes", doc: []byte("a: [x, # ] }\n  z] # ok\nb: [\"x[#\", 'y,#', {\"k\": 1}] # ok\n"), want: map[string]any{
			"a": []any{"x", "z"}, "b": []any{"x[#", "y,#", map[string]any{"k": int64(1)}},
		}},
		{name: "block scalar headers with chomping only", doc: []byte("a: |-\n  x\nb: >+\n  y\n"), want: map[string]any{"a": "x", "b": "y\n"}},
		{name: "quoted scalars with escapes inside flow collections", doc: []byte("a: [\"x\", {\"k\": 'v'}, 'it''s', \"a\\\"b\"]\nb: ['x\n  y',\n  z]\n"), want: map[string]any{
			"a": []any{"x", map[string]any{"k": "v"}, "it's", "a\"b"}, "b": []any{"x y", "z"},
		}},
		{name: "document end marker, then comments", doc: []byte("a: 1\n... # c\n\n# d\n"), want: map[string]any{"a": int64(1)}},
		{name: "a single document end marker", doc: []byte("a: 1\n...\n"), want: map[string]any{"a": int64(1)}},
		{name: "document end marker with a tab", doc: []byte("a: 1\n...\t\n"), want: map[string]any{"a": int64(1)}},
		{name: "explicit null document", doc: []byte("null\n"), want: nil},
		{name: "BOM", doc: []byte("\xef\xbb\xbfa: 1\nb: two\n"), want: map[string]any{"a": int64(1), "b": "two"}},
		{name: "CRLF", doc: []byte("a: 1\r\nb:\r\n  - x\r\n  - z\r\nc: foo\r\n  bar\r\n"), want: map[string]any{
			"a": int64(1), "b": []any{"x", "z"}, "c": "foo bar",
		}},
		{name: "BOM and CRLF", doc: []byte("\xef\xbb\xbf# c\r\na: yes_it_is\r\n"), want: map[string]any{"a": "yes_it_is"}},
		{name: "non-ASCII before a plain value", doc: []byte("{a: \"é\", b: 5}\n"), want: map[string]any{"a": "é", "b": int64(5)}},
	}
}

func TestParseYAMLValues(t *testing.T) {
	t.Parallel()
	tests := valueCases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := spec.ParseYAML(tt.doc)
			if err != nil {
				t.Fatalf("ParseYAML() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseYAML() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseYAMLNesting(t *testing.T) {
	t.Parallel()
	const depth = spec.MaxDepth
	tests := []struct {
		name    string
		doc     string
		wantErr bool // any error
		col     int  // when set, the doc must fail with ErrTooDeep at line 1 and this column
	}{
		{name: "flow sequences", doc: strings.Repeat("[", depth) + "1" + strings.Repeat("]", depth)},
		{name: "flow mappings", doc: strings.Repeat("{a: ", depth) + "1" + strings.Repeat("}", depth)},
		{name: "block mappings", doc: blockMappings(depth)},
		{name: "one level too many", doc: strings.Repeat("[", depth+1) + strings.Repeat("]", depth+1), col: depth + 1},
		{name: "beyond the parser limit is an error, not a crash", doc: strings.Repeat("[", 100000) + strings.Repeat("]", 100000), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.ParseYAML([]byte(tt.doc))
			switch {
			case tt.col != 0:
				assertSubsetError(t, err, spec.ErrTooDeep, 1, tt.col)
			case tt.wantErr && err == nil:
				t.Fatal("ParseYAML() accepted the document, want an error")
			case !tt.wantErr && err != nil:
				t.Fatalf("ParseYAML() error = %v", err)
			}
		})
	}
}

func blockMappings(depth int) string {
	var b strings.Builder
	for i := range depth {
		b.WriteString(strings.Repeat(" ", i))
		b.WriteString("k:\n")
	}
	b.WriteString(strings.Repeat(" ", depth) + "v\n")
	return b.String()
}

// TestTypeChecksAgreeWithClassifier covers texts that yaml.v3 types as numbers
// but the subset calls strings: the string checks of the loaders must accept
// them, as the JS loader does.
func TestTypeChecksAgreeWithClassifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
	}{
		{name: "0e0_", text: "0e0_"},
		{name: "1e_0", text: "1e_0"},
		{name: "0o-1", text: "0o-1"},
		{name: "0b+1", text: "0b+1"},
		{name: "0_x1", text: "0_x1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := spec.ParseCase([]byte(strings.Replace(okCase, "description: d", "description: "+tt.text, 1)))
			if err != nil {
				t.Fatalf("ParseCase() error = %v", err)
			}
			if c.Description != tt.text {
				t.Errorf("Description = %q, want %q", c.Description, tt.text)
			}
		})
	}
}

func TestSubsetErrorMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  *spec.SubsetError
		want string
	}{
		{name: "with column", err: &spec.SubsetError{Line: 3, Column: 7, Reason: spec.ErrTag}, want: "line 3, column 7: explicit tag not allowed"},
		{name: "without column", err: &spec.SubsetError{Line: 3, Reason: spec.ErrAnchor}, want: "line 3: anchor not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func readProbes(tb testing.TB) map[string][]byte {
	tb.Helper()
	entries, err := os.ReadDir(probeDir)
	if err != nil {
		tb.Fatalf("read probe directory: %v", err)
	}
	files := make(map[string][]byte, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(probeDir, e.Name()))
		if err != nil {
			tb.Fatal(err)
		}
		files[e.Name()] = data
	}
	return files
}

// canonicalJSON renders a decoded value (from ParseYAML or from JSON decoded
// with UseNumber) in one canonical form, so integers compare exactly.
func canonicalJSON(tb testing.TB, v any) string {
	tb.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		tb.Fatalf("marshal %#v: %v", v, err)
	}
	return string(b)
}

func decodeJSONExact(tb testing.TB, data []byte) any {
	tb.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		tb.Fatalf("decode JSON: %v", err)
	}
	return v
}

// probeCase is one row of TestProbes, built from the probe directory listing.
type probeCase struct {
	name    string
	reject  bool
	data    []byte
	sidecar []byte
	problem string // a structural problem of the directory, reported as a failure
}

// probeCases builds one row per probe from the directory listing, and one
// row per file that is neither a probe nor the sidecar of an accept probe.
func probeCases(tb testing.TB) []probeCase {
	tb.Helper()
	files := readProbes(tb)
	names := slices.Sorted(maps.Keys(files))
	var cases []probeCase
	for _, name := range names {
		slug, ext, _ := strings.Cut(name, ".")
		switch {
		case strings.HasPrefix(name, "reject-") && ext == "yaml":
			cases = append(cases, probeCase{name: slug, reject: true, data: files[name]})
		case strings.HasPrefix(name, "accept-") && ext == "yaml":
			sidecar, ok := files[slug+".json"]
			if !ok {
				cases = append(cases, probeCase{name: slug, problem: name + " has no .json sidecar"})
				continue
			}
			cases = append(cases, probeCase{name: slug, data: files[name], sidecar: sidecar})
		case strings.HasPrefix(name, "accept-") && ext == "json":
			if _, ok := files[slug+".yaml"]; !ok {
				cases = append(cases, probeCase{name: name, problem: name + " is an orphan sidecar"})
			}
		default:
			cases = append(cases, probeCase{name: name, problem: "unexpected probe file " + name})
		}
	}
	return cases
}

func TestProbes(t *testing.T) {
	t.Parallel()
	tests := probeCases(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			switch {
			case tt.problem != "":
				t.Error(tt.problem)
			case tt.reject:
				checkRejectProbe(t, tt.name, tt.data)
			default:
				checkAcceptProbe(t, tt.data, tt.sidecar)
			}
		})
	}
}

func TestProbeCounts(t *testing.T) {
	t.Parallel()
	cases := probeCases(t)
	count := func(reject bool) int {
		return len(slices.DeleteFunc(slices.Clone(cases), func(c probeCase) bool { return c.problem != "" || c.reject != reject }))
	}
	tests := []struct {
		name string
		got  int
		min  int
	}{
		{name: "reject probes", got: count(true), min: minReject},
		{name: "accept probes", got: count(false), min: minAccept},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got < tt.min {
				t.Errorf("found %d %s, want at least %d", tt.got, tt.name, tt.min)
			}
		})
	}
}

// probeReasons maps the name prefix of a reject probe to the error it must
// fail with, so that it fails for the reason its name says; the first matching
// prefix wins (spec/probes/README.md lists the same table). A nil reason is a
// YAML syntax error of yaml.v3 itself, which is not a subset violation.
var probeReasons = []struct {
	prefix string
	reason error
}{
	{"reject-empty-document-end-marker", nil}, // a syntax error of yaml.v3; the JS loader calls it an empty document
	{"reject-empty-", spec.ErrEmptyDocument},
	{"reject-directive-after-start-marker", nil},
	{"reject-directive-indented", nil},
	{"reject-directive-", spec.ErrDirective},
	{"reject-invalid-utf8", spec.ErrEncoding},
	{"reject-surrogate-utf8", spec.ErrEncoding},
	{"reject-bom-", spec.ErrCharacter},
	{"reject-lone-cr", spec.ErrCharacter},
	{"reject-nel-", spec.ErrCharacter},
	{"reject-line-separator-", spec.ErrCharacter},
	{"reject-paragraph-separator-", spec.ErrCharacter},
	{"reject-form-feed", spec.ErrCharacter},
	{"reject-nul", spec.ErrCharacter},
	{"reject-delete-char", spec.ErrCharacter},
	{"reject-tab-", spec.ErrTabIndent},
	{"reject-tag-after-quoted-key", nil}, // syntax errors of yaml.v3; the JS loader would drop the property
	{"reject-tag-", spec.ErrTag},
	{"reject-anchor-after-quoted-key", nil},
	{"reject-anchor", spec.ErrAnchor},
	{"reject-alias", spec.ErrAnchor}, // an alias needs an anchor, which is rejected first
	{"reject-key-complex-", spec.ErrKeyNotScalar},
	{"reject-key-", spec.ErrKeyNotString},
	{"reject-duplicate-keys", spec.ErrDuplicateKey},
	{"reject-two-documents", spec.ErrMultipleDocuments},
	{"reject-trailing-document-marker", spec.ErrMultipleDocuments},
	{"reject-too-deep", spec.ErrTooDeep},
	{"reject-block-indicator", spec.ErrBlockIndicator},
	{"reject-flow-scalar-", spec.ErrFlowScalar},
	{"reject-out-of-range-", spec.ErrIntRange},
	{"reject-ambiguous-", spec.ErrAmbiguousScalar},
}

// checkRejectProbe asserts the failure named by the probe's prefix.
func checkRejectProbe(t *testing.T, name string, data []byte) {
	t.Helper()
	_, err := spec.ParseYAML(data)
	name += ".yaml"
	for _, p := range probeReasons {
		if !strings.HasPrefix(name, p.prefix) {
			continue
		}
		switch {
		case err == nil:
			t.Fatal("ParseYAML() accepted a reject probe")
		case p.reason == nil && errors.Is(err, spec.ErrSubset):
			t.Fatalf("ParseYAML() error = %v, want a yaml.v3 syntax error", err)
		case p.reason != nil && !errors.Is(err, p.reason):
			t.Fatalf("ParseYAML() error = %v, want it to wrap %v", err, p.reason)
		}
		return
	}
	t.Fatalf("no entry in probeReasons matches %s", name)
}

func checkAcceptProbe(t *testing.T, data, sidecar []byte) {
	t.Helper()
	got, err := spec.ParseYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML() error = %v", err)
	}
	if g, w := canonicalJSON(t, got), canonicalJSON(t, decodeJSONExact(t, sidecar)); g != w {
		t.Errorf("ParseYAML() = %s, want %s", g, w)
	}
}

func TestProbesCoverTheSafeIntegerBounds(t *testing.T) {
	t.Parallel()
	files := readProbes(t)
	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "max", file: "accept-int-max-safe.json", want: maxSafeText},
		{name: "min", file: "accept-int-min-safe.json", want: "-" + maxSafeText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := string(files[tt.file]); !strings.Contains(got, tt.want) {
				t.Errorf("%s = %q, want it to contain %s", tt.file, got, tt.want)
			}
		})
	}
}

// realFiles lists the YAML files the Go loaders read in this repository.
func realFiles(tb testing.TB) []string {
	tb.Helper()
	var paths []string
	for _, root := range []string{"../../../spec/rules.yaml", "../../../spec/fixtures", "testdata/rules.yaml", "../../conformance/testdata"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".yaml") {
				paths = append(paths, path)
			}
			return err
		})
		if err != nil {
			tb.Fatalf("walk %s: %v", root, err)
		}
	}
	return paths
}

// TestRealFilesStillLoad checks that every YAML file the Go loaders read in
// this repository satisfies the subset, through the loader that reads it.
func TestRealFilesStillLoad(t *testing.T) {
	t.Parallel()
	paths := realFiles(t)
	tests := make([]struct{ name, path string }, 0, len(paths))
	for _, path := range paths {
		tests = append(tests, struct{ name, path string }{name: path, path: path})
	}
	if len(tests) == 0 {
		t.Fatal("no YAML files found")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			switch filepath.Base(tt.path) {
			case "rules.yaml":
				_, err = spec.ParseRules(data)
			case "case.yaml":
				_, err = spec.ParseCase(data)
			default:
				_, err = spec.ParseYAML(data)
			}
			if err != nil {
				t.Errorf("%s: %v", tt.path, err)
			}
		})
	}
}

func TestRealFilesCoverTheLoaders(t *testing.T) {
	t.Parallel()
	visited := map[string]int{}
	for _, path := range realFiles(t) {
		visited[filepath.Base(path)]++
	}
	tests := []struct {
		name string
		base string
	}{
		{name: "rules", base: "rules.yaml"},
		{name: "cases", base: "case.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if visited[tt.base] == 0 {
				t.Errorf("no %s file was found", tt.base)
			}
		})
	}
}

// fuzzSeeds are the documents the fuzzer starts from: every probe and every
// input of the tables above.
func fuzzSeeds(tb testing.TB) [][]byte {
	tb.Helper()
	probes := readProbes(tb)
	seeds := make([][]byte, 0, len(probes)+64)
	seeds = append(seeds, []byte("a: [1, {b: ~}]\n"))
	for _, data := range probes {
		seeds = append(seeds, data)
	}
	for _, c := range scalarCases() {
		seeds = append(seeds, []byte("a: "+c.value+"\n"))
	}
	for _, c := range rejectionCases() {
		seeds = append(seeds, []byte(c.doc))
	}
	for _, c := range valueCases() {
		seeds = append(seeds, c.doc)
	}
	return seeds
}

func FuzzParseYAML(f *testing.F) {
	for _, data := range fuzzSeeds(f) {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := spec.ParseYAML(data)
		if err != nil {
			return
		}
		if bad := disallowedType(v); bad != "" {
			t.Fatalf("ParseYAML(%q) returned a value containing %s", data, bad)
		}
	})
}

// disallowedType returns the type of the first value that is not one of
// map[string]any, []any, string, bool, int64 or nil, or "".
func disallowedType(v any) string {
	switch x := v.(type) {
	case nil, string, bool:
		return ""
	case int64:
		if x > 9007199254740991 || x < -9007199254740991 {
			return fmt.Sprintf("out-of-range int64 %d", x)
		}
		return ""
	case []any:
		for _, e := range x {
			if bad := disallowedType(e); bad != "" {
				return bad
			}
		}
		return ""
	case map[string]any:
		for _, e := range x {
			if bad := disallowedType(e); bad != "" {
				return bad
			}
		}
		return ""
	}
	return fmt.Sprintf("%T", v)
}
