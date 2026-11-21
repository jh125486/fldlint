package main

// Whitebox: run and main live in package main and cannot be imported.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()
	stub, err := filepath.Abs("../testdata/rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	stubBytes, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		rules   string // content of <spec>/rules.yaml; empty means the spec root is missing
		useSpec bool   // pass -spec explicitly
		wantErr string
	}{
		{name: "explicit valid", rules: string(stubBytes), useSpec: true},
		{name: "explicit missing", useSpec: true, wantErr: "-spec"},
		{name: "explicit invalid", rules: "version: 2\n", useSpec: true, wantErr: "rules"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "spec")
			if tt.rules != "" {
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "rules.yaml"), []byte(tt.rules), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out := filepath.Join(t.TempDir(), "gen.go")
			args := []string{"-out", out}
			if tt.useSpec {
				args = append(args, "-spec", root)
			}
			err := run(args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				if _, statErr := os.Stat(out); statErr == nil {
					t.Fatal("output written despite error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(out); err != nil {
				t.Fatal(err)
			}
		})
	}
}
