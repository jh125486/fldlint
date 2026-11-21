package spec_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
	}{
		{name: "stub spec", path: "testdata/rules.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rf, err := spec.LoadRules(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			a, err := spec.Generate(rf)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := spec.Generate(rf)
			if string(a) != string(b) {
				t.Fatal("generation is not deterministic")
			}
			if !strings.HasPrefix(string(a), spec.Header+"\n") {
				t.Fatal("missing generated header")
			}
			if !strings.Contains(string(a), "Message: ") {
				t.Fatal("generated rules do not carry Message")
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "rules_gen.go", a, parser.AllErrors); err != nil {
				t.Fatalf("generated source does not parse: %v", err)
			}
		})
	}
}
