package spec_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jh125486/fldlint/go/internal/spec"
)

func TestGenerateJava(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "stub spec",
			path: "testdata/spec.json",
			want: []string{
				spec.JavaHeader + "\n",
				"package io.github.jh125486.fldlint.spec;",
				"public final class RulesGen",
				`new Rule("bool-variation", "boolean", "low",`,
				`ctxArg("BoolVariationCtx", 0)`,
				`categories.put("bulk", new SkipCategory("warning",`,
			},
		},
		{
			name: "real spec with a java binding",
			path: "../../../spec/generated/spec.json",
			want: []string{
				`"java", new Binding(list("boolVariation"),`,
				`list("any(dev.openfeature.sdk.Client)", "key", "default", "context")`,
				`ctxArg("BoolVariationCtx", 0)`,
				`ctxArg("BoolVariationDetailCtx", 0),
                                3, 3,`,
				`ctxArg(),
                                1, null,`,
				`ctxArg(),
                                null, null,`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sp, err := spec.LoadSpec(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			a := spec.GenerateJava(sp.Rules)
			if string(a) != string(spec.GenerateJava(sp.Rules)) {
				t.Fatal("generation is not deterministic")
			}
			for _, w := range tt.want {
				if !strings.Contains(string(a), w) {
					t.Errorf("generated Java lacks %q", w)
				}
			}
		})
	}
}

func TestTemplateArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{name: "empty", template: "", want: nil},
		{name: "mustache", template: "{{client}}.getX({{key}}, {{default}})", want: []string{"client", "key", "default"}},
		{
			name:     "java anonymous and named",
			template: "#{any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key}, #{default}, #{context})",
			want:     []string{"any(dev.openfeature.sdk.Client)", "key", "default", "context"},
		},
		{name: "java typed", template: "#{client:any(Client)}.f(#{key:any(java.lang.String)})", want: []string{"client", "key"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := spec.TemplateArgs(tt.template); !slices.Equal(got, tt.want) {
				t.Fatalf("TemplateArgs(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}
