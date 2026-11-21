// Package spec loads the language-neutral fldlint spec (rules.yaml and
// fixture case.yaml files) and exposes the generated Go rules table.
package spec

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
)

// SkipCategory describes a report-only category.
type SkipCategory struct {
	Severity string
	Message  string
}

// GoBinding is the Go-specific part of a rule.
type GoBinding struct {
	Methods  []string
	CtxArg   map[string]int
	Template string
	Requires []string
}

// Rule is one entry of rules.yaml. Argument indexes are -1 when absent.
type Rule struct {
	ID         string
	ValueType  string
	Risk       string
	KeyArg     int
	DefaultArg int
	ContextArg int
	Skip       string
	Go         *GoBinding
}

// RulesFile is the parsed form of rules.yaml (Go view only).
type RulesFile struct {
	Version        int
	SkipCategories map[string]SkipCategory
	Rules          []Rule
}

// Placeholders is the closed set of template placeholders.
var Placeholders = []string{"client", "key", "default", "context", "goctx", "evalctx"}

var placeholderRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

type rawRules struct {
	Version        int                   `yaml:"version"`
	SkipCategories map[string]rawSkipCat `yaml:"skipCategories"`
	Rules          []rawRule             `yaml:"rules"`
}

type rawSkipCat struct {
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
}

type rawRule struct {
	ID         string               `yaml:"id"`
	ValueType  string               `yaml:"valueType"`
	Risk       string               `yaml:"risk"`
	KeyArg     *int                 `yaml:"keyArg"`
	DefaultArg *int                 `yaml:"defaultArg"`
	ContextArg *int                 `yaml:"contextArg"`
	Skip       string               `yaml:"skip"`
	Languages  map[string]yaml.Node `yaml:"languages"`
}

type rawGo struct {
	Methods  []string       `yaml:"methods"`
	CtxArg   map[string]int `yaml:"ctxArg"`
	Template string         `yaml:"template"`
	Requires []string       `yaml:"requires"`
}

func argOrAbsent(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

// ParseRules parses and validates rules.yaml content.
func ParseRules(data []byte) (*RulesFile, error) {
	var raw rawRules
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	rf := &RulesFile{Version: raw.Version, SkipCategories: map[string]SkipCategory{}}
	for k, c := range raw.SkipCategories {
		rf.SkipCategories[k] = SkipCategory(c)
	}
	for _, rr := range raw.Rules {
		r := Rule{
			ID: rr.ID, ValueType: rr.ValueType, Risk: rr.Risk, Skip: rr.Skip,
			KeyArg: argOrAbsent(rr.KeyArg), DefaultArg: argOrAbsent(rr.DefaultArg), ContextArg: argOrAbsent(rr.ContextArg),
		}
		if n, ok := rr.Languages["go"]; ok {
			var g rawGo
			if err := n.Decode(&g); err != nil {
				return nil, fmt.Errorf("rule %q: go binding: %w", rr.ID, err)
			}
			r.Go = &GoBinding{Methods: g.Methods, CtxArg: g.CtxArg, Template: g.Template, Requires: g.Requires}
		}
		rf.Rules = append(rf.Rules, r)
	}
	if err := rf.Validate(); err != nil {
		return nil, err
	}
	return rf, nil
}

// LoadRules reads, parses and validates a rules.yaml file.
func LoadRules(path string) (*RulesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	return ParseRules(data)
}

// Validate checks structural invariants of the rules file.
func (rf *RulesFile) Validate() error {
	if rf.Version != 1 {
		return fmt.Errorf("rules: unsupported version %d", rf.Version)
	}
	ids := map[string]bool{}
	methods := map[string]string{}
	for _, r := range rf.Rules {
		if r.ID == "" {
			return fmt.Errorf("rules: rule with empty id")
		}
		if ids[r.ID] {
			return fmt.Errorf("rules: duplicate rule id %q", r.ID)
		}
		ids[r.ID] = true
		if r.Skip != "" {
			if _, ok := rf.SkipCategories[r.Skip]; !ok {
				return fmt.Errorf("rule %q: unknown skip category %q", r.ID, r.Skip)
			}
		}
		g := r.Go
		if g == nil {
			continue
		}
		if len(g.Methods) == 0 {
			return fmt.Errorf("rule %q: go binding has no methods", r.ID)
		}
		for _, m := range g.Methods {
			if prev, dup := methods[m]; dup {
				return fmt.Errorf("rule %q: go method %q already bound by rule %q", r.ID, m, prev)
			}
			methods[m] = r.ID
		}
		for m := range g.CtxArg {
			if !slices.Contains(g.Methods, m) {
				return fmt.Errorf("rule %q: ctxArg for unlisted method %q", r.ID, m)
			}
		}
		for _, c := range g.Requires {
			if _, ok := rf.SkipCategories[c]; !ok {
				return fmt.Errorf("rule %q: requires unknown category %q", r.ID, c)
			}
		}
		switch {
		case r.Skip != "" && g.Template != "":
			return fmt.Errorf("rule %q: skip and go template are mutually exclusive", r.ID)
		case r.Skip == "" && g.Template == "":
			return fmt.Errorf("rule %q: go binding needs a template or the rule needs skip", r.ID)
		}
		for _, m := range placeholderRe.FindAllStringSubmatch(g.Template, -1) {
			if !slices.Contains(Placeholders, m[1]) {
				return fmt.Errorf("rule %q: unknown placeholder %q", r.ID, m[1])
			}
		}
	}
	return nil
}

// SkipCategoryNames returns the sorted skip category names.
func (rf *RulesFile) SkipCategoryNames() []string {
	names := make([]string, 0, len(rf.SkipCategories))
	for k := range rf.SkipCategories {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Rules returns the generated rules table (a copy of the slice header; do not mutate).
func Rules() []Rule { return genRules }

// SkipCategories returns the generated skip categories.
func SkipCategories() map[string]SkipCategory { return genSkipCategories }

// LookupGoMethod finds the rule bound to a Go SDK method name.
func LookupGoMethod(method string) (Rule, bool) {
	i, ok := genGoIndex[method]
	if !ok {
		return Rule{}, false
	}
	return genRules[i], true
}
