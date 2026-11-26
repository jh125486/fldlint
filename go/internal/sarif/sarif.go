// Package sarif converts the output of `fldlint -json` (the go/analysis JSON
// tree) into SARIF 2.1.0 as defined by spec/contracts/sarif.md.
//
// The analysis JSON carries byte columns, no flag key and no occurrence
// number, so Convert re-reads each reported source file: it converts the byte
// column to the UTF-16 column of the contract, recovers the static flag key
// from the call's key argument, and numbers duplicates per file
// (spec/contracts/fingerprint.md).
package sarif

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jh125486/fldlint/go/internal/fingerprint"
	"github.com/jh125486/fldlint/go/internal/position"
	"github.com/jh125486/fldlint/go/internal/spec"
)

// Schema is the SARIF 2.1.0 JSON schema URL ($schema).
const Schema = "https://json.schemastore.org/sarif-2.1.0.json"

// ErrInput wraps every failure to read or understand the input: malformed
// `fldlint -json` text, an analysis error reported inside it, a source file
// that cannot be read or parsed, or a path outside the scan root.
var ErrInput = errors.New("invalid input")

func inputErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInput, fmt.Sprintf(format, args...))
}

// Options configures Convert.
type Options struct {
	// Root is the scan root; result paths are relative to it. Empty means the
	// current directory.
	Root string
	// Version is tool.driver.version. Empty means "0.0.0".
	Version string
	// Level maps a rule id to a SARIF level ("error", "warning", "note"); the
	// empty key is the default for rules not listed. A missing default is
	// "warning" (the rules.yaml recommended severity).
	Level map[string]string
	// Baseline, when non-nil, sets baselineState on every result.
	Baseline map[string]struct{}
	// ReadFile reads a source file; nil means os.ReadFile.
	ReadFile func(string) ([]byte, error)
}

// diag is one analysis diagnostic of the go/analysis -json tree.
type diag struct {
	Category string `json:"category"`
	Posn     string `json:"posn"`
	End      string `json:"end"`
	Message  string `json:"message"`
}

var (
	posnRE   = regexp.MustCompile(`^(.*):(\d+):(\d+)$`)
	suffixRE = regexp.MustCompile(`^(.*) \[(?:skip: ([a-z0-9-]+)|(no-fix: comment))\]$`)
)

// Output structs; field order is the emitted key order.
type (
	logDoc struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    [1]run `json:"runs"`
	}
	run struct {
		Tool       tool     `json:"tool"`
		ColumnKind string   `json:"columnKind"`
		Results    []result `json:"results"`
	}
	tool struct {
		Driver driver `json:"driver"`
	}
	driver struct {
		Name    string   `json:"name"`
		Version string   `json:"version"`
		Rules   []ruleID `json:"rules"`
	}
	ruleID struct {
		ID string `json:"id"`
	}
	result struct {
		RuleID              string            `json:"ruleId"`
		Level               string            `json:"level"`
		Message             text              `json:"message"`
		Locations           [1]location       `json:"locations"`
		PartialFingerprints map[string]string `json:"partialFingerprints"`
		BaselineState       string            `json:"baselineState,omitempty"`
		Properties          properties        `json:"properties"`
	}
	text struct {
		Text string `json:"text"`
	}
	location struct {
		PhysicalLocation physicalLocation `json:"physicalLocation"`
	}
	physicalLocation struct {
		ArtifactLocation artifactLocation `json:"artifactLocation"`
		Region           region           `json:"region"`
	}
	artifactLocation struct {
		URI       string `json:"uri"`
		URIBaseID string `json:"uriBaseId"`
	}
	region struct {
		StartLine   int `json:"startLine"`
		StartColumn int `json:"startColumn"`
	}
	properties struct {
		Risk  string `json:"risk"`
		Skip  string `json:"skip,omitempty"`
		NoFix string `json:"noFix,omitempty"`
	}
)

// pending is a result before occurrence numbers are known.
type pending struct {
	res  result
	path string
	key  *string
	line int
	col  int
}

// Convert reads `fldlint -json` text from r and returns the SARIF document:
// indented by two spaces, ending in one newline. Failures wrap [ErrInput].
func Convert(r io.Reader, o Options) ([]byte, error) {
	diags, err := readDiags(r)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(cmp.Or(o.Root, "."))
	if err != nil {
		return nil, inputErr("scan root: %v", err)
	}
	read := o.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	rules := map[string]spec.Rule{}
	for _, ru := range spec.Rules() {
		rules[ru.ID] = ru
	}

	cache := map[string]*source{}
	var items []*pending
	seen := map[diag]bool{}
	for _, d := range diags {
		if seen[d] { // the same file is analysed once per package variant
			continue
		}
		seen[d] = true
		p, err := build(d, root, o, rules, cache, read)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	slices.SortStableFunc(items, func(a, b *pending) int {
		return cmp.Or(strings.Compare(a.path, b.path), cmp.Compare(a.line, b.line),
			cmp.Compare(a.col, b.col), strings.Compare(a.res.RuleID, b.res.RuleID))
	})
	assignFingerprints(items, o.Baseline)

	used := map[string]bool{}
	doc := logDoc{Schema: Schema, Version: "2.1.0"}
	doc.Runs[0] = run{
		Tool:       tool{Driver: driver{Name: "fldlint", Version: cmp.Or(o.Version, "0.0.0"), Rules: []ruleID{}}},
		ColumnKind: "utf16CodeUnits",
		Results:    []result{},
	}
	for _, it := range items {
		doc.Runs[0].Results = append(doc.Runs[0].Results, it.res)
		used[it.res.RuleID] = true
	}
	for _, id := range slices.Sorted(maps.Keys(used)) {
		doc.Runs[0].Tool.Driver.Rules = append(doc.Runs[0].Tool.Driver.Rules, ruleID{id})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// readDiags decodes {pkg: {analyzer: [diag...] | {"error": "..."}}} and
// returns the fldlint diagnostics in a deterministic order.
func readDiags(r io.Reader) ([]diag, error) {
	dec := json.NewDecoder(r)
	var tree map[string]map[string]json.RawMessage
	if err := dec.Decode(&tree); err != nil {
		return nil, inputErr("reading fldlint -json: %v", err)
	}
	if tree == nil {
		return nil, inputErr("reading fldlint -json: expected a JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, inputErr("reading fldlint -json: trailing data after the JSON object")
	}
	var out []diag
	for _, pkg := range slices.Sorted(maps.Keys(tree)) {
		for _, name := range slices.Sorted(maps.Keys(tree[pkg])) {
			raw := bytes.TrimSpace(tree[pkg][name])
			if len(raw) > 0 && raw[0] == '{' {
				var e struct {
					Error string `json:"error"`
				}
				_ = json.Unmarshal(raw, &e)
				return nil, inputErr("analysis of %s failed (%s): %s", pkg, name, e.Error)
			}
			if name != "fldlint" {
				continue
			}
			var ds []diag
			if err := json.Unmarshal(raw, &ds); err != nil {
				return nil, inputErr("package %s: %v", pkg, err)
			}
			out = append(out, ds...)
		}
	}
	return out, nil
}

// source is a parsed source file, shared by every diagnostic in it.
type source struct {
	src  []byte
	fset *token.FileSet
	ast  *ast.File
}

func load(path string, cache map[string]*source, read func(string) ([]byte, error)) (*source, error) {
	if s, ok := cache[path]; ok {
		return s, nil
	}
	b, err := read(path)
	if err != nil {
		return nil, inputErr("reading source: %v", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, b, parser.SkipObjectResolution)
	if err != nil {
		return nil, inputErr("parsing %s: %v", path, err)
	}
	s := &source{src: b, fset: fset, ast: f}
	cache[path] = s
	return s, nil
}

// splitPosn splits "file:line:col" from the right, so a drive letter or a
// colon in the path stays in the file part.
func splitPosn(posn string) (file string, line, col int, err error) {
	m := posnRE.FindStringSubmatch(posn)
	if m == nil {
		return "", 0, 0, inputErr("malformed position %q", posn)
	}
	line, _ = strconv.Atoi(m[2])
	col, _ = strconv.Atoi(m[3])
	if line < 1 || col < 1 {
		return "", 0, 0, inputErr("malformed position %q", posn)
	}
	return m[1], line, col, nil
}

// offsetOf is the byte offset of a 1-based line and byte column in src.
func offsetOf(src []byte, line, col int) (int, bool) {
	off := 0
	for l := 1; l < line; l++ {
		i := bytes.IndexByte(src[off:], '\n')
		if i < 0 {
			return 0, false
		}
		off += i + 1
	}
	off += col - 1
	return off, off <= len(src)
}

func build(d diag, root string, o Options, rules map[string]spec.Rule, cache map[string]*source, read func(string) ([]byte, error)) (*pending, error) {
	rule, found := rules[d.Category]
	if !found {
		return nil, inputErr("unknown rule id %q in diagnostic at %s", d.Category, d.Posn)
	}
	file, line, col, err := splitPosn(d.Posn)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, inputErr("%v", err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, inputErr("%s is outside the scan root %s (use -root)", file, root)
	}
	s, err := load(abs, cache, read)
	if err != nil {
		return nil, err
	}
	off, inFile := offsetOf(s.src, line, col)
	if !inFile {
		return nil, inputErr("position %s is outside the file", d.Posn)
	}
	endOff := -1
	if d.End != "" {
		if ef, el, ec, err := splitPosn(d.End); err == nil && ef == file {
			if eo, ok := offsetOf(s.src, el, ec); ok {
				endOff = eo
			}
		}
	}

	msg, skip, noFix := d.Message, "", ""
	if m := suffixRE.FindStringSubmatch(d.Message); m != nil {
		msg, skip = m[1], m[2]
		if m[3] != "" {
			noFix = "comment"
		}
	}
	p := &pending{
		path: fingerprint.NormalizePath(filepath.ToSlash(rel)),
		key:  staticKey(s, rule, off, endOff),
		line: line,
		col:  position.UTF16Col(s.src, off),
	}
	p.res = result{
		RuleID:  d.Category,
		Level:   cmp.Or(o.Level[d.Category], o.Level[""], "warning"),
		Message: text{msg},
		Locations: [1]location{{PhysicalLocation: physicalLocation{
			ArtifactLocation: artifactLocation{URI: p.path, URIBaseID: "%SRCROOT%"},
			Region:           region{StartLine: line, StartColumn: p.col},
		}}},
		Properties: properties{Risk: rule.Risk, Skip: skip, NoFix: noFix},
	}
	return p, nil
}

// staticKey returns the decoded flag key of the call that starts at off (and
// ends at endOff when known), or nil when the key argument is not a string
// literal, the call spreads a slice, or no such call is found. It mirrors the
// analyzer's own rule: only a string literal counts, never a constant.
func staticKey(s *source, rule spec.Rule, off, endOff int) *string {
	if rule.KeyArg < 0 || rule.Go == nil {
		return nil
	}
	var key *string
	ast.Inspect(s.ast, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || key != nil {
			return key == nil
		}
		tf := s.fset.File(call.Pos())
		if tf.Offset(call.Pos()) != off || (endOff >= 0 && tf.Offset(call.End()) != endOff) {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || call.Ellipsis.IsValid() {
			return true
		}
		i := rule.KeyArg
		if at, ok := rule.Go.CtxArg[sel.Sel.Name]; ok && i >= at {
			i++
		}
		if i >= len(call.Args) {
			return true
		}
		if lit, ok := call.Args[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if v, err := strconv.Unquote(lit.Value); err == nil {
				key = fingerprint.StaticKey(&v)
			}
		}
		return true
	})
	return key
}

// assignFingerprints numbers duplicates per file and sets each result's
// fingerprint and baselineState. items is sorted by (path, line, col).
func assignFingerprints(items []*pending, baseline map[string]struct{}) {
	for start := 0; start < len(items); {
		end := start
		for end < len(items) && items[end].path == items[start].path {
			end++
		}
		group := items[start:end]
		pos := make([]fingerprint.Positioned, len(group))
		for i, it := range group {
			pos[i] = fingerprint.Positioned{RuleID: it.res.RuleID, Key: it.key, Line: it.line, Col: it.col}
		}
		for i, occ := range fingerprint.AssignOccurrences(pos) {
			it := group[i]
			fp := fingerprint.Fingerprint(fingerprint.Input{RuleID: it.res.RuleID, Path: it.path, Key: it.key, Occ: occ})
			it.res.PartialFingerprints = map[string]string{"fldlint/fingerprint": fp}
			if baseline != nil {
				it.res.BaselineState = "new"
				if _, ok := baseline[fp]; ok {
					it.res.BaselineState = "unchanged"
				}
			}
		}
		start = end
	}
}
