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
  1-based; `rule-id` is an id from `spec/rules.yaml`. For a finding that is
  reported but not rewritten, the message is the skip category's message,
  suffixed with `[skip: <category>]`.
- `json`: a single JSON document on stdout:
  `{"version": 1, "findings": [...], "summary": {"errors": n, "warnings": n, "baselined": n}}`.
  Each finding has `file`, `line`, `col`, `ruleId`, `severity`, `message`,
  `fingerprint`, `skip` (category or absent), `baselined` (bool).
- `sarif`: SARIF 2.1.0; see [sarif.md](sarif.md).

An unknown `--format` is exit `2`.

## Flags

- `--quiet`: report only error-severity findings. Output only; does not change
  the exit code.
- `--max-warnings N`: see [exit-codes.md](exit-codes.md).

## Ordering and determinism

Output is a pure function of the inputs. Findings are sorted by `file`
(byte-wise, on the normalized path), then `line`, then `col`, then `ruleId`.
No timestamps, absolute paths, or machine-specific data appear in `text`,
`json` or `sarif` output. JSON object keys are emitted in a fixed order;
output ends with a single trailing newline. File discovery and processing
order must not influence the sort.
