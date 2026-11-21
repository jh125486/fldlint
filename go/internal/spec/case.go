package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Languages is the closed set of language keys allowed under expect.
var Languages = []string{"js", "go", "java"}

// callSiteSkips are the skip categories allowed on a rewriting rule's
// diagnostics (the call is left alone because of how or where it is written).
var callSiteSkips = []string{"dynamic-key", "unbound-client", "context-unproven"}

// Diagnostic is one expected report. Line and Col are 1-based.
type Diagnostic struct {
	Line   int
	Col    int
	RuleID string
	Skip   string
}

// LangExpect is the expectation for one language.
type LangExpect struct {
	Diagnostics []Diagnostic
	Output      bool
}

// Case is a parsed fixture case.yaml.
type Case struct {
	ID          string
	Rule        string
	Tags        []string
	Description string
	Expect      map[string]LangExpect
}

// ParseCase strictly parses case.yaml content. It accepts and rejects the same
// inputs as spec/schema/fixture.schema.json; cross-references into rules.yaml
// are checked by Validate.
func ParseCase(data []byte) (*Case, error) {
	root, err := decodeDoc(data)
	if err != nil {
		return nil, fmt.Errorf("parse case: %w", err)
	}
	c, err := buildCase(root)
	if err != nil {
		return nil, fmt.Errorf("parse case: %w", err)
	}
	return c, nil
}

func buildCase(root *yaml.Node) (*Case, error) {
	mp, err := newMapping(root, "case", []string{"id", "rule", "tags", "description", "expect"})
	if err != nil {
		return nil, err
	}
	if err := mp.require("id", "rule", "tags", "description", "expect"); err != nil {
		return nil, err
	}
	c := &Case{Expect: map[string]LangExpect{}}
	if c.ID, err = asString(mp.m["id"], mp.child("id")); err != nil {
		return nil, err
	}
	if !caseIDRe.MatchString(c.ID) {
		return nil, failAt(mp.m["id"], mp.child("id"), "%q does not match the case id pattern", c.ID)
	}
	if c.Rule, err = asString(mp.m["rule"], mp.child("rule")); err != nil {
		return nil, err
	}
	if !ruleIDRe.MatchString(c.Rule) {
		return nil, failAt(mp.m["rule"], mp.child("rule"), "%q does not match the rule id pattern", c.Rule)
	}
	tagOK := func(s string) error {
		if !caseIDRe.MatchString(s) {
			return fmt.Errorf("%q does not match the tag pattern", s)
		}
		return nil
	}
	if c.Tags, err = asStringSeq(mp.m["tags"], mp.child("tags"), 0, tagOK); err != nil {
		return nil, err
	}
	if c.Description, err = asString(mp.m["description"], mp.child("description")); err != nil {
		return nil, err
	}
	if c.Description == "" {
		return nil, failAt(mp.m["description"], mp.child("description"), "must not be empty")
	}
	em, err := newMapping(mp.m["expect"], mp.child("expect"), Languages)
	if err != nil {
		return nil, err
	}
	if len(em.keys) == 0 {
		return nil, failAt(em.node, em.path, "needs at least 1 language")
	}
	for _, lang := range em.keys {
		le, err := parseLangExpect(em.m[lang], em.child(lang))
		if err != nil {
			return nil, err
		}
		c.Expect[lang] = le
	}
	return c, nil
}

func parseLangExpect(n *yaml.Node, path string) (LangExpect, error) {
	var le LangExpect
	mp, err := newMapping(n, path, []string{"diagnostics", "output"})
	if err != nil {
		return le, err
	}
	if err := mp.require("diagnostics", "output"); err != nil {
		return le, err
	}
	if le.Output, err = asBool(mp.m["output"], mp.child("output")); err != nil {
		return le, err
	}
	items, err := asSeq(mp.m["diagnostics"], mp.child("diagnostics"))
	if err != nil {
		return le, err
	}
	le.Diagnostics = []Diagnostic{}
	for i, it := range items {
		dp := fmt.Sprintf("%s[%d]", mp.child("diagnostics"), i)
		dm, err := newMapping(it, dp, []string{"line", "col", "ruleId", "skip"})
		if err != nil {
			return le, err
		}
		if err := dm.require("line", "col", "ruleId"); err != nil {
			return le, err
		}
		var d Diagnostic
		if d.Line, err = asInt(dm.m["line"], dm.child("line"), 1); err != nil {
			return le, err
		}
		if d.Col, err = asInt(dm.m["col"], dm.child("col"), 1); err != nil {
			return le, err
		}
		if d.RuleID, err = asString(dm.m["ruleId"], dm.child("ruleId")); err != nil {
			return le, err
		}
		if !ruleIDRe.MatchString(d.RuleID) {
			return le, failAt(dm.m["ruleId"], dm.child("ruleId"), "%q does not match the rule id pattern", d.RuleID)
		}
		if v, ok := dm.m["skip"]; ok {
			if d.Skip, err = asEnum(v, dm.child("skip"), SkipNames); err != nil {
				return le, err
			}
		}
		le.Diagnostics = append(le.Diagnostics, d)
	}
	return le, nil
}

// LoadCase reads and strictly parses a case.yaml file.
func LoadCase(path string) (*Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read case: %w", err)
	}
	return ParseCase(data)
}

// Validate checks the cross-references into rf and the semantic rules of the
// fixture contract: the case rule must exist and bind every expected language;
// every diagnostic ruleId must equal the case rule; on a skip rule every diagnostic must carry the rule's skip, on a rewriting
// rule a diagnostic carries no skip or a call-site skip; any skip implies
// output false, and output true needs at least one diagnostic.
func (c *Case) Validate(rf *RulesFile) error {
	idx := slices.IndexFunc(rf.Rules, func(r Rule) bool { return r.ID == c.Rule })
	if idx < 0 {
		return fmt.Errorf("case %q: unknown rule %q", c.ID, c.Rule)
	}
	rule := rf.Rules[idx]
	for _, lang := range Languages {
		e, ok := c.Expect[lang]
		if !ok {
			continue
		}
		if rule.Langs[lang] == nil {
			return fmt.Errorf("case %q: expect.%s is not allowed because rule %q has no %s binding", c.ID, lang, c.Rule, lang)
		}
		for i, d := range e.Diagnostics {
			if !slices.ContainsFunc(rf.Rules, func(r Rule) bool { return r.ID == d.RuleID }) {
				return fmt.Errorf("case %q: %s diagnostic %d: unknown ruleId %q", c.ID, lang, i, d.RuleID)
			}
			if d.RuleID != c.Rule {
				return fmt.Errorf("case %q: %s diagnostic %d: ruleId %q must equal the case rule %q", c.ID, lang, i, d.RuleID, c.Rule)
			}
			if d.Skip != "" {
				if _, ok := rf.SkipCategories[d.Skip]; !ok {
					return fmt.Errorf("case %q: %s diagnostic %d: unknown skip category %q", c.ID, lang, i, d.Skip)
				}
			}
			switch {
			case rule.Skip != "" && d.Skip != rule.Skip:
				return fmt.Errorf("case %q: %s diagnostic %d: skip %q must equal the rule's skip %q", c.ID, lang, i, d.Skip, rule.Skip)
			case rule.Skip == "" && d.Skip != "" && !slices.Contains(callSiteSkips, d.Skip):
				return fmt.Errorf("case %q: %s diagnostic %d: skip %q is not a call-site skip (dynamic-key, unbound-client, context-unproven) allowed on rewriting rule %q", c.ID, lang, i, d.Skip, c.Rule)
			}
			if d.Skip != "" && e.Output {
				return fmt.Errorf("case %q: expect.%s.output must be false when a diagnostic carries skip %q", c.ID, lang, d.Skip)
			}
		}
		if e.Output && len(e.Diagnostics) == 0 {
			return fmt.Errorf("case %q: expect.%s.output is true but there are no diagnostics", c.ID, lang)
		}
	}
	return nil
}

// DiscoverFixtures returns the sorted case.yaml paths under root, which must be
// laid out as <root>/<rule>/<case>/case.yaml. A case directory without
// case.yaml, a case.yml, or a case file at any other depth is an error.
// Regular files directly under root or a rule directory are ignored.
func DiscoverFixtures(root string) ([]string, error) {
	if fi, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("fixtures: %w", err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("fixtures: %s is not a directory", root)
	}
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		depth := len(strings.Split(rel, string(filepath.Separator)))
		if d.IsDir() {
			if depth == 2 {
				if _, err := os.Stat(filepath.Join(p, "case.yaml")); err != nil {
					return fmt.Errorf("fixtures: %s has no case.yaml", filepath.ToSlash(rel))
				}
			}
			return nil
		}
		switch d.Name() {
		case "case.yml":
			return fmt.Errorf("fixtures: %s: case.yml is not allowed, use case.yaml", filepath.ToSlash(rel))
		case "case.yaml":
			if depth != 3 {
				return fmt.Errorf("fixtures: %s is at an unexpected depth, expected <rule>/<case>/case.yaml", filepath.ToSlash(rel))
			}
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// CheckUniqueIDs reports an error when two cases share an id.
func CheckUniqueIDs(cases map[string]*Case) error {
	seen := map[string]string{}
	paths := make([]string, 0, len(cases))
	for p := range cases {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		id := cases[p].ID
		if prev, dup := seen[id]; dup {
			return fmt.Errorf("duplicate case id %q in %s and %s", id, prev, p)
		}
		seen[id] = p
	}
	return nil
}

// langFiles maps each language to its fixture input and expected-output file.
var langFiles = map[string][2]string{
	"js":   {"js/input.ts", "js/output.ts"},
	"go":   {"go/input.go", "go/output.go.golden"},
	"java": {"java/Before.java", "java/After.java"},
}

// CheckFixtureDir verifies the case against its directory, which must be
// <...>/<rule-id>/<case-id>: the rule must equal the parent directory name,
// an expected language needs its input file (and its output file exactly when
// output is true), and a language without an expectation must have no output
// file. It mirrors the semantic checks of the JS validator.
func (c *Case) CheckFixtureDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("case %q: %w", c.ID, err)
	}
	if parent := filepath.Base(filepath.Dir(abs)); parent != c.Rule {
		return fmt.Errorf("case %q: rule %q does not match its directory name %q", c.ID, c.Rule, parent)
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(abs, filepath.FromSlash(rel)))
		return err == nil
	}
	for _, lang := range Languages {
		in, out := langFiles[lang][0], langFiles[lang][1]
		hasOut := exists(out)
		exp, ok := c.Expect[lang]
		if !ok {
			if hasOut {
				return fmt.Errorf("case %q: %s exists but case.yaml has no expect.%s", c.ID, out, lang)
			}
			continue
		}
		if !exists(in) {
			return fmt.Errorf("case %q: expect.%s requires %s, which does not exist", c.ID, lang, in)
		}
		if exp.Output && !hasOut {
			return fmt.Errorf("case %q: expect.%s.output is true but %s does not exist", c.ID, lang, out)
		}
		if !exp.Output && hasOut {
			return fmt.Errorf("case %q: expect.%s.output is false but %s exists", c.ID, lang, out)
		}
		if exp.Output {
			a, errA := os.ReadFile(filepath.Join(abs, filepath.FromSlash(in)))
			b, errB := os.ReadFile(filepath.Join(abs, filepath.FromSlash(out)))
			if errA != nil || errB != nil {
				return fmt.Errorf("case %q: read %s/%s: %w", c.ID, in, out, errors.Join(errA, errB))
			}
			if bytes.Equal(a, b) {
				return fmt.Errorf("case %q: expect.%s.output is true but %s is identical to %s", c.ID, lang, out, in)
			}
		}
	}
	return nil
}
