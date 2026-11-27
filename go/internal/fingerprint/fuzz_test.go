package fingerprint_test

import (
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jh125486/fldlint/go/internal/fingerprint"
)

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

// pathSeeds are the path rows of spec/contracts/fingerprint-vectors.json plus
// awkward shapes (runs of separators, dot segments, mixed slashes, UTF-8).
var pathSeeds = []string{
	"", ".", "./", "/", "//", "a", "a/", "a//b", "a/./b", "a/././b", "././a", "a/b/",
	`.\a\b`, `a\\b`, `.\.\a`, ".//./x", "a/.//", "/./", "/./.", "a/.", "src/ünï/a.ts",
	"src/é.ts", "src/e\u0301.ts", "../a", "a/../b", "\x00", "a\xffb", "C:\\dir\\f.go",
}

// FuzzNormalizePath checks the normalization contract of fingerprint.md:
// normalizing twice equals normalizing once, the result has none of the
// shapes normalization removes, and a fingerprint does not change when its
// path is normalized first (so re-normalization is harmless).
//
// NormalizePath is only claimed for paths relative to the scan root: an
// absolute input such as "/./x" legitimately keeps its leading "/" (the
// contract's "never starts with / or ./" describes scanned, relative paths),
// so the property does not assert that.
// Explore with `go test -run=^$ -fuzz=FuzzNormalizePath -fuzztime=30s ./internal/fingerprint`.
func FuzzNormalizePath(f *testing.F) {
	for _, p := range pathSeeds {
		f.Add(p, "flag", 0)
	}
	f.Add("a.ts", "a\xff", 1)      // ill-formed key
	f.Add("a.ts", "", 2)           // empty key is not absent
	f.Add("a.ts", "\U0001F600", 0) // astral key
	f.Fuzz(func(t *testing.T, path, key string, occ int) {
		n := fingerprint.NormalizePath(path)
		if again := fingerprint.NormalizePath(n); again != n {
			t.Fatalf("NormalizePath not idempotent: %q -> %q -> %q", path, n, again)
		}
		switch {
		case strings.Contains(n, `\`):
			t.Fatalf("backslash left in %q (from %q)", n, path)
		case strings.Contains(n, "//"):
			t.Fatalf("separator run left in %q (from %q)", n, path)
		case strings.Contains(n, "/./"):
			t.Fatalf("interior dot segment left in %q (from %q)", n, path)
		case strings.HasPrefix(n, "./"):
			t.Fatalf("leading ./ left in %q (from %q)", n, path)
		case strings.HasSuffix(n, "/"):
			t.Fatalf("trailing / left in %q (from %q)", n, path)
		}

		occ &= math.MaxInt // occ is a non-negative ordinal (-MinInt would stay negative)
		in := fingerprint.Input{RuleID: "bool-variation", Path: path, Key: &key, Occ: occ}
		got := fingerprint.Fingerprint(in)
		if !hex32.MatchString(got) {
			t.Fatalf("fingerprint %q is not 32 lowercase hex chars", got)
		}
		in.Path = n
		if renorm := fingerprint.Fingerprint(in); renorm != got {
			t.Fatalf("fingerprint changed under re-normalization of %q: %s != %s", path, got, renorm)
		}
		// A key that is not valid UTF-8 is treated as absent.
		if !utf8.ValidString(key) {
			in.Key = nil
			if absent := fingerprint.Fingerprint(in); absent != got {
				t.Fatalf("ill-formed key %q does not equal the absent key: %s != %s", key, got, absent)
			}
		}
	})
}
