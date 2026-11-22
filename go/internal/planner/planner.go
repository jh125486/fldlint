// Package planner decides, for one detected call site, whether the call is
// rewritten or reported only. All front ends (analyzer, fixer) call Plan;
// none re-implement detection.
//
// Step order (identical across languages): identity (the caller), rule-level
// skip, dynamic-key, context-unproven, unbound-client, default-type-unproven.
package planner

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/jh125486/fldlint/go/internal/fix"
	"github.com/jh125486/fldlint/go/internal/spec"
)

// Kind is the outcome class of planning.
type Kind string

const (
	// KindRewrite means the call site has a safe rewrite.
	KindRewrite Kind = "rewrite"
	// KindSkip means the call site is reported but not rewritten.
	KindSkip Kind = "skip"
)

// Call-site skip categories (spec/rules.yaml skipCategories).
const (
	CategoryDynamicKey          = "dynamic-key"
	CategoryContextUnproven     = "context-unproven"
	CategoryUnboundClient       = "unbound-client"
	CategoryDefaultTypeUnproven = "default-type-unproven"
)

// Edit is a single text replacement of [Pos, End).
type Edit = fix.Edit

// Result is the planner outcome. Edit is set only for KindRewrite; Category
// only for KindSkip. Message is the diagnostic text (rule message with the
// key substituted, or the skip category message).
type Result struct {
	Kind     Kind
	Category string
	RuleID   string
	Message  string
	Edit     *Edit
	// Imports holds the import edits belonging to the rewrite; set by
	// PlanFile only.
	Imports []Edit
	// AddImports are the import paths the rewrite needs. Consumed are the
	// PkgName identifiers (e.g. the `ldcontext` in `ldcontext.New`) that no
	// longer exist after the rewrite.
	AddImports []string
	Consumed   []*ast.Ident
}

// Facts are the call-site proofs the detector established. Text fields are
// verbatim source copies.
type Facts struct {
	KeyStatic     bool
	Key           string
	Default       string
	DefaultProven bool
	ContextProven bool
	EvalCtx       string
	GoCtx         string
	ClientBound   bool
	Client        string
	AddImports    []string
	Consumed      []*ast.Ident
}

// CallSite is a detected SDK call.
type CallSite struct {
	Call   *ast.CallExpr
	Method string
	Facts  Facts
}

func skip(rule spec.Rule, category string) Result {
	r := Result{Kind: KindSkip, Category: category, RuleID: rule.ID}
	if c, ok := spec.SkipCategories()[category]; ok {
		r.Message = c.Message
	}
	return r
}

// Plan returns the plan for a call site.
func Plan(site CallSite) Result {
	rule, ok := spec.LookupGoMethod(site.Method)
	if !ok {
		// Identity is the caller's job; an unknown method is never rewritten.
		return skip(spec.Rule{}, CategoryUnboundClient)
	}
	if rule.Skip != "" {
		return skip(rule, rule.Skip)
	}
	f := site.Facts
	switch {
	case rule.Go == nil || rule.Go.Template == "":
		// Unreachable for a validated spec: a rewriting rule has a template.
		return skip(rule, CategoryUnboundClient)
	case !f.KeyStatic:
		return skip(rule, CategoryDynamicKey)
	case slices.Contains(rule.Go.Requires, CategoryContextUnproven) && !f.ContextProven:
		return skip(rule, CategoryContextUnproven)
	case !f.ClientBound:
		return skip(rule, CategoryUnboundClient)
	case !f.DefaultProven:
		return skip(rule, CategoryDefaultTypeUnproven)
	}
	text, err := fix.Expand(rule.Go.Template, map[string]string{
		"client": f.Client, "key": f.Key, "default": f.Default,
		"goctx": f.GoCtx, "evalctx": f.EvalCtx,
	})
	if err != nil {
		return skip(rule, CategoryContextUnproven)
	}
	return Result{
		Kind:       KindRewrite,
		RuleID:     rule.ID,
		Message:    strings.ReplaceAll(rule.Message, "{{key}}", f.Key),
		Edit:       &Edit{Pos: site.Call.Pos(), End: site.Call.End(), NewText: []byte(text)},
		AddImports: f.AddImports,
		Consumed:   f.Consumed,
	}
}

// FileInfo is the per-file context PlanFile needs for import edits.
type FileInfo struct {
	Fset *token.FileSet
	File *ast.File
	Src  []byte
	// Removable maps an import spec that is a removal candidate (for example
	// ldcontext) to every identifier in the file that references it.
	Removable map[*ast.ImportSpec][]*ast.Ident
}

// PlanFile plans every site of one file and attaches the import edits to the
// rewrites. A removal candidate is dropped from every rewrite's edits when all
// of its uses disappear, assuming all of the file's rewrites are applied
// together ("remove an import only if the file no longer references it").
func PlanFile(fi FileInfo, sites []CallSite) []Result {
	results := make([]Result, len(sites))
	gone := map[*ast.Ident]bool{}
	for i, s := range sites {
		results[i] = Plan(s)
		if results[i].Kind == KindRewrite {
			for _, id := range results[i].Consumed {
				gone[id] = true
			}
		}
	}
	var remove []*ast.ImportSpec
	for sp, uses := range fi.Removable {
		if len(uses) > 0 && slices.IndexFunc(uses, func(id *ast.Ident) bool { return !gone[id] }) < 0 {
			remove = append(remove, sp)
		}
	}
	slices.SortFunc(remove, func(a, b *ast.ImportSpec) int { return int(a.Pos()) - int(b.Pos()) })
	for i := range results {
		if results[i].Kind != KindRewrite {
			continue
		}
		results[i].Imports = fix.ImportEdits(fi.Fset, fi.File, fi.Src, fix.ImportChange{Add: results[i].AddImports, Remove: remove})
	}
	return results
}
