package fix

import (
	"go/ast"
	"go/token"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Edit is a single text replacement of [Pos, End).
type Edit struct {
	Pos     token.Pos
	End     token.Pos
	NewText []byte
}

// ImportChange lists import additions and removals for one file.
type ImportChange struct {
	// Add holds import paths to add. The caller guarantees none is present.
	Add []string
	// Remove holds import specs to delete.
	Remove []*ast.ImportSpec
}

// ImportEdits returns the edits that apply ch to file, whose bytes are src.
// It works on source text (not the AST), so the result is a minimal edit set.
// The file is assumed gofmt-formatted: one spec per line, groups separated by
// blank lines.
func ImportEdits(fset *token.FileSet, file *ast.File, src []byte, ch ImportChange) []Edit {
	adds := slices.Clone(ch.Add)
	sort.Strings(adds)
	adds = slices.Compact(adds)
	if len(adds) == 0 && len(ch.Remove) == 0 {
		return nil
	}
	tf := fset.File(file.Pos())
	if tf == nil {
		return nil
	}
	w := &editor{fset: fset, tf: tf, src: src}

	var decls []*ast.GenDecl
	for _, d := range file.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			decls = append(decls, gd)
		}
	}
	w.removed = map[*ast.ImportSpec]bool{}
	for _, spec := range ch.Remove {
		w.removed[spec] = true
	}
	replaced := (*ast.GenDecl)(nil) // decl rewritten whole by the additions
	if len(adds) > 0 {
		target := pickTarget(decls)
		switch {
		case target == nil:
			w.addEdit(Edit{Pos: file.Name.End(), End: file.Name.End(), NewText: []byte("\n\nimport " + block(adds, nil))})
		case !target.Lparen.IsValid() || w.remaining(target) == 0:
			w.addEdit(rewriteDecl(w, target, adds))
			replaced = target
		default:
			w.insertInto(target, adds)
		}
	}
	for _, d := range decls {
		if d == replaced {
			continue
		}
		n := 0
		for _, sp := range d.Specs {
			if w.removed[sp.(*ast.ImportSpec)] {
				n++
			}
		}
		switch {
		case n == 0:
		case n == len(d.Specs):
			w.deleteDecl(d)
		default:
			for _, sp := range d.Specs {
				if w.removed[sp.(*ast.ImportSpec)] {
					w.remove(d, sp.(*ast.ImportSpec))
				}
			}
		}
	}
	return w.result()
}

func pickTarget(decls []*ast.GenDecl) *ast.GenDecl {
	for _, d := range decls {
		if d.Lparen.IsValid() {
			return d
		}
	}
	if len(decls) > 0 {
		return decls[0]
	}
	return nil
}

func isStd(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

func specPath(s *ast.ImportSpec) string {
	p, err := strconv.Unquote(s.Path.Value)
	if err != nil {
		return s.Path.Value
	}
	return p
}

// block renders an import block from std and other-group lines.
func block(adds []string, existing []string) string {
	var std, other []string
	for _, p := range adds {
		if isStd(p) {
			std = append(std, strconv.Quote(p))
		} else {
			other = append(other, strconv.Quote(p))
		}
	}
	for _, e := range existing {
		if isStd(pathOf(e)) {
			std = append(std, e)
		} else {
			other = append(other, e)
		}
	}
	sortSpecs := func(s []string) {
		sort.Slice(s, func(i, j int) bool { return pathOf(s[i]) < pathOf(s[j]) })
	}
	sortSpecs(std)
	sortSpecs(other)
	var b strings.Builder
	b.WriteString("(\n")
	for _, s := range std {
		b.WriteString("\t" + s + "\n")
	}
	if len(std) > 0 && len(other) > 0 {
		b.WriteString("\n")
	}
	for _, s := range other {
		b.WriteString("\t" + s + "\n")
	}
	b.WriteString(")")
	return b.String()
}

func pathOf(line string) string {
	i := strings.Index(line, "\"")
	if i < 0 {
		return line
	}
	p, err := strconv.Unquote(line[i:])
	if err != nil {
		return line
	}
	return p
}

// rewriteDecl replaces a single-line `import "x"` (or a decl whose specs are
// all being removed) by one group holding the additions and the specs that
// stay.
func rewriteDecl(w *editor, d *ast.GenDecl, adds []string) Edit {
	var existing []string
	for _, s := range d.Specs {
		if w.removed[s.(*ast.ImportSpec)] {
			continue
		}
		existing = append(existing, string(w.src[w.off(s.Pos()):w.off(s.End())]))
	}
	return Edit{Pos: d.Pos(), End: d.End(), NewText: []byte("import " + block(adds, existing))}
}

type editor struct {
	fset    *token.FileSet
	tf      *token.File
	src     []byte
	edits   []Edit
	removed map[*ast.ImportSpec]bool
}

// remaining counts the specs of d that are not being removed.
func (w *editor) remaining(d *ast.GenDecl) int {
	n := 0
	for _, s := range d.Specs {
		if !w.removed[s.(*ast.ImportSpec)] {
			n++
		}
	}
	return n
}

// deleteDecl removes a whole import decl, line by line.
func (w *editor) deleteDecl(d *ast.GenDecl) {
	w.addEdit(Edit{Pos: w.lineStart(w.line(d.Pos())), End: w.lineStart(w.line(d.End()) + 1)})
}

func (w *editor) off(p token.Pos) int { return w.tf.Offset(p) }

func (w *editor) line(p token.Pos) int { return w.fset.Position(p).Line }

// lineStart returns the position of the first byte of line n.
func (w *editor) lineStart(n int) token.Pos {
	if n > w.tf.LineCount() {
		return token.Pos(w.tf.Base() + w.tf.Size())
	}
	return w.tf.LineStart(n)
}

func (w *editor) startLine(s *ast.ImportSpec) int {
	if s.Doc != nil {
		return w.line(s.Doc.Pos())
	}
	return w.line(s.Pos())
}

func (w *editor) endLine(s *ast.ImportSpec) int {
	if s.Comment != nil {
		return w.line(s.Comment.End())
	}
	return w.line(s.End())
}

func (w *editor) addEdit(e Edit) { w.edits = append(w.edits, e) }

// groups splits a parenthesized decl's specs into blank-line-separated runs.
func (w *editor) groups(d *ast.GenDecl) [][]*ast.ImportSpec {
	var out [][]*ast.ImportSpec
	for i, raw := range d.Specs {
		s := raw.(*ast.ImportSpec)
		if i == 0 || w.startLine(s) > w.endLine(d.Specs[i-1].(*ast.ImportSpec))+1 {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], s)
	}
	return out
}

// insertInto adds paths to a parenthesized import decl.
func (w *editor) insertInto(d *ast.GenDecl, adds []string) {
	// Groups are computed on the original layout, then the specs being
	// removed are dropped so that nothing is inserted next to (or into the
	// blank line widened around) a deletion.
	var groups [][]*ast.ImportSpec
	for _, g := range w.groups(d) {
		var keep []*ast.ImportSpec
		for _, sp := range g {
			if !w.removed[sp] {
				keep = append(keep, sp)
			}
		}
		if len(keep) > 0 {
			groups = append(groups, keep)
		}
	}
	pending := map[token.Pos][]string{} // insertion point -> spec lines
	var newStd, newOther []string
	for _, p := range adds {
		std := isStd(p)
		g := slices.IndexFunc(groups, func(g []*ast.ImportSpec) bool {
			return slices.ContainsFunc(g, func(s *ast.ImportSpec) bool { return isStd(specPath(s)) == std }) &&
				!slices.ContainsFunc(g, func(s *ast.ImportSpec) bool { return isStd(specPath(s)) != std })
		})
		if g < 0 {
			if std {
				newStd = append(newStd, p)
			} else {
				newOther = append(newOther, p)
			}
			continue
		}
		at := w.lineStart(w.endLine(groups[g][len(groups[g])-1]) + 1)
		for _, s := range groups[g] {
			if specPath(s) > p {
				at = w.lineStart(w.startLine(s))
				break
			}
		}
		pending[at] = append(pending[at], "\t"+strconv.Quote(p)+"\n")
	}
	for at, lines := range pending {
		w.addEdit(Edit{Pos: at, End: at, NewText: []byte(strings.Join(lines, ""))})
	}
	if len(newStd) > 0 {
		// Right after "(": the existing newline then separates the new group.
		at := d.Lparen + 1
		w.addEdit(Edit{Pos: at, End: at, NewText: []byte("\n\t" + strings.Join(quoteAll(newStd), "\n\t") + "\n")})
	}
	if len(newOther) > 0 {
		at := w.lineStart(w.line(d.Rparen))
		w.addEdit(Edit{Pos: at, End: at, NewText: []byte("\n\t" + strings.Join(quoteAll(newOther), "\n\t") + "\n")})
	}
}

func quoteAll(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = strconv.Quote(p)
	}
	return out
}

// remove deletes spec (and a now-redundant blank line) from a parenthesized
// decl that keeps at least one other spec.
func (w *editor) remove(d *ast.GenDecl, spec *ast.ImportSpec) {
	i := slices.IndexFunc(d.Specs, func(s ast.Spec) bool { return s == spec })
	if i < 0 {
		return
	}
	first, last := w.startLine(spec), w.endLine(spec)
	startAlone := i == 0 || first > w.endLine(d.Specs[i-1].(*ast.ImportSpec))+1
	endAlone := i == len(d.Specs)-1 || w.startLine(d.Specs[i+1].(*ast.ImportSpec)) > last+1
	from, to := w.lineStart(first), w.lineStart(last+1)
	if startAlone && endAlone {
		switch {
		case i < len(d.Specs)-1:
			to = w.lineStart(last + 2)
		case i > 0:
			from = w.lineStart(first - 1)
		}
	}
	w.addEdit(Edit{Pos: from, End: to})
}

// result sorts the edits and makes them non-overlapping: an insertion that
// lands on or inside a deletion is merged into it (a replacement), and
// overlapping deletions are united. Drivers reject overlapping edits.
func (w *editor) result() []Edit {
	sort.SliceStable(w.edits, func(i, j int) bool {
		a, b := w.edits[i], w.edits[j]
		if a.Pos != b.Pos {
			return a.Pos < b.Pos
		}
		return a.End < b.End // insertions first
	})
	var out []Edit
	for _, e := range w.edits {
		n := len(out)
		if n == 0 {
			out = append(out, e)
			continue
		}
		prev := out[n-1]
		switch {
		case prev.Pos == e.Pos && (prev.End == prev.Pos || e.End == e.Pos):
			out[n-1] = Edit{Pos: prev.Pos, End: max(prev.End, e.End), NewText: slices.Concat(prev.NewText, e.NewText)}
		case e.Pos < prev.End && e.End == e.Pos && len(prev.NewText) == 0:
			// Insertion inside a pure deletion: keep the text at the
			// insertion point and delete the rest around it.
			out[n-1] = Edit{Pos: prev.Pos, End: e.Pos}
			out = append(out, Edit{Pos: e.Pos, End: prev.End, NewText: e.NewText})
		case e.Pos < prev.End:
			out[n-1] = Edit{Pos: prev.Pos, End: max(prev.End, e.End), NewText: slices.Concat(prev.NewText, e.NewText)}
		default:
			out = append(out, e)
		}
	}
	return slices.DeleteFunc(out, func(e Edit) bool { return e.Pos == e.End && len(e.NewText) == 0 })
}
