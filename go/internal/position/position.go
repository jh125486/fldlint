// Package position converts Go byte offsets to the UTF-16 columns used by
// fldlint's shared output contract (spec/contracts/output-format.md#positions).
package position

import "unicode/utf8"

// UTF16Col returns the 1-based column, counted in UTF-16 code units, of the
// byte offset within src. The column is measured from the start of the line
// that contains offset; a tab counts as one unit. An offset outside src is
// clamped to the nearest end. Invalid UTF-8 bytes count as one unit each.
func UTF16Col(src []byte, offset int) int {
	offset = max(0, min(offset, len(src)))
	start := 0
	for i := offset - 1; i >= 0; i-- {
		if src[i] == '\n' {
			start = i + 1
			break
		}
	}
	col := 1
	for i := start; i < offset; {
		r, size := utf8.DecodeRune(src[i:offset])
		if r >= 0x10000 {
			col += 2
		} else {
			col++
		}
		i += size
	}
	return col
}
