package fix_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"

	"github.com/jh125486/fldlint/go/internal/fix"
)

func apply(t *testing.T, src string, edits []fix.Edit, fset *token.FileSet, f *ast.File) string {
	t.Helper()
	tf := fset.File(f.Pos())
	out := []byte(src)
	sorted := slices.Clone(edits)
	slices.SortFunc(sorted, func(a, b fix.Edit) int { return int(b.Pos) - int(a.Pos) })
	for _, e := range sorted {
		out = slices.Concat(out[:tf.Offset(e.Pos)], e.NewText, out[tf.Offset(e.End):])
	}
	return string(out)
}

func TestImportEdits(t *testing.T) {
	t.Parallel()
	const ldctx = "github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	tests := []struct {
		name   string
		src    string
		add    []string
		remove []string // import paths
		want   string
	}{
		{
			name: "add to existing std group, sorted first",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\n\t\"x.com/a\"\n)\n",
			add:  []string{"context"},
			want: "package p\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name: "add to existing std group, sorted last",
			src:  "package p\n\nimport (\n\t\"bytes\"\n\n\t\"x.com/a\"\n)\n",
			add:  []string{"context"},
			want: "package p\n\nimport (\n\t\"bytes\"\n\t\"context\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name: "new std group above third party",
			src:  "package p\n\nimport (\n\t\"x.com/a\"\n)\n",
			add:  []string{"context"},
			want: "package p\n\nimport (\n\t\"context\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name: "new third party group below std",
			src:  "package p\n\nimport (\n\t\"fmt\"\n)\n",
			add:  []string{"x.com/a"},
			want: "package p\n\nimport (\n\t\"fmt\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name: "single line import becomes a group",
			src:  "package p\n\nimport \"x.com/a\"\n",
			add:  []string{"context"},
			want: "package p\n\nimport (\n\t\"context\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name: "single line aliased import keeps alias",
			src:  "package p\n\nimport a \"x.com/a\"\n",
			add:  []string{"context", "x.com/b"},
			want: "package p\n\nimport (\n\t\"context\"\n\n\ta \"x.com/a\"\n\t\"x.com/b\"\n)\n",
		},
		{
			name: "no import decl",
			src:  "package p\n\nvar x = 1\n",
			add:  []string{"context", "x.com/a"},
			want: "package p\n\nimport (\n\t\"context\"\n\n\t\"x.com/a\"\n)\n\nvar x = 1\n",
		},
		{
			name:   "remove from a shared group",
			src:    "package p\n\nimport (\n\t\"x.com/a\"\n\t\"" + ldctx + "\"\n)\n",
			remove: []string{ldctx},
			want:   "package p\n\nimport (\n\t\"x.com/a\"\n)\n",
		},
		{
			name:   "remove a spec alone in its group drops the blank line",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\n\t\"" + ldctx + "\"\n\n\t\"x.com/a\"\n)\n",
			remove: []string{ldctx},
			want:   "package p\n\nimport (\n\t\"fmt\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name:   "remove an aliased spec at the end",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\n\tld \"" + ldctx + "\"\n)\n",
			remove: []string{ldctx},
			want:   "package p\n\nimport (\n\t\"fmt\"\n)\n",
		},
		{
			name:   "never empties the decl",
			src:    "package p\n\nimport (\n\t\"" + ldctx + "\"\n)\n",
			remove: []string{ldctx},
			want:   "package p\n\nimport (\n\t\"" + ldctx + "\"\n)\n",
		},
		{
			name:   "add and remove together, insertion before the removed spec",
			src:    "package p\n\nimport (\n\t\"" + ldctx + "\"\n\t\"x.com/a\"\n)\n",
			add:    []string{"context"},
			remove: []string{ldctx},
			want:   "package p\n\nimport (\n\t\"context\"\n\n\t\"x.com/a\"\n)\n",
		},
		{
			name:   "mixed group is not a std group, so a new one is made",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t\"" + ldctx + "\"\n)\n",
			add:    []string{"context"},
			remove: []string{ldctx},
			want:   "package p\n\nimport (\n\t\"context\"\n\n\t\"fmt\"\n)\n",
		},
		{
			name: "no changes",
			src:  "package p\n",
			want: "package p\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "p.go", tt.src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			var rm []*ast.ImportSpec
			for _, s := range f.Imports {
				if p, _ := strconv.Unquote(s.Path.Value); slices.Contains(tt.remove, p) {
					rm = append(rm, s)
				}
			}
			edits := fix.ImportEdits(fset, f, []byte(tt.src), fix.ImportChange{Add: tt.add, Remove: rm})
			got := apply(t, tt.src, edits, fset, f)
			if got != tt.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tt.want)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "p.go", got, 0); err != nil {
				t.Fatalf("result does not parse: %v", err)
			}
		})
	}
}
