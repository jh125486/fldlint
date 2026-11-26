// Package golangci registers the fldlint analyzer as a golangci-lint module
// plugin. It lives in its own module so that users of the analyzer library do
// not pull in github.com/golangci/plugin-module-register.
//
// Build a custom golangci-lint binary with `golangci-lint custom`; see README.md.
package golangci

import (
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/plugin-module-register/register"

	"github.com/jh125486/fldlint/go/analyzer"
)

// Name is the linter name used in .golangci.yml.
const Name = "fldlint"

func init() {
	register.Plugin(Name, New)
}

// Settings are the plugin settings. There are none; the struct exists so that
// unknown keys under linters.settings.custom.fldlint.settings are rejected.
type Settings struct{}

// Plugin is the golangci-lint module plugin for fldlint.
type Plugin struct{}

// New is the register.NewPlugin constructor. conf is the raw `settings` value
// from .golangci.yml (nil when absent).
func New(conf any) (register.LinterPlugin, error) {
	if _, err := register.DecodeSettings[Settings](conf); err != nil {
		return nil, err
	}
	return Plugin{}, nil
}

// BuildAnalyzers returns the fldlint analyzer.
func (Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{analyzer.Analyzer}, nil
}

// GetLoadMode reports that the analyzer needs type information (call sites are
// identified by type).
func (Plugin) GetLoadMode() string { return register.LoadModeTypesInfo }
