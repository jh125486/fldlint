package fingerprint_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jh125486/fldlint/go/internal/fingerprint"
)

const contracts = "../../../spec/contracts"

func readJSON(t *testing.T, name string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(contracts, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// TestFingerprint runs the shared vectors, so the Go engine agrees with JS.
func TestFingerprint(t *testing.T) {
	t.Parallel()
	var shared []struct {
		RuleID      string  `json:"ruleId"`
		Path        string  `json:"path"`
		Key         *string `json:"key"`
		Occ         int     `json:"occ"`
		Fingerprint string  `json:"fingerprint"`
	}
	readJSON(t, "fingerprint-vectors.json", &shared)
	var illformed []struct {
		RuleID      string `json:"ruleId"`
		Path        string `json:"path"`
		KeyUtf8Hex  string `json:"keyUtf8Hex"`
		Occ         int    `json:"occ"`
		Fingerprint string `json:"fingerprint"`
	}
	readJSON(t, "fingerprint-illformed-vectors.json", &illformed)
	if len(shared) < 30 || len(illformed) == 0 {
		t.Fatalf("vector files look truncated: %d shared, %d ill-formed", len(shared), len(illformed))
	}

	type tc struct {
		name string
		in   fingerprint.Input
		want string
	}
	var tests []tc
	for _, v := range shared {
		tests = append(tests, tc{v.RuleID + " " + v.Path + " " + v.Fingerprint, fingerprint.Input{RuleID: v.RuleID, Path: v.Path, Key: v.Key, Occ: v.Occ}, v.Fingerprint})
	}
	for _, v := range illformed {
		b, err := hex.DecodeString(v.KeyUtf8Hex)
		if err != nil {
			t.Fatal(err)
		}
		key := string(b)
		tests = append(tests, tc{"ill-formed " + v.KeyUtf8Hex, fingerprint.Input{RuleID: v.RuleID, Path: v.Path, Key: &key, Occ: v.Occ}, v.Fingerprint})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fingerprint.Fingerprint(tt.in); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"a//b", "a/b"},
		{"a/./b", "a/b"},
		{"a/././b", "a/b"},
		{"././a", "a"},
		{"a/b/", "a/b"},
		{`.\a\b`, "a/b"},
		{"src/ünï/a.ts", "src/ünï/a.ts"},
		{"A/b", "A/b"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := fingerprint.NormalizePath(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStaticKey(t *testing.T) {
	t.Parallel()
	str := func(s string) *string { return &s }
	tests := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil", nil, nil},
		{"empty", str(""), str("")},
		{"valid", str("clé 😀"), str("clé 😀")},
		{"invalid byte", str("a\xff"), nil},
		{"encoded surrogate", str("a\xed\xa0\x80"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fingerprint.StaticKey(tt.in)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAssignOccurrences(t *testing.T) {
	t.Parallel()
	str := func(s string) *string { return &s }
	tests := []struct {
		name string
		in   []fingerprint.Positioned
		want []int
	}{
		{"empty", nil, []int{}},
		{"orders by line then col, preserving input alignment", []fingerprint.Positioned{
			{RuleID: "r", Key: str("k"), Line: 5, Col: 1},
			{RuleID: "r", Key: str("k"), Line: 2, Col: 9},
			{RuleID: "r", Key: str("k"), Line: 2, Col: 3},
		}, []int{2, 1, 0}},
		{"groups by rule and key", []fingerprint.Positioned{
			{RuleID: "r", Key: str("a"), Line: 1, Col: 1},
			{RuleID: "r", Key: str("b"), Line: 2, Col: 1},
			{RuleID: "s", Key: str("a"), Line: 3, Col: 1},
			{RuleID: "r", Key: str("a"), Line: 4, Col: 1},
		}, []int{0, 0, 0, 1}},
		{"ill-formed and absent keys share the keyless group", []fingerprint.Positioned{
			{RuleID: "r", Key: nil, Line: 1, Col: 1},
			{RuleID: "r", Key: str("a\xff"), Line: 2, Col: 1},
		}, []int{0, 1}},
		{"empty key differs from absent", []fingerprint.Positioned{
			{RuleID: "r", Key: str(""), Line: 1, Col: 1},
			{RuleID: "r", Key: nil, Line: 2, Col: 1},
		}, []int{0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fingerprint.AssignOccurrences(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
