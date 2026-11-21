# ADR 006: Context conversion is skipped unless provable

Date: 2026-11
Status: Accepted

## Context

A LaunchDarkly context can be multi-kind, carry custom and private attributes, and be built dynamically. An OpenFeature `EvaluationContext` has a targeting key plus a flat attribute map. A rewrite that changes the context argument can silently change who a flag targets, which is exactly the kind of regression a migration tool must not introduce. The first scaffold only skipped Go; JS rewrote complex contexts.

## Decision

Rewrite a call's context argument only when the LaunchDarkly context is a simple single-kind literal; otherwise report the call as `skip: context-unproven`. In JS the argument must be an object literal whose only properties are `key` and optionally `kind: "user"`, emitted as `{ targetingKey: <key expr> }`. In Go it must be a direct `ldcontext.New(<expr>)`, emitted as `openfeature.NewEvaluationContext(<expr>, nil)`. Every rewriting rule in both languages carries `requires: [context-unproven]`. Details in [placeholders.md](../../spec/contracts/placeholders.md).

## Consequences

- Safe by default: no silent change of targeting.
- Many real call sites (contexts held in variables, builders, multi-kind) are reported, not migrated, until a later decision proves more shapes.
- The JS `complex-context` fixture becomes a skip, matching Go.
- Widening the policy later needs fixtures and a new ADR.
