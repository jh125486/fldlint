package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadedCase is one fixture case with its location below spec/fixtures.
type LoadedCase struct {
	// Path is "<rule>/<case>", the case directory relative to fixtures/.
	Path string
	Case *Case
}

// Dir returns the case directory below the fixtures root.
func (lc LoadedCase) Dir(fixtures string) string {
	return filepath.Join(fixtures, filepath.FromSlash(lc.Path))
}

// Spec is the decoded spec/generated/spec.json: the rules and every case.
type Spec struct {
	Rules *RulesFile
	Cases []LoadedCase
}

type wireSpec struct {
	Rules json.RawMessage `json:"rules"`
	Cases []struct {
		Path string          `json:"path"`
		Case json.RawMessage `json:"case"`
	} `json:"cases"`
}

// LoadSpec reads and decodes a spec.json file.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is the caller-supplied spec.json, by design
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	s, err := ParseSpec(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// ParseSpec decodes spec.json content. Content validation lives in the JS
// loader (see ADR 011); beyond strict decoding this checks only the
// references Go itself relies on: every case path is "<rule>/<case>", is
// unique, and names a rule that exists and equals the case's own rule.
func ParseSpec(data []byte) (*Spec, error) {
	var w wireSpec
	if err := decodeStrict(data, &w); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	if w.Rules == nil {
		return nil, fmt.Errorf("parse spec: missing \"rules\"")
	}
	if w.Cases == nil {
		return nil, fmt.Errorf("parse spec: missing \"cases\"")
	}
	rf, err := ParseRules(w.Rules)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, r := range rf.Rules {
		known[r.ID] = true
	}
	s := &Spec{Rules: rf}
	seen := map[string]bool{}
	for _, wc := range w.Cases {
		rule, name, ok := strings.Cut(wc.Path, "/")
		if !ok || rule == "" || name == "" || strings.Contains(name, "/") {
			return nil, fmt.Errorf("parse spec: case path %q is not <rule>/<case>", wc.Path)
		}
		if seen[wc.Path] {
			return nil, fmt.Errorf("parse spec: duplicate case path %q", wc.Path)
		}
		seen[wc.Path] = true
		c, err := ParseCase(wc.Case)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", wc.Path, err)
		}
		if c.Rule != rule || !known[rule] {
			return nil, fmt.Errorf("case %s: rule %q must be a known rule equal to the directory %q", wc.Path, c.Rule, rule)
		}
		s.Cases = append(s.Cases, LoadedCase{Path: wc.Path, Case: c})
	}
	return s, nil
}
