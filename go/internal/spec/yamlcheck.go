package spec

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// blockHeaderDigit matches a block scalar header with an indentation indicator.
var blockHeaderDigit = regexp.MustCompile(`^[|>][-+]?[0-9]`)

// checker enforces the "Document level" rules of the contract on the node
// tree of one document and sets the Tag of every scalar to the tag of its
// classification, so that the type checks of the loaders (which read Tag)
// cannot disagree with the subset: yaml.v3 types some texts the subset calls
// strings, such as 0e0_ and 0o-1, as numbers.
type checker struct {
	src    []byte // the input without its BOM, which yaml.v3 does not count
	starts []int  // byte offset of the start of each line of src

	// The last position resolved: nodes are visited in document order, so
	// lookups on one very long line stay linear.
	cacheLine, cacheCol, cacheOff int
}

func newChecker(data []byte) *checker {
	src := bytes.TrimPrefix(data, utf8BOM)
	c := &checker{src: src, starts: []int{0}}
	for i, b := range src {
		if b == '\n' {
			c.starts = append(c.starts, i+1)
		}
	}
	return c
}

// checkSubset checks the single document doc parsed from data.
func checkSubset(data []byte, doc *yaml.Node) error {
	c := newChecker(data)
	for _, n := range doc.Content {
		if err := c.node(n, false, 0); err != nil {
			return err
		}
	}
	return nil
}

// rest returns the source from a 1-based line and rune column on, or nil when
// the position is outside the source.
func (c *checker) rest(line, col int) []byte {
	if line < 1 || line > len(c.starts) {
		return nil
	}
	i, off := 1, c.starts[line-1]
	if c.cacheLine == line && c.cacheCol <= col {
		i, off = c.cacheCol, c.cacheOff
	}
	for ; i < col && off < len(c.src); i++ {
		_, size := utf8.DecodeRune(c.src[off:])
		off += size
	}
	c.cacheLine, c.cacheCol, c.cacheOff = line, i, off
	return c.src[off:]
}

func (c *checker) fail(n *yaml.Node, reason error, detail string) error {
	return newSubsetError(n.Line, n.Column, reason, detail)
}

func (c *checker) node(n *yaml.Node, inFlow bool, depth int) error {
	if err := c.properties(n); err != nil {
		return err
	}
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		if depth >= MaxDepth {
			return c.fail(n, ErrTooDeep, fmt.Sprintf("limit is %d levels", MaxDepth))
		}
		inFlow = inFlow || n.Style&yaml.FlowStyle != 0
		if n.Kind == yaml.MappingNode {
			return c.mapping(n, inFlow, depth+1)
		}
		return c.sequence(n, inFlow, depth+1)
	}
	s, err := c.scalar(n, inFlow)
	if err != nil {
		return err
	}
	n.Tag = s.tag()
	return nil
}

// properties rejects aliases, anchors and explicit tags on any node.
func (c *checker) properties(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode {
		return c.fail(n, ErrAlias, "*"+n.Value)
	}
	if n.Anchor != "" {
		return c.fail(n, ErrAnchor, "&"+n.Anchor)
	}
	// yaml.v3 reports the position of a node at the start of its properties,
	// and only a tag can start a node with "!" (a plain scalar cannot, and the
	// other node styles start with another character). This also finds the
	// non-specific tag "!", which yaml.v3 drops from the node.
	if bytes.HasPrefix(c.rest(n.Line, n.Column), []byte("!")) {
		return c.fail(n, ErrTag, "")
	}
	return nil
}

// scalar types the scalar node n and rejects what the subset does not accept.
func (c *checker) scalar(n *yaml.Node, inFlow bool) (scalar, error) {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		rest := c.rest(n.Line, n.Column)
		if blockHeaderDigit.Match(rest[:min(len(rest), 4)]) {
			return scalar{}, c.fail(n, ErrBlockIndicator, "")
		}
	}
	s := scalarOf(n)
	// yaml.v3 reads "[a:]" as the string "a:", where the colon ends a pair in
	// the YAML spec and the JavaScript parser.
	if inFlow && n.Style&plainMask == 0 && strings.HasSuffix(s.text, ":") {
		return s, c.fail(n, ErrFlowScalar, fmt.Sprintf("%q (quote it)", s.text))
	}
	switch s.kind {
	case kindAmbiguous:
		return s, c.fail(n, ErrAmbiguousScalar, fmt.Sprintf("%q (quote it to make it a string)", s.text))
	case kindOutOfRange:
		return s, c.fail(n, ErrIntRange, fmt.Sprintf("%q (limit is +-%s)", s.text, maxSafeInt))
	case kindString, kindBool, kindInt, kindNull:
	}
	return s, nil
}

func (c *checker) sequence(n *yaml.Node, inFlow bool, depth int) error {
	for _, item := range n.Content {
		if err := c.node(item, inFlow, depth); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) mapping(n *yaml.Node, inFlow bool, depth int) error {
	seen := make(map[string]bool, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		if err := c.key(n.Content[i], seen, inFlow); err != nil {
			return err
		}
		if err := c.node(n.Content[i+1], inFlow, depth); err != nil {
			return err
		}
	}
	return nil
}

// key checks a mapping key and records it in seen.
func (c *checker) key(k *yaml.Node, seen map[string]bool, inFlow bool) error {
	if err := c.properties(k); err != nil {
		return err
	}
	if k.Kind != yaml.ScalarNode {
		return c.fail(k, ErrKeyNotScalar, "")
	}
	s, err := c.scalar(k, inFlow)
	if err != nil {
		return err
	}
	if s.kind != kindString {
		return c.fail(k, ErrKeyNotString, fmt.Sprintf("%q is not a string; quote it", k.Value))
	}
	k.Tag = s.tag()
	if seen[s.text] {
		return c.fail(k, ErrDuplicateKey, fmt.Sprintf("%q", s.text))
	}
	seen[s.text] = true
	return nil
}
