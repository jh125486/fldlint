// Package detect identifies LaunchDarkly Go SDK calls by type (go/types, never
// by method name alone) and collects the call-site facts the planner needs:
// static key, context proof, OpenFeature client binding, default-value type and
// the `{{goctx}}` source.
package detect

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/jh125486/fldlint/go/internal/fingerprint"
	"github.com/jh125486/fldlint/go/internal/planner"
	"github.com/jh125486/fldlint/go/internal/spec"
)

// Import paths of the types this package recognises. Only go-server-sdk v7 is
// handled; v6 (a different module path) is not.
const (
	PkgSDK           = "github.com/launchdarkly/go-server-sdk/v7"
	PkgSDKInterfaces = PkgSDK + "/interfaces"
	PkgLDContext     = "github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	PkgOpenFeature   = "github.com/open-feature/go-sdk/openfeature"
)

// sdkReceivers are the named types whose methods are LaunchDarkly evaluation
// calls: the concrete client and the client interfaces. A method promoted
// through LDClientInterface is declared on LDClientEvaluations (which it
// embeds), so that is the receiver type go/types reports for such calls.
var sdkReceivers = map[[2]string]bool{
	{PkgSDK, "LDClient"}:                      true,
	{PkgSDKInterfaces, "LDClientEvaluations"}: true,
	{PkgSDKInterfaces, "LDClientInterface"}:   true,
}

// namedOf returns the package path and name of t after removing one pointer
// level and aliases.
func namedOf(t types.Type) (path, name string, ok bool) {
	t = types.Unalias(t)
	if p, isPtr := t.(*types.Pointer); isPtr {
		t = types.Unalias(p.Elem())
	}
	n, isNamed := t.(*types.Named)
	if !isNamed || n.Obj().Pkg() == nil {
		return "", "", false
	}
	return n.Obj().Pkg().Path(), n.Obj().Name(), true
}

// Method returns the SDK method name when call invokes a method of a
// LaunchDarkly client type that the rules table knows.
func Method(info *types.Info, call *ast.CallExpr) (string, bool) {
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	if fn == nil {
		return "", false
	}
	sig, _ := fn.Type().(*types.Signature)
	if sig == nil || sig.Recv() == nil {
		return "", false
	}
	path, name, ok := namedOf(sig.Recv().Type())
	if !ok || !sdkReceivers[[2]string{path, name}] {
		return "", false
	}
	rule, known := spec.LookupGoMethod(fn.Name())
	if !known {
		return "", false
	}
	// Argument positions and the template are fixed per rule, so a method
	// whose signature differs (an SDK revision, a fork) is not a finding.
	if rule.Skip == "" && rule.Go != nil && rule.Go.Template != "" && !signatureOK(sig, rule, fn.Name()) {
		return "", false
	}
	// Report-only rules have no template to pin the signature; their
	// argument count (rules.yaml argCount) does the same job.
	if rule.Skip != "" && !argCountOK(info, call, rule, fn.Name()) {
		return "", false
	}
	return fn.Name(), true
}

// argCountOK reports whether the call passes a number of arguments the rule's
// argCount accepts; *Ctx methods take one more (the leading context.Context).
// A `xs...` spread is accepted only for a variadic method (no max): the
// spread may be empty, so the arguments before it must reach the minimum.
// Otherwise the count is unknowable and the call is unrecognized, as in JS.
// A single call that supplies several values (f(g())) counts as that many
// arguments.
func argCountOK(info *types.Info, call *ast.CallExpr, rule spec.Rule, method string) bool {
	ac := rule.Go.ArgCount
	if ac == nil {
		return true
	}
	shift := 0
	if _, ok := rule.Go.CtxArg[method]; ok {
		shift = 1
	}
	// A method expression, (*Client).M(c, args...), passes the receiver as
	// the first argument; it is not one of the method's own arguments.
	recv := 0
	if isMethodExpr(info, call) {
		recv = 1
	}
	if call.Ellipsis.IsValid() {
		return ac.Max == 0 && len(call.Args)-1-recv >= ac.Min+shift
	}
	n := len(call.Args)
	if n == 1 {
		if tup, ok := info.TypeOf(call.Args[0]).(*types.Tuple); ok {
			n = tup.Len()
		}
	}
	return ac.Accepts(n-recv, shift)
}

// isMethodExpr reports whether call invokes a method expression,
// (*T).M(recv, args...), whose first argument is the receiver.
func isMethodExpr(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	s := info.Selections[sel]
	return s != nil && s.Kind() == types.MethodExpr
}

// callArgs returns the method's own arguments: for a method expression the
// leading receiver argument is not one of them, so rule argument positions
// index the result, not call.Args.
func callArgs(info *types.Info, call *ast.CallExpr) []ast.Expr {
	if isMethodExpr(info, call) && len(call.Args) > 0 {
		return call.Args[1:]
	}
	return call.Args
}

// valueKind maps a rule value type to the Go basic kind of its default and
// result.
var valueKind = map[string]types.BasicKind{
	"boolean": types.Bool,
	"string":  types.String,
	"number":  types.Float64,
}

// signatureOK reports whether sig is the signature the rule's argument
// positions and template assume: ([ctx context.Context,] key string, context
// ldcontext.Context, default T) (T, error), not variadic.
func signatureOK(sig *types.Signature, rule spec.Rule, method string) bool {
	kind, ok := valueKind[rule.ValueType]
	if !ok || sig.Variadic() {
		return false
	}
	_, hasCtx := rule.Go.CtxArg[method]
	want := 3
	if hasCtx {
		want++
	}
	if sig.Params().Len() != want || sig.Results().Len() != 2 {
		return false
	}
	basic := func(t types.Type, k types.BasicKind) bool {
		b, ok := types.Unalias(t).(*types.Basic)
		return ok && b.Kind() == k
	}
	param := func(i int) types.Type { return sig.Params().At(argIndex(rule, method, i)).Type() }
	if hasCtx && !isGoContext(sig.Params().At(rule.Go.CtxArg[method]).Type()) {
		return false
	}
	ldctx, ok := types.Unalias(param(rule.ContextArg)).(*types.Named)
	if !ok || ldctx.Obj().Pkg() == nil || ldctx.Obj().Pkg().Path() != PkgLDContext || ldctx.Obj().Name() != "Context" {
		return false
	}
	errType := types.Universe.Lookup("error").Type()
	return basic(param(rule.KeyArg), types.String) &&
		basic(param(rule.DefaultArg), kind) &&
		basic(sig.Results().At(0).Type(), kind) &&
		types.Identical(sig.Results().At(1).Type(), errType)
}

// File bundles a type-checked file with its bytes.
type File struct {
	AST *ast.File
	Src []byte
}

// Env is the type-checked package being analysed.
type Env struct {
	Fset *token.FileSet
	Pkg  *types.Package
	Info *types.Info
	// Files are the package's syntax trees. They enable the package-level
	// client checks (build membership of the declaring file, initialization
	// cycles); without them a package-level client binds as before.
	Files []*ast.File

	files map[*ast.File]*fileFacts
	graph map[types.Object][]types.Object
}

func (e *Env) text(f File, n ast.Node) string {
	tf := e.Fset.File(n.Pos())
	return string(f.Src[tf.Offset(n.Pos()):tf.Offset(n.End())])
}

// argIndex maps a rule-level argument index to the call's argument position:
// *Ctx variants carry the context.Context first, which shifts later arguments.
func argIndex(rule spec.Rule, method string, i int) int {
	if i < 0 {
		return -1
	}
	if at, ok := rule.Go.CtxArg[method]; ok && i >= at {
		return i + 1
	}
	return i
}

// Site collects the facts for one recognised call. Rule-level skips and
// rules without a Go template carry only the fingerprint key: the planner
// skips them first.
func (e *Env) Site(f File, call *ast.CallExpr, method string) planner.CallSite {
	site := planner.CallSite{Call: call, Method: method}
	rule, ok := spec.LookupGoMethod(method)
	if !ok || call.Ellipsis.IsValid() {
		return site
	}
	args := callArgs(e.Info, call)
	arg := func(i int) ast.Expr {
		if j := argIndex(rule, method, i); j >= 0 && j < len(args) {
			return args[j]
		}
		return nil
	}
	fa := &site.Facts

	// A parenthesized string literal is as static as the bare one: ESLint's
	// parser drops the parentheses, so every engine reads the same key.
	lit, isLit := ast.Unparen(arg(rule.KeyArg)).(*ast.BasicLit)
	isLit = isLit && lit.Kind == token.STRING
	if isLit {
		// A key that is not valid UTF-8 is not a fingerprint key: JS cannot
		// represent it, so the engines would disagree (fingerprint.md).
		if v, err := strconv.Unquote(lit.Value); err == nil {
			fa.KeyValue = fingerprint.StaticKey(&v)
		}
	}
	if rule.Skip != "" || rule.Go == nil || rule.Go.Template == "" {
		return site
	}
	if isLit {
		fa.KeyStatic, fa.Key = true, e.text(f, lit)
	}
	if d := arg(rule.DefaultArg); d != nil {
		fa.Default = e.text(f, d)
		fa.DefaultProven = defaultTypeOK(e.Info.TypeOf(d), rule.ValueType)
	}
	fa.Client, fa.ClientBound = e.client(call)
	e.receiver(f, call, fa)

	ofName, addOF, ofOK := e.openFeatureName(f, call.Pos())
	if x := arg(rule.ContextArg); x != nil {
		e.context(f, x, ofName, fa)
	}
	if !ofOK {
		// The openfeature name is shadowed or taken: the client binding is
		// not usable, whatever the context proof says.
		fa.ClientBound = false
	}
	if addOF {
		fa.AddImports = append(fa.AddImports, PkgOpenFeature)
	}
	goctx, addCtx, ok := e.goctx(f, call, method, rule)
	if !ok {
		fa.ContextProven = false
	}
	fa.GoCtx = goctx
	if addCtx {
		fa.AddImports = append(fa.AddImports, "context")
	}
	fa.Notes = e.notes(f, call, method, rule)
	return site
}

// defaultTypeOK reports whether t is provably the Go type the rule's value type
// maps to (bool, string, float64), or an untyped constant of that family.
func defaultTypeOK(t types.Type, valueType string) bool {
	b, ok := t.(*types.Basic)
	if !ok {
		return false
	}
	switch valueType {
	case "boolean":
		return b.Kind() == types.Bool || b.Kind() == types.UntypedBool
	case "string":
		return b.Kind() == types.String || b.Kind() == types.UntypedString
	case "number":
		switch b.Kind() {
		case types.Float64, types.UntypedFloat, types.UntypedInt, types.UntypedRune:
			return true
		}
	}
	return false
}

// context proves the LD context argument is a direct ldcontext.New(x) call and
// fills fa.EvalCtx and the consumed identifier.
func (e *Env) context(f File, x ast.Expr, ofName string, fa *planner.Facts) {
	inner, ok := ast.Unparen(x).(*ast.CallExpr)
	if !ok || len(inner.Args) != 1 || inner.Ellipsis.IsValid() {
		return
	}
	fn, _ := typeutil.Callee(e.Info, inner).(*types.Func)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != PkgLDContext || fn.Name() != "New" {
		return
	}
	sel, ok := ast.Unparen(inner.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	if _, isPkg := e.Info.Uses[id].(*types.PkgName); !isPkg {
		return
	}
	fa.ContextProven = true
	fa.EvalCtx = ofName + ".NewEvaluationContext(" + e.text(f, inner.Args[0]) + ", nil)"
	fa.Consumed = append(fa.Consumed, id)
}

// openFeatureName returns the identifier to use for the openfeature package at
// pos, and whether its import must be added. ok is false when no usable name
// can be proven (shadowed or taken).
func (e *Env) openFeatureName(f File, pos token.Pos) (name string, add, ok bool) {
	scope := e.Pkg.Scope().Innermost(pos)
	unusable := false
	for _, spc := range f.AST.Imports {
		if p, _ := strconv.Unquote(spc.Path.Value); p != PkgOpenFeature {
			continue
		}
		pn := e.Info.PkgNameOf(spc)
		if pn == nil {
			unusable = true // blank or dot import: not usable by name
			continue
		}
		if _, obj := scope.LookupParent(pn.Name(), pos); obj == types.Object(pn) {
			return pn.Name(), false, true
		}
		unusable = true // shadowed
	}
	if unusable {
		// The path is already imported but cannot be named here; adding a
		// second import of it is not an option, so the client is unbound.
		return "openfeature", false, false
	}
	name = "openfeature"
	_, obj := scope.LookupParent(name, pos)
	return name, true, obj == nil
}

// goctx resolves {{goctx}}. For *Ctx variants it is the source of argument 0.
// For plain variants it is the enclosing function's single context.Context
// parameter, else context.TODO(), which needs the `context` identifier to mean
// the standard library package (ok is false when it is shadowed).
func (e *Env) goctx(f File, call *ast.CallExpr, method string, rule spec.Rule) (src string, addImport, ok bool) {
	if at, isCtx := rule.Go.CtxArg[method]; isCtx {
		args := callArgs(e.Info, call)
		if at >= len(args) {
			return "", false, false
		}
		return e.text(f, args[at]), false, true
	}
	if name := e.ctxParam(f, call); name != "" {
		return name, false, true
	}
	scope := e.Pkg.Scope().Innermost(call.Pos())
	// Reuse any usable import of the standard library package, including an
	// aliased one (`stdctx "context"`), rather than adding a second import.
	imported := false
	for _, spc := range f.AST.Imports {
		if p, _ := strconv.Unquote(spc.Path.Value); p != "context" {
			continue
		}
		imported = true
		pn := e.Info.PkgNameOf(spc)
		if pn == nil {
			continue
		}
		if _, obj := scope.LookupParent(pn.Name(), call.Pos()); obj == types.Object(pn) {
			return pn.Name() + ".TODO()", false, true
		}
	}
	if imported {
		// "context" is imported but no import of it is nameable here (all
		// shadowed, blank or dot): adding another import would duplicate it,
		// so the {{goctx}} source is unproven (context-unproven).
		return "", false, false
	}
	_, obj := scope.LookupParent("context", call.Pos())
	if obj == nil {
		return "context.TODO()", true, true
	}
	return "", false, false
}

// ctxParam returns the name of the single context.Context parameter of the
// innermost function enclosing call, when it is exactly one, named, and still
// the object the name resolves to at the call.
func (e *Env) ctxParam(f File, call *ast.CallExpr) string {
	path, _ := astutil.PathEnclosingInterval(f.AST, call.Pos(), call.End())
	var ftype *ast.FuncType
	for _, n := range path {
		if fd, ok := n.(*ast.FuncDecl); ok {
			ftype = fd.Type
			break
		}
		if fl, ok := n.(*ast.FuncLit); ok {
			ftype = fl.Type
			break
		}
	}
	if ftype == nil || ftype.Params == nil {
		return ""
	}
	var found []*ast.Ident
	total := 0
	for _, fld := range ftype.Params.List {
		if !isGoContext(e.Info.TypeOf(fld.Type)) {
			continue
		}
		if len(fld.Names) == 0 {
			total++
			continue
		}
		total += len(fld.Names)
		found = append(found, fld.Names...)
	}
	if total != 1 || len(found) != 1 || found[0].Name == "_" {
		return ""
	}
	scope := e.Pkg.Scope().Innermost(call.Pos())
	_, obj := scope.LookupParent(found[0].Name, call.Pos())
	if obj == nil || obj != e.Info.Defs[found[0]] {
		return ""
	}
	return found[0].Name
}

// isGoContext reports whether t is exactly context.Context (aliases resolved,
// no pointer level removed: *context.Context is not a context.Context).
func isGoContext(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "context" && n.Obj().Name() == "Context"
}

// isOpenFeatureClient reports whether t is *openfeature.Client or an interface
// type that satisfies openfeature.IClient.
func isOpenFeatureClient(t types.Type, iclient *types.Interface) bool {
	if p, n, ok := namedOf(t); ok && p == PkgOpenFeature {
		if _, isPtr := types.Unalias(t).(*types.Pointer); isPtr && n == "Client" {
			return true
		}
		if n == "IClient" {
			return true
		}
	}
	if iclient != nil && types.IsInterface(t) {
		return types.Implements(t, iclient)
	}
	return false
}

// client proves the OpenFeature client binding at call: the innermost scope
// that declares an OpenFeature client variable must declare exactly one, and
// that name must resolve to it at the call (not be shadowed).
func (e *Env) client(call *ast.CallExpr) (string, bool) {
	iclient := e.iclient()
	pos := call.Pos()
	inner := e.Pkg.Scope().Innermost(pos)
	for s := inner; s != nil && s != types.Universe; s = s.Parent() {
		var cands []*types.Var
		for _, name := range s.Names() {
			v, ok := s.Lookup(name).(*types.Var)
			if !ok || v.IsField() || name == "_" || !isOpenFeatureClient(v.Type(), iclient) {
				continue
			}
			if s != e.Pkg.Scope() && v.Pos() >= pos {
				continue // declared after the call
			}
			if s == e.Pkg.Scope() && !e.sameVariant(v.Pos(), pos) {
				continue // exists only in another build of the package
			}
			cands = append(cands, v)
		}
		if len(cands) == 0 {
			continue
		}
		if len(cands) != 1 {
			return "", false
		}
		_, obj := inner.LookupParent(cands[0].Name(), pos)
		if obj != types.Object(cands[0]) {
			return "", false
		}
		if s == e.Pkg.Scope() && e.initCycle(cands[0], pos) {
			return "", false
		}
		return cands[0].Name(), true
	}
	return "", false
}

// fileAt returns the syntax tree containing pos, or nil.
func (e *Env) fileAt(pos token.Pos) *ast.File {
	for _, f := range e.Files {
		if f.FileStart <= pos && pos < f.FileEnd {
			return f
		}
	}
	return nil
}

// buildConstraint returns the //go:build expression of f ("" when none).
func buildConstraint(f *ast.File) string {
	if f == nil {
		return ""
	}
	for _, cg := range f.Comments {
		if cg.End() >= f.Package {
			break
		}
		for _, c := range cg.List {
			if constraint.IsGoBuild(c.Text) {
				return strings.TrimSpace(strings.TrimPrefix(c.Text, "//go:build"))
			}
		}
	}
	return ""
}

// sameVariant reports whether a declaration at decl is present in every
// build of the package in which the file at use is: a _test.go file exists
// only in the test variant (so it cannot serve a non-test file), and a file
// with a build constraint only where that constraint holds (so it must
// carry the same constraint as the file that uses it).
func (e *Env) sameVariant(decl, use token.Pos) bool {
	dn, un := e.Fset.File(decl).Name(), e.Fset.File(use).Name()
	if strings.HasSuffix(dn, "_test.go") && !strings.HasSuffix(un, "_test.go") {
		return false
	}
	dc := buildConstraint(e.fileAt(decl))
	return dc == "" || dc == buildConstraint(e.fileAt(use))
}

// initCycle reports whether rewriting the call at pos into a use of the
// package-level variable v would create an initialization cycle: the call
// sits in v's own initializer, or in a function or variable that v's
// initializer transitively refers to.
func (e *Env) initCycle(v *types.Var, pos token.Pos) bool {
	f := e.fileAt(pos)
	if f == nil {
		return false
	}
	var from []types.Object // objects of the top-level declaration holding the call
	for _, d := range f.Decls {
		if d.Pos() > pos || pos >= d.End() {
			continue
		}
		switch d := d.(type) {
		case *ast.FuncDecl:
			from = append(from, e.Info.Defs[d.Name])
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				if vs, ok := sp.(*ast.ValueSpec); ok && vs.Pos() <= pos && pos < vs.End() {
					for _, n := range vs.Names {
						from = append(from, e.Info.Defs[n])
					}
				}
			}
		}
	}
	if len(from) == 0 {
		return false
	}
	g := e.depGraph()
	seen := map[types.Object]bool{}
	var visit func(o types.Object) bool
	visit = func(o types.Object) bool {
		if seen[o] {
			return false
		}
		seen[o] = true
		if slices.Contains(from, o) {
			return true
		}
		return slices.ContainsFunc(g[o], visit)
	}
	return visit(v)
}

// depGraph maps each package-level function, method and variable to the
// package-level functions, methods and variables its body or initializer
// refers to.
func (e *Env) depGraph() map[types.Object][]types.Object {
	if e.graph != nil {
		return e.graph
	}
	g := map[types.Object][]types.Object{}
	refs := func(n ast.Node) []types.Object {
		var out []types.Object
		ast.Inspect(n, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			switch o := e.Info.Uses[id].(type) {
			case *types.Func:
				if o.Pkg() == e.Pkg {
					out = append(out, o)
				}
			case *types.Var:
				if o.Parent() == e.Pkg.Scope() {
					out = append(out, o)
				}
			}
			return true
		})
		return out
	}
	for _, f := range e.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if o := e.Info.Defs[d.Name]; o != nil {
					g[o] = append(g[o], refs(d)...)
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					var r []types.Object
					for _, val := range vs.Values {
						r = append(r, refs(val)...)
					}
					for _, n := range vs.Names {
						if o := e.Info.Defs[n]; o != nil {
							g[o] = append(g[o], r...)
						}
					}
				}
			}
		}
	}
	e.graph = g
	return g
}

// iclient finds openfeature.IClient among the package's imports, if loaded.
func (e *Env) iclient() *types.Interface {
	var found *types.Interface
	seen := map[*types.Package]bool{}
	var visit func(p *types.Package)
	visit = func(p *types.Package) {
		if found != nil || seen[p] {
			return
		}
		seen[p] = true
		if p.Path() == PkgOpenFeature {
			if o := p.Scope().Lookup("IClient"); o != nil {
				found, _ = o.Type().Underlying().(*types.Interface)
			}
			return
		}
		for _, imp := range p.Imports() {
			visit(imp)
		}
	}
	visit(e.Pkg)
	return found
}

// Removable returns, for the file, each named import spec with every
// identifier that references it. The planner removes an import only when one
// fix consumes all of its remaining uses (ldcontext, a package-qualified
// client receiver). The caller passes it to planner.PlanFile.
func (e *Env) Removable(f File) map[*ast.ImportSpec][]*ast.Ident {
	out := map[*ast.ImportSpec][]*ast.Ident{}
	byPkg := map[*types.PkgName]*ast.ImportSpec{}
	for _, spc := range f.AST.Imports {
		if pn := e.Info.PkgNameOf(spc); pn != nil {
			byPkg[pn] = spc
			out[spc] = nil
		}
	}
	if len(byPkg) == 0 {
		return out
	}
	ast.Inspect(f.AST, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if pn, ok := e.Info.Uses[id].(*types.PkgName); ok {
			if spc, ok := byPkg[pn]; ok {
				out[spc] = append(out[spc], id)
			}
		}
		return true
	})
	for _, ids := range out {
		slices.SortFunc(ids, func(a, b *ast.Ident) int { return int(a.Pos()) - int(b.Pos()) })
	}
	return out
}

// fileFacts holds the per-file data the receiver analysis needs.
type fileFacts struct {
	// locals maps each function-local variable (not a parameter, result,
	// field or package-level variable) to its non-write uses.
	locals map[*types.Var]*planner.Local
	// implicit holds the per-clause variables of type switches, which the
	// compiler counts as used together; they are never treated as removable.
	implicit map[*types.Var]bool
}

// facts computes (once per file) the local-variable use sets.
func (e *Env) facts(f File) *fileFacts {
	if ff, ok := e.files[f.AST]; ok {
		return ff
	}
	ff := &fileFacts{locals: map[*types.Var]*planner.Local{}, implicit: map[*types.Var]bool{}}
	if e.files == nil {
		e.files = map[*ast.File]*fileFacts{}
	}
	e.files[f.AST] = ff

	for _, obj := range e.Info.Implicits {
		if v, ok := obj.(*types.Var); ok {
			ff.implicit[v] = true
		}
	}
	// Idents that are only written: a bare identifier on the left of =, :=,
	// op=, ++ or --, or a range key/value assigned with =.
	writes := map[*ast.Ident]bool{}
	mark := func(x ast.Expr) {
		if id, ok := ast.Unparen(x).(*ast.Ident); ok {
			writes[id] = true
		}
	}
	params := map[*types.Var]bool{}
	addFields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, fld := range fl.List {
			for _, n := range fld.Names {
				if v, ok := e.Info.Defs[n].(*types.Var); ok {
					params[v] = true
				}
			}
		}
	}
	ast.Inspect(f.AST, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(n.X)
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				if n.Key != nil {
					mark(n.Key)
				}
				if n.Value != nil {
					mark(n.Value)
				}
			}
		case *ast.FuncDecl:
			addFields(n.Recv)
			addFields(n.Type.Params)
			addFields(n.Type.Results)
		case *ast.FuncLit:
			addFields(n.Type.Params)
			addFields(n.Type.Results)
		}
		return true
	})
	ast.Inspect(f.AST, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		v, ok := e.Info.Uses[id].(*types.Var)
		if !ok || v.IsField() || params[v] || v.Parent() == nil || v.Parent() == e.Pkg.Scope() {
			return true
		}
		l := ff.locals[v]
		if l == nil {
			l = &planner.Local{}
			ff.locals[v] = l
		}
		if !writes[id] {
			l.Uses = append(l.Uses, id)
		}
		return true
	})
	return ff
}

// receiver records whether the call's receiver expression is pure (an
// identifier or a selector chain of identifiers and fields) and which
// function-local variable, if any, it is rooted in.
func (e *Env) receiver(f File, call *ast.CallExpr, fa *planner.Facts) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return
	}
	root := e.pureRoot(sel.X)
	if root == nil {
		return
	}
	ff := e.facts(f)
	if v, ok := e.Info.Uses[root].(*types.Var); ok && ff.implicit[v] {
		return
	}
	fa.ReceiverPure = true
	fa.ReceiverRoot = root
	switch o := e.Info.Uses[root].(type) {
	case *types.Var:
		fa.Receiver = ff.locals[o] // nil for package-level variables and parameters
	case *types.PkgName:
		// pkg.Client.Method(...): the rewrite removes this use of the import.
		fa.Consumed = append(fa.Consumed, root)
	}
}

// pureRoot returns the leftmost identifier of x when x is an identifier or a
// selector chain of identifiers and struct fields (or a package-qualified
// variable), and nil otherwise (call, index, type assertion, explicit
// dereference, method value, ...).
func (e *Env) pureRoot(x ast.Expr) *ast.Ident {
	for {
		switch n := ast.Unparen(x).(type) {
		case *ast.Ident:
			switch e.Info.Uses[n].(type) {
			case *types.Var, *types.PkgName:
				return n
			}
			return nil
		case *ast.SelectorExpr:
			if s, ok := e.Info.Selections[n]; ok {
				if s.Kind() != types.FieldVal {
					return nil
				}
				x = n.X
				continue
			}
			// No selection: a package-qualified identifier (pkg.Client).
			pkg, ok := ast.Unparen(n.X).(*ast.Ident)
			if !ok {
				return nil
			}
			if _, isPkg := e.Info.Uses[pkg].(*types.PkgName); !isPkg {
				return nil
			}
			if _, isVar := e.Info.Uses[n.Sel].(*types.Var); !isVar {
				return nil
			}
			return pkg
		default:
			return nil
		}
	}
}
