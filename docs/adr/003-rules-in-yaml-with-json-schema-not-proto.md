# ADR 003: Rules in YAML with JSON Schema, not proto

Date: 2026-11
Status: Accepted; the YAML-typing and validation-duplication consequences are superseded by [ADR 011](011-parse-yaml-once-emit-canonical-json.md)
Amended 2026-11: corrected before any release; the Go validator is hand-written, not generated from the schema. YAML scalar typing is no longer a deferred divergence: the [contract](../../spec/contracts/validation-parity.md#fldlint-yaml-subset) pins a YAML subset and the suites share YAML probes, which supersedes the statements below that the suites share no probes and that typing is not fixed.

> Historical: the Go YAML parsing described under Decision was removed by [ADR 011](011-parse-yaml-once-emit-canonical-json.md); only the JS loader reads YAML now.

## Context

The shared rule table needs a source format with a validator and per-language codegen. Protobuf is a common choice for language-neutral schemas, but rules are authored and reviewed by hand, need comments, and carry code templates with placeholders.

## Decision

Author rules in `spec/rules.yaml`, validated by JSON Schema (draft 2020-12) in `spec/schema/`. YAML supports comments and readable multi-line templates; JSON Schema validates both rules and fixture `case.yaml` files with a validator in JS (ajv). Go does not generate its checks from the schema: it parses with yaml.v3 and re-implements the same checks by hand, and parity between the two is a stated requirement ([validation-parity.md](../../spec/contracts/validation-parity.md)) kept honest by each suite's own negative tests (the JS and Go suites do not share a probe set). Template placeholders are a closed enum. Each language generates its own typed rule table (the table only, not the validation); no proto toolchain is introduced.

## Consequences

- Low authoring friction and easy review of rule diffs.
- No protoc or generated-code dependency in any language.
- We lose proto's compile-time cross-language types; the schema, codegen and `make spec-check` drift checks stand in for them.
- Superseded by [ADR 011](011-parse-yaml-once-emit-canonical-json.md): the JS loader is the only validator and the only YAML reader; other languages read the emitted `spec/generated/spec.json`, so there are no two implementations to drift.
- Superseded by [ADR 011](011-parse-yaml-once-emit-canonical-json.md): YAML implicit typing is handled once, by the JS loader enforcing the YAML subset, so no second parser can disagree.
