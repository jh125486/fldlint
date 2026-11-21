# Exit codes

Applies to every fldlint CLI (JS and Go). Status: planned; not yet implemented.

| Code  | Meaning |
|-------|---------|
| `0`   | Clean: no reportable problems (see [baseline.md](baseline.md) and `--max-warnings` below). |
| `1`   | Problems found: at least one error-severity finding, or warnings exceeded `--max-warnings`. |
| `2`   | Usage or configuration error: unknown flag, invalid `--format`, unreadable/invalid config, a missing explicit `--config` path, missing or malformed baseline file. Nothing is scanned. |
| `130` | Interrupted by SIGINT (128 + 2). |

Rules:

- Absence of a config file found by auto-discovery is not an error. A path
  passed explicitly with `--config` that does not exist is exit `2`.
- Findings that are matched by `--baseline` do not count toward exit `1`.
- `--quiet` changes what is printed only ([output-format.md](output-format.md#flags));
  the exit code is computed as if it were absent, and the summary line still
  prints to stderr.
- `--max-warnings N`: exit `1` if the number of non-baselined warnings exceeds
  `N`. Default is `0` for the CLI (any finding fails the run); `-1` disables
  the check. This deliberately differs from ESLint, where the default is
  unlimited, because every fldlint finding is migration work. Every skip category and
  every rewritable finding is `warning` severity, so the default `0` means any
  finding fails the run; with `--quiet` the failing warnings are still printed.
- Unexpected internal errors (bugs) also exit `2` with a message on stderr
  asking for a bug report; there is no separate internal-error code.
- A run that rewrites files (codemod) exits `0` when it completed, even if it
  reported skipped findings; skipped findings are listed on stdout.

## Deliberate difference from flaglint

flaglint (ADR 010 in flaglint-js, ADR 003 in flaglint-go) defines `scan` and
`audit` as inventory commands that never exit `1` for findings, reserves `1`
for `validate`, and uses `3` for internal errors. fldlint is a linter, not an
inventory tool: finding problems is the point, so it follows the ESLint
convention. See [ADR 004](../../docs/adr/004-exit-codes-follow-eslint.md).
Baseline files and fingerprints are not interchangeable with flaglint's.
