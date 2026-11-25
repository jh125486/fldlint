package baseline_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/baseline"
)

const (
	fpA = "0c1ab39ac97767156d8a818304b68367"
	fpB = "d1a64654f64fa217171b2841c61090fd"
)

var negative = []struct{ name, text string }{
	{"malformed JSON", "{"},
	{"not an object", "[]"},
	{"null", "null"},
	{"unknown field", `{"version":1,"fingerprints":[],"extra":1}`},
	{"flaglint format", `{"version":"1","createdAt":"2026-01-01","fingerprints":[]}`},
	{"version 2", `{"version":2,"fingerprints":[]}`},
	{"string version", `{"version":"1","fingerprints":[]}`},
	{"missing fingerprints", `{"version":1}`},
	{"missing version", `{"fingerprints":[]}`},
	{"fingerprints not an array", `{"version":1,"fingerprints":"` + fpA + `"}`},
	{"fingerprints null", `{"version":1,"fingerprints":null}`},
	{"non-hex entry", `{"version":1,"fingerprints":["zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"]}`},
	{"uppercase entry", `{"version":1,"fingerprints":["` + strings.ToUpper(fpA) + `"]}`},
	{"short entry", `{"version":1,"fingerprints":["abc"]}`},
	{"non-string entry", `{"version":1,"fingerprints":[1]}`},
}

func TestFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"sorted and deduplicated", []string{fpB, fpA, fpB}, "{\n  \"version\": 1,\n  \"fingerprints\": [\n    \"" + fpA + "\",\n    \"" + fpB + "\"\n  ]\n}\n"},
		{"empty", nil, "{\n  \"version\": 1,\n  \"fingerprints\": []\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := baseline.Format(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("rejects invalid fingerprint", func(t *testing.T) {
		t.Parallel()
		if _, err := baseline.Format([]string{"XYZ"}); !errors.Is(err, baseline.ErrInvalid) {
			t.Errorf("got %v, want ErrInvalid", err)
		}
	})
}

func TestParse(t *testing.T) {
	t.Parallel()
	for _, tt := range negative {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := baseline.Parse([]byte(tt.text)); !errors.Is(err, baseline.ErrInvalid) {
				t.Errorf("got %v, want ErrInvalid", err)
			}
		})
	}
	t.Run("accepts valid, ignoring order and duplicates", func(t *testing.T) {
		t.Parallel()
		set, err := baseline.Parse([]byte(`{"version":1,"fingerprints":["` + fpB + `","` + fpA + `","` + fpB + `"]}`))
		if err != nil {
			t.Fatal(err)
		}
		if _, okA := set[fpA]; len(set) != 2 || !okA {
			t.Errorf("got %v", set)
		}
	})
}

func TestRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, tt := range negative {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), "bad.json")
			if err := os.WriteFile(p, []byte(tt.text), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := baseline.Read(p); !errors.Is(err, baseline.ErrInvalid) {
				t.Errorf("got %v, want ErrInvalid", err)
			}
		})
	}
	t.Run("rejects missing file", func(t *testing.T) {
		t.Parallel()
		if _, err := baseline.Read(filepath.Join(dir, "missing.json")); !errors.Is(err, baseline.ErrInvalid) {
			t.Errorf("got %v", err)
		}
	})
}

func TestWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "b.json")
	if err := baseline.Write(p, []string{fpB, fpA}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	set, err := baseline.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	list := make([]string, 0, len(set))
	for k := range set {
		list = append(list, k)
	}
	if err := baseline.Write(p, list); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("round trip changed bytes: %q vs %q", first, second)
	}
	if err := baseline.Write(filepath.Join(dir, "no", "such", "b.json"), []string{fpA}); !errors.Is(err, baseline.ErrInvalid) {
		t.Errorf("unwritable: got %v", err)
	}
}
