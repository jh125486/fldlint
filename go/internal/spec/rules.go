// Package spec loads the language-neutral fldlint spec (rules.yaml and
// fixture case.yaml files) and exposes the generated Go rules table.
package spec

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
)

// SkipCategory describes a report-only category.
type SkipCategory struct {
	Severity string
	Message  string
}

// Binding is the per-language part of a rule.
type Binding struct {
	Methods  []string
	CtxArg   map[string]int
	Template string
	Requires []string
	Async    bool
}

// GoBinding is the Go-specific part of a rule.
type GoBinding = Binding

// Rule is one entry of rules.yaml. Argument indexes are -1 when absent.
// Langs holds every language binding when the rule comes from ParseRules; the
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

// RulesFile is the parsed form of rules.yaml.
type RulesFile struct {
	Version        int
	SkipCategories map[string]SkipCategory
	Rules          []Rule
}

// Placeholders is the closed set of js/go template placeholders.
var Placeholders = []string{"client", "key", "default", "context", "goctx", "evalctx"}

// ParseRules strictly parses and validates rules.yaml content. It accepts and
// rejects the same inputs as spec/schema/rules.schema.json plus the semantic
// checks the schema cannot express.
func ParseRules(data []byte) (*RulesFile, error) {
	root, err := decodeDoc(data)
	if err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	top, err := newMapping(root, "rules.yaml", []string{"version", "skipCategories", "rules"})
	if err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	rf := &RulesFile{SkipCategories: map[string]SkipCategory{}}
	if v, ok := top.m["version"]; ok {
		if rf.Version, err = asInt(v, top.child("version"), 0); err != nil {
			return nil, fmt.Errorf("parse rules: %w", err)
		}
		if rf.Version != 1 {
			return nil, fmt.Errorf("rules: unsupported version %d", rf.Version)
		}
	}
	if err := top.require("version", "skipCategories", "rules"); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	if err := parseSkipCategories(top.m["skipCategories"], top.child("skipCategories"), rf); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	items, err := asSeq(top.m["rules"], top.child("rules"))
	if err == nil && len(items) == 0 {
		err = failAt(top.m["rules"], top.child("rules"), "needs at least 1 rule")
	}
	if err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	for i, it := range items {
		r, err := parseRule(it, fmt.Sprintf("rules[%d]", i))
		if err != nil {
			return nil, fmt.Errorf("parse rules: %w", err)
		}
		rf.Rules = append(rf.Rules, r)
	}
	if err := rf.Validate(); err != nil {
		return nil, err
	}
	return rf, nil
}

func parseSkipCategories(n *yaml.Node, path string, rf *RulesFile) error {
	mp, err := newMapping(n, path, SkipNames)
	if err != nil {
		return err
	}
	if err := mp.require(SkipNames...); err != nil {
		return err
	}
	for _, name := range SkipNames {
		cp := mp.child(name)
		cm, err := newMapping(mp.m[name], cp, []string{"severity", "message"})
		if err != nil {
			return err
		}
		if err := cm.require("severity", "message"); err != nil {
			return err
		}
		sev, err := asEnum(cm.m["severity"], cm.child("severity"), severities)
		if err != nil {
			return err
		}
		msg, err := asString(cm.m["message"], cm.child("message"))
		if err != nil {
			return err
		}
		if msg == "" {
			return failAt(cm.m["message"], cm.child("message"), "must not be empty")
		}
		rf.SkipCategories[name] = SkipCategory{Severity: sev, Message: msg}
	}
	return nil
}

func parseRule(n *yaml.Node, path string) (Rule, error) {
	r := Rule{KeyArg: -1, DefaultArg: -1, ContextArg: -1, Langs: map[string]*Binding{}}
	mp, err := newMapping(n, path, []string{"id", "valueType", "risk", "keyArg", "defaultArg", "contextArg", "skip", "message", "languages"})
	if err != nil {
		return r, err
	}
	if err := mp.require("id", "risk", "languages"); err != nil {
		return r, err
	}
	if r.ID, err = asString(mp.m["id"], mp.child("id")); err != nil {
		return r, err
	}
	if !ruleIDRe.MatchString(r.ID) {
		return r, failAt(mp.m["id"], mp.child("id"), "%q does not match the rule id pattern", r.ID)
	}
	path = fmt.Sprintf("%s(%s)", path, r.ID)
	if r.Risk, err = asEnum(mp.m["risk"], path+".risk", risks); err != nil {
		return r, err
	}
	if v, ok := mp.m["valueType"]; ok {
		if r.ValueType, err = asEnum(v, path+".valueType", valueTypes); err != nil {
			return r, err
		}
	}
	for _, a := range []struct {
		key string
		dst *int
	}{{"keyArg", &r.KeyArg}, {"defaultArg", &r.DefaultArg}, {"contextArg", &r.ContextArg}} {
		if v, ok := mp.m[a.key]; ok {
			if *a.dst, err = asInt(v, path+"."+a.key, 0); err != nil {
				return r, err
			}
		}
	}
	if v, ok := mp.m["skip"]; ok {
		if r.Skip, err = asEnum(v, path+".skip", SkipNames); err != nil {
			return r, err
		}
	}
	if v, ok := mp.m["message"]; ok {
		if r.Message, err = asString(v, path+".message"); err != nil {
			return r, err
		}
		if r.Message == "" || !messageRe.MatchString(r.Message) {
			return r, failAt(v, path+".message", "must be non-empty and use only the {{key}} placeholder")
		}
	}
	lm, err := newMapping(mp.m["languages"], path+".languages", langKeys)
	if err != nil {
		return r, err
	}
	if len(lm.keys) == 0 {
		return r, failAt(lm.node, lm.path, "needs at least 1 language")
	}
	for _, lang := range lm.keys {
		b, err := parseBinding(lm.m[lang], lm.child(lang), lang)
		if err != nil {
			return r, err
		}
		r.Langs[lang] = b
	}
	r.Go = r.Langs["go"]
	return r, nil
}

func parseBinding(n *yaml.Node, path, lang string) (*Binding, error) {
	mp, err := newMapping(n, path, langBindKeys)
	if err != nil {
		return nil, err
	}
	if err := mp.require("methods"); err != nil {
		return nil, err
	}
	b := &Binding{}
	nonEmpty := func(s string) error {
		if s == "" {
			return errors.New("must not be empty")
		}
		return nil
	}
	if b.Methods, err = asStringSeq(mp.m["methods"], mp.child("methods"), 1, nonEmpty); err != nil {
		return nil, err
	}
	if v, ok := mp.m["async"]; ok {
		if b.Async, err = asBool(v, mp.child("async")); err != nil {
			return nil, err
		}
	}
	if v, ok := mp.m["ctxArg"]; ok {
		cm, err := newMapping(v, mp.child("ctxArg"), nil)
		if err != nil {
			return nil, err
		}
		b.CtxArg = map[string]int{}
		for _, k := range cm.keys {
			if b.CtxArg[k], err = asInt(cm.m[k], cm.child(k), 0); err != nil {
				return nil, err
			}
		}
	}
	if v, ok := mp.m["requires"]; ok {
		known := func(s string) error {
			if !slices.Contains(SkipNames, s) {
				return fmt.Errorf("%q is not a skip category", s)
			}
			return nil
		}
		if b.Requires, err = asStringSeq(v, mp.child("requires"), 0, known); err != nil {
			return nil, err
		}
	}
	if v, ok := mp.m["template"]; ok {
		if b.Template, err = asString(v, mp.child("template")); err != nil {
			return nil, err
		}
		re, what := tmplJSGoRe, "{{client}} {{key}} {{default}} {{context}} {{goctx}} {{evalctx}}"
		if lang == "java" {
			re, what = tmplJavaRe, "#{key} #{default} #{context} #{any(Type)}"
		}
		if b.Template == "" || !re.MatchString(b.Template) {
			return nil, failAt(v, mp.child("template"), "template must be non-empty and use only the placeholders %s", what)
		}
	}
	return b, nil
}

// LoadRules reads, parses and validates a rules.yaml file.
func LoadRules(path string) (*RulesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	return ParseRules(data)
}

// Validate checks the semantic invariants the schema cannot express.
func (rf *RulesFile) Validate() error {
	if rf.Version != 1 {
		return fmt.Errorf("rules: unsupported version %d", rf.Version)
	}
	ids := map[string]bool{}
	methods := map[string]map[string]string{} // lang -> method -> rule id
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
			if r.Message != "" {
				return fmt.Errorf("rule %q: skip rules must not carry a message (the skipCategories message is used)", r.ID)
			}
		} else {
			for name, v := range map[string]int{"keyArg": r.KeyArg, "defaultArg": r.DefaultArg, "contextArg": r.ContextArg} {
				if v < 0 {
					return fmt.Errorf("rule %q: rewriting rule requires %s", r.ID, name)
				}
			}
			if r.ValueType == "" {
				return fmt.Errorf("rule %q: rewriting rule requires valueType", r.ID)
			}
			if r.Message == "" {
				return fmt.Errorf("rule %q: rewriting rule requires message", r.ID)
			}
		}
		langs := r.Langs
		if langs == nil && r.Go != nil {
			langs = map[string]*Binding{"go": r.Go}
		}
		for _, lang := range langKeys {
			g := langs[lang]
			if g == nil {
				continue
			}
			if len(g.Methods) == 0 {
				return fmt.Errorf("rule %q: %s binding has no methods", r.ID, lang)
			}
			if methods[lang] == nil {
				methods[lang] = map[string]string{}
			}
			for _, m := range g.Methods {
				if prev, dup := methods[lang][m]; dup {
					return fmt.Errorf("rule %q: %s method %q already bound by rule %q", r.ID, lang, m, prev)
				}
				methods[lang][m] = r.ID
			}
			for m := range g.CtxArg {
				if !slices.Contains(g.Methods, m) {
					return fmt.Errorf("rule %q: %s ctxArg for unlisted method %q", r.ID, lang, m)
				}
			}
			for _, c := range g.Requires {
				if _, ok := rf.SkipCategories[c]; !ok {
					return fmt.Errorf("rule %q: requires unknown category %q", r.ID, c)
				}
			}
			switch {
			case r.Skip != "" && g.Template != "":
				return fmt.Errorf("rule %q: skip and %s template are mutually exclusive", r.ID, lang)
			case r.Skip == "" && g.Template == "":
				return fmt.Errorf("rule %q: %s binding needs a template or the rule needs skip", r.ID, lang)
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
