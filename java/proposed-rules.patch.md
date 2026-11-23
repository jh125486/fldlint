# Proposed Java changes to `spec/` (issue #7)

Not applied here: the orchestrator merges spec changes. Evidence is in
[spike-findings.md](spike-findings.md) (F3, F6) and ADR 009.

## What the patch does

- Replaces the `# UNVERIFIED` Java entry and adds Java bindings to `string-variation`
  and `number-variation` (double). Adds `java-int-variation` (Go's `int-variation` is a
  type mismatch there, so Java needs its own rule) and report-only `java-json-variation`.
  Adds Java methods to the detail, `migration-variation` and `all-flags-state` rules.
- Every Java template appends the unboxing call (`.booleanValue()`, `.intValue()`,
  `.doubleValue()`) so the rewrite keeps LaunchDarkly's primitive typing (ADR 009 D8).
  String needs none.
- Rewritable bindings declare `requires: [context-unproven, default-type-unproven]`.
  `default-type-unproven` is needed because boxing does not widen (VERIFIED with `javac`).
- Template args bind by order of appearance, so the generated code passes (client, key,
  default, context) although the LD call order is (key, context, default).
- The schema (`templateJava`) and Go (`tmplJavaRe`) must accept typed
  `#{name:any(<Type>)}` placeholders; both hunks are in the patch. The patch also
  needs `make spec-gen` to regenerate `rules_gen.go` and `rules.gen.ts`.

## Tested

VERIFIED: applied to a scratch copy of the repo at this branch's base, then
`make spec-check` (JS `spec:validate` OK, 22 cases; after `make spec-gen`,
`spec:tags` OK), `go test ./internal/spec/...` and `npm test` in `js` all pass; the only
`spec-check` failure is generated-file drift, which `make spec-gen` fixes (expected).
`requires` is informational in both validators. UNVERIFIED: Java fixtures wired into
`case.yaml` (that is #9). Also add the ADR 009 row to `docs/adr/README.md` (done by the
orchestrator).

## Diff (`git apply`-able at this branch's base)

```diff
diff --git a/go/internal/spec/yamlnode.go b/go/internal/spec/yamlnode.go
index bb931aa..660298f 100644
--- a/go/internal/spec/yamlnode.go
+++ b/go/internal/spec/yamlnode.go
@@ -24,7 +24,7 @@ var (
 	ruleIDRe     = regexp.MustCompile(`\A[a-z][a-z0-9]*(?:-[a-z0-9]+)*\z`)
 	caseIDRe     = regexp.MustCompile(`\A[a-z0-9]+(?:-[a-z0-9]+)*\z`)
 	tmplJSGoRe   = regexp.MustCompile(`\A(?:[^{}]|\{\{(?:client|key|default|context|goctx|evalctx)\}\})*\z`)
-	tmplJavaRe   = regexp.MustCompile(`\A(?:[^#{}]|#\{(?:key|default|context|any\([A-Za-z0-9_.]+\))\})*\z`)
+	tmplJavaRe   = regexp.MustCompile(`\A(?:[^#{}]|#\{(?:(?:client|key|default|context):any\([A-Za-z0-9_.]+\)|key|default|context|any\([A-Za-z0-9_.]+\))\})*\z`)
 	messageRe    = regexp.MustCompile(`\A(?:[^{}]|\{\{key\}\})*\z`)
 	valueTypes   = []string{"boolean", "string", "number", "integer", "object"}
 	risks        = []string{"low", "medium", "high"}
diff --git a/spec/rules.yaml b/spec/rules.yaml
index 95202d4..78e4e80 100644
--- a/spec/rules.yaml
+++ b/spec/rules.yaml
@@ -45,9 +45,8 @@ rules:
       go:   {methods: [BoolVariation, BoolVariationCtx], ctxArg: {BoolVariationCtx: 0},
              template: "{{client}}.BooleanValue({{goctx}}, {{key}}, {{default}}, {{evalctx}})",
              requires: [context-unproven]}
-      # UNVERIFIED: Java mapping not checked against the LD Java SDK / OpenFeature Java SDK (M3).
-      java: {methods: [boolVariation],
-             template: "#{any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key}, #{default}, #{context})"}
+      java: {methods: [boolVariation], requires: [context-unproven, default-type-unproven],
+             template: "#{client:any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key:any(java.lang.String)}, #{default:any(boolean)}, #{context:any(dev.openfeature.sdk.EvaluationContext)}).booleanValue()"}
   - id: string-variation
     message: "Migrate string flag {{key}} to OpenFeature"
     valueType: string
@@ -62,6 +61,8 @@ rules:
       go: {methods: [StringVariation, StringVariationCtx], ctxArg: {StringVariationCtx: 0},
            template: "{{client}}.StringValue({{goctx}}, {{key}}, {{default}}, {{evalctx}})",
            requires: [context-unproven]}
+      java: {methods: [stringVariation], requires: [context-unproven, default-type-unproven],
+             template: "#{client:any(dev.openfeature.sdk.Client)}.getStringValue(#{key:any(java.lang.String)}, #{default:any(java.lang.String)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})"}
   - id: number-variation
     message: "Migrate number flag {{key}} to OpenFeature"
     valueType: number
@@ -76,6 +77,18 @@ rules:
       go: {methods: [Float64Variation, Float64VariationCtx], ctxArg: {Float64VariationCtx: 0},
            template: "{{client}}.FloatValue({{goctx}}, {{key}}, {{default}}, {{evalctx}})",
            requires: [context-unproven]}
+      java: {methods: [doubleVariation], requires: [context-unproven, default-type-unproven],
+             template: "#{client:any(dev.openfeature.sdk.Client)}.getDoubleValue(#{key:any(java.lang.String)}, #{default:any(double)}, #{context:any(dev.openfeature.sdk.EvaluationContext)}).doubleValue()"}
+  - id: java-int-variation
+    message: "Migrate integer flag {{key}} to OpenFeature"
+    valueType: integer
+    risk: low
+    keyArg: 0
+    defaultArg: 2
+    contextArg: 1
+    languages:
+      java: {methods: [intVariation], requires: [context-unproven, default-type-unproven],
+             template: "#{client:any(dev.openfeature.sdk.Client)}.getIntegerValue(#{key:any(java.lang.String)}, #{default:any(int)}, #{context:any(dev.openfeature.sdk.EvaluationContext)}).intValue()"}
   - id: json-variation
     message: "Migrate JSON flag {{key}} to OpenFeature"
     valueType: object
@@ -104,6 +117,13 @@ rules:
     languages:
       go: {methods: [JSONVariation, JSONVariationCtx], ctxArg: {JSONVariationCtx: 0}}
     skip: type-mismatch
+  # Java jsonValueVariation takes/returns LDValue; OpenFeature getObjectValue uses Value.
+  - id: java-json-variation
+    valueType: object
+    risk: medium
+    languages:
+      java: {methods: [jsonValueVariation]}
+    skip: type-mismatch
   # JS generic variation() returns any; the target typed method cannot be
   # chosen from the call alone.
   - id: variation
@@ -118,6 +138,7 @@ rules:
     languages:
       js: {methods: [migrationVariation]}
       go: {methods: [MigrationVariation, MigrationVariationCtx], ctxArg: {MigrationVariationCtx: 0}}
+      java: {methods: [migrationVariation]}
     skip: type-mismatch
 
   # ---- report-only: detail ----
@@ -126,29 +147,34 @@ rules:
     languages:
       js: {methods: [boolVariationDetail]}
       go: {methods: [BoolVariationDetail, BoolVariationDetailCtx], ctxArg: {BoolVariationDetailCtx: 0}}
+      java: {methods: [boolVariationDetail]}
     skip: detail
   - id: string-variation-detail
     risk: high
     languages:
       js: {methods: [stringVariationDetail]}
       go: {methods: [StringVariationDetail, StringVariationDetailCtx], ctxArg: {StringVariationDetailCtx: 0}}
+      java: {methods: [stringVariationDetail]}
     skip: detail
   - id: number-variation-detail
     risk: high
     languages:
       js: {methods: [numberVariationDetail]}
       go: {methods: [Float64VariationDetail, Float64VariationDetailCtx], ctxArg: {Float64VariationDetailCtx: 0}}
+      java: {methods: [doubleVariationDetail]}
     skip: detail
   - id: int-variation-detail
     risk: high
     languages:
       go: {methods: [IntVariationDetail, IntVariationDetailCtx], ctxArg: {IntVariationDetailCtx: 0}}
+      java: {methods: [intVariationDetail]}
     skip: detail
   - id: json-variation-detail
     risk: high
     languages:
       js: {methods: [jsonVariationDetail]}
       go: {methods: [JSONVariationDetail, JSONVariationDetailCtx], ctxArg: {JSONVariationDetailCtx: 0}}
+      java: {methods: [jsonValueVariationDetail]}
     skip: detail
   - id: variation-detail
     risk: high
@@ -162,4 +188,5 @@ rules:
     languages:
       js: {methods: [allFlagsState]}
       go: {methods: [AllFlagsState]}
+      java: {methods: [allFlagsState]}
     skip: bulk
diff --git a/spec/schema/rules.schema.json b/spec/schema/rules.schema.json
index 200ce6d..05317a8 100644
--- a/spec/schema/rules.schema.json
+++ b/spec/schema/rules.schema.json
@@ -118,10 +118,10 @@
       "pattern": "^(?:[^{}]|\\{\\{(?:client|key|default|context|goctx|evalctx)\\}\\})*$"
     },
     "templateJava": {
-      "description": "Only #{key} #{default} #{context} #{any(<fully.qualified.Type>)} placeholders.",
+      "description": "Only #{key} #{default} #{context} #{any(<Type>)} and typed #{name:any(<Type>)} (name in client, key, default, context) placeholders; text outside placeholders is literal.",
       "type": "string",
       "minLength": 1,
-      "pattern": "^(?:[^#{}]|#\\{(?:key|default|context|any\\([A-Za-z0-9_.]+\\))\\})*$"
+      "pattern": "^(?:[^#{}]|#\\{(?:(?:client|key|default|context):any\\([A-Za-z0-9_.]+\\)|key|default|context|any\\([A-Za-z0-9_.]+\\))\\})*$"
     },
     "rule": {
       "type": "object",
```
