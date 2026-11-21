package planner_test

import (
	"testing"

	"github.com/jh125486/fldlint/go/internal/planner"
)

func TestPlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		method string
		want   planner.Result
	}{
		{name: "known method", method: "BoolVariation", want: planner.Result{Kind: planner.KindSkip, Category: "unbound-client", RuleID: "bool-variation"}},
		{name: "unknown method", method: "Nope", want: planner.Result{Kind: planner.KindSkip, Category: "unbound-client"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := planner.Plan(planner.CallSite{Method: tt.method})
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
