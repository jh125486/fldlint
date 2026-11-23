package position_test

import (
	"testing"

	"github.com/jh125486/fldlint/go/internal/position"
)

func TestUTF16Col(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		src    string
		offset int
		want   int
	}{
		{name: "start of file", src: "abc", offset: 0, want: 1},
		{name: "ascii", src: "abc", offset: 2, want: 3},
		{name: "end of file", src: "abc", offset: 3, want: 4},
		{name: "tab is one unit", src: "\t\tx", offset: 2, want: 3},
		{name: "two byte rune is one unit", src: "é=x", offset: len("é="), want: 3},
		{name: "three byte rune is one unit", src: "€x", offset: len("€"), want: 2},
		{name: "astral rune is two units", src: "😀x", offset: len("😀"), want: 3},
		{name: "mixed prefix", src: "/* café 😀 */ x", offset: len("/* café 😀 */ "), want: 15},
		{name: "second line restarts", src: "é\nab😀c", offset: len("é\nab😀"), want: 5},
		{name: "start of second line", src: "😀\nx", offset: len("😀\n"), want: 1},
		{name: "offset past end clamps", src: "ab", offset: 99, want: 3},
		{name: "negative offset clamps", src: "ab", offset: -4, want: 1},
		{name: "invalid byte counts one", src: "\xffx", offset: 1, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := position.UTF16Col([]byte(tt.src), tt.offset); got != tt.want {
				t.Fatalf("UTF16Col(%q, %d) = %d, want %d", tt.src, tt.offset, got, tt.want)
			}
		})
	}
}
