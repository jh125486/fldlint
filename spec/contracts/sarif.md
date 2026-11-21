# SARIF output

`--format sarif` emits SARIF 2.1.0 on stdout. Status: planned.

The SARIF shape is shared across languages. In the JS/TS tools it is the
ESLint `-f sarif` equivalent; in the Go tools it is an optional extra, not
default behaviour. Exit codes and flags are per ecosystem
([exit-codes.md](exit-codes.md)).

## Document

- `$schema`: `https://json.schemastore.org/sarif-2.1.0.json`, `version`: `"2.1.0"`.
- One `run`; `tool.driver.name` is `fldlint`. `tool.driver.version` is the CLI version.
- `tool.driver.rules` has one entry per rule in `spec/rules.yaml` that produced
  a result, with `id` equal to the `rules.yaml` rule id, sorted by id.
- Locations use `artifactLocation.uri` = normalized relative path (see
  [fingerprint.md](fingerprint.md#path-normalization)) and `uriBaseId: "%SRCROOT%"`;
  `region.startLine`/`startColumn` are 1-based. `startColumn` counts UTF-16
  code units from the start of the line with a tab as one column, which is
  SARIF's default `columnKind`; the run sets it explicitly (see the example).
  Go implementations must convert from byte offsets
  ([output-format.md](output-format.md#positions)).
- Results are in the deterministic order of [output-format.md](output-format.md).
  No timestamps or absolute paths.

## Example

```json
{
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  "version": "2.1.0",
  "runs": [{
    "tool": {"driver": {"name": "fldlint", "version": "0.0.0",
      "rules": [{"id": "bool-variation"}]}},
    "columnKind": "utf16CodeUnits",
    "results": [{
      "ruleId": "bool-variation",
      "level": "warning",
      "message": {"text": "Migrate boolean flag 'new-ui' to OpenFeature"},
      "locations": [{"physicalLocation": {
        "artifactLocation": {"uri": "src/flags.ts", "uriBaseId": "%SRCROOT%"},
        "region": {"startLine": 3, "startColumn": 18}}}],
      "partialFingerprints": {"fldlint/fingerprint": "<see fingerprint.md>"},
      "baselineState": "new",
      "properties": {"risk": "low"}
    }]
  }]
}
```

## Rule ids

`ruleId` is the `rules.yaml` rule id verbatim (`bool-variation`,
`bool-variation-detail`, ...). There is no language or tool prefix: the id is
language-independent, and `tool.driver.name` already namespaces it.

## Levels

- `level` is the finding's **configured** severity: `error` -> `error`,
  `warn` -> `warning`, info -> `note`. Severity is configuration, not data;
  the `rules.yaml` severities (all `warning`) are the `recommended` defaults,
  so an unconfigured run reports `warning`.
- `--quiet` (JS/TS) filters SARIF results exactly as it filters `text`, and
  never alters the exit code ([output-format.md](output-format.md#flags)).

## Skip categories

`result.message.text` is the category's `message` for a skipped finding and
the rule's `message` (with `{{key}}` substituted) for a rewritable one, the
same strings as the `text` formatter minus its `[skip: ...]` suffix
([output-format.md](output-format.md)). The category key is recorded
in `result.properties.skip` (e.g. `"dynamic-key"`). Results with no `skip`
property are rewritable. The rule's `risk` is in `result.properties.risk`.

## Fingerprints and baseline

- `partialFingerprints` has one key, `fldlint/fingerprint`, whose value is the
  fingerprint from [fingerprint.md](fingerprint.md). Code Scanning uses it to
  deduplicate across runs.
- With `--baseline`, `result.baselineState` is `"unchanged"` for baselined
  findings and `"new"` otherwise ([baseline.md](baseline.md)). Without
  `--baseline`, `baselineState` is omitted. SARIF has no summary object; the
  `summary.baselined`/`info` counters of the `json` format have no SARIF
  equivalent, consumers count by `baselineState` and `level`.
