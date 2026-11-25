# Spec validation and the YAML subset

Status: the single loader is part of the M0 harness; the YAML-subset probes
are asserted by its suite. Nothing here is a released guarantee yet.

This concerns the shared spec data only; it says nothing about tool exit codes
or flags, which are per ecosystem ([exit-codes.md](exit-codes.md)).

## One loader, one JSON output

The spec is authored as YAML (`spec/rules.yaml` and every fixture
`case.yaml`) but **parsed in exactly one place**: the JS loader in
`js/packages/spec`. It enforces the YAML subset below, validates against
`spec/schema/*.schema.json` with ajv, and runs the semantic checks (the
cross-reference and fixture-file rules below). It then emits
**`spec/generated/spec.json`**:

```json
{"rules": <rules.yaml as JSON>, "cases": [{"path": "<rule>/<case>", "case": <case.yaml as JSON>}]}
```

Cases are sorted by path, key order is the source order, indent is two spaces,
the file ends in a newline. `npm run gen -w @fldlint/spec` (called by
`task spec:gen`) writes it and **refuses to write anything when validation
fails**. It is committed, and `task spec:check` fails on drift.

Every other language (Go and Java) **must read `spec.json` and needs
no YAML code and no validator**: decode the JSON (reject unknown fields and trailing data; the file is generated and drift-checked, so this is a sanity check, not a validator), and
keep only the few cross-checks its own code relies on. Java (the `java/` conformance
harness, test scope) reads it with Jackson, not SnakeYAML, and does not validate
against the JSON Schema. Do not edit `spec.json` by hand; edit the YAML
and regenerate.

Consequences: no per-language YAML parser to keep in agreement, no hand-written
second validator, no shared parity probes. The YAML probes are asserted by the
JS suite only. See [ADR 011](../../docs/adr/011-parse-yaml-once-emit-canonical-json.md).

## fldlint YAML subset

YAML parsers type some plain scalars differently (`col: 017` is 17 in one
parser and 15 in another; `1e17`, `0b101`, `1_0`, `!!float 5`, `%YAML 1.1`,
custom tags and dates also differ), and the schema cannot mitigate this
because it validates what the parser produced. So specs are written in one
**fldlint YAML subset**, enforced at the syntax level **before any type
resolution**: the parser's implicit typing is not trusted.

This section is normative. The one YAML loader (JS) enforces it **before**
schema or semantic validation and passes every probe in
[`spec/probes/yaml/`](../probes/README.md). Nothing else reads YAML, so no
other language implements it; the rules stay as the single definition of what
a spec file may contain, and anyone adding a second YAML reader would have to
meet the same probes.

### Input text

The loader reads the file as bytes and applies these rules to the raw text,
before it hands anything to the YAML parser. They are checked in this order and
the first violation rejects the input. They exist because parsers read the
same bytes differently (some treat a lone CR, NEL, LS or PS as a line break or
reject control characters, some repair invalid UTF-8).

1. **Encoding.** The bytes must be valid UTF-8. Invalid sequences (including
   encoded surrogates and overlong forms) are rejected, never repaired.
2. **BOM.** One leading U+FEFF is stripped; everything below applies to the rest.
3. **Characters.** Only these may occur, anywhere (also in comments and quoted
   scalars): tab, LF, U+0020-U+007E, U+00A0-U+D7FF, U+E000-U+FFFD except
   U+FEFF, U+10000-U+10FFFF, and CR only as the first half of CRLF. So NUL,
   every other control character (form feed, DEL, NEL), LS, PS, a lone CR, a
   BOM that is not the first character and U+FFFE/U+FFFF are rejected.
   Consequently **a line break is LF or CRLF, and nothing else**; a *line* is
   the text between two line breaks. If the last line has no line break, the
   text is parsed as if an LF followed it (this matters for the final
   newline of a block scalar: `a: >` / `  x` without a final newline is
   `"x\n"`).
4. **Tab indentation.** A tab must not appear in the leading whitespace of any
   line: after zero or more spaces, optionally followed by `-` or `?` indicators
   and spaces. The rule is textual: it applies to blank and comment lines and to
   lines inside quoted and block scalars too. (Matches the regular expression
   `^ *([-?] *)*\t` on a line.)
5. **No document.** If every line is blank (only spaces and tabs) or a comment
   (`#` is its first non-space character), including empty input and a lone
   BOM, the input is **empty** and is rejected with its own reason ("empty
   document"). A bare `---` is different: it starts one document whose content
   is null, and is accepted as `null`; the loaders (and the schema) reject a null
   root later, because the root must be a mapping. A lone `...` is no document
   either and is rejected.
6. **Directive.** If the first line that is not blank and not a comment starts
   with `%` (column 1), reject. A `%` line after a `---` or an indented `%` line
   is not a directive but is a YAML syntax error all the same.

### Document level

The text is then parsed as YAML 1.2 syntax. The loader rejects the input if it is
not well-formed YAML, or if any of these holds (the numbers are used only to
refer to a rule):

1. **More than one document.** A trailing `---` with nothing after it counts as
   a second document. One `...` after the last line of the document is fine.
2. **Explicit tag** on any node (scalar, mapping or sequence), including the
   non-specific tag `!`, `!!type` (`!!str`, `!!int`, `!!null`, ...) and
   `!custom`. A tag must never be dropped silently: a loader that reads the
   parser's syntax tree must also look at its tokens (the `yaml` package drops
   a tag or anchor written on a later line after a quoted key, as in
   `a: { 'q'` / `  !!str}`).
3. **Anchor** (`&a`) or **alias** (`*a`), with the same care. An alias needs an
   anchor, so it is usually reported as the anchor.
4. **Mapping key** that is not a scalar, or whose classified type (below) is
   not string. So `017: x`, `true: x`, `1: x`, `~: x` and `null: x` are
   rejected; quoted keys are strings and fine. (The explicit `? key` form is
   not checked: a scalar key written that way is read like any other.)
5. **Duplicate mapping keys.**
6. **Nesting.** At most **100** levels of nested collections (mappings and
   sequences, block or flow); the root collection is level 1, so `a: [[...]]`
   with 99 pairs of brackets is accepted and with 100 is not. This bounds
   recursion; parsers also fail on very deep
   input, but at different depths.
7. **Flow collections:** a plain scalar or key must not end with `:` (`[a:]`,
   `{a:}`, `{a:[1]}`); the parsers read `[a:]` as a pair with a null value and
   as the string `a:`. A quoted key may be followed directly by its value, as
   in JSON (`{"a":1}`).
8. **Block scalar headers** carry no indentation indicator (`|2`, `>-1`);
   chomping indicators (`|-`, `>+`) are fine. The parsers disagree on the
   whitespace-only lines of a block scalar with an explicit indentation.

Allowed: an explicit `---` start marker without directives, comments, flow and
block collections, CRLF line endings, a leading BOM, a missing final line break,
a final `...`, and plain, quoted and block scalars on one or several lines.

### Scalar typing

A scalar that is not plain (double-quoted, single-quoted, literal `|`, folded
`>`) is **always a string**. A plain scalar is classified from the parser's
scalar text (already trimmed, multi-line plain scalars already folded). First
match wins. `(?i)` means case-insensitive **over ASCII letters only**: an
implementation must not fold other characters, so `falſe`, `yeſ` (U+017F, which
Unicode simple case folding maps to `s`, as Go's `(?i)` does but the JS `i` flag
without `u` does not), `ＴＲＵＥ` and `0x` followed by the Kelvin sign are strings.
The regexes use only `^`, `$` and `[0-9]` (never `\d`) and no lookaround, so
they mean the same in Go RE2 and JavaScript (JS without the `u` or `v` flag);
each is matched against the whole scalar text, which may contain any
characters. Only the parser's *syntax tree* is used: a loader must take no
scalar type from the parser itself (`yaml.v3` types `0e0_`, `1e_0`, `0o-1`,
`0b+1` and `0_x1` as numbers; here they are strings by R5).

- **R1**: exactly `true` or `false` is a **boolean**.
- **R2**: `^(0|-?[1-9][0-9]*)$` is an **integer**; if its absolute value
  exceeds 9007199254740991 (2^53 - 1) it is an **error** (out of safe-integer
  range). `-0` does not match R2 and falls to R4b.
- **R3**: exactly the empty string, `~`, `null`, `Null` or `NULL` is **null**.
- **R4**: **error** "ambiguous plain scalar" if any of these matches:
  - R4a: `(?i)^(true|false|null|y|n|yes|no|on|off|=|<<)$` (other casings of
    true/false/null, the YAML 1.1 boolean words, value and merge indicators)
  - R4b: `^[-+]?([0-9][0-9_,]*(\.[0-9_]*)?|\.[0-9][0-9_]*)([eE][-+]?[0-9]+)?$`
    (leading-zero, signed, underscore, comma, float and exponent forms,
    including `-0`)
  - R4c: `(?i)^[-+]?\.(inf|nan)$`
  - R4d: `(?i)^[-+]?0[xob][0-9a-f_]+$` (so `0x` alone is a string)
  - R4e: `^[-+]?[0-9][0-9_]*(:[0-5]?[0-9])+(\.[0-9_]*)?$` (YAML 1.1
    sexagesimal)
  - R4f: `^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}([Tt ]|$)` (dates and timestamps; a
    deliberate prefix check, so `2001-12-14 is a date` is also rejected)
- **R5**: anything else is a **string**.

Null is kept as a value (it is not an error), so the schema-level "must not be
null" checks keep producing their messages.

### Rationale

The rejected set is the union of what YAML 1.1 and YAML 1.2 core would resolve
to a non-string. A third (Java) loader is planned and Java YAML libraries
commonly resolve 1.1-style, so rejecting is safer than picking a winner.
Values that merely start with a digit (`1st`, `10.0.0.1`, `3 flags`) stay
strings, so the subset is not over-strict.

### Getting a rejected form as a string

Quote it: `col: "017"`, `answer: "yes"`, `when: '2001-12-14'`. Any quoted or
block scalar is a string.

### Notes

- Positions are 1-based; a column counts UTF-16 code units, and a leading BOM
  is not counted.
- Some rules restrict valid YAML on purpose (a tab after `-`, `|2`, `[a:]`) so
  that the subset stays small and unambiguous; quote or rewrite the text.

## Fixture discovery and semantic rules (enforced by the JS loader)

- Every directory at `spec/fixtures/<rule>/<case>/` must contain `case.yaml`
  (not `case.yml`). Any other directory depth, or a directory without
  `case.yaml`, is an error. Case ids are unique across the whole spec.
- A case's `rule` must exist. Every `diagnostics[].ruleId` must equal the
  case's `rule`.
- `diagnostics[].skip`, when present, must equal that rule's own `skip`; a rule
  with `skip` requires every diagnostic to carry it. A rewriting rule's
  diagnostics carry no `skip`, unless the language has no template for it, in
  which case the language must not appear in `expect` at all.
- `skip` on a diagnostic implies `output: false`. `output: true` requires at
  least one diagnostic, and the output file must differ from the input file.
- `expect.<lang>` is only allowed when the rule binds that language
  (`languages.<lang>` exists).
- An explicit spec root (`--spec-root` in JS, `-spec` in the Go tools, `-Dfldlint.specRoot` in the Java harness) that is missing or
  invalid is a **hard error**, never a silent fallback to the stub. The stub
  fallback applies only when no spec root is given at all.

## Required checks (JS loader)

Structure (schema-level):

1. Reject unknown keys at every level.
2. Enforce enums (`valueType`, `risk`, `skip`/skip-category names, placeholder
   names), patterns and required fields.
3. `diagnostics[].line` and `col` are integers (1-based).
4. `expect.<lang>.output` is a boolean.
5. Language entries (`languages.<lang>`, `expect.<lang>`) are non-null.

Cross-reference (beyond what the schema can say):

6. A case's `rule`, and every `diagnostics[].ruleId`, must exist in `rules.yaml`.
7. `ctxArg` may only name a method listed in that language's `methods`.
8. The same method must not appear in two rules for one language.
8a. `argCount` (required on every binding of a report-only rule, forbidden on
    rewriting rules, by the schema) must satisfy `min >= 1`, `min <= max` when
    `max` is present, and `min > keyArg` when the rule has a `keyArg`. Go
    consumes it unvalidated (ADR 011).
9. A case that promises `expect.<lang>` must have that language's input file
   (`js/input.ts`, `go/input.go`, `java/Before.java`).
10. `output: true` requires the output file (`output.ts` / `output.go.golden` / `After.java`),
    and an output file without `output: true` is rejected (and vice versa).

Negative tests for these live in the JS suite. A check added here needs a
negative test there.

## Fixture coverage (separate from the loaders)

`npm run spec:tags` (called by `task spec:tags`) fails, naming the rule and
what is missing, when coverage is not met:

- `bool-variation` (the reference rule) covers the seven shape tags.
- Every rewriting rule (no `skip`) needs at least one rewrite case
  (`output: true`, at least one diagnostic) for **each language that has a
  template**.

There is no separate skip-case requirement
([ADR 007](../../docs/adr/007-shape-tag-coverage-reference-rule-only.md)).
