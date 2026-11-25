package main

// Whitebox: run and main live in package main and cannot be imported.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()
	stub, err := filepath.Abs("../testdata/spec.json")
	if err != nil {
		t.Fatal(err)
	}
	stubBytes, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		rules   string // content of <spec>/generated/spec.json; empty means the spec root is missing
		useSpec bool   // pass -spec explicitly
		wantErr string
	}{
		{name: "explicit valid", rules: string(stubBytes), useSpec: true},
		{name: "explicit missing", useSpec: true, wantErr: "-spec"},
		{name: "explicit invalid", rules: `{"rules": {"version": 2, "skipCategories": {}, "rules": []}, "cases": []}`, useSpec: true, wantErr: "unsupported version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "spec")
			if tt.rules != "" {
				if err := os.MkdirAll(filepath.Join(root, "generated"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "generated", "spec.json"), []byte(tt.rules), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out := filepath.Join(t.TempDir(), "gen.go")
			javaOut := filepath.Join(t.TempDir(), "RulesGen.java")
			args := []string{"-out", out, "-java-out", javaOut}
			if tt.useSpec {
				args = append(args, "-spec", root)
			}
			err := run(args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				for _, f := range []string{out, javaOut} {
					if _, statErr := os.Stat(f); statErr == nil {
						t.Fatalf("%s written despite error", f)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range []string{out, javaOut} {
				if _, err := os.Stat(f); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing")
	out := filepath.Join(t.TempDir(), "g.go")
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "no error", err: nil, want: 0},
		{name: "help", err: run([]string{"-h", "-out", out}), want: 0},
		{name: "plain failure", err: errors.New("write failed"), want: 1},
		{name: "explicit missing spec", err: run([]string{"-spec", missing, "-out", out, "-java-out", ""}), want: 2},
		{name: "unknown flag", err: run([]string{"-bogus"}), want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := exitCode(tt.err); got != tt.want {
				t.Fatalf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
