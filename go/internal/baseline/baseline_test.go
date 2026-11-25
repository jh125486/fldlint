package baseline_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/baseline"
)

const (
	fpA = "0c1ab39ac97767156d8a818304b68367"
	fpB = "d1a64654f64fa217171b2841c61090fd"
)

func doc(fps string) string { return `{"version":1,"fingerprints":[` + fps + `]}` }

func q(s string) string { return `"` + s + `"` }

// rejected are texts every reader must refuse with ErrInvalid.
var rejected = []struct{ name, text string }{
	{"malformed JSON", "{"},
	{"empty input", ""},
	{"trailing data", doc("") + " x"},
	{"second document", doc("") + doc("")},
	{"BOM", "\xef\xbb\xbf" + doc("")},
	{"not an object", "[]"},
	{"null", "null"},
	{"unknown field", `{"version":1,"fingerprints":[],"extra":1}`},
	{"flaglint format", `{"version":"1","createdAt":"2026-01-01","fingerprints":[]}`},
	{"version 2", `{"version":2,"fingerprints":[]}`},
	{"string version", `{"version":"1","fingerprints":[]}`},
	{"null version", `{"version":null,"fingerprints":[]}`},
	{"version 1.5", `{"version":1.5,"fingerprints":[]}`},
	{"missing fingerprints", `{"version":1}`},
	{"missing version", `{"fingerprints":[]}`},
	{"fingerprints not an array", `{"version":1,"fingerprints":` + q(fpA) + `}`},
	{"fingerprints null", `{"version":1,"fingerprints":null}`},
	{"non-hex entry", doc(q("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"))},
	{"uppercase entry", doc(q(strings.ToUpper(fpA)))},
	{"short entry", doc(q("abc"))},
	{"non-string entry", doc("1")},
	{"null entry", doc("null")},
}

func TestFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []string
		want    string
		wantErr bool
	}{
		{"sorted and deduplicated", []string{fpB, fpA, fpB}, "{\n  \"version\": 1,\n  \"fingerprints\": [\n    \"" + fpA + "\",\n    \"" + fpB + "\"\n  ]\n}\n", false},
		{"nil", nil, "{\n  \"version\": 1,\n  \"fingerprints\": []\n}\n", false},
		{"empty non-nil", []string{}, "{\n  \"version\": 1,\n  \"fingerprints\": []\n}\n", false},
		{"invalid entry", []string{"XYZ"}, "", true},
		{"duplicate invalid entries", []string{"XYZ", "XYZ"}, "", true},
		{"invalid entry among valid", []string{fpA, "XYZ"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := baseline.Format(tt.in)
			if tt.wantErr {
				if !errors.Is(err, baseline.ErrInvalid) {
					t.Errorf("got %v, want ErrInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		text    string
		want    []string
		wantErr bool
		errText string
	}{
		{name: "ignores order and duplicates", text: doc(q(fpB) + "," + q(fpA) + "," + q(fpB)), want: []string{fpA, fpB}},
		{name: "empty list", text: doc(""), want: []string{}},
		{name: "version 1.0 is the number 1", text: `{"version":1.0,"fingerprints":[]}`, want: []string{}},
		{name: "duplicate keys, last wins", text: `{"version":2,"version":1,"fingerprints":[` + q(fpB) + `],"fingerprints":[` + q(fpA) + `]}`, want: []string{fpA}},
		{name: "surrounding whitespace", text: "\n " + doc(q(fpA)) + "\n", want: []string{fpA}},
		{name: "syntax error is reported", text: "{", wantErr: true, errText: "not valid JSON"},
		{name: "trailing data is reported", text: doc("") + " x", wantErr: true, errText: "not valid JSON"},
	}
	for _, r := range rejected {
		tests = append(tests, struct {
			name    string
			text    string
			want    []string
			wantErr bool
			errText string
		}{name: "rejects " + r.name, text: r.text, wantErr: true})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			set, err := baseline.Parse([]byte(tt.text))
			if tt.wantErr {
				if !errors.Is(err, baseline.ErrInvalid) {
					t.Fatalf("got %v, want ErrInvalid", err)
				}
				if !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("error %q lacks %q", err, tt.errText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(set))
			for k := range set {
				got = append(got, k)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRead(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content *string // nil: the file does not exist
		want    int
		wantErr bool
	}{
		{name: "valid", content: ptr(doc(q(fpA) + "," + q(fpB))), want: 2},
		{name: "missing file", wantErr: true},
	}
	for _, r := range rejected {
		tests = append(tests, struct {
			name    string
			content *string
			want    int
			wantErr bool
		}{name: "rejects " + r.name, content: ptr(r.text), wantErr: true})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), "b.json")
			if tt.content != nil {
				if err := os.WriteFile(p, []byte(*tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			set, err := baseline.Read(p)
			if tt.wantErr {
				if !errors.Is(err, baseline.ErrInvalid) {
					t.Errorf("got %v, want ErrInvalid", err)
				}
				return
			}
			if err != nil || len(set) != tt.want {
				t.Errorf("got %v, %v; want %d entries", set, err, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestWrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []string
		subdir  string // path segments that do not exist, making the write fail
		want    string
		wantErr bool
	}{
		{name: "sorted and deduplicated", in: []string{fpB, fpA, fpB}, want: "{\n  \"version\": 1,\n  \"fingerprints\": [\n    \"" + fpA + "\",\n    \"" + fpB + "\"\n  ]\n}\n"},
		{name: "empty", in: []string{}, want: "{\n  \"version\": 1,\n  \"fingerprints\": []\n}\n"},
		{name: "invalid entry", in: []string{"XYZ"}, wantErr: true},
		{name: "unwritable path", in: []string{fpA}, subdir: "no/such", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), tt.subdir, "b.json")
			err := baseline.Write(p, tt.in)
			if tt.wantErr {
				if !errors.Is(err, baseline.ErrInvalid) {
					t.Errorf("got %v, want ErrInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			// Round trip: reading and rewriting is byte-stable.
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
			again, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != tt.want {
				t.Errorf("round trip changed bytes: %q", again)
			}
		})
	}
}
