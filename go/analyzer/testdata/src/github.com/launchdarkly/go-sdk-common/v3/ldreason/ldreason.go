// Package ldreason is a test stub of go-sdk-common v3 ldreason.
package ldreason

// EvalReasonKind is the kind of an evaluation reason.
type EvalReasonKind string

// EvaluationReason is a stub reason.
type EvaluationReason struct{}

// GetKind returns the kind.
func (r EvaluationReason) GetKind() EvalReasonKind { return "" }

// EvaluationDetail is a stub detail.
type EvaluationDetail struct{ Reason EvaluationReason }
