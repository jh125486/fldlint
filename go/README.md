# fldlint for Go

`go/analysis` analyzer that finds LaunchDarkly Go SDK v7 flag evaluations and
migrates the provably safe ones to OpenFeature through suggested fixes. Calls
it cannot prove safe are reported with a skip category and left alone. The
analyzer is `go/analyzer`; two binaries wrap it.

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
go install ./cmd/fldlint ./cmd/fldlint-fix
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

## Not available yet

golangci-lint module plugin, a real-SDK compile check module and SARIF output
are tracked in #18 and not implemented. Behaviour under golangci-lint and gopls
is unverified.

## Tests

`go test ./...` in this directory. `go/conformance` runs every shared fixture
that has a `go` expectation (diagnostics, UTF-16 columns, golden bytes, and a
type-check against the SDK stubs in `go/analyzer/testdata`).
