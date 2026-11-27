package analyzer_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/jh125486/fldlint/go/analyzer"
)

const ldctx = "github.com/launchdarkly/go-sdk-common/v3/ldcontext"

func TestAnalyzer(t *testing.T) {
	t.Parallel()
	// withErrors runs the analyzer on a package that deliberately does not
	// type-check against the real SDK signatures.
	withErrors := *analyzer.Analyzer
	withErrors.RunDespiteErrors = true

	tests := []struct {
		name string
		a    *analysis.Analyzer
		pkg  string
		// dir is the testdata root; empty means analysistest.TestData().
		dir string
		// fixes reports whether the package has .golden files to compare.
		fixes bool
	}{
		{name: "add context import and remove ldcontext", a: analyzer.Analyzer, pkg: "addctx", fixes: true},
		{name: "keep ldcontext when still used", a: analyzer.Analyzer, pkg: "keepldctx", fixes: true},
		{name: "two rewrites share one import edit set", a: analyzer.Analyzer, pkg: "twocalls", fixes: true},
		{name: "goctx resolution", a: analyzer.Analyzer, pkg: "goctx", fixes: true},
		{name: "openfeature import added from another file's client", a: analyzer.Analyzer, pkg: "twofiles", fixes: true},
		{name: "import aliases", a: analyzer.Analyzer, pkg: "aliases", fixes: true},
		{name: "ldcontext alone in its group, openfeature client in another file", a: analyzer.Analyzer, pkg: "overlap", fixes: true},
		{name: "lone ldcontext import decl", a: analyzer.Analyzer, pkg: "loneimport", fixes: true},
		{name: "parenthesized decl whose only spec is ldcontext", a: analyzer.Analyzer, pkg: "parenonly", fixes: true},
		{name: "receiver stranding and purity", a: analyzer.Analyzer, pkg: "stranded"},
		{name: "shadowed openfeature name", a: analyzer.Analyzer, pkg: "shadowedof"},
		{name: "package-qualified receiver import is removed with its last use", a: analyzer.Analyzer, pkg: "pkgrecv", fixes: true},
		{name: "package-qualified receiver shared by two fixes", a: analyzer.Analyzer, pkg: "pkgrecvtwo", fixes: true},
		{name: "nested rewrites fix only the inner call", a: analyzer.Analyzer, pkg: "nested", fixes: true},
		{name: "comments inside a call move with their arguments", a: analyzer.Analyzer, pkg: "comments", fixes: true},
		{name: "a comment with no home withholds the fix", a: analyzer.Analyzer, pkg: "unfixable"},
		{name: "a comment with no home does not hide receiver-unproven", a: analyzer.Analyzer, pkg: "unfixablestrand"},
		{name: "a changed SDK signature is not a finding", a: analyzer.Analyzer, pkg: "forkuse", dir: "testdata/fork"},
		{name: "blank and dot openfeature imports are unbound", a: analyzer.Analyzer, pkg: "badof"},
		{name: "a shadowed or unusable context import is context-unproven", a: analyzer.Analyzer, pkg: "badctx"},
		{name: "a call in a package-level client's initializer chain is unbound", a: analyzer.Analyzer, pkg: "initcycle"},
		{name: "a call in a package-level client's own initializer is unbound", a: analyzer.Analyzer, pkg: "initcycleself"},
		{name: "a test-only client does not bind a non-test call", a: analyzer.Analyzer, pkg: "variants"},
		{name: "a client under another build constraint does not bind", a: analyzer.Analyzer, pkg: "constraints"},
		{name: "skip categories", a: analyzer.Analyzer, pkg: "skips"},
		{name: "client identity and binding", a: analyzer.Analyzer, pkg: "clients"},
		{name: "default type unproven", a: &withErrors, pkg: "defaulttype"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := analysistest.TestData()
			if tt.dir != "" {
				abs, err := filepath.Abs(tt.dir)
				if err != nil {
					t.Fatal(err)
				}
				dir = abs
			}
			if tt.fixes {
				analysistest.RunWithSuggestedFixes(t, dir, tt.a, tt.pkg)
				return
			}
			analysistest.Run(t, dir, tt.a, tt.pkg)
		})
	}
}

func TestAnalyzerMetadata(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		got  func() any
		want any
	}{
		{name: "name", got: func() any { return analyzer.Analyzer.Name }, want: "fldlint"},
		{name: "valid", got: func() any { return analysis.Validate([]*analysis.Analyzer{analyzer.Analyzer}) }, want: error(nil)},
		{name: "requires inspect", got: func() any { return len(analyzer.Analyzer.Requires) }, want: 1},
		{name: "documented", got: func() any { return analyzer.Analyzer.Doc != "" }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.got(); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// stubTree reads the SDK stubs under testdata/src as a GOPATH file map.
func stubTree(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	root := filepath.Join("testdata", "src", "github.com")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Join("testdata", "src"), p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestAnalyzer_CommentEdits checks the exact text a fix writes when comments
// sit inside the call, including CRLF files and a leading BOM, which the
// golden-file tests above (formatted by gofmt) cannot show.
//
// Rule 7 exception: same as TestAnalyzer_SuggestedFixes (one exported value,
// structurally different checks).
func TestAnalyzer_CommentEdits(t *testing.T) {
	t.Parallel()
	const head = "package flags\n\nimport (\n\t\"context\"\n\n\t\"github.com/launchdarkly/go-sdk-common/v3/ldcontext\"\n" +
		"\t\"github.com/launchdarkly/go-server-sdk/v7\"\n\t\"github.com/open-feature/go-sdk/openfeature\"\n)\n\n" +
		"var client = openfeature.NewClient(\"flags\")\n\nvar ldClient *ldclient.LDClient\n\nvar _ = ldcontext.New\n\n"
	const marker = "/* want `Migrate boolean flag \"f\" to OpenFeature` */ "
	body := func(call string) string {
		return head + "func F(ctx context.Context, userKey string) bool {\n\t" + marker + "on, _ := " + call + "\n\treturn on\n}\n"
	}
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	bom := string(rune(0xFEFF))

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "no comment: plain substitution",
			src:  body(`ldClient.BoolVariationCtx(ctx, "f", ldcontext.New(userKey), false)`),
			want: body(`client.BooleanValue(ctx, "f", false, openfeature.NewEvaluationContext(userKey, nil))`),
		},
		{
			name: "block comments travel with their arguments",
			src:  body(`ldClient.BoolVariationCtx(ctx, "f" /* flag */, /* who */ ldcontext.New(userKey), false /* off */)`),
			want: body(`client.BooleanValue(ctx, "f" /* flag */, false /* off */, /* who */ openfeature.NewEvaluationContext(userKey, nil))`),
		},
		{
			name: "a line comment moves behind the next comma, LF",
			src:  body("ldClient.BoolVariationCtx(ctx, \"f\", // note\n\t\tldcontext.New(userKey), false)"),
			want: body("client.BooleanValue(ctx, \"f\", // note\n false, openfeature.NewEvaluationContext(userKey, nil))"),
		},
		{
			name: "a line comment keeps its CRLF terminator",
			src:  crlf(body("ldClient.BoolVariationCtx(ctx, \"f\", // note\n\t\tldcontext.New(userKey), false)")),
			want: crlf(body("client.BooleanValue(ctx, \"f\", // note\n false, openfeature.NewEvaluationContext(userKey, nil))")),
		},
		{
			name: "a leading BOM does not shift where comments land",
			src:  bom + body(`ldClient.BoolVariationCtx(ctx, "f", /* who */ ldcontext.New(userKey), false)`),
			want: bom + body(`client.BooleanValue(ctx, "f", false, /* who */ openfeature.NewEvaluationContext(userKey, nil))`),
		},
		{
			name: "BOM and CRLF together",
			src:  bom + crlf(body("ldClient.BoolVariationCtx(ctx, \"f\", // note\n\t\tldcontext.New(userKey), false)")),
			want: bom + crlf(body("client.BooleanValue(ctx, \"f\", // note\n false, openfeature.NewEvaluationContext(userKey, nil))")),
		},
		{
			name: "comments before and after the call stay where they are",
			src:  body(`/* before */ ldClient.BoolVariationCtx(ctx, "f", ldcontext.New(userKey), false) /* after */`),
			want: body(`/* before */ client.BooleanValue(ctx, "f", false, openfeature.NewEvaluationContext(userKey, nil)) /* after */`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := stubTree(t)
			files["flags/flags.go"] = tt.src
			dir, cleanup, err := analysistest.WriteFiles(files)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			results := analysistest.Run(t, dir, analyzer.Analyzer, "flags")
			if len(results) != 1 || len(results[0].Diagnostics) != 1 || len(results[0].Diagnostics[0].SuggestedFixes) != 1 {
				t.Fatalf("want exactly one diagnostic with one fix, got %+v", results)
			}
			edits := slices.Clone(results[0].Diagnostics[0].SuggestedFixes[0].TextEdits)
			slices.SortFunc(edits, func(a, b analysis.TextEdit) int { return int(b.Pos) - int(a.Pos) })
			got := []byte(tt.src)
			fset := results[0].Pass.Fset
			for _, e := range edits {
				got = slices.Concat(got[:fset.Position(e.Pos).Offset], e.NewText, got[fset.Position(e.End).Offset:])
			}
			if string(got) != tt.want {
				t.Fatalf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// TestAnalyzer_SuggestedFixes checks the raw edits of each fix, which
// RunWithSuggestedFixes cannot: it formats the result with the driver's
// remove-unused-imports pass, so an import removal that is missing (or
// present when it must not be) cannot fail a golden.
//
// Rule 7 exception: analyzer.Analyzer is one exported value, not a function,
// and the three tests of this file check structurally different things
// (golden output, metadata, raw edits), so they stay separate.
func TestAnalyzer_SuggestedFixes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pkg  string
		// removes has one entry per diagnostic that carries a fix, in report
		// order: the comma-joined import paths that fix deletes ("" for none).
		removes []string
		// noFix is the number of diagnostics expected to carry no fix.
		noFix int
	}{
		{name: "a fix that consumes the last use removes the import", pkg: "addctx", removes: []string{ldctx}},
		{name: "fixes that share the import leave it to the driver", pkg: "twocalls", removes: []string{"", ""}},
		{name: "a remaining use keeps the import", pkg: "keepldctx", removes: []string{""}},
		{name: "a lone import decl is removed", pkg: "loneimport", removes: []string{ldctx}},
		{name: "a parenthesized decl with one spec is removed", pkg: "parenonly", removes: []string{ldctx}},
		{name: "ldcontext alone in its group next to an added import", pkg: "overlap", removes: []string{ldctx}},
		{name: "a //line-adjusted position gets no fix", pkg: "lines", noFix: 1},
		{name: "a generated file gets no fix", pkg: "generated", noFix: 1},
		{name: "a package-qualified receiver import goes with its last use", pkg: "pkgrecv", removes: []string{ldctx + ",example.com/pkgrecv/clients"}},
		{name: "a package-qualified receiver import shared by two fixes stays", pkg: "pkgrecvtwo", removes: []string{"", ""}},
		{name: "the outer of nested rewrites has no fix", pkg: "nested", removes: []string{""}, noFix: 1},
		{name: "a comment with no home withholds the fix and keeps the imports", pkg: "unfixable", removes: []string{""}, noFix: 6},
	}
	candidates := []string{ldctx, "example.com/pkgrecv/clients", "example.com/pkgrecvtwo/clients"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := analysistest.Run(t, analysistest.TestData(), analyzer.Analyzer, tt.pkg)
			var got []string
			noFix := 0
			for _, res := range results {
				for _, d := range res.Diagnostics {
					if len(d.SuggestedFixes) == 0 {
						noFix++
						continue
					}
					edits := slices.Clone(d.SuggestedFixes[0].TextEdits)
					slices.SortFunc(edits, func(a, b analysis.TextEdit) int { return int(a.Pos) - int(b.Pos) })
					var removed []string
					for i, e := range edits {
						if i > 0 && edits[i-1].End > e.Pos {
							t.Errorf("overlapping edits [%d,%d) and [%d,%d)", edits[i-1].Pos, edits[i-1].End, e.Pos, e.End)
						}
						tf := res.Pass.Fset.File(e.Pos)
						src, err := res.Pass.ReadFile(tf.Name())
						if err != nil {
							t.Fatal(err)
						}
						old := src[tf.Offset(e.Pos):tf.Offset(e.End)]
						for _, c := range candidates {
							q := []byte(`"` + c + `"`)
							if bytes.Contains(old, q) && !bytes.Contains(e.NewText, q) && !slices.Contains(removed, c) {
								removed = append(removed, c)
							}
						}
					}
					got = append(got, strings.Join(removed, ","))
				}
			}
			if !slices.Equal(got, tt.removes) || noFix != tt.noFix {
				t.Fatalf("removal per fix = %q (no-fix diagnostics %d), want %q (%d)", got, noFix, tt.removes, tt.noFix)
			}
		})
	}
}
