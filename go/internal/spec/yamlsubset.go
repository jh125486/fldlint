package spec

import (
	"errors"
	"fmt"
)

// This package enforces the "fldlint YAML subset" defined in
// spec/contracts/validation-parity.md. yamlprescan.go holds the rules on the
// input text, yamlcheck.go the rules on the yaml.v3 node tree and
// yamlscalar.go the scalar typing, which never trusts the implicit typing of
// yaml.v3.

// Sentinel errors, one per rejection reason of the subset. Every error the
// subset check itself returns is a *SubsetError that wraps ErrSubset and
// exactly one of these, so callers can use errors.Is for either. Input that
// is not well-formed YAML is rejected by yaml.v3 with its own error.
var (
	// ErrSubset is wrapped by every subset violation.
	ErrSubset = errors.New("fldlint YAML subset violation")
	// ErrEncoding marks input that is not valid UTF-8.
	ErrEncoding = errors.New("input is not valid UTF-8")
	// ErrCharacter marks a character outside YAML's printable set, a CR that
	// does not start a CRLF, or the line breaks U+2028 and U+2029.
	ErrCharacter = errors.New("character not allowed (printable characters, LF and CRLF only)")
	// ErrTabIndent marks a tab in the leading whitespace of a line, whatever
	// the line holds: after zero or more spaces, or after leading "-" or
	// "?" indicators and spaces.
	ErrTabIndent = errors.New("tab in the leading whitespace of a line, or after a leading \"-\" or \"?\", not allowed")
	// ErrEmptyDocument marks input with no document: empty, blank, BOM only
	// or comments only. A bare "---" is a document (a null), not empty.
	ErrEmptyDocument = errors.New("empty document: no content")
	// ErrDirective marks a %YAML or %TAG directive.
	ErrDirective = errors.New("directive not allowed")
	// ErrMultipleDocuments marks input with more than one YAML document; a
	// trailing "---" starts a second one.
	ErrMultipleDocuments = errors.New("multiple YAML documents")
	// ErrTag marks an explicit tag (including the non-specific tag !).
	ErrTag = errors.New("explicit tag not allowed")
	// ErrAnchor marks an anchor, and so an alias, which needs one.
	ErrAnchor = errors.New("anchor not allowed")
	// ErrKeyNotScalar marks a mapping key that is not a scalar.
	ErrKeyNotScalar = errors.New("mapping key must be a scalar")
	// ErrKeyNotString marks a scalar mapping key that is not a string.
	ErrKeyNotString = errors.New("mapping key must be a string")
	// ErrDuplicateKey marks a repeated mapping key.
	ErrDuplicateKey = errors.New("duplicate key")
	// ErrBlockIndicator marks a block scalar header with an indentation
	// indicator (|2, >1-): the parsers disagree on the whitespace-only lines
	// of such a scalar.
	ErrBlockIndicator = errors.New("block scalar indentation indicator not allowed")
	// ErrFlowScalar marks a plain scalar inside a flow collection that ends
	// with ":" ("[a:]"): the parsers read it as a pair and as a string.
	ErrFlowScalar = errors.New("plain scalar ending with a colon not allowed in a flow collection")
	// ErrTooDeep marks collections nested more than MaxDepth levels.
	ErrTooDeep = errors.New("nesting too deep")
	// ErrAmbiguousScalar marks a plain scalar that YAML 1.1 or 1.2 could type
	// as a non-string but that the subset does not accept (quote it).
	ErrAmbiguousScalar = errors.New("ambiguous plain scalar")
	// ErrIntRange marks an integer outside +-9007199254740991.
	ErrIntRange = errors.New("integer outside the safe range")
)

// SubsetError is a violation of the fldlint YAML subset at a source position.
// It unwraps to ErrSubset and to the specific reason sentinel.
type SubsetError struct {
	Line   int   // 1-based line of the offending node
	Column int   // 1-based column, counted in runes; 0 when unknown
	Reason error // one of the Err* reason sentinels, possibly with detail
}

func (e *SubsetError) Error() string {
	if e.Column > 0 {
		return fmt.Sprintf("line %d, column %d: %v", e.Line, e.Column, e.Reason)
	}
	return fmt.Sprintf("line %d: %v", e.Line, e.Reason)
}

// Unwrap lets errors.Is and errors.As reach ErrSubset and the reason.
func (e *SubsetError) Unwrap() []error { return []error{ErrSubset, e.Reason} }

func newSubsetError(line, col int, reason error, detail string) *SubsetError {
	if detail != "" {
		reason = fmt.Errorf("%w: %s", reason, detail)
	}
	return &SubsetError{Line: line, Column: col, Reason: reason}
}

// MaxDepth is the deepest nesting of collections (mappings and sequences,
// block or flow) the subset accepts; the root collection is level 1.
const MaxDepth = 100
