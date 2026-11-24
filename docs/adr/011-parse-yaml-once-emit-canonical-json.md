# ADR 011: Parse YAML in exactly one place and emit canonical JSON

Date: 2026-11
Status: Accepted

Supersedes the YAML-typing consequence and the "two implementations of the
validation" consequence of [ADR 003](003-rules-in-yaml-with-json-schema-not-proto.md).

## Context

ADR 003 authors the spec in YAML. To keep two independent YAML parsers (the
`yaml` npm package in JS, `gopkg.in/yaml.v3` in Go) in agreement, the repo
pinned a restricted "fldlint YAML subset", re-implemented it by hand in Go,
shared a probe corpus, and kept the JS and Go validators in step with parity
tests. A third parser (Java) would have meant a third copy. Even so, the
contract kept a growing "known limits" list of inputs the parsers still read
differently. The cost was all spent on making parsers agree about a format the
engines never needed to parse at run time.

A proposal in the discussion of PR #39 pointed out the way out: parse once.

## Decision

The JS loader (`js/packages/spec`) is the only YAML reader. It enforces the
subset, validates `spec/rules.yaml` and every `spec/fixtures/*/*/case.yaml`
against the JSON Schemas, runs the semantic checks (including fixture-file
presence), and emits `spec/generated/spec.json`:

```json
{"rules": {...}, "cases": [{"path": "<rule>/<case>", "case": {...}}]}
```

Cases are sorted by path, key order is the source order, indent is two spaces
and the file ends in a newline. `npm run gen -w @fldlint/spec` writes it and
refuses to write anything when validation fails. It is committed and listed in
`GEN_FILES`, so `task spec:check` fails on drift.

Go (and later Java) read only that JSON, decoding with a basic structural check
(unknown fields and trailing data are rejected; the file is generated and
drift-checked, so it is not a hostile-input boundary), and contain no YAML code. They re-validate nothing the JS loader
already guarantees; they keep only the few references their own code relies on
(for example that a case's rule exists). The YAML probes in `spec/probes/yaml/`
are asserted by the JS suite only.

## Consequences

- One YAML parser, one validator. The parser-disagreement problem and its
  "known limits" list no longer exist; the subset remains as the rules that
  one loader enforces, for authors and for a stable, reviewable spec language.
- `gopkg.in/yaml.v3` leaves the Go module; Java needs only a JSON library
  (Jackson or Gson), not SnakeYAML.
- The Go loader and its YAML-subset and validator-parity tests are deleted.
- A new language needs no spec-parsing logic beyond JSON decoding.
- Node and the JS toolchain are required to regenerate the spec output, but
  not to build or test Go from a clean checkout, because the JSON is committed.
- `spec.json` is generated output: edit the YAML and regenerate, never the JSON.
