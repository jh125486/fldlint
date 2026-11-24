# ADR 003: Rules in YAML with JSON Schema, not proto

Date: 2026-11
Status: Accepted; superseded in part by [ADR 011](011-parse-yaml-once-emit-canonical-json.md)
Amended 2026-11: corrected before any release; the Go validator is hand-written, not generated from the schema. YAML scalar typing is no longer a deferred divergence: the [contract](../../spec/contracts/validation-parity.md#fldlint-yaml-subset) pins a YAML subset and the suites share YAML probes, which supersedes the statements below that the suites share no probes and that typing is not fixed.

Superseded in part: the YAML-typing and validation-duplication consequences below no longer apply; only the JS loader reads YAML.

## Context

The shared rule table needs a source format with a validator and per-language codegen. Protobuf is a common choice for language-neutral schemas, but rules are authored and reviewed by hand, need comments, and carry code templates with placeholders.

## Decision

Author rules in `spec/rules.yaml`, validated by JSON Schema (draft 2020-12) in `spec/schema/`. YAML supports comments and readable multi-line templates; JSON Schema validates both rules and fixture `case.yaml` files with a validator in JS (ajv). Go does not generate its checks from the schema: it parses with yaml.v3 and re-implements the same checks by hand, and parity between the two is a stated requirement ([validation-parity.md](../../spec/contracts/validation-parity.md)) kept honest by each suite's own negative tests (the JS and Go suites do not share a probe set). Template placeholders are a closed enum. Each language generates its own typed rule table (the table only, not the validation); no proto toolchain is introduced.

## Consequences

- Low authoring friction and easy review of rule diffs.
- No protoc or generated-code dependency in any language.
- We lose proto's compile-time cross-language types; the schema, codegen and `make spec-check` drift checks stand in for them.
- Two implementations of the validation can drift; each suite's own negative tests catch only what that suite thought to test, so gaps are closed in review.
- YAML implicit typing is **not** mitigated by the schema, which validates what the parser produced. The two parsers differ on some scalars (octal/binary/underscore/large integers, timestamps, custom tags, `%YAML 1.1`; e.g. `col: 017` reads as 17 in one and 15 in the other). This is a known issue tracked as deferred; it is not fixed.
