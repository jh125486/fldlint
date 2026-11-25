// Package fingerprint computes the stable identity of a finding, as defined
// by spec/contracts/fingerprint.md. It is the Go side of the shared vectors in
// spec/contracts/fingerprint-vectors.json.
package fingerprint

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Input holds the identity inputs of a finding.
type Input struct {
	RuleID string
	// Path is relative to the scan root; Fingerprint normalizes it.
	Path string
	// Key is the decoded value of a static flag key, or nil when there is
	// none. A key that is not valid UTF-8 counts as none.
	Key *string
	// Occ is the 0-based ordinal among same (RuleID, kind, key) findings in
	// the file.
	Occ int
}

var slashRuns = regexp.MustCompile(`/{2,}`)

// NormalizePath replaces backslashes with `/`, collapses `/` runs, strips
// leading `./` (repeatedly) and interior `/./`, and strips trailing `/`. It
// does no case folding or Unicode normalization.
func NormalizePath(p string) string {
	s := slashRuns.ReplaceAllString(strings.ReplaceAll(p, "\\", "/"), "/")
	for strings.HasPrefix(s, "./") {
		s = s[2:]
	}
	for strings.Contains(s, "/./") {
		s = strings.ReplaceAll(s, "/./", "/")
	}
	return strings.TrimRight(s, "/")
}

// StaticKey returns the key as a fingerprint input: the key itself when it
// is valid UTF-8, else nil. An invalid byte sequence would hash as U+FFFD in
// other engines' encoders, so such a key is treated as not static.
func StaticKey(key *string) *string {
	if key == nil || !utf8.ValidString(*key) {
		return nil
	}
	return key
}

// Fingerprint returns 32 lowercase hex chars: SHA-256 of
// `ruleId NUL path NUL kind NUL key NUL occ`, truncated.
func Fingerprint(in Input) string {
	kind, key := "-", ""
	if k := StaticKey(in.Key); k != nil {
		kind, key = "s", *k
	}
	pre := strings.Join([]string{in.RuleID, NormalizePath(in.Path), kind, key, strconv.Itoa(in.Occ)}, "\x00")
	sum := sha256.Sum256([]byte(pre))
	return hex.EncodeToString(sum[:])[:32]
}

// Positioned is a finding's position for [AssignOccurrences]. Col is the
// shared UTF-16 column (position.UTF16Col), not a byte column.
type Positioned struct {
	RuleID string
	Key    *string
	Line   int
	Col    int
}

// AssignOccurrences returns the `occ` of each finding of ONE file, index-aligned
// with the input: the 0-based rank among findings with the same
// (RuleID, kind, key), ordered by (Line, Col). Ties keep input order.
func AssignOccurrences(findings []Positioned) []int {
	groups := map[[3]string][]int{}
	for i, f := range findings {
		id := [3]string{f.RuleID, "-", ""}
		if k := StaticKey(f.Key); k != nil {
			id[1], id[2] = "s", *k
		}
		groups[id] = append(groups[id], i)
	}
	occ := make([]int, len(findings))
	for _, idx := range groups {
		slices.SortStableFunc(idx, func(a, b int) int {
			return cmp.Or(cmp.Compare(findings[a].Line, findings[b].Line), cmp.Compare(findings[a].Col, findings[b].Col))
		})
		for rank, i := range idx {
			occ[i] = rank
		}
	}
	return occ
}
