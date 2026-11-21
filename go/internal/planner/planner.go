// Package planner decides, for one detected call site, whether the call is
// rewritten or reported only. All front ends (analyzer, fixer) call Plan;
// none re-implement detection.
package planner

import (
	"go/ast"
	"go/token"

	"github.com/jh125486/fldlint/go/internal/spec"
)

// Kind is the outcome class of planning.
type Kind string

const (
	KindRewrite Kind = "rewrite"
	KindSkip    Kind = "skip"
)

// CategoryUnboundClient is the skip category used by the M0 stub.
const CategoryUnboundClient = "unbound-client"

// Edit is a single text replacement of [Pos, End).
type Edit struct {
	Pos     token.Pos
	End     token.Pos
	NewText []byte
}

// Result is the planner outcome. Edit is set only for KindRewrite; Category
// only for KindSkip.
type Result struct {
	Kind     Kind
	Category string
	RuleID   string
	Edit     *Edit
}

// CallSite is a detected SDK call.
type CallSite struct {
	Call   *ast.CallExpr
	Method string
}

// Plan returns the plan for a call site. M0 stub: always skips as
// unbound-client; real detection arrives in M2.
func Plan(site CallSite) Result {
	res := Result{Kind: KindSkip, Category: CategoryUnboundClient}
	if r, ok := spec.LookupGoMethod(site.Method); ok {
		res.RuleID = r.ID
	}
	return res
}
