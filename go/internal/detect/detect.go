// Package detect identifies LaunchDarkly Go SDK calls by type (go/types, never
// by method name alone) and collects the call-site facts the planner needs:
// static key, context proof, OpenFeature client binding, default-value type and
// the `{{goctx}}` source.
package detect

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/types/typeutil"

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
// calls: the concrete client and the two client interfaces.
var sdkReceivers = map[[2]string]bool{
	{PkgSDK, "LDClient"}:                     true,
	{PkgSDKInterfaces, "LDClientEvaluation"}: true,
	{PkgSDKInterfaces, "LDClientInterface"}:  true,
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
	if _, known := spec.LookupGoMethod(fn.Name()); !known {
		return "", false
	}
	return fn.Name(), true
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
// rules without a Go template carry no facts: the planner skips them first.
func (e *Env) Site(f File, call *ast.CallExpr, method string) planner.CallSite {
	site := planner.CallSite{Call: call, Method: method}
	rule, ok := spec.LookupGoMethod(method)
	if !ok || rule.Skip != "" || rule.Go == nil || rule.Go.Template == "" || call.Ellipsis.IsValid() {
		return site
	}
	arg := func(i int) ast.Expr {
		if j := argIndex(rule, method, i); j >= 0 && j < len(call.Args) {
			return call.Args[j]
		}
		return nil
	}
	fa := &site.Facts

	if lit, ok := arg(rule.KeyArg).(*ast.BasicLit); ok && lit.Kind == token.STRING {
		fa.KeyStatic, fa.Key = true, e.text(f, lit)
	}
	if d := arg(rule.DefaultArg); d != nil {
		fa.Default = e.text(f, d)
		fa.DefaultProven = defaultTypeOK(e.Info.TypeOf(d), rule.ValueType)
	}
	fa.Client, fa.ClientBound = e.client(call)

	ofName, addOF, ofOK := e.openFeatureName(f, call.Pos())
	if x := arg(rule.ContextArg); x != nil && ofOK {
		e.context(f, x, ofName, fa)
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
	for _, spc := range f.AST.Imports {
		if p, _ := strconv.Unquote(spc.Path.Value); p != PkgOpenFeature {
			continue
		}
		pn := e.Info.PkgNameOf(spc)
		if pn == nil {
			continue // blank or dot import: not usable by name
		}
		_, obj := scope.LookupParent(pn.Name(), pos)
		return pn.Name(), false, obj == types.Object(pn)
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
		if at >= len(call.Args) {
			return "", false, false
		}
		return e.text(f, call.Args[at]), false, true
	}
	if name := e.ctxParam(f, call); name != "" {
		return name, false, true
	}
	scope := e.Pkg.Scope().Innermost(call.Pos())
	_, obj := scope.LookupParent("context", call.Pos())
	switch o := obj.(type) {
	case nil:
		return "context.TODO()", true, true
	case *types.PkgName:
		return "context.TODO()", false, o.Imported().Path() == "context"
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
		if p, n, ok := namedOf(e.Info.TypeOf(fld.Type)); !ok || p != "context" || n != "Context" {
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
		return cands[0].Name(), true
	}
	return "", false
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

// Removable returns, for the file, each ldcontext import spec with every
// identifier that references it. The caller passes it to planner.PlanFile.
func (e *Env) Removable(f File) map[*ast.ImportSpec][]*ast.Ident {
	out := map[*ast.ImportSpec][]*ast.Ident{}
	byPkg := map[*types.PkgName]*ast.ImportSpec{}
	for _, spc := range f.AST.Imports {
		if p, _ := strconv.Unquote(spc.Path.Value); p != PkgLDContext {
			continue
		}
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
