# Output format

Applies to every fldlint CLI. Status: planned; not yet implemented.

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
  1-based; `rule-id` is an id from `spec/rules.yaml`.

  Messages: a **rewritable** finding uses the rule's `message` field in
  `spec/rules.yaml` (placeholder `{{key}}` only, substituted with the flag key
  source text). A finding that is **reported but not rewritten** uses the skip
  category's `message` from `skipCategories`, suffixed with
  `[skip: <category>]`. The same strings are the `message` in `json` output.
- `json`: a single JSON document on stdout:
  `{"version": 1, "findings": [...], "summary": {"errors": n, "warnings": n, "baselined": n}}`.
  Each finding has `file`, `line`, `col`, `ruleId`, `severity`, `message`,
  `fingerprint`, `skip` (category or absent), `baselined` (bool).
- `sarif`: SARIF 2.1.0; see [sarif.md](sarif.md).

An unknown `--format` is exit `2`.

## Flags

- `--quiet`: suppress the printing of findings **below the failing
  threshold**. Errors always fail the run, so they are always printed.
  Warnings are printed when they are what fails the run (non-baselined
  warnings exceed `--max-warnings`) and suppressed when they are not (within
  the allowance, or `--max-warnings -1`). `--quiet` affects printing only: it
  never changes the exit code, and the summary line still prints to stderr.
- `--max-warnings N`: see [exit-codes.md](exit-codes.md). Default `0`, so any
  non-baselined warning fails the run and `--quiet` therefore hides nothing by
  default; `--quiet` only has an effect together with a positive `N` or `-1`.

Severity note: every skip category and every rewritable finding is `warning`
today. Because `--quiet` prints whatever fails the run, a run that exits `1`
never prints nothing (the old "error-severity only" definition did, which
contradicted the exit code).

## Ordering and determinism

Output is a pure function of the inputs. Findings are sorted by `file`
(byte-wise, on the normalized path), then `line`, then `col`, then `ruleId`.
No timestamps, absolute paths, or machine-specific data appear in `text`,
`json` or `sarif` output. JSON object keys are emitted in a fixed order;
output ends with a single trailing newline. File discovery and processing
order must not influence the sort.
