# fldlint golangci-lint module plugin

Runs the fldlint analyzer inside golangci-lint (v2) as a
[module plugin](https://golangci-lint.run/docs/plugins/module-plugins/). It is a
separate Go module (`github.com/jh125486/fldlint/go/golangci`) so that users of
the analyzer library do not depend on
`github.com/golangci/plugin-module-register` (v0.1.2).

The plugin registers the linter `fldlint` and loads type information (call
sites are identified by type). It has no settings; an unknown key under
`settings` is rejected.

## Use

1. `.custom-gcl.yml` (pin the golangci-lint version and the fldlint tag):

   ```yaml
   version: v2.15.0
   plugins:
     - module: 'github.com/jh125486/fldlint/go/golangci'
       import: 'github.com/jh125486/fldlint/go/golangci'
       version: vX.Y.Z
   ```

2. `golangci-lint custom` builds `./custom-gcl`.
3. `.golangci.yml`:

   ```yaml
   version: "2"
   linters:
     default: none
     enable:
       - fldlint
     settings:
       custom:
         fldlint:
           type: module
           description: Report LaunchDarkly Go SDK calls that can migrate to OpenFeature
   ```

4. `./custom-gcl run ./...` reports findings; `./custom-gcl run --fix ./...`
   applies the suggested fixes. golangci-lint decides severity and the exit code
   (see `spec/contracts/exit-codes.md`); use its `new-from-rev` / `new-from-patch`
   instead of an fldlint baseline.

Working examples are under [testdata](testdata): `.custom-gcl.yml` (local
`path:` form used in this repository), `.golangci.yml` and a `smoke` module
with SDK stubs. `task go:plugin-smoke` builds and runs them (needs network).

In this repository the analyzer is unreleased, so `.custom-gcl.yml` lists the
analyzer module too with `path:`: the `replace` in `go.mod` is not honoured by
the generated build module.
