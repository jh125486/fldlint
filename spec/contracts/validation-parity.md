# Validator parity

Status: planned; the loaders are part of the M0 harness work and are tested by
shared negative probes. Nothing here is a released guarantee yet.

The JS and Go spec loaders must accept and reject **the same inputs** for
`spec/rules.yaml` and every fixture `case.yaml`. A malformed spec must fail
the same way regardless of which language's tests load it.

## Who validates what

- **JS owns JSON-Schema validation.** It validates `rules.yaml` and each
  `case.yaml` against `spec/schema/*.schema.json` with ajv (`npm run
  spec:validate`).
- **Go does not use a JSON-Schema library and does not generate its checks from
  the schema.** It re-implements the same checks by hand in its loader.
- Parity is therefore a requirement, not a by-product, and it is verified by
  **shared negative probes**: a set of deliberately invalid spec inputs that
  both test suites feed to their loader, each of which must be rejected. A
  check added to one side without a probe, or a probe one side accepts, is a
  parity bug. See [ADR 003](../../docs/adr/003-rules-in-yaml-with-json-schema-not-proto.md).

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
the missing tag, when coverage is not met: the seven shape tags are mandatory
for the reference rule `bool-variation` only, and every other rewriting rule
needs at least one rewrite case and, where a language skips it, one skip case
([ADR 007](../../docs/adr/007-shape-tag-coverage-reference-rule-only.md)).
