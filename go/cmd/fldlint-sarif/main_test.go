package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "a.go")
	if err := os.WriteFile(src, []byte("package p\n\nfunc f() {\n\t_ = c.BoolVariation(\"k\", c, false)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := `{"p": {"fldlint": [{"category": "bool-variation", "posn": "` + filepath.ToSlash(src) + `:4:6", "message": "Migrate"}]}}`
	badBaseline := write("bad.json", `{"version": 2, "fingerprints": []}`)
	okBaseline := write("ok.json", `{"version": 1, "fingerprints": []}`)

	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantStdout string // substring
		wantStderr string // substring
	}{
		{name: "ok", args: []string{"-root", dir}, stdin: good, wantCode: 0, wantStdout: `"level": "warning"`},
		{name: "empty run", args: nil, stdin: "{}", wantCode: 0, wantStdout: `"results": []`},
		{name: "severity default", args: []string{"-root", dir, "-severity", "error"}, stdin: good, wantCode: 0, wantStdout: `"level": "error"`},
		{name: "severity per rule", args: []string{"-root", dir, "-severity", "bool-variation=info"}, stdin: good, wantCode: 0, wantStdout: `"level": "note"`},
		{name: "baseline", args: []string{"-root", dir, "-baseline", okBaseline}, stdin: good, wantCode: 0, wantStdout: `"baselineState": "new"`},
		{name: "help", args: []string{"-h"}, wantCode: 0, wantStderr: "usage:"},
		{name: "unknown flag", args: []string{"-bogus"}, wantCode: 2, wantStderr: "flag provided but not defined"},
		{name: "positional argument", args: []string{"x"}, wantCode: 2, wantStderr: "unexpected argument"},
		{name: "bad severity", args: []string{"-severity", "loud"}, wantCode: 2, wantStderr: "level must be error, warn or info"},
		{name: "unsupported baseline", args: []string{"-baseline", badBaseline}, stdin: good, wantCode: 2, wantStderr: "unsupported baseline version"},
		{name: "missing baseline", args: []string{"-baseline", filepath.Join(dir, "nope.json")}, stdin: good, wantCode: 2, wantStderr: "invalid baseline"},
		{name: "empty stdin", args: nil, stdin: "", wantCode: 1, wantStderr: "reading fldlint -json"},
		{name: "analysis error", args: nil, stdin: `{"p": {"fldlint": {"error": "boom"}}}`, wantCode: 1, wantStderr: "boom"},
		{name: "path outside root", args: []string{"-root", filepath.Join(dir, "sub")}, stdin: good, wantCode: 1, wantStderr: "outside the scan root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr); code != tt.wantCode {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, tt.wantCode, &stdout, &stderr)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want containing %q", &stdout, tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want containing %q", &stderr, tt.wantStderr)
			}
			if tt.wantCode != 0 && stdout.Len() != 0 {
				t.Errorf("stdout must be empty on failure, got %q", &stdout)
			}
		})
	}
}
