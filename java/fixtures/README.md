# Java fixtures (drafts)

`primitive-typing/<case>/{Before,After}.java` pin ADR 009 D8: rewritten calls keep
primitive typing via `.booleanValue()`, `.intValue()` or `.doubleValue()`. `After.java`
shows the typing shape only; it does not compile as is, because the `LDContext` is
passed as the `EvaluationContext` (context conversion is ADR 006 and #7/#9). Issue #9
wires these into `spec/fixtures` `case.yaml` files and `RewriteTest`s.

The `ternary` case uses an `int : Integer` conditional (not `int : long`, which is `long`
either way, JLS 15.25). VERIFIED (javac 21, 2026-11-22): `false ? 5 : (Integer) null`
throws `NullPointerException` (numeric conditional, type `int`), while
`false ? (Integer) 5 : (Integer) null` returns `null` (reference conditional, type
`Integer`). Omitting `.intValue()` therefore changes behavior.
