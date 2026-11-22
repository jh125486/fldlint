package conformance_test

import (
	"bytes"
	"fmt"
	"go/format"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/jh125486/fldlint/go/analyzer"
	"github.com/jh125486/fldlint/go/internal/position"
	"github.com/jh125486/fldlint/go/internal/spec"
)

// sdkStubs holds stub packages for the LaunchDarkly and OpenFeature SDKs. They
// are shared with the analyzer's own tests and match the real signatures.
const sdkStubs = "../analyzer/testdata/src/github.com"

// workspace is a throwaway GOPATH tree: the SDK stubs plus one package,
// "flags", holding a single file.
type workspace struct {
	dir string
}

// newWorkspace also copies <caseDir>/go/extra, when present, into src/ so a
// case can bring helper packages (for example a package of shared clients).
func newWorkspace(t *testing.T, file string, content []byte, caseDir string) *workspace {
	t.Helper()
	dir := t.TempDir()
	stubs, err := filepath.Abs(sdkStubs)
	if err != nil {
		t.Fatal(err)
	}
	copyTree(t, stubs, filepath.Join(dir, "src", "github.com"))
	if extra := filepath.Join(caseDir, "go", "extra"); dirExists(extra) {
		copyTree(t, extra, filepath.Join(dir, "src"))
	}
	pkg := filepath.Join(dir, "src", "flags")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, file), content, 0o644); err != nil {
		t.Fatal(err)
	}
	return &workspace{dir: dir}
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// copyTree copies the files under src into dst, creating directories.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// load type-checks the "flags" package in GOPATH mode, as analysistest does.
func (w *workspace) load(t *testing.T) *packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedTypesSizes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps,
		Dir: w.dir,
		Env: append(os.Environ(), "GOPATH="+w.dir, "GO111MODULE=off", "GOWORK=off"),
	}
	pkgs, err := packages.Load(cfg, "flags")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name == "" {
		t.Fatalf("expected the flags package to load, got %v", pkgs)
	}
	return pkgs[0]
}

// finding is one reported diagnostic in the contract's coordinates.
type finding struct {
	line, col int
	ruleID    string
	skip      string
	hasFix    bool
	edits     []analysis.TextEdit
}

// runEngine runs the analyzer over the fixture input and returns its findings
// with UTF-16 columns, plus the input bytes and file name.
func runEngine(t *testing.T, caseDir string, tolerateTypeErrors bool) (out []finding, src []byte, offset func(token.Pos) int) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(caseDir, "go", "input.go"))
	if err != nil {
		t.Fatal(err)
	}
	out, offset = runSource(t, src, caseDir, tolerateTypeErrors)
	return out, src, offset
}

// runSource runs the analyzer over src as the "flags" package.
func runSource(t *testing.T, src []byte, caseDir string, tolerateTypeErrors bool) (out []finding, offset func(token.Pos) int) {
	t.Helper()
	pkg := newWorkspace(t, "input.go", src, caseDir).load(t)
	if !tolerateTypeErrors && len(pkg.Errors) > 0 {
		t.Fatalf("fixture does not type-check against the SDK stubs: %v", pkg.Errors)
	}
	a := *analyzer.Analyzer
	a.RunDespiteErrors = tolerateTypeErrors
	g, err := checker.Analyze([]*analysis.Analyzer{&a}, []*packages.Package{pkg}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range g.Roots {
		if act.Err != nil {
			t.Fatalf("analysis failed: %v", act.Err)
		}
		for _, d := range act.Diagnostics {
			p := act.Package.Fset.Position(d.Pos)
			f := finding{line: p.Line, col: position.UTF16Col(src, p.Offset), ruleID: d.Category}
			if _, rest, ok := strings.Cut(d.Message, "[skip: "); ok {
				f.skip = strings.TrimSuffix(rest, "]")
			}
			for _, fx := range d.SuggestedFixes {
				f.hasFix = true
				f.edits = append(f.edits, fx.TextEdits...)
			}
			fset := act.Package.Fset
			offset = func(p token.Pos) int { return fset.Position(p).Offset }
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].line != out[j].line {
			return out[i].line < out[j].line
		}
		return out[i].col < out[j].col
	})
	return out, offset
}

// applyEdits applies the union of the fixes' edits (identical edits from
// different fixes count once) to src.
func applyEdits(t *testing.T, src []byte, offset func(token.Pos) int, edits []analysis.TextEdit) []byte {
	t.Helper()
	type span struct {
		start, end int
		text       string
	}
	var spans []span
	for _, e := range edits {
		sp := span{offset(e.Pos), offset(e.End), string(e.NewText)}
		if !slices.Contains(spans, sp) {
			spans = append(spans, sp)
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end < spans[j].end
	})
	var out bytes.Buffer
	last := 0
	for _, sp := range spans {
		if sp.start < last {
			t.Fatalf("overlapping edits at offset %d", sp.start)
		}
		out.Write(src[last:sp.start])
		out.WriteString(sp.text)
		last = sp.end
	}
	out.Write(src[last:])
	return out.Bytes()
}

// checkEngine runs one case through the analyzer and compares findings, the
// applied fixes and the compiled result against the case expectation.
func checkEngine(t *testing.T, c *spec.Case, caseDir string) {
	t.Helper()
	want := c.Expect["go"]
	tolerate := slices.ContainsFunc(want.Diagnostics, func(d spec.Diagnostic) bool { return d.Skip == "default-type-unproven" })
	got, src, offset := runEngine(t, caseDir, tolerate)

	var gotS, wantS []string
	for _, f := range got {
		gotS = append(gotS, fmt.Sprintf("%d:%d %s skip=%s", f.line, f.col, f.ruleID, f.skip))
	}
	for _, d := range want.Diagnostics {
		wantS = append(wantS, fmt.Sprintf("%d:%d %s skip=%s", d.Line, d.Col, d.RuleID, d.Skip))
	}
	if !slices.Equal(gotS, wantS) {
		t.Fatalf("diagnostics differ\n got: %q\nwant: %q", gotS, wantS)
	}
	for _, f := range got {
		if f.hasFix == (f.skip != "") {
			t.Errorf("%d:%d: hasFix=%v for skip=%q (a fix is offered exactly for rewritten calls)", f.line, f.col, f.hasFix, f.skip)
		}
	}

	if !want.Output {
		return
	}

	// Each fix must compile ALONE (editors apply one at a time, and the
	// suggested fix must not depend on its siblings).
	for _, f := range got {
		if !f.hasFix {
			continue
		}
		alone, err := format.Source(applyEdits(t, src, offset, f.edits))
		if err != nil {
			t.Fatalf("%d:%d: fix alone does not parse: %v", f.line, f.col, err)
		}
		if pkg := newWorkspace(t, "input.go", alone, caseDir).load(t); len(pkg.Errors) > 0 {
			t.Fatalf("%d:%d: fix applied alone does not compile: %v\n%s", f.line, f.col, pkg.Errors, alone)
		}
	}

	// The union of all fixes, then what a batch driver does (remove the
	// imports the union left unused), must match the golden and compile.
	var edits []analysis.TextEdit
	for _, f := range got {
		edits = append(edits, f.edits...)
	}
	fixed := applyEdits(t, src, offset, edits)
	if _, err := format.Source(fixed); err != nil {
		t.Fatalf("union of fixes does not parse: %v\n%s", err, fixed)
	}
	cleaned := removeUnusedImports(t, fixed, caseDir)
	wantSrc, err := os.ReadFile(filepath.Join(caseDir, "go", "output.go.golden"))
	if err != nil {
		t.Fatal(err)
	}
	gotFmt, err := format.Source(cleaned)
	if err != nil {
		t.Fatalf("fixed source does not parse: %v\n%s", err, cleaned)
	}
	wantFmt, err := format.Source(wantSrc)
	if err != nil {
		t.Fatalf("golden does not parse: %v", err)
	}
	if !bytes.Equal(gotFmt, wantFmt) {
		t.Fatalf("fixed output differs from output.go.golden\n--- got ---\n%s\n--- want ---\n%s", gotFmt, wantFmt)
	}
	// Go must compile: the rewritten file type-checks against the stubs.
	if pkg := newWorkspace(t, "input.go", gotFmt, caseDir).load(t); len(pkg.Errors) > 0 {
		t.Fatalf("fixed output does not compile: %v\n%s", pkg.Errors, gotFmt)
	}
}

// removeUnusedImports emulates the cleanup batch drivers run after applying
// fixes (driverutil.FormatSourceRemoveImports): every import the compiler
// reports as unused is deleted.
func removeUnusedImports(t *testing.T, src []byte, caseDir string) []byte {
	t.Helper()
	pkg := newWorkspace(t, "input.go", src, caseDir).load(t)
	drop := map[int]bool{}
	for _, e := range pkg.Errors {
		if !strings.Contains(e.Msg, "imported and not used") && !strings.Contains(e.Msg, "imported as") {
			continue
		}
		// e.Pos is "file:line:col".
		parts := strings.Split(e.Pos, ":")
		if len(parts) < 3 {
			t.Fatalf("unexpected error position %q", e.Pos)
		}
		var line int
		if _, err := fmt.Sscanf(parts[len(parts)-2], "%d", &line); err != nil {
			t.Fatalf("unexpected error position %q: %v", e.Pos, err)
		}
		drop[line] = true
	}
	if len(drop) == 0 {
		return src
	}
	var out [][]byte
	for i, l := range bytes.Split(src, []byte("\n")) {
		if !drop[i+1] {
			out = append(out, l)
		}
	}
	return bytes.Join(out, []byte("\n"))
}

// converge emulates `go fix` style driving: apply every fix of a run, clean up
// unused imports, and run again until no fix is offered. It returns the final
// source, which must compile; it fails if the loop does not settle.
func converge(t *testing.T, src []byte, caseDir string) []byte {
	t.Helper()
	const maxRuns = 5
	for run := 1; run <= maxRuns; run++ {
		got, offset := runSource(t, src, caseDir, false)
		var edits []analysis.TextEdit
		for _, f := range got {
			edits = append(edits, f.edits...)
		}
		if len(edits) == 0 {
			if pkg := newWorkspace(t, "input.go", src, caseDir).load(t); len(pkg.Errors) > 0 {
				t.Fatalf("converged source does not compile: %v\n%s", pkg.Errors, src)
			}
			return src
		}
		fixed, err := format.Source(removeUnusedImports(t, applyEdits(t, src, offset, edits), caseDir))
		if err != nil {
			t.Fatalf("run %d: fixed source does not parse: %v", run, err)
		}
		if bytes.Equal(fixed, src) {
			t.Fatalf("run %d: fixes were offered but changed nothing", run)
		}
		src = fixed
	}
	t.Fatalf("fixes did not settle after %d runs:\n%s", maxRuns, src)
	return nil
}
