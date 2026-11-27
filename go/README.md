# fldlint for Go

`go/analysis` analyzer that finds LaunchDarkly Go SDK v7 flag evaluations and
migrates the provably safe ones to OpenFeature through suggested fixes. Calls
it cannot prove safe are reported with a skip category and left alone. The
analyzer is `go/analyzer`; two binaries wrap it, and `fldlint-sarif` is an
optional converter (see [SARIF](#sarif-fldlint-sarif)).

| Binary | Built with | Use it for |
|--------|------------|------------|
| `fldlint` | `singlechecker` | standalone: `fldlint ./...` |
| `fldlint-fix` | `unitchecker` | `go vet -vettool=` and `go fix -fixtool=` only |

The binary is `fldlint-fix`, the name used in
[docs/m0-contract.md](../docs/m0-contract.md). It also reports findings under
`go vet`.

The Go tools have no `--max-warnings` and no severity setting. Exit codes are
the Go analysis framework's; see the Go section of
[spec/contracts/exit-codes.md](../spec/contracts/exit-codes.md) for the observed
table. Requires Go 1.26.0 or newer (`go/go.mod`).

## Install

From a checkout (works today):

```sh
cd go
go install ./cmd/fldlint ./cmd/fldlint-fix ./cmd/fldlint-sarif
```

UNVERIFIED until the first tag exists (#20): `go install
github.com/jh125486/fldlint/go/cmd/fldlint@vX.Y.Z`. The module lives in the
`go/` subdirectory, so tags are expected as `go/vX.Y.Z` and the version after
`@` is `vX.Y.Z`. Nothing is tagged and nothing is on the Go proxy yet.

## Standalone (`fldlint`)

```sh
fldlint ./...              # report; exit 3 when there are findings
fldlint -fix ./...         # rewrite files in place; exit 0
fldlint -fix -diff ./...   # preview the rewrite as a unified diff; files untouched
```

## `go vet`

```sh
go vet -vettool="$(command -v fldlint-fix)" ./...        # exit 1 on findings
go vet -vettool="$(command -v fldlint-fix)" -fix ./...   # apply fixes, exit 0
go vet -vettool="$(command -v fldlint-fix)" -fldlint=false ./...   # disable
```

`fldlint-fix` refuses direct invocation (`invoking "go tool fldlint-fix"
directly is unsupported`).

## `go fix`

```sh
go fix -fixtool="$(command -v fldlint-fix)" ./...        # apply fixes
go fix -diff -fixtool="$(command -v fldlint-fix)" ./...  # preview
```

`go fix -diff` exits 1 on a non-empty diff with go1.27.4, but exits 0 with
go1.26.0 (the `go.mod` floor). Do not rely on its exit code there. On go1.26.0
the `go help fix` text documents `-diff` and `-fixtool` but no exit-code rule.

## JSON

```sh
fldlint -json ./...
go vet -vettool="$(command -v fldlint-fix)" -json ./...
```

This is the analysis framework's JSON (`{package: {analyzer: [...]}}`, positions
as `file:line:col` strings, byte offsets in edits; `category` is the rule id),
not fldlint's own JSON format. The exit code is 0 even with findings.

## SARIF (`fldlint-sarif`)

`fldlint-sarif` is an optional extra. `fldlint` has no SARIF output (and does
not change); this command reads `fldlint -json` on stdin and writes SARIF 2.1.0
on stdout, per [`spec/contracts/sarif.md`](../spec/contracts/sarif.md).

```sh
set -o pipefail   # keep fldlint's status visible through the pipe
fldlint -json ./... | fldlint-sarif > fldlint.sarif
go vet -vettool="$(command -v fldlint-fix)" -json ./... 2>&1 | fldlint-sarif > fldlint.sarif
```

`go vet -json` prints one JSON object per package; `fldlint-sarif` reads any
number of them and merges them (observed with two packages).

What it does beyond reformatting:

- `startColumn` is in UTF-16 code units. `fldlint -json` reports byte columns,
  so the command re-reads each reported file and converts.
- `partialFingerprints["fldlint/fingerprint"]` is the shared fingerprint
  ([`fingerprint.md`](../spec/contracts/fingerprint.md)). The flag key is
  re-read from the call's key argument (a string literal, as in the analyzer),
  and `occ` is assigned per file by `(ruleId, key)` in `(line, col)` order.
  Identical diagnostics (a file analysed in two package variants) are
  collapsed first.
- `message.text` has the `[skip: ...]` / `[no-fix: comment]` suffix of the
  console message removed; the category goes to `properties.skip` (or
  `properties.noFix`), and `properties.risk` is the rule's risk.
- Results are sorted by file, line, column, rule id. Paths are relative to
  `-root` (default: the current directory) with `uriBaseId: "%SRCROOT%"`. A
  file outside the root (after resolving symlinks) is an error.

### Flags

| Flag | Meaning |
|------|---------|
| `-root dir` | Scan root that result paths are relative to. Default: the current directory. |
| `-baseline file` | A [baseline](../spec/contracts/baseline.md) file. Every result gets `baselineState`: `unchanged` if its fingerprint is in the file, else `new`. Without it, `baselineState` is omitted. |
| `-severity [rule=]level` | Configured severity (`error`, `warn`, `info`) mapped to SARIF `level` (`error`, `warning`, `note`). `level` alone sets the default; `rule=level` one rule. Repeatable; an unknown rule id is exit 2. Default: `warn`, so an unconfigured run reports `warning`. |

Version: `tool.driver.version` is the module version of a `go install`ed binary,
or `0.0.0` for a local build.

### Exit codes

These are this command's own; they do not depend on `fldlint`'s (which exits 0
with `-json`, see the exit-code notes in the Go section of
[`exit-codes.md`](../spec/contracts/exit-codes.md)). Findings, baselined or
not, never change them.

| Code | Meaning |
|------|---------|
| `0` | SARIF written (also when there are no findings). `-h` also exits 0. |
| `1` | The input cannot be used: empty or malformed JSON, an analysis error reported inside it (`fldlint -json` prints `{"pkg": {"fldlint": {"error": ...}}}` when packages do not load or type-check), an unknown rule id, a source file that cannot be read or parsed, or a path outside `-root`. Nothing is written to stdout. |
| `2` | Usage: unknown flag, positional argument, bad `-severity` (including an unknown rule id); or an invalid `-baseline` (missing, malformed, unknown field, `version` other than 1), as in [`baseline.md`](../spec/contracts/baseline.md). Nothing is scanned. |

When `fldlint` itself fails to load packages it exits 1 and prints an error
object (and its own message on stderr); with `set -o pipefail` the pipeline
then exits non-zero.

### Example

Run against the shared fixture `spec/fixtures/bool-variation/non-ascii-prefix`,
whose call follows `/* café 😀 */` on the same line: the byte column is 35 and
the UTF-16 column 32. The SDK stubs are the analyzer's test stubs (a GOPATH tree,
as `analysistest` uses):

```sh
W=$(mktemp -d); mkdir -p $W/src/flags
cp -r go/analyzer/testdata/src/github.com $W/src/
cp spec/fixtures/bool-variation/non-ascii-prefix/go/input.go $W/src/flags/
(cd go && go build -o $W/bin ./cmd/fldlint ./cmd/fldlint-sarif)
cd $W/src
export GOPATH=$W GO111MODULE=off GOWORK=off
set -o pipefail
../bin/fldlint -json ./flags | ../bin/fldlint-sarif -root .   # exit 0
```

```json
{
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "fldlint",
          "version": "0.0.0",
          "rules": [
            {
              "id": "bool-variation"
            }
          ]
        }
      },
      "columnKind": "utf16CodeUnits",
      "results": [
        {
          "ruleId": "bool-variation",
          "level": "warning",
          "message": {
            "text": "Migrate boolean flag \"cafe-menu\" to OpenFeature"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "flags/input.go",
                  "uriBaseId": "%SRCROOT%"
                },
                "region": {
                  "startLine": 20,
                  "startColumn": 32
                }
              }
            }
          ],
          "partialFingerprints": {
            "fldlint/fingerprint": "87978fb58d6fc554876accf055a31851"
          },
          "properties": {
            "risk": "low"
          }
        }
      ]
    }
  ]
}
```

With a baseline containing that fingerprint and `-severity error`, the same
result gets `"level": "error"` and `"baselineState": "unchanged"`:

```sh
printf '{\n  "version": 1,\n  "fingerprints": [\n    "87978fb58d6fc554876accf055a31851"\n  ]\n}\n' > ../baseline.json
../bin/fldlint -json ./flags | ../bin/fldlint-sarif -root . -baseline ../baseline.json -severity error
```

Failures, as observed:

```text
$ fldlint-sarif -bogus < /dev/null; echo "exit=$?"
flag provided but not defined: -bogus
usage: fldlint -json ./... | fldlint-sarif [-baseline file] [-root dir] [-severity [rule=]error|warn|info]...
...
exit=2

$ fldlint -json ./nonexistent | fldlint-sarif; echo "exit=$?"
fldlint-sarif: invalid input: analysis of nonexistent failed (fldlint): analysis skipped due to errors in package
exit=1
```

### Upload to GitHub code scanning

```sh
fldlint -json ./... | fldlint-sarif > fldlint.sarif
# then upload with github/codeql-action/upload-sarif (sarif_file: fldlint.sarif)
```

(Not run here: uploading needs a GitHub repository and token.)

### Limitations

- A `//line` directive or a cgo-generated file makes the analyzer report a
  position in another file than the one on disk. `fldlint-sarif` re-reads the
  reported file, so it either fails (exit 1, file missing) or reads the wrong
  text: not supported.
- A column past the end of its line, or a line past the end of the file, is
  exit 1.

## Not available yet

The golangci-lint module plugin is in [go/golangci](golangci/README.md) (usage
unverified until a tag exists). A real-SDK compile check module is tracked in
#18 and not implemented. Behaviour under gopls is unverified.

## Tests

`go test ./...` in this directory. `go/conformance` runs every shared fixture
that has a `go` expectation (diagnostics, UTF-16 columns, golden bytes, and a
type-check against the SDK stubs in `go/analyzer/testdata`).
