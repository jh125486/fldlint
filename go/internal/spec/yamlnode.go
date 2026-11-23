package spec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

// The loaders walk the yaml.Node tree instead of decoding into structs so that
// they behave like the JSON Schema validation used on the JS side: unknown keys,
// wrong scalar types (yaml.v3 happily coerces `yes` to bool, `3.5` to int and
// `5` to string), null entries and duplicate keys are all rejected.

// SkipNames is the closed set of skipCategories keys (mirrors the schemas).
var SkipNames = []string{"detail", "bulk", "dynamic-key", "unbound-client", "type-mismatch", "context-unproven", "default-type-unproven", "receiver-unproven"}

var (
	ruleIDRe     = regexp.MustCompile(`\A[a-z][a-z0-9]*(?:-[a-z0-9]+)*\z`)
	caseIDRe     = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
	tmplJSGoRe   = regexp.MustCompile(`\A(?:[^{}]|\{\{(?:client|key|default|context|goctx|evalctx)\}\})*\z`)
	tmplJavaRe   = regexp.MustCompile(`\A(?:[^#{}]|#\{(?:key|default|context|any\([A-Za-z0-9_.]+\))\})*\z`)
	messageRe    = regexp.MustCompile(`\A(?:[^{}]|\{\{key\}\})*\z`)
	valueTypes   = []string{"boolean", "string", "number", "integer", "object"}
	risks        = []string{"low", "medium", "high"}
	severities   = []string{"error", "warning", "info"}
	langKeys     = []string{"js", "go", "java"}
	langBindKeys = []string{"methods", "async", "ctxArg", "requires", "template"}
)

func failAt(n *yaml.Node, path, format string, a ...any) error {
	return fmt.Errorf("%s (line %d): %s", path, n.Line, fmt.Sprintf(format, a...))
}

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// decodeDoc parses exactly one YAML document and returns its root node.
func decodeDoc(data []byte) (*yaml.Node, error) {
	if err := prescan(data); err != nil {
		return nil, err
	}
	data = withFinalLF(data)
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, err // never io.EOF: prescan found content, and a lone "..." is a yaml.v3 syntax error
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, newSubsetError(extra.Line, extra.Column, ErrMultipleDocuments, "")
	}
	if err := checkSubset(data, &doc); err != nil {
		return nil, err
	}
	return deref(doc.Content[0]), nil
}

type mapping struct {
	path string
	node *yaml.Node
	m    map[string]*yaml.Node
	keys []string
}

// newMapping checks n is a mapping whose keys are unique and all in allowed
// (a nil allowed accepts any key).
func newMapping(n *yaml.Node, path string, allowed []string) (*mapping, error) {
	n = deref(n)
	if n.Kind != yaml.MappingNode {
		if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null" {
			return nil, failAt(n, path, "must not be null")
		}
		return nil, failAt(n, path, "must be a mapping")
	}
	mp := &mapping{path: path, node: n, m: map[string]*yaml.Node{}}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := deref(n.Content[i])
		if k.Kind != yaml.ScalarNode {
			return nil, failAt(k, path, "keys must be scalars")
		}
		if allowed != nil && !slices.Contains(allowed, k.Value) {
			return nil, failAt(k, path, "unknown key %q", k.Value)
		}
		if _, dup := mp.m[k.Value]; dup {
			return nil, failAt(k, path, "duplicate key %q", k.Value)
		}
		mp.m[k.Value] = n.Content[i+1]
		mp.keys = append(mp.keys, k.Value)
	}
	return mp, nil
}

func (mp *mapping) require(keys ...string) error {
	for _, k := range keys {
		if _, ok := mp.m[k]; !ok {
			return failAt(mp.node, mp.path, "missing required key %q", k)
		}
	}
	return nil
}

func (mp *mapping) child(k string) string { return mp.path + "." + k }

func asString(n *yaml.Node, path string) (string, error) {
	n = deref(n)
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!str" {
		return "", failAt(n, path, "must be a string")
	}
	return n.Value, nil
}

func asEnum(n *yaml.Node, path string, allowed []string) (string, error) {
	s, err := asString(n, path)
	if err != nil {
		return "", err
	}
	if !slices.Contains(allowed, s) {
		return "", failAt(n, path, "%q is not one of %v", s, allowed)
	}
	return s, nil
}

func asInt(n *yaml.Node, path string, minimum int) (int, error) {
	n = deref(n)
	var v int
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" || n.Decode(&v) != nil {
		return 0, failAt(n, path, "must be an integer")
	}
	if v < minimum {
		return 0, failAt(n, path, "must be >= %d", minimum)
	}
	return v, nil
}

func asBool(n *yaml.Node, path string) (bool, error) {
	n = deref(n)
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!bool" {
		return false, failAt(n, path, "must be a boolean (true or false)")
	}
	var b bool
	if err := n.Decode(&b); err != nil {
		return false, failAt(n, path, "must be a boolean (true or false)")
	}
	return b, nil
}

func asSeq(n *yaml.Node, path string) ([]*yaml.Node, error) {
	n = deref(n)
	if n.Kind != yaml.SequenceNode {
		if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null" {
			return nil, failAt(n, path, "must not be null")
		}
		return nil, failAt(n, path, "must be a sequence")
	}
	return n.Content, nil
}

// asStringSeq reads a unique sequence of strings; each must satisfy check.
func asStringSeq(n *yaml.Node, path string, minItems int, check func(string) error) ([]string, error) {
	items, err := asSeq(n, path)
	if err != nil {
		return nil, err
	}
	if len(items) < minItems {
		return nil, failAt(n, path, "needs at least %d item(s)", minItems)
	}
	out := make([]string, 0, len(items))
	for i, it := range items {
		p := fmt.Sprintf("%s[%d]", path, i)
		s, err := asString(it, p)
		if err != nil {
			return nil, err
		}
		if err := check(s); err != nil {
			return nil, failAt(it, p, "%v", err)
		}
		if slices.Contains(out, s) {
			return nil, failAt(it, p, "duplicate item %q", s)
		}
		out = append(out, s)
	}
	return out, nil
}
