package spec

import "fmt"

// Diagnostic is one expected report. Line and Col are 1-based.
type Diagnostic struct {
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	RuleID string `json:"ruleId"`
	Skip   string `json:"skip,omitempty"`
	// NoFix is "comment" for a rewritable call reported with no fix (every
	// language that reports one).
	NoFix string `json:"noFix,omitempty"`
}

// LangExpect is the expectation for one language.
type LangExpect struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Output      bool         `json:"output"`
}

// Case is a fixture case.yaml as it appears in spec.json.
type Case struct {
	ID          string                `json:"id"`
	Rule        string                `json:"rule"`
	Tags        []string              `json:"tags"`
	Description string                `json:"description"`
	Expect      map[string]LangExpect `json:"expect"`
}

// ParseCase decodes one "case" object of spec.json strictly (unknown fields
// are errors). The case content, its cross-references into the rules and the
// fixture files it promises were validated by the JS loader before it emitted
// spec.json; they are not re-validated here.
func ParseCase(data []byte) (*Case, error) {
	var c Case
	if err := decodeStrict(data, &c); err != nil {
		return nil, fmt.Errorf("parse case: %w", err)
	}
	if c.Expect == nil {
		c.Expect = map[string]LangExpect{}
	}
	return &c, nil
}
