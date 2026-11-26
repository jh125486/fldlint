package position_test

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jh125486/fldlint/go/internal/position"
)

// FuzzUTF16Col checks UTF16Col against its definition: for valid UTF-8 and a
// byte offset on a rune boundary, the column is the number of UTF-16 code
// units between the start of the line and the offset, plus one. The seeds run
// as a normal unit test; explore with
// `go test -run=^$ -fuzz=FuzzUTF16Col -fuzztime=30s ./internal/position`.
func FuzzUTF16Col(f *testing.F) {
	for _, s := range []struct {
		before, line string
		off          uint16
	}{
		{"", "", 0},
		{"", "abc", 2},
		{"x\ny\n", "\tindented", 3},
		{"line one\r", "aéb", 3},                     // 2-byte rune
		{"", "café \U0001F600 call()", 11},           // astral rune: 2 UTF-16 units
		{"\U0001F600\n", "\U0001F600\U0001F600x", 8}, // earlier lines are ignored
		{"", "e\u0301\u200d\ufeff", 6},               // combining mark, ZWJ, BOM
		{"", "\u2028\u2029", 3},                      // line separators are not newlines
		{"", "end", 100},                             // offset past the line end
	} {
		f.Add(s.before, s.line, s.off)
	}
	f.Fuzz(func(t *testing.T, before, line string, off uint16) {
		if !utf8.ValidString(before) || !utf8.ValidString(line) || strings.Contains(line, "\n") {
			t.Skip("the property is stated for valid UTF-8 and a single line")
		}
		// Snap the offset down to a rune boundary inside the line.
		o := min(int(off), len(line))
		for o < len(line) && !utf8.RuneStart(line[o]) {
			o--
		}
		src := []byte(before + "\n" + line)
		offset := len(before) + 1 + o
		prefix := line[:o]
		want := len(utf16.Encode([]rune(prefix))) + 1
		if got := position.UTF16Col(src, offset); got != want {
			t.Fatalf("UTF16Col(%q, %d) = %d, want %d (prefix %q)", src, offset, got, want, prefix)
		}
	})
}
