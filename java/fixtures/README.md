# Java fixtures (drafts)

`primitive-typing/<case>/{Before,After}.java` pin ADR 009 D8: rewritten calls keep
primitive typing via `.booleanValue()`, `.intValue()` or `.doubleValue()`. `After.java`
shows the typing shape only; it does not compile as is, because the `LDContext` is
passed as the `EvaluationContext` (context conversion is ADR 006 and #7/#9). Issue #9
wires these into `spec/fixtures` `case.yaml` files and `RewriteTest`s.
