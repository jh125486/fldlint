// Package spec loads the language-neutral fldlint spec and exposes the
// generated Go rules table.
//
// The spec is authored as YAML (spec/rules.yaml and the fixture case.yaml
// files) but parsed in exactly one place, the JS loader, which validates it
// (YAML subset, JSON Schema, semantic checks) and emits spec/generated/spec.json.
// This package reads only that JSON; it contains no YAML code. See
// docs/adr/009-parse-yaml-once-emit-canonical-json.md.
package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// SkipCategory describes a report-only category.
type SkipCategory struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Binding is the per-language part of a rule.
type Binding struct {
	Methods  []string       `json:"methods"`
	CtxArg   map[string]int `json:"ctxArg,omitempty"`
	Template string         `json:"template,omitempty"`
	Requires []string       `json:"requires,omitempty"`
	Async    bool           `json:"async,omitempty"`
}

// GoBinding is the Go-specific part of a rule.
type GoBinding = Binding

// Rule is one entry of rules.yaml. Argument indexes are -1 when absent.
// Langs holds every language binding when the rule comes from ParseSpec; the
// generated table only populates Go.
type Rule struct {
	ID         string
	ValueType  string
	Risk       string
	KeyArg     int
	DefaultArg int
	ContextArg int
	Skip       string
	Message    string
	Go         *GoBinding
	Langs      map[string]*Binding
}

// RulesFile is the parsed form of the "rules" object of spec.json.
type RulesFile struct {
	Version        int
	SkipCategories map[string]SkipCategory
	Rules          []Rule
}

// wireRule is a rule as it appears in spec.json (rules.yaml as JSON).
type wireRule struct {
	ID         string              `json:"id"`
	ValueType  string              `json:"valueType"`
	Risk       string              `json:"risk"`
	KeyArg     *int                `json:"keyArg"`
	DefaultArg *int                `json:"defaultArg"`
	ContextArg *int                `json:"contextArg"`
	Skip       string              `json:"skip"`
	Message    string              `json:"message"`
	Languages  map[string]*Binding `json:"languages"`
}

// wireRules is the "rules" object of spec.json.
type wireRules struct {
	Version        int                     `json:"version"`
	SkipCategories map[string]SkipCategory `json:"skipCategories"`
	Rules          []wireRule              `json:"rules"`
}

// decodeStrict decodes one JSON value into v, rejecting unknown fields and
// trailing data. The structure itself was validated by the JS loader; strict
// decoding only guards against a stale or hand-edited spec.json.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected data after the JSON value")
	}
	return nil
}

// ParseRules decodes the "rules" object of spec.json. Everything the schema
// and the semantic checks guarantee is validated once, by the JS loader, and
// not repeated here; this only decodes strictly, applies the -1 default for
// absent argument indexes, and rejects an unknown format version.
func ParseRules(data []byte) (*RulesFile, error) {
	var w wireRules
	if err := decodeStrict(data, &w); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	if w.Version != 1 {
		return nil, fmt.Errorf("parse rules: unsupported version %d", w.Version)
	}
	rf := &RulesFile{Version: w.Version, SkipCategories: w.SkipCategories}
	if rf.SkipCategories == nil {
		rf.SkipCategories = map[string]SkipCategory{}
	}
	goMethods := map[string]string{}
	for _, wr := range w.Rules {
		r := Rule{
			ID: wr.ID, ValueType: wr.ValueType, Risk: wr.Risk,
			KeyArg: argOr(wr.KeyArg), DefaultArg: argOr(wr.DefaultArg), ContextArg: argOr(wr.ContextArg),
			Skip: wr.Skip, Message: wr.Message,
			Langs: wr.Languages,
		}
		if r.Langs == nil {
			r.Langs = map[string]*Binding{}
		}
		r.Go = r.Langs["go"]
		// The generated Go index maps method -> rule, so a method bound twice
		// would silently shadow a rule: cheap to guard here.
		if r.Go != nil {
			for _, m := range r.Go.Methods {
				if prev, dup := goMethods[m]; dup {
					return nil, fmt.Errorf("parse rules: go method %q bound by rules %q and %q", m, prev, r.ID)
				}
				goMethods[m] = r.ID
			}
		}
		rf.Rules = append(rf.Rules, r)
	}
	return rf, nil
}

func argOr(p *int) int {
	if p == nil {
		return -1
	}
	return *p
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
