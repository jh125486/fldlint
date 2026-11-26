package position_test

import (
	"bytes"
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
		{"a\xff\n", "\xc3(\xf0\x9f", 3},              // invalid UTF-8: one unit per bad byte
	} {
		f.Add(s.before, s.line, s.off, int16(min(s.off, 1<<14)))
	}
	f.Add("ab", "cd", uint16(0), int16(-5))   // negative offset clamps to 0
	f.Add("ab", "cd", uint16(0), int16(1000)) // past the end clamps to len
	f.Fuzz(func(t *testing.T, rawBefore, rawLine string, off uint16, rawOff int16) {
		// Oracle 2 (raw bytes): any bytes and any offset. Documented
		// behaviour: an out-of-range offset is clamped, an invalid byte
		// counts as one unit. Checked first, on the untransformed input.
		raw := []byte(rawBefore + "\n" + rawLine)
		clamped := max(0, min(int(rawOff), len(raw)))
		lineStart := bytes.LastIndexByte(raw[:clamped], '\n') + 1
		wantRaw := 1
		for rest := raw[lineStart:clamped]; len(rest) > 0; {
			r, size := utf8.DecodeRune(rest)
			if r >= 0x10000 {
				wantRaw += 2 // a surrogate pair; RuneError is U+FFFD, one unit
			} else {
				wantRaw++
			}
			rest = rest[size:]
		}
		if got := position.UTF16Col(raw, int(rawOff)); got != wantRaw {
			t.Fatalf("raw: UTF16Col(%q, %d) = %d, want %d", raw, rawOff, got, wantRaw)
		}

		// Oracle 1: transform instead of rejecting (a skip would starve the
		// fuzzer), then the definition on valid UTF-8 at a rune boundary.
		before := strings.ToValidUTF8(rawBefore, "\ufffd")
		line := strings.ReplaceAll(strings.ToValidUTF8(rawLine, "\ufffd"), "\n", " ")
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
