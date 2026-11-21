package spec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// Languages is the closed set of language keys allowed under expect.
var Languages = []string{"js", "go", "java"}

// Diagnostic is one expected report. Line and Col are 1-based.
type Diagnostic struct {
	Line   int    `yaml:"line"`
	Col    int    `yaml:"col"`
	RuleID string `yaml:"ruleId"`
	Skip   string `yaml:"skip"`
}

// LangExpect is the expectation for one language.
type LangExpect struct {
	Diagnostics []Diagnostic `yaml:"diagnostics"`
	Output      bool         `yaml:"output"`
}

// Case is a parsed fixture case.yaml.
type Case struct {
	ID          string                `yaml:"id"`
	Rule        string                `yaml:"rule"`
	Tags        []string              `yaml:"tags"`
	Description string                `yaml:"description"`
	Expect      map[string]LangExpect `yaml:"expect"`
}

// ParseCase parses case.yaml content strictly (unknown fields are errors).
func ParseCase(data []byte) (*Case, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c Case
	if err := dec.Decode(&c); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("parse case: empty document")
		}
		return nil, fmt.Errorf("parse case: %w", err)
	}
	return &c, nil
}

// LoadCase reads and parses a case.yaml file (without semantic validation).
func LoadCase(path string) (*Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read case: %w", err)
	}
	return ParseCase(data)
}

// Validate checks required fields, skip categories, rule references and tags
// against rf.
func (c *Case) Validate(rf *RulesFile) error {
	if c.ID == "" {
		return fmt.Errorf("case: missing id")
	}
	if c.Rule == "" {
		return fmt.Errorf("case %q: missing rule", c.ID)
	}
	if !slices.ContainsFunc(rf.Rules, func(r Rule) bool { return r.ID == c.Rule }) {
		return fmt.Errorf("case %q: unknown rule %q", c.ID, c.Rule)
	}
	if c.Description == "" {
		return fmt.Errorf("case %q: missing description", c.ID)
	}
	seen := map[string]bool{}
	for _, t := range c.Tags {
		if t == "" {
			return fmt.Errorf("case %q: empty tag", c.ID)
		}
		if seen[t] {
			return fmt.Errorf("case %q: duplicate tag %q", c.ID, t)
		}
		seen[t] = true
	}
	if len(c.Expect) == 0 {
		return fmt.Errorf("case %q: expect must name at least one language", c.ID)
	}
	for lang, e := range c.Expect {
		if !slices.Contains(Languages, lang) {
			return fmt.Errorf("case %q: unknown language %q", c.ID, lang)
		}
		for i, d := range e.Diagnostics {
			if d.Line < 1 || d.Col < 1 {
				return fmt.Errorf("case %q: %s diagnostic %d: line/col must be >= 1", c.ID, lang, i)
			}
			if d.RuleID == "" {
				return fmt.Errorf("case %q: %s diagnostic %d: missing ruleId", c.ID, lang, i)
			}
			if d.Skip != "" {
				if _, ok := rf.SkipCategories[d.Skip]; !ok {
					return fmt.Errorf("case %q: %s diagnostic %d: unknown skip category %q", c.ID, lang, i, d.Skip)
				}
			}
		}
	}
	return nil
}
