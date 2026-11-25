# Template placeholders and conversion policy

Applies to every engine (JS, Go, Java).

A rewriting rule in `spec/rules.yaml` carries a per-language `template`. The
only placeholders allowed are the closed enum below; the schema rejects any
other `{{...}}`. A rule's human `message` may use `{{key}}` only.

## Placeholders

| Placeholder | Languages | Meaning |
|-------------|-----------|---------|
| `{{client}}` | JS, Go | Identifier of the OpenFeature client binding that the engine **proved** is in scope at the call. If no binding is proven the call is `skip: unbound-client`. |
| `{{key}}` | JS, Go | Source text of the flag-key argument, verbatim (quotes included). Only reached when the key is a static string; otherwise `skip: dynamic-key`. |
| `{{default}}` | JS, Go | Source text of the default-value argument, verbatim. |
| `{{context}}` | JS | The converted evaluation context, per the JS policy below. |
| `{{evalctx}}` | Go | The converted evaluation context, per the Go policy below. |
| `{{goctx}}` | Go | The `context.Context` argument. For `*Ctx` variants: the source text of argument 0. For plain variants: the enclosing function's `context.Context` parameter name if there is exactly one, otherwise `context.TODO()` (and `"context"` is added to the imports if absent). |
| `#{...}` | Java | OpenRewrite template parameters (`#{any(dev.openfeature.sdk.Client)}`, `#{key}`, `#{default}`, `#{context}`). They bind by order of appearance. The recipe types each one before use (`#{key}` is `#{any(java.lang.String)}`, `#{default}` the rule's primitive, `#{context}` its own template `new ImmutableContext(#{any(java.lang.String)})`), and the client parameter is the proven client's declaration name. |

Substitution is textual and exact: an engine never reformats the argument
text it copies. In Java the arguments are the original syntax-tree elements
passed to `JavaTemplate.apply`, so their text is copied unchanged; the
whitespace of the template itself (around `,` and `(`) is whatever OpenRewrite's
autoformat produces, which is the one place Java is not textually exact.

## Comments inside a call

Rewriting replaces the whole call range, so comments inside it are placed
explicitly, identically in JS and Go (issue #31). Comments inside an argument's
own text are copied with it. The others:

- A comment touching an argument (before the separating comma, trailing it on
  its line, or on or above its line ahead of it) moves with that argument when
  the rewrite reorders the arguments.
- Comments between the receiver, the method name and the `(` are hoisted, in
  order, before the new call.
- A comment on its own line after the last argument goes before the closing `)`.
- A `//` comment, or a block comment containing a line break, that trails an
  argument is written behind the next `,` of the template (a comma is added
  before `)` for the last one): Go would insert a semicolon at the line break
  otherwise. Line terminators (LF or CRLF) are copied from the source. The
  same comma is added before `)` for own-line tail comments, block comments
  included.
- A comment with no home (inside the converted context literal outside the
  key expression, `ldcontext.New(...)` likewise, or a `//` or multi-line comment
  between the receiver and the `(`) makes the call a finding with no fix. It is
  not a skip category and has no `[skip: ...]` suffix: the message is the
  rule's own, followed by `[no-fix: comment]`
  ([output-format.md](output-format.md)). The comment is never dropped.
- A directive-shaped comment (`@ts-expect-error`, `@ts-ignore`, `@ts-nocheck`,
  `eslint-disable*`, `eslint-enable`, `nolint`, `//go:`, `lint:ignore`,
  `istanbul ignore`) outside an argument's own text has no home either: moving
  it would change which line or argument it governs.
- Java: only the argument elements are copied, so any comment outside an
  argument's own text has no home: in the receiver, between the receiver and
  the name, before the `(`, beside an argument (before it or before its
  separating comma), or in `LDContext.create(..)` around its key. The call is
  then a finding with no fix (`[no-fix: comment]`); homing comments by moving
  them with their arguments is not implemented in Java. A comment inside an
  argument's own text is copied with it.
- JS only: a comment inside a parenthesized callee, `(ld).boolVariation(...)`
  or any callee whose `(` cannot be located behind trivia, is conservatively
  unfixable. JS argument text is copied from the argument node, so parentheses
  around an argument (`f((a), b)`) are dropped from the copy; Go copies them.

A call with no comment outside its argument text rewrites exactly as before.

## Planner step order

Identical in every language; the first matching step decides the call:

1. rule-level skip (`detail`, `bulk`, `type-mismatch`)
2. `dynamic-key`
3. `context-unproven`
4. `default-type-unproven`
5. `unbound-client`
6. `receiver-unproven`
7. otherwise rewrite

Rationale and the receiver/stranded-local rules: [ADR 010](../../docs/adr/010-planner-step-order-and-receiver-removal.md).

## Static key

A flag key is static only if it is a string literal, or (JS/TS) a template
literal with zero `${}` expressions. In Go only a string literal counts,
interpreted (`"x"`) or raw (`` `x` ``); a named constant is `dynamic-key`.
In Java only a `String` literal counts (a text block is one); a concatenation,
a `static final String` constant and a method call are `dynamic-key`.

## Java planner notes

The step order above is the Java order too, with these Java facts:

- Match: the receiver type must be an `LDClientInterface` (overrides included)
  and the argument count must fit the binding (M0.10). An unresolved method
  type yields no finding.
- `context-unproven`: the context argument must be `LDContext.create(<String>)`
  (static import allowed). The v6 `LDUser` overload is `context-unproven` too,
  but only after `dynamic-key`: the step order is the same for every call.
- `default-type-unproven`: proven by static type, like Go. The argument's type
  must be exactly `boolean` for `bool-variation`; a boxed `Boolean` is not
  proven (the rewrite would unbox it and could throw on `null`).
- `unbound-client`: the innermost scope that declares an accessible
  `dev.openfeature.sdk.Client` must declare exactly one; an inner declaration
  of the same name (any type, a pattern binding included) shadows it; an
  instance field is not accessible from a static context; a captured local
  must be effectively final. Inherited fields are not looked at.
- `receiver-unproven`: the receiver must be absent (implicit `this`), an
  identifier or a field-access chain of identifiers. Unused Java locals compile,
  so nothing is ever stranded (ADR 010 item 6).

## Context-conversion policy

LaunchDarkly contexts (multi-kind, private attributes, custom attributes) do
not map 1:1 onto an OpenFeature `EvaluationContext`. A rewrite that changes
the context argument is allowed **only when the LaunchDarkly context is a
simple single-kind literal**. Anything else is reported, not rewritten, as
`skip: context-unproven`. Both languages' `bool-variation`-style rules
therefore carry `requires: [context-unproven]`. See
[ADR 006](../../docs/adr/006-context-conversion-skipped-unless-provable.md).

### JS

Rewritten only when the context argument is an object literal whose only
properties are `key` (any expression) and, optionally, `kind: "user"`. The
engine emits `{ targetingKey: <key expr> }`.

```ts
// input
client.boolVariation('new-ui', { key: user.id, kind: 'user' }, false)
// output (client is the proven OpenFeature client binding)
ofClient.getBooleanValue('new-ui', false, { targetingKey: user.id })
```

Skipped as `context-unproven`: an identifier (`ctx`), a spread, any extra
attribute (`{ key, email }`), a kind other than `"user"`, a multi-kind
context, or any other expression.

### Java

Rewritten only for a direct `LDContext.create(<expr>)` call; the engine emits
`new ImmutableContext(<expr>)` and the rewrite ends in `.booleanValue()` (ADR 009 D8).

```java
// input
boolean on = ldClient.boolVariation("new-ui", LDContext.create(userKey), false);
// output (client is the proven OpenFeature Client)
boolean on = client.getBooleanValue("new-ui", false, new ImmutableContext(userKey)).booleanValue();
```

Skipped as `context-unproven`: a variable, `LDContext.builder(..)`,
`LDContext.createMulti(..)`, `LDContext.create(kind, key)`, or the v6 `LDUser`
overload.

### Go

Rewritten only for a direct `ldcontext.New(<expr>)` call; the engine emits
`openfeature.NewEvaluationContext(<expr>, nil)`.

```go
// input
v, _ := ldClient.BoolVariation("new-ui", ldcontext.New(userID), false)
// output
v, err := ofClient.BooleanValue(context.TODO(), "new-ui", false, openfeature.NewEvaluationContext(userID, nil))
```

`BooleanValue` takes a `context.Context` first and returns `(bool, error)`, so
a rewrite that keeps the result must bind both values (`v, err :=`, or
`v, _ :=` when the original discarded the second value). The snippet above was
compile-checked against go-server-sdk v7.23.0 and open-feature go-sdk v1.20.0.

Skipped as `context-unproven`: a variable (`ctx`), `ldcontext.NewBuilder(...)`,
`ldcontext.NewMulti(...)`, `ldcontext.NewWithKind(...)`, or anything that is
not literally `ldcontext.New(...)`.

(The output above is illustrative; the golden files in `spec/fixtures/` are
authoritative for exact text, including the error-return shape.)

## The `context` shadowing rule (Go)

When `{{goctx}}` resolves to `context.TODO()`, the engine must be sure the
identifier `context` still means the standard-library package at the call. If
`context` is shadowed in scope there (for example a parameter or local named
`context` that is not a `context.Context` the engine can use), the call is
`skip: context-unproven`. If the enclosing function has exactly one
`context.Context` parameter, even one named `context`, that name is used and
no import is added.

## Compilable-output policy

Rewrites must leave code that compiles, but the two languages differ.

- **Go must compile.** Go rejects unused imports and unused local variables.
  So an engine removes an import only if the file no longer references it
  (e.g. `ldcontext` once its only use is rewritten), and a rewrite must not
  strand a local. Go fixtures keep the LaunchDarkly client referenced
  elsewhere (e.g. `defer ldClient.Close()`).
- **TypeScript may keep an unused client.** TypeScript tolerates unused
  locals and imports by default (not under `--noUnusedLocals`, TS6133), so JS outputs may keep the now-unused LaunchDarkly
  client. The codemod does not delete bindings it cannot prove are dead;
  removing them is left to the user's linter.

The policies differ because the compilers do: unused identifiers are a hard
error in Go and at most a lint warning in TS, and deleting code the engine
has not proven dead is riskier than leaving it.
