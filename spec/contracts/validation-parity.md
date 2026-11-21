# Validator parity

Status: planned; the loaders are part of the M0 harness work and each suite
has its own negative tests. Nothing here is a released guarantee yet.

This concerns the shared spec data only; it says nothing about tool exit codes
or flags, which are per ecosystem ([exit-codes.md](exit-codes.md)).

The JS and Go spec loaders must accept and reject **the same inputs** for
`spec/rules.yaml` and every fixture `case.yaml`. A malformed spec must fail
the same way regardless of which language's tests load it.

## Who validates what

- **JS owns JSON-Schema validation.** It validates `rules.yaml` and each
  `case.yaml` against `spec/schema/*.schema.json` with ajv (`npm run
  spec:validate`).
- **Go does not use a JSON-Schema library and does not generate its checks from
  the schema.** It re-implements the same checks by hand in its loader.
- Parity is therefore a requirement, not a by-product. The JS and Go test
  suites do **not** share a probe set: each keeps its own negative tests
  (deliberately invalid inputs that its loader must reject). A check added to
  one side without a matching negative test on the other is a parity gap to be
  closed in review. See
  [ADR 003](../../docs/adr/003-rules-in-yaml-with-json-schema-not-proto.md).

## Known divergence: YAML scalar typing (deferred)

ajv's YAML parser (`yaml`) and Go's `yaml.v3` do not type every scalar the
same way. Known differences: octal, binary, underscore-separated and very large
(e.g. `1e17`) integers, timestamps, custom tags, and `%YAML 1.1` documents. For
example `col: 017` reads as 17 in one parser and 15 in the other. The schema
does **not** mitigate this (it validates what the parser produced). This is
**not fixed**; it is tracked as a deferred issue. Until then, specs should use
plain decimal integers and untagged scalars.

## Fixture discovery and semantic rules (both validators)

- Every directory at `spec/fixtures/<rule>/<case>/` must contain `case.yaml`
  (not `case.yml`). Any other directory depth, or a directory without
  `case.yaml`, is an error. Case ids are unique across the whole spec.
- A case's `rule` must exist. Every `diagnostics[].ruleId` must equal the
  case's `rule`.
- `diagnostics[].skip`, when present, must equal that rule's own `skip`; a rule
  with `skip` requires every diagnostic to carry it. A rewriting rule's
  diagnostics carry no `skip`, unless the language has no template for it, in
  which case the language must not appear in `expect` at all.
- `skip` on a diagnostic implies `output: false`. `output: true` requires at
  least one diagnostic, and the output file must differ from the input file.
- `expect.<lang>` is only allowed when the rule binds that language
  (`languages.<lang>` exists).
- An explicit spec root (`-spec` in Go, `--spec-root` in JS) that is missing or
  invalid is a **hard error**, never a silent fallback to the stub. The stub
  fallback applies only when no spec root is given at all.
- The Go message regex rejects a lone `}`, like the schema (`[^{}]`).

## Required checks (both validators)

Structure (schema-level):

1. Reject unknown keys at every level.
2. Enforce enums (`valueType`, `risk`, `skip`/skip-category names, placeholder
   names), patterns and required fields.
3. `diagnostics[].line` and `col` are integers (1-based).
4. `expect.<lang>.output` is a boolean.
5. Language entries (`languages.<lang>`, `expect.<lang>`) are non-null.

Cross-reference (beyond what the schema can say):

6. A case's `rule`, and every `diagnostics[].ruleId`, must exist in `rules.yaml`.
7. `ctxArg` may only name a method listed in that language's `methods`.
8. The same method must not appear in two rules for one language.
9. A case that promises `expect.<lang>` must have that language's input file
   (`js/input.ts`, `go/input.go`).
10. `output: true` requires the output file (`output.ts` / `output.go.golden`),
    and an output file without `output: true` is rejected (and vice versa).

## Fixture coverage (separate from the loaders)

`npm run spec:tags` (called by `make fixture-tags`) fails, naming the rule and
what is missing, when coverage is not met:

- `bool-variation` (the reference rule) covers the seven shape tags.
- Every rewriting rule (no `skip`) needs at least one rewrite case
  (`output: true`, at least one diagnostic) for **each language that has a
  template**.

There is no separate skip-case requirement
([ADR 007](../../docs/adr/007-shape-tag-coverage-reference-rule-only.md)).
