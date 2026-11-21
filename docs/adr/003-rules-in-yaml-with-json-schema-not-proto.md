# ADR 003: Rules in YAML with JSON Schema, not proto

Date: 2026-11
Status: Accepted

## Context

The shared rule table needs a source format with a validator and per-language codegen. Protobuf is a common choice for language-neutral schemas, but rules are authored and reviewed by hand, need comments, and carry code templates with placeholders.

## Decision

Author rules in `spec/rules.yaml`, validated by JSON Schema (draft 2020-12) in `spec/schema/`. YAML supports comments and readable multi-line templates; JSON Schema validates both rules and fixture `case.yaml` files with widely available validators (ajv in JS, yaml.v3 plus generated checks in Go). Template placeholders are a closed enum. Each language generates its own typed table; no proto toolchain is introduced.

## Consequences

- Low authoring friction and easy review of rule diffs.
- No protoc or generated-code dependency in any language.
- We lose proto's compile-time cross-language types; the schema, codegen and `make spec-check` drift checks stand in for them.
- YAML pitfalls (implicit typing) are mitigated by the schema.
