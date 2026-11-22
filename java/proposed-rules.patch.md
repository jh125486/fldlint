# Proposed Java entries for `spec/rules.yaml` (issue #7)

Not applied: the orchestrator merges spec changes. This replaces the
`# UNVERIFIED` Java entry. Status of each claim is in ADR 009 (F3, F6).

## Verification (2026-11-22)

Checked with `javap` on the published Central jars, not only source:

- `com.launchdarkly:launchdarkly-java-server-sdk` **7.18.1**:
  `com.launchdarkly.sdk.server.interfaces.LDClientInterface` (implemented by
  `final class com.launchdarkly.sdk.server.LDClient`) declares, all
  `(java.lang.String, com.launchdarkly.sdk.LDContext, T)`:
  `boolean boolVariation`, `int intVariation`, `double doubleVariation`,
  `String stringVariation`, `LDValue jsonValueVariation`,
  `EvaluationDetail<Boolean|Integer|Double|String|LDValue>` for each `*Detail`,
  `MigrationVariation migrationVariation(.., MigrationStage)`,
  `FeatureFlagsState allFlagsState(LDContext, FlagsStateOption...)` (class
  `com.launchdarkly.sdk.server.FeatureFlagsState`).
  `LDContext.create(String)`, `create(ContextKind, String)`, `createMulti`,
  `fromUser(LDUser)`, `builder(...)` all exist and are public static.
- 5.10.9: only `(String, LDUser, T)` overloads. 6.3.0: `LDContext` plus
  `default` `LDUser` overloads. 7.0.0: only `LDContext`. Package
  `com.launchdarkly.sdk.server` in all three.
- `dev.openfeature:sdk` **1.23.0** `Features`: `Boolean getBooleanValue(String, Boolean, EvaluationContext)`,
  `String getStringValue(..)`, `Integer getIntegerValue(..)`,
  `Double getDoubleValue(..)`, `Value getObjectValue(..)`; `ImmutableContext(String)`.
- Template shape VERIFIED to rewrite correctly in OpenRewrite 8.90.4 (see ADR 009 F3).

## Correction to the draft in issue #7

Boxing does not widen (VERIFIED with `javac`: `getDoubleValue("k", 1, e)`,
`getIntegerValue("k", (short) s, e)` and `getIntegerValue("k", 'a', e)` fail to
compile). The recipe must rewrite only when the default argument is provably
exactly `boolean`, `int`, `double` or `String`, otherwise
`skip: default-type-unproven` (M0.4). No YAML change is needed, but the Java
fixtures in #9/#10 must include an int-literal-to-`doubleVariation` case.

Template arguments bind by **order of appearance**, so generated code must pass
(client, key, default, context) even though the LD call order is
(key, context, default).

## YAML

```yaml
  - id: bool-variation          # existing rule: replace its java entry
    languages:
      java: {methods: [boolVariation], requires: [context-unproven],
             template: "#{client:any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key:any(java.lang.String)}, #{default:any(boolean)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})"}
  - id: string-variation        # existing rule: add java
    languages:
      java: {methods: [stringVariation], requires: [context-unproven],
             template: "#{client:any(dev.openfeature.sdk.Client)}.getStringValue(#{key:any(java.lang.String)}, #{default:any(java.lang.String)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})"}
  - id: number-variation        # existing rule: add java
    languages:
      java: {methods: [doubleVariation], requires: [context-unproven],
             template: "#{client:any(dev.openfeature.sdk.Client)}.getDoubleValue(#{key:any(java.lang.String)}, #{default:any(double)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})"}
  - id: java-int-variation      # new rule (Go's int-variation is a type mismatch)
    message: "Migrate integer flag {{key}} to OpenFeature"
    valueType: integer
    risk: low
    keyArg: 0
    defaultArg: 2
    contextArg: 1
    languages:
      java: {methods: [intVariation], requires: [context-unproven],
             template: "#{client:any(dev.openfeature.sdk.Client)}.getIntegerValue(#{key:any(java.lang.String)}, #{default:any(int)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})"}
  - id: java-json-variation     # new report-only rule, LDValue vs dev.openfeature.sdk.Value
    valueType: object
    risk: medium
    languages:
      java: {methods: [jsonValueVariation]}
    skip: type-mismatch
  # java entries for existing report-only rules:
  #   bool-variation-detail: boolVariationDetail      string-variation-detail: stringVariationDetail
  #   number-variation-detail: doubleVariationDetail  int-variation-detail: intVariationDetail
  #   json-variation-detail: jsonValueVariationDetail migration-variation: migrationVariation
  #   all-flags-state: allFlagsState
```

Header comment: drop `# UNVERIFIED`, add "Java signatures checked against
launchdarkly-java-server-sdk 7.18.1 and dev.openfeature:sdk 1.23.0 jars (javap)".

Open for #7: `rules.schema.json` `templateJava` and the Go `tmplJavaRe` must accept
`#{name:any(<fqn|primitive>)}` (ADR 009 F3 shows OpenRewrite accepts it).
`json-variation` (JS) gets no Java entry. Also add a row to
`docs/adr/README.md` for ADR 009: "Java recipes are imperative OpenRewrite
recipes built on frozen Maven Central versions".
