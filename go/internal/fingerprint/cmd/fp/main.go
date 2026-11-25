// Command fp is the Go side of the parity check (task parity): it recomputes
// every fingerprint in a vectors file.
//
//	fp [vectors.json]                  prints one fingerprint per line
//	fp --baseline out.json [vectors]   writes a baseline from them
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jh125486/fldlint/go/internal/baseline"
	"github.com/jh125486/fldlint/go/internal/fingerprint"
)

type vector struct {
	RuleID string  `json:"ruleId"`
	Path   string  `json:"path"`
	Key    *string `json:"key"`
	Occ    int     `json:"occ"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fp:", err)
		os.Exit(2)
	}
}

func run(args []string) error {
	var out string
	if len(args) >= 2 && args[0] == "--baseline" {
		out, args = args[1], args[2:]
	}
	path := "../spec/contracts/fingerprint-vectors.json"
	if len(args) > 0 {
		path = args[0]
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is the caller's baseline/vectors file
	if err != nil {
		return err
	}
	var vectors []vector
	if err := json.Unmarshal(data, &vectors); err != nil {
		return err
	}
	fps := make([]string, len(vectors))
	for i, v := range vectors {
		fps[i] = fingerprint.Fingerprint(fingerprint.Input{RuleID: v.RuleID, Path: v.Path, Key: v.Key, Occ: v.Occ})
	}
	if out != "" {
		return baseline.Write(out, fps)
	}
	for _, f := range fps {
		fmt.Println(f)
	}
	return nil
}
