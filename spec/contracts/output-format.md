# Output format

Status: planned; not yet implemented.

**Shared vs per ecosystem.** The finding shape, messages, positions, ordering
and the `json`/`sarif` document shapes below are the shared contract. Flags,
exit codes and severity configuration are per ecosystem; see
[exit-codes.md](exit-codes.md). The `--format`/`--quiet`/`--max-warnings`
flags described here are the JS/TS (ESLint-style) tools'; the Go tools use the
`go/analysis` flags (`-json`, `-c`, `-fix`) and may offer SARIF as an extra.

## Streams

- **stdout**: results only (the findings in the selected format). Safe to pipe
  to `jq` or redirect to a file.
- **stderr**: everything else: progress, summaries, warnings about the run,
  usage and configuration errors.

## Formatters (`--format`)

- `text` (default): one line per finding:

  ```
  file:line:col  rule-id  message
  ```

  Two spaces between fields. `file` is the normalized relative path (see
  [fingerprint.md](fingerprint.md#path-normalization)); `line` and `col` are
  1-based (see [Positions](#positions)); `rule-id` is an id from `spec/rules.yaml`.

  Messages: a **rewritable** finding uses the rule's `message` field in
  `spec/rules.yaml` (placeholder `{{key}}` only, substituted with the flag key
  source text). A finding that is **reported but not rewritten** uses the skip
  category's `message` from `skipCategories`, suffixed with
  `[skip: <category>]`. The same strings are the `message` in `json` output.
  A third state, **rewritable but unfixable**, is a rewritable finding whose
  fix is withheld because a comment inside the call has no home in the rewrite
  ([placeholders.md](placeholders.md#comments-inside-a-call)). It is not a
  skip: it uses the rule's own `message` (key substituted), suffixed with
  `[no-fix: comment]`, and carries no `skip`. In `json` it has an optional
  `noFix: "comment"` field (absent otherwise; additive, `version` stays 1).
  Engines that do not emit `json` or `sarif` yet only emit the text form.
- `json`: a single JSON document on stdout:
  `{"version": 1, "findings": [...], "summary": {"errors": n, "warnings": n, "info": n, "baselined": n}}`.
  `errors`, `warnings` and `info` count **non-baselined** findings by their
  **configured** severity (ESLint `error`/`warn`; `info` for severity `info`);
  baselined findings are excluded from those three and counted only in
  `baselined`. Example:

  ```json
  {
    "version": 1,
    "findings": [
      {"file": "src/flags.ts", "line": 3, "col": 18, "ruleId": "bool-variation",
       "severity": "warning", "message": "Migrate boolean flag 'new-ui' to OpenFeature",
       "fingerprint": "<see fingerprint.md>", "baselined": false},
      {"file": "src/old.ts", "line": 9, "col": 5, "ruleId": "bool-variation",
       "severity": "warning", "message": "Migrate boolean flag 'legacy' to OpenFeature",
       "fingerprint": "<see fingerprint.md>", "baselined": true}
    ],
    "summary": {"errors": 0, "warnings": 1, "info": 0, "baselined": 1}
  }
  ```

  The `skip` key is shown absent because both findings are rewritable. Each finding has `file`, `line`, `col`, `ruleId`, `severity`, `message`,
  `fingerprint`, `skip` (category or absent), `baselined` (bool).
- `sarif`: SARIF 2.1.0; see [sarif.md](sarif.md).

An unknown `--format` is exit `2`.

## Flags

JS/TS tools (ESLint semantics):

- `--quiet`: report errors only, like ESLint. It affects printing only and
  never changes the exit code.
- `--max-warnings N`: see [exit-codes.md](exit-codes.md). Default `-1`
  (unlimited), exactly like ESLint.

Severity note: a finding's severity is configuration (`off`/`warn`/`error`
per rule). The severities in `spec/rules.yaml` are the defaults of the
`recommended` preset, which sets every rule to `warn`. The `severity` field
in `json` output and `level` in SARIF carry the configured value.

## Positions

`line` and `col` are 1-based. `col` counts **UTF-16 code units** from the start
of the line, and a tab counts as one column. This is SARIF's default
`columnKind` (`utf16CodeUnits`), so `text`, `json` and `sarif` all report the
same position. Engines whose native offsets are bytes (Go) must convert from
byte offsets to UTF-16 code units; a non-ASCII character before the call on the
same line (a fixture covers this) is what exposes a missing conversion.

## Fix mode

A run that applies fixes uses the same exit-code and summary rules as a check
run, evaluated on what **remains** after fixing (like ESLint `--fix`): the
summary counts and the printed findings are the ones left unfixed. (JS/TS
tools; Go `-fix` follows the Go checker, see exit-codes.md.) See
[exit-codes.md](exit-codes.md).

## Ordering and determinism

Output is a pure function of the inputs. Findings are sorted by `file`
(byte-wise, on the normalized path), then `line`, then `col`, then `ruleId`.
No timestamps, absolute paths, or machine-specific data appear in `text`,
`json` or `sarif` output. JSON object keys are emitted in a fixed order;
output ends with a single trailing newline. File discovery and processing
order must not influence the sort.
