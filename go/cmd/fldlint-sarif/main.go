// Command fldlint-sarif converts `fldlint -json` output on stdin into SARIF
// 2.1.0 on stdout (spec/contracts/sarif.md). It is an optional extra: fldlint
// itself has no SARIF output.
//
//	fldlint -json ./... | fldlint-sarif [-baseline file] [-root dir] [-severity [rule=]level]...
//
// Exit codes: 0 success, 1 unreadable or malformed input, 2 usage error or
// invalid baseline.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/jh125486/fldlint/go/internal/baseline"
	"github.com/jh125486/fldlint/go/internal/sarif"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const (
	exitOK    = 0
	exitInput = 1
	exitUsage = 2
)

// severities is a repeatable flag: `level` sets the default, `rule=level` one rule.
type severities map[string]string

func (s severities) String() string { return fmt.Sprint(map[string]string(s)) }

func (s severities) Set(v string) error {
	rule, level, ok := strings.Cut(v, "=")
	if !ok {
		rule, level = "", v
	}
	lv, ok := map[string]string{"error": "error", "warn": "warning", "warning": "warning", "info": "note", "note": "note"}[level]
	if !ok {
		return fmt.Errorf("severity %q: level must be error, warn or info", v)
	}
	s[rule] = lv
	return nil
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fldlint-sarif", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: fldlint -json ./... | fldlint-sarif [-baseline file] [-root dir] [-severity [rule=]error|warn|info]...")
		fs.PrintDefaults()
	}
	baselinePath := fs.String("baseline", "", "baseline `file` (baseline.md); sets baselineState on every result")
	root := fs.String("root", "", "scan root that result paths are relative to (default: the current directory)")
	sev := severities{}
	fs.Var(sev, "severity", "configured severity, `[rule=]level`: error, warn or info; repeatable (default warn)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "fldlint-sarif: unexpected argument %q (input is read from stdin)\n", fs.Arg(0))
		return exitUsage
	}

	opts := sarif.Options{Root: *root, Version: version(), Level: sev}
	if *baselinePath != "" {
		set, err := baseline.Read(*baselinePath)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "fldlint-sarif:", err)
			return exitUsage
		}
		opts.Baseline = set
	}
	out, err := sarif.Convert(stdin, opts)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "fldlint-sarif:", err)
		return exitInput
	}
	if _, err := stdout.Write(out); err != nil {
		_, _ = fmt.Fprintln(stderr, "fldlint-sarif:", err)
		return exitInput
	}
	return exitOK
}

// version is the module version of a `go install`ed binary, else "0.0.0".
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "0.0.0"
}
