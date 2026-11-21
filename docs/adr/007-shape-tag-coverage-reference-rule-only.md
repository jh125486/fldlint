# ADR 007: Mandatory shape-tag coverage applies to the reference rule only

Date: 2026-11
Status: Accepted

## Context

The seven shape tags (`quote-single`, `quote-double`, `typed-decl`, `multi-declarator`, `shadowed-client`, `dynamic-key`, `aliased-import`) exercise how a call site is detected, not which method it calls. Detection is shared engine behaviour. Requiring all seven for every rule multiplies near-identical fixtures across about two dozen rules for no added signal.

## Decision

The seven tags are mandatory for the reference rule `bool-variation` only. Every other rewriting rule needs at least one rewrite case and, where a language skips it, at least one skip case, in a directory named for the rule. `npm run spec:tags` enforces this and names the rule and tag that is missing; `make fixture-tags` calls it.

## Consequences

- Small fixture set that stays reviewable.
- A detection bug that only affects one method's shape would not be caught by tags alone; per-rule rewrite and skip cases are the backstop.
- If a rule gains method-specific detection behaviour, add cases for it explicitly.
