// Package planner decides, for one detected call site, whether the call is
// rewritten or reported only. All front ends (analyzer, fixer) call Plan;
// none re-implement detection.
//
// Step order (identical across languages, ADR 010): identity (the caller),
// rule-level skip, dynamic-key, context-unproven, default-type-unproven,
// unbound-client, receiver-unproven, else rewrite (or, when a comment inside
// the call has no home in the rewritten call, report it without a fix:
// KindUnfixable). A rewrite that would leave
// a function-local client variable with no remaining use is skipped as
// receiver-unproven for every call on that variable (see PlanFile).
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
	// KindUnfixable means the call is rewritable but a comment inside it has
	// no home in the rewritten call, so it is reported without a fix (issue
	// #31). It is not a skip category: the message is the rule's own.
	KindUnfixable Kind = "unfixable"
)

// Call-site skip categories (spec/rules.yaml skipCategories).
const (
	CategoryDynamicKey          = "dynamic-key"
	CategoryContextUnproven     = "context-unproven"
	CategoryUnboundClient       = "unbound-client"
	CategoryDefaultTypeUnproven = "default-type-unproven"
	CategoryReceiverUnproven    = "receiver-unproven"
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
	// Nested is set on a rewrite whose call contains another rewritten call:
	// both would edit overlapping ranges, so only the inner one carries a
	// fix. The outer rewrite is picked up on the next run.
	Nested bool
}

// Facts are the call-site proofs the detector established. Text fields are
// verbatim source copies.
type Facts struct {
	KeyStatic bool
	Key       string
	// KeyValue is the decoded value of a static string-literal key, the
	// fingerprint input. Unlike Key (source text) it is also collected for
	// report-only rules, and is nil for a dynamic key or one that is not valid
	// UTF-8.
	KeyValue      *string
	Default       string
	DefaultProven bool
	ContextProven bool
	EvalCtx       string
	GoCtx         string
	ClientBound   bool
	Client        string
	AddImports    []string
	Consumed      []*ast.Ident
	// ReceiverPure is true when the receiver expression is an identifier or a
	// selector chain of identifiers and fields, so dropping it loses no side
	// effects. ReceiverRoot is its leftmost identifier.
	ReceiverPure bool
	ReceiverRoot *ast.Ident
	// Receiver is set when the root is a function-local variable that the
	// rewrite could leave without a use; nil for package-level variables,
	// parameters, named results and package names, which never strand.
	Receiver *Local
	// Notes places the comments inside the call range (issue #31); nil when
	// the call has none outside its copied argument text.
	Notes *fix.Notes
}

// Local describes a function-local variable: Uses are its references that are
// not plain assignment targets (=, :=, op=, ++), i.e. the ones that keep the
// variable "used" in the compiler's sense.
type Local struct {
	Uses []*ast.Ident
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

// requiredPlaceholders maps each placeholder to the skip category reported
// when a template needs it but no value was proven.
var requiredPlaceholders = []struct{ name, category string }{
	{"key", CategoryDynamicKey},
	{"evalctx", CategoryContextUnproven},
	{"goctx", CategoryContextUnproven},
	{"default", CategoryDefaultTypeUnproven},
	{"client", CategoryUnboundClient},
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
	case !f.DefaultProven:
		return skip(rule, CategoryDefaultTypeUnproven)
	case !f.ClientBound:
		return skip(rule, CategoryUnboundClient)
	case !f.ReceiverPure:
		return skip(rule, CategoryReceiverUnproven)
	}
	vals := map[string]string{
		"client": f.Client, "key": f.Key, "default": f.Default,
		"goctx": f.GoCtx, "evalctx": f.EvalCtx,
	}
	// An empty value for a placeholder the template uses is a planner
	// invariant violation (the earlier steps prove each one): never emit
	// code with a hole in it.
	for _, p := range requiredPlaceholders {
		if vals[p.name] == "" && strings.Contains(rule.Go.Template, "{{"+p.name+"}}") {
			return skip(rule, p.category)
		}
	}
	message := strings.ReplaceAll(rule.Message, "{{key}}", f.Key)
	// Comments inside the call move with their arguments; one with no home
	// withholds the fix (the call is still reported). Not a skip category.
	if f.Notes != nil && f.Notes.Unhomed {
		return Result{Kind: KindUnfixable, RuleID: rule.ID, Message: message}
	}
	text, placed, err := fix.ExpandNotes(rule.Go.Template, vals, f.Notes)
	if err != nil {
		return skip(rule, CategoryContextUnproven)
	}
	if !placed {
		return Result{Kind: KindUnfixable, RuleID: rule.ID, Message: message}
	}
	return Result{
		Kind:       KindRewrite,
		RuleID:     rule.ID,
		Message:    message,
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
// rewrites.
//
// Stranding: when every rewritable call on a function-local variable would
// leave it with no remaining use (Go rejects an unused local), all of those
// calls are skipped as receiver-unproven, never a subset.
//
// Nesting: when one rewritten call contains another, only the inner call
// carries a fix (see Result.Nested).
//
// Imports: each fix must be sound when applied alone, so a removal candidate
// (for example ldcontext) is attached to a fix only when that fix alone
// consumes every remaining use of it in the file. Batch drivers clean up the
// imports that become unused once several fixes are applied together.
func PlanFile(fi FileInfo, sites []CallSite) []Result {
	results := make([]Result, len(sites))
	for i, s := range sites {
		results[i] = Plan(s)
	}
	strand(sites, results)
	markNested(sites, results)

	order := make([]*ast.ImportSpec, 0, len(fi.Removable))
	for sp := range fi.Removable {
		order = append(order, sp)
	}
	slices.SortFunc(order, func(a, b *ast.ImportSpec) int { return int(a.Pos()) - int(b.Pos()) })
	for i := range results {
		if results[i].Kind != KindRewrite || results[i].Nested {
			continue
		}
		consumed := map[*ast.Ident]bool{}
		for _, id := range results[i].Consumed {
			consumed[id] = true
		}
		var remove []*ast.ImportSpec
		for _, sp := range order {
			uses := fi.Removable[sp]
			if len(uses) > 0 && slices.IndexFunc(uses, func(id *ast.Ident) bool { return !consumed[id] }) < 0 {
				remove = append(remove, sp)
			}
		}
		results[i].Imports = fix.ImportEdits(fi.Fset, fi.File, fi.Src, fix.ImportChange{Add: results[i].AddImports, Remove: remove})
	}
	return results
}

// markNested flags every rewrite whose call range contains another rewrite's.
func markNested(sites []CallSite, results []Result) {
	for i := range sites {
		if results[i].Kind != KindRewrite {
			continue
		}
		for j := range sites {
			if i != j && results[j].Kind == KindRewrite &&
				sites[i].Call.Pos() <= sites[j].Call.Pos() && sites[j].Call.End() <= sites[i].Call.End() {
				results[i].Nested = true
				break
			}
		}
	}
}

// strand turns into receiver-unproven skips every rewrite whose receiver root
// is a local variable with no surviving use. A surviving use is any recorded
// use that is not the receiver root of a call still planned as a rewrite.
func strand(sites []CallSite, results []Result) {
	groups := map[*Local][]int{}
	var order []*Local
	for i, s := range sites {
		if results[i].Kind != KindRewrite || s.Facts.Receiver == nil {
			continue
		}
		l := s.Facts.Receiver
		if _, ok := groups[l]; !ok {
			order = append(order, l)
		}
		groups[l] = append(groups[l], i)
	}
	for _, l := range order {
		roots := map[*ast.Ident]bool{}
		for _, i := range groups[l] {
			roots[sites[i].Facts.ReceiverRoot] = true
		}
		if slices.IndexFunc(l.Uses, func(id *ast.Ident) bool { return !roots[id] }) >= 0 {
			continue
		}
		for _, i := range groups[l] {
			rule, _ := spec.LookupGoMethod(sites[i].Method)
			results[i] = skip(rule, CategoryReceiverUnproven)
		}
	}
}
