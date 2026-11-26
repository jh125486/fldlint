package golangci_test

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"

	"github.com/jh125486/fldlint/go/analyzer"
	"github.com/jh125486/fldlint/go/golangci"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		conf    any
		wantErr bool
	}{
		{name: "nil settings", conf: nil},
		{name: "empty settings", conf: map[string]any{}},
		{name: "unknown key", conf: map[string]any{"bogus": true}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := golangci.New(tt.conf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New(%v) error = %v, wantErr %v", tt.conf, err, tt.wantErr)
			}
			if !tt.wantErr && got == nil {
				t.Fatal("New returned a nil plugin without an error")
			}
		})
	}
}

func TestNew_registered(t *testing.T) {
	t.Parallel()

	newPlugin, err := register.GetPlugin(golangci.Name)
	if err != nil {
		t.Fatalf("GetPlugin(%q): %v", golangci.Name, err)
	}
	if _, err := newPlugin(nil); err != nil {
		t.Fatalf("registered constructor: %v", err)
	}
}

func TestPlugin_BuildAnalyzers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "fldlint analyzer", want: analyzer.Analyzer.Name},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := golangci.Plugin{}.BuildAnalyzers()
			if err != nil {
				t.Fatalf("BuildAnalyzers: %v", err)
			}
			if len(got) != 1 || got[0] != analyzer.Analyzer || got[0].Name != tt.want {
				t.Fatalf("BuildAnalyzers = %v, want only %q", got, tt.want)
			}
		})
	}
}

func TestPlugin_GetLoadMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "types info", want: register.LoadModeTypesInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := (golangci.Plugin{}).GetLoadMode(); got != tt.want {
				t.Fatalf("GetLoadMode = %q, want %q", got, tt.want)
			}
		})
	}
}
