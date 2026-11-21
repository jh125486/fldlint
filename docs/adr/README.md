# Architecture decision records

ADRs record decisions and why they were made. They are immutable once
accepted: do not edit one to change the decision. (Exception: ADRs 001-007
were never released and ADR 003 was corrected in place, marked "Amended
2026-11" in its header.) To change course, add a new
ADR that supersedes it and set the old one's status to
`Superseded by NNN` (that status line is the only permitted edit).

Format: title, `Date`, `Status`, then Context, Decision, Consequences. Keep
them short. Number sequentially.

| ADR | Title |
|-----|-------|
| [001](001-monorepo-with-shared-spec.md) | Monorepo with a shared spec |
| [002](002-spec-as-data-not-shared-engine.md) | Share the spec as data, not an engine |
| [003](003-rules-in-yaml-with-json-schema-not-proto.md) | Rules in YAML with JSON Schema, not proto |
| [004](004-exit-codes-follow-eslint.md) | Exit codes follow ESLint |
| [005](005-attribution-and-porting-policy.md) | Attribution and porting policy |
| [006](006-context-conversion-skipped-unless-provable.md) | Context conversion is skipped unless provable |
| [007](007-shape-tag-coverage-reference-rule-only.md) | Mandatory shape-tag coverage applies to the reference rule only |
