# SARIF output

`--format sarif` emits SARIF 2.1.0 on stdout. Status: planned.

## Document

- `$schema`: `https://json.schemastore.org/sarif-2.1.0.json`, `version`: `"2.1.0"`.
- One `run`; `tool.driver.name` is `fldlint`. `tool.driver.version` is the CLI version.
- `tool.driver.rules` has one entry per rule in `spec/rules.yaml` that produced
  a result, with `id` equal to the `rules.yaml` rule id, sorted by id.
- Locations use `artifactLocation.uri` = normalized relative path (see
  [fingerprint.md](fingerprint.md#path-normalization)) and `uriBaseId: "%SRCROOT%"`;
  `region.startLine`/`startColumn` are 1-based.
- Results are in the deterministic order of [output-format.md](output-format.md).
  No timestamps or absolute paths.

## Rule ids

`ruleId` is the `rules.yaml` rule id verbatim (`bool-variation`,
`bool-variation-detail`, ...). There is no language or tool prefix: the id is
language-independent, and `tool.driver.name` already namespaces it.

## Levels

- A finding with a skip category takes its level from
  `skipCategories.<category>.severity`: `error` -> `error`, `warning` ->
  `warning`, `info` -> `note`.
- A finding that is rewritable (no skip) has level `warning`.

## Skip categories

`result.message.text` is the category's `message`. The category key is recorded
in `result.properties.skip` (e.g. `"dynamic-key"`). Results with no `skip`
property are rewritable. The rule's `risk` is in `result.properties.risk`.

## Fingerprints and baseline

- `partialFingerprints` has one key, `fldlint/fingerprint`, whose value is the
  fingerprint from [fingerprint.md](fingerprint.md). Code Scanning uses it to
  deduplicate across runs.
- With `--baseline`, `result.baselineState` is `"unchanged"` for baselined
  findings and `"new"` otherwise ([baseline.md](baseline.md)). Without
  `--baseline`, `baselineState` is omitted.
