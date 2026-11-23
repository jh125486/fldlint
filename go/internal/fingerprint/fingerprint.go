// Package fingerprint computes the stable identity of a finding, as defined
// by spec/contracts/fingerprint.md. It is the Go side of the shared vectors in
// spec/contracts/fingerprint-vectors.json.
package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
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
