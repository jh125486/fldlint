package detect

import (
	"bytes"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/jh125486/fldlint/go/internal/fix"
	"github.com/jh125486/fldlint/go/internal/spec"
)

// Comments inside a call (issue #31). Every comment in the call's range is
// either copied with the argument text that contains it, attached to the
// argument it touches, hoisted before the new call (between the callee and
// the "("), or put before the closing ")" (own line after the last argument).
// A comment with no home marks the notes Unhomed and the call gets no fix.
// The JS engine places comments the same way (js/packages/core/src/detect.ts).

// srcComment converts an ast comment to a fix.Comment using the source bytes,
// because go/scanner strips carriage returns from comment text.
func srcComment(src []byte, start int, isLine bool) (c fix.Comment, end int) {
	if isLine {
		end = start + bytes.IndexByte(src[start:], '\n')
		if end < start {
			end = len(src)
		}
		eol := "\n"
		if end > start && src[end-1] == '\r' {
			end--
			eol = "\r\n"
		}
		return fix.Comment{Text: string(src[start:end]), Line: true, EOL: eol}, end
	}
	end = start + bytes.Index(src[start+2:], []byte("*/")) + 4
	return fix.Comment{Text: string(src[start:end])}, end
}

type placedComment struct {
	fix.Comment
	start, end int
}

// notes places the comments inside call. It returns nil when the call has
// none that need placing.
func (e *Env) notes(f File, call *ast.CallExpr, method string, rule spec.Rule) *fix.Notes {
	tf := e.Fset.File(call.Pos())
	off := func(p token.Pos) int { return tf.Offset(p) }
	start, end := off(call.Pos()), off(call.End())

	// File comment groups are sorted: binary-search the first group that can
	// reach the call, then stop at the first group past it.
	var inCall []placedComment
	groups := f.AST.Comments
	for gi := sort.Search(len(groups), func(i int) bool { return off(groups[i].End()) > start }); gi < len(groups); gi++ {
		if off(groups[gi].Pos()) >= end {
			break
		}
		for _, c := range groups[gi].List {
			if cs := off(c.Slash); cs >= start && cs < end {
				fc, ce := srcComment(f.Src, cs, strings.HasPrefix(c.Text, "//"))
				inCall = append(inCall, placedComment{fc, cs, ce})
			}
		}
	}
	if len(inCall) == 0 {
		return nil
	}

	at, hasCtx := rule.Go.CtxArg[method]
	roleOf := func(j int) string {
		if hasCtx && j == at {
			return "goctx"
		}
		i := j
		if hasCtx && j > at {
			i = j - 1
		}
		switch i {
		case rule.KeyArg:
			return "key"
		case rule.DefaultArg:
			return "default"
		case rule.ContextArg:
			return "evalctx"
		}
		return ""
	}
	// The kept part of the context argument: the expression ldcontext.New wraps.
	var ctxKept ast.Expr
	if j := argIndex(rule, method, rule.ContextArg); j >= 0 && j < len(call.Args) {
		if inner, ok := ast.Unparen(call.Args[j]).(*ast.CallExpr); ok && len(inner.Args) == 1 {
			ctxKept = inner.Args[0]
		}
	}

	n := &fix.Notes{Args: map[string]*fix.ArgNotes{}}
	slot := func(role string) *fix.ArgNotes {
		if n.Args[role] == nil {
			n.Args[role] = &fix.ArgNotes{}
		}
		return n.Args[role]
	}
	attach := func(j int, c fix.Comment, trail bool) {
		role := roleOf(j)
		switch {
		case role == "":
			n.Unhomed = true
		case trail:
			slot(role).Trail = append(slot(role).Trail, c)
		default:
			slot(role).Lead = append(slot(role).Lead, c)
		}
	}
	argStart := func(j int) int { return off(call.Args[j].Pos()) }
	argEnd := func(j int) int { return off(call.Args[j].End()) }
	lparen := off(call.Lparen)
	hasBreak := func(a, b int) bool { return bytes.ContainsAny(f.Src[a:b], "\n\r") }
	commaAfter := func(from int) int {
		pos := from
		for pos < len(f.Src) {
			if i := indexComment(inCall, pos); i >= 0 {
				pos = inCall[i].end
			} else if strings.IndexByte(" \t\r\n)", f.Src[pos]) >= 0 {
				pos++
			} else {
				break
			}
		}
		return pos
	}

	for _, c := range inCall {
		inside := -1
		for j := range call.Args {
			if argStart(j) <= c.start && c.end <= argEnd(j) {
				inside = j
			}
		}
		if inside >= 0 {
			// Copied verbatim with the argument, except inside the converted context.
			role := roleOf(inside)
			kept := role == "key" || role == "default" || role == "goctx" ||
				(role == "evalctx" && ctxKept != nil && off(ctxKept.Pos()) <= c.start && c.end <= off(ctxKept.End()))
			if !kept {
				n.Unhomed = true
			}
			continue
		}
		// Moving a directive changes what it governs.
		if fix.IsDirective(c.Text) {
			n.Unhomed = true
			continue
		}
		if c.start < lparen {
			if c.Breaks() {
				n.Unhomed = true
			} else {
				n.Head = append(n.Head, c.Comment)
			}
			continue
		}
		next := -1
		for j := range call.Args {
			if argStart(j) >= c.end {
				next = j
				break
			}
		}
		prev := len(call.Args) - 1
		if next >= 0 {
			prev = next - 1
		}
		switch {
		case prev < 0:
			attach(next, c.Comment, false)
		case next < 0:
			if hasBreak(argEnd(prev), c.start) {
				n.Tail = append(n.Tail, c.Comment)
			} else {
				attach(prev, c.Comment, true)
			}
		case c.start < commaAfter(argEnd(prev)):
			attach(prev, c.Comment, true)
		case !hasBreak(argEnd(prev), c.start) && (c.Line || hasBreak(c.end, argStart(next))):
			attach(prev, c.Comment, true)
		default:
			attach(next, c.Comment, false)
		}
	}
	return n
}

func indexComment(cs []placedComment, pos int) int {
	for i, c := range cs {
		if c.start == pos {
			return i
		}
	}
	return -1
}
