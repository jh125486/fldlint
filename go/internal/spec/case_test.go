package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

const okCase = `
id: c1
rule: bool-variation
tags: [quote-double]
description: d
expect:
  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], output: false}
`

func stubRules(t *testing.T) *spec.RulesFile {
	t.Helper()
	rf, err := spec.LoadRules("testdata/rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return rf
}

func TestParseCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{name: "ok", yaml: okCase},
		{name: "empty", yaml: "", wantErr: true},
		{name: "unknown field", yaml: okCase + "bogus: 1\n", wantErr: true},
		{name: "bad yaml", yaml: "id: [", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := spec.ParseCase([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && c.Expect["go"].Diagnostics[0].Col != 12 {
				t.Fatalf("unexpected parse: %+v", c)
			}
		})
	}
}

func TestLoadCase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "case.yaml")
	if err := os.WriteFile(good, []byte(okCase), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "ok", path: good},
		{name: "missing", path: filepath.Join(dir, "nope.yaml"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadCase(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCase_Validate(t *testing.T) {
	t.Parallel()
	rf := stubRules(t)
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{name: "ok", yaml: okCase},
		{name: "missing id", yaml: strings.Replace(okCase, "id: c1", "", 1), wantErr: "missing id"},
		{name: "missing rule", yaml: strings.Replace(okCase, "rule: bool-variation", "", 1), wantErr: "missing rule"},
		{name: "unknown rule", yaml: strings.Replace(okCase, "rule: bool-variation", "rule: zzz", 1), wantErr: "unknown rule"},
		{name: "missing description", yaml: strings.Replace(okCase, "description: d", "", 1), wantErr: "missing description"},
		{name: "duplicate tag", yaml: strings.Replace(okCase, "[quote-double]", "[a, a]", 1), wantErr: "duplicate tag"},
		{name: "empty tag", yaml: strings.Replace(okCase, "[quote-double]", `[""]`, 1), wantErr: "empty tag"},
		{name: "no expect", yaml: "id: c1\nrule: bool-variation\ndescription: d\n", wantErr: "expect"},
		{name: "unknown language", yaml: strings.Replace(okCase, "  go:", "  rust:", 1), wantErr: "unknown language"},
		{name: "zero line", yaml: strings.Replace(okCase, "line: 9", "line: 0", 1), wantErr: "line/col"},
		{name: "missing ruleId", yaml: strings.Replace(okCase, "ruleId: bool-variation, ", "", 1), wantErr: "missing ruleId"},
		{name: "bad skip", yaml: strings.Replace(okCase, "context-unproven", "nope", 1), wantErr: "unknown skip category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := spec.ParseCase([]byte(tt.yaml))
			if err != nil {
				t.Fatal(err)
			}
			err = c.Validate(rf)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
