package spec

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// ParseYAML parses data as one document of the fldlint YAML subset and returns
// it as plain Go values: map[string]any, []any, string, bool, int64 and nil.
// A bare "---" returns (nil, nil). It is the test-side view of decodeDoc,
// which the loaders use to read the document.
func ParseYAML(data []byte) (any, error) {
	root, err := decodeDoc(data)
	if err != nil {
		return nil, err
	}
	return toValue(root), nil
}

// toValue converts a node that passed checkSubset.
func toValue(n *yaml.Node) any {
	switch n.Kind {
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = toValue(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		s := make([]any, len(n.Content))
		for i, c := range n.Content {
			s[i] = toValue(c)
		}
		return s
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
	}
	switch s := scalarOf(n); s.kind {
	case kindBool:
		return s.text == "true"
	case kindInt:
		v, _ := strconv.ParseInt(s.text, 10, 64)
		return v
	case kindNull:
		return nil
	case kindString, kindAmbiguous, kindOutOfRange:
	}
	return n.Value
}

// CheckSubset exposes checkSubset so that node trees yaml.v3 cannot produce
// (an alias that is not preceded by an anchor) can be tested.
func CheckSubset(data []byte, doc *yaml.Node) error { return checkSubset(data, doc) }

// SourceRest exposes checker.rest so that positions outside the source, which
// yaml.v3 never reports, can be tested.
func SourceRest(data []byte, line, col int) []byte {
	return newChecker(data).rest(line, col)
}
