# Template placeholders and conversion policy

Applies to every engine (JS, Go, later Java). Status: planned; M0 ships only
the data and the harness, no engine rewrites anything yet.

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
| `#{...}` | Java | OpenRewrite template parameters (`#{any(dev.openfeature.sdk.Client)}`, `#{key}`, `#{default}`, `#{context}`). **Unverified**: written down for M3 and not exercised by anything yet. They may change when the Java recipe is built. |

Substitution is textual and exact: an engine never reformats the argument
text it copies.

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
  locals and imports, so JS outputs may keep the now-unused LaunchDarkly
  client. The codemod does not delete bindings it cannot prove are dead;
  removing them is left to the user's linter.

The policies differ because the compilers do: unused identifiers are a hard
error in Go and at most a lint warning in TS, and deleting code the engine
has not proven dead is riskier than leaving it.
