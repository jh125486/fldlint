# ADR 010: Planner step order and receiver removal

Date: 2026-11
Status: Accepted

## Context

A rewrite replaces `ld.BoolVariation(...)` with an OpenFeature call, so the
LaunchDarkly client expression disappears from that call. If it was a function
call it may have had side effects; if it was a local variable it may have no
remaining use, which Go rejects ("declared and not used"). Skip categories also
overlapped (an unproven default on a call with no OpenFeature client), so
engines could report different categories for the same call.

## Decision

1. One step order in every language, first match wins: rule-level skip,
   `dynamic-key`, `context-unproven`, `default-type-unproven`, `unbound-client`,
   `receiver-unproven`, else rewrite.
2. `receiver-unproven`: a call that passed steps 1-5 is skipped when its
   receiver is not pure (an identifier or a selector chain of identifiers and
   fields; no call, index, type assertion or explicit dereference), or when a
   local variable in the receiver would have no remaining use after the
   rewrite. A remaining use is any read that is not the receiver of another
   call still planned as a rewrite; assignment targets (`=`, `:=`, `op=`, `++`)
   do not count. When a local would be stranded, every rewritable call on it is
   skipped, never a subset. Package-level vars, struct fields and parameters
   never strand.
3. We never delete declarations, insert `_ = x`, or add `Close()` calls. A
   client's lifecycle (`MakeClient` connects and blocks, returns `(client,
   err)`, needs `Close`) is not ours to change.
4. Each Go `SuggestedFix` is sound alone and in any combination (drivers merge
   fixes diff3-style; editors apply one at a time). A fix removes an import
   only if that fix removes the file's last remaining use; batch drivers clean
   up the rest. The conformance harness compiles each fix alone and the union.
5. A default is proven by its static type where the language has one (Go: a
   `bool` variable or typed constant is fine) and by its literal kind where it
   does not (JS/TS: only a literal of the right kind is proven). A default that
   is not proven, including a literal of the wrong kind (e.g. `"yes"` for a
   boolean rule), is `default-type-unproven`. Rule-level
   `type-mismatch` remains only for rule-level mismatches; M0.2a forbids it on
   rewriting rules.
6. JS emits `receiver-unproven` only for impure receivers (it detects only
   identifier receivers today). Unused locals are lint, not compile, errors,
   but TypeScript `--noUnusedLocals` (TS6133) makes them errors for such users.
   Java applies the purity rule (unused Java locals compile).
7. Java constraint only: rewrites preserve primitive typing by emitting
   unboxing (`.booleanValue()`, `.intValue()`, `.doubleValue()`). Details live
   in ADR 009.

## Consequences

- The same call gets the same category in JS, Go and Java; fixtures pin the
  order (`skip-order-default-before-unbound`).
- Some safe-looking rewrites are skipped (a local client used only once in Go);
  the user migrates those by hand, which is the intended trade for never
  producing code that fails to compile.
- Go fixes can be applied individually by editors without breaking the build.
