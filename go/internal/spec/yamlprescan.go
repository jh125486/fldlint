package spec

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// utf8BOM is the byte order mark yaml.v3 skips at the start of the input.
var utf8BOM = []byte("\xef\xbb\xbf")

// forbidden reports whether r at src[i:] is a character the subset does not
// allow: outside YAML's printable set (tab, LF, CR, U+0020-007E, U+0085,
// U+00A0-D7FF, U+E000-FFFD, U+10000-10FFFF), except U+FEFF (a BOM is only
// stripped at the very start), a CR not followed by LF, NEL (U+0085,
// printable in YAML but a line break to yaml.v3), LS or PS.
func forbidden(src []byte, i int, r rune) bool {
	switch {
	case r == '\r':
		return i+1 >= len(src) || src[i+1] != '\n'
	case r == '\t' || r == '\n' || (r >= 0x20 && r <= 0x7E) || (r >= 0xA0 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD && r != 0xFEFF) || r >= 0x10000:
		return r == ' ' || r == ' '
	}
	return true
}

// blankOrComment reports whether a line holds only spaces and tabs, or is a
// comment: "#" is its first non-space character.
func blankOrComment(line []byte) bool {
	trimmed := bytes.TrimLeft(line, " \t")
	return len(trimmed) == 0 || trimmed[0] == '#'
}

// prescan implements the text-level rules of the contract on the raw bytes,
// before yaml.v3 sees them, in this order of precedence: valid UTF-8, only
// allowed characters, no tab indentation, some content, no directive. yaml.v3
// would consume a directive (and silently accept %YAML 1.1), treats NEL, LS,
// PS and a lone CR as line breaks and rejects control characters that the
// JavaScript parser accepts, so these cannot be left to it.
func prescan(data []byte) error {
	if !utf8.Valid(data) {
		return newSubsetError(1, 1, ErrEncoding, "")
	}
	src := bytes.TrimPrefix(data, utf8BOM)
	line, col, indent := 1, 1, true // indent: only spaces, "-" and "?" so far on this line
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if forbidden(src, i, r) {
			return newSubsetError(line, col, ErrCharacter, fmt.Sprintf("U+%04X", r))
		}
		if r == '\t' && indent {
			return newSubsetError(line, col, ErrTabIndent, "")
		}
		i += size
		col++
		indent = (indent && (r == ' ' || r == '-' || r == '?')) || r == '\n'
		if r == '\n' {
			line, col = line+1, 1
		}
	}
	for n, text := range bytes.Split(src, []byte{'\n'}) {
		if blankOrComment(bytes.TrimSuffix(text, []byte{'\r'})) {
			continue
		}
		if text[0] == '%' {
			return newSubsetError(n+1, 1, ErrDirective, "")
		}
		return nil
	}
	return newSubsetError(1, 1, ErrEmptyDocument, "")
}

// withFinalLF appends a line feed to data that does not end with one. A
// missing final line break is treated as present because the parsers differ
// on whether a block scalar at the end of the input keeps its line break.
func withFinalLF(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	return append(data[:len(data):len(data)], '\n')
}
