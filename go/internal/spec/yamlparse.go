package spec

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// ParseYAML parses data as one document of the fldlint YAML subset (see
// spec/contracts/validation-parity.md) and returns it as plain Go values:
// map[string]any, []any, string, bool, int64 (within +-9007199254740991) and
// nil. A bare "---" is a document and returns (nil, nil). Subset violations
// are returned as a *SubsetError; input that is not well-formed YAML is
// rejected with the error of yaml.v3.
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
