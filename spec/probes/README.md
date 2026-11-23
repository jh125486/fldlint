# Shared probes

Probes for the **fldlint YAML subset** (syntax and scalar typing), defined in
[validation-parity.md](../contracts/validation-parity.md#fldlint-yaml-subset).
They are not rules or fixture domain checks: a probe says nothing about
`rules.yaml` or `case.yaml` content, only whether the loader's YAML layer
accepts the text and what value it produces.

## Convention

One probe per file in `spec/probes/yaml/`; slugs are lowercase kebab-case and
unique.

- `reject-<slug>.yaml`: the loader must reject it, for the reason its name
  prefix says (see Rules). The message is not asserted.
- `accept-<slug>.yaml` with a sibling `accept-<slug>.json`: the loader must
  accept it and produce exactly that JSON value (mappings are objects,
  integers are numbers, plus booleans, null and strings).

A `.yaml` is minimal and usually starts with a `# probe: ...` comment line.
Probes where a leading comment would change what is tested are left bare.

## Rules

- **Every loader's test suite (JS, Go, any future one) must read this
  directory and assert it**: each `reject-*.yaml` fails to load for its reason, each
  `accept-*.yaml` loads to its sidecar JSON. Run the subset check before schema
  or semantic validation.
- Every `accept-*.yaml` has a `.json` sidecar and vice versa; no other files.
- Read a probe the way the loader reads a file (bytes, not a decoded string):
  some probes are not valid UTF-8 on purpose.
- Guard against a suite silently reading nothing: there must be at least
  **102** `reject-*.yaml` and at least **62** `accept-*.yaml` files. Raise these
  numbers here and in the suites (`MIN_REJECT` and `MIN_ACCEPT` in the JS suite,
  `minReject` and `minAccept` in the Go suite) when adding probes.
- Every suite asserts **why** a reject probe fails, not only that it does: the
  name prefix selects the error it must fail with (first matching prefix wins),
  using the table below. Each suite keeps the table with its own error names
  (sentinels in Go, codes in JS); a probe that fails for another reason, for
  example a stray tab, is a bug in the probe.

| Prefix | Reason |
| --- | --- |
| `reject-empty-` | no document (empty, blank, BOM only, comments only, a lone `...`) |
| `reject-directive-` | `%` directive; except `-after-start-marker` and `-indented`, which are syntax errors |
| `reject-invalid-utf8`, `reject-surrogate-utf8` | encoding |
| `reject-bom-`, `reject-lone-cr`, `reject-nel-`, `reject-line-separator-`, `reject-paragraph-separator-`, `reject-form-feed`, `reject-nul`, `reject-delete-char` | character not allowed |
| `reject-tab-` | tab in leading whitespace |
| `reject-tag-`, `reject-anchor`, `reject-alias` | explicit tag, anchor, alias (an alias needs an anchor, so `reject-alias` fails as an anchor) |
| `reject-key-complex-` | mapping key that is not a scalar |
| `reject-key-` | mapping key that is not a string |
| `reject-duplicate-keys`, `reject-two-documents`, `reject-trailing-document-marker` | duplicate key, more than one document |
| `reject-too-deep`, `reject-block-indicator`, `reject-flow-scalar-` | nesting, block scalar indentation indicator, flow scalar ending with a colon |
| `reject-out-of-range-`, `reject-ambiguous-` | integer outside the safe range, ambiguous plain scalar |

Where yaml.v3 or the `yaml` package reports its own syntax error for a probe
before the subset sees it, the suite of that language lists the probe as a
syntax error instead: `reject-empty-document-end-marker`,
`reject-anchor-after-quoted-key` and `reject-tag-after-quoted-key` in Go (the
JS loader reports an empty document, an anchor and a tag).

## Adding a probe

1. Pick a unique slug; create `reject-<slug>.yaml`, or `accept-<slug>.yaml`
   plus `accept-<slug>.json`.
2. A reject probe must fail for the reason its name says and no other (no
   stray tabs, duplicate keys or syntax errors).
3. Do not depend on trailing whitespace or a missing final newline: the
   `.editorconfig` trims and inserts them. A probe that must break an
   editorconfig rule needs a `[spec/probes/yaml/<file>]` override there
   (`.gitattributes` already marks every probe `-text`, so git never rewrites
   its bytes).
4. Update the counts above and run the JS and Go suites.

## Probe index

| Group | Probes | What they probe |
| --- | --- | --- |
| No document | `reject-empty-*` | empty input, blank lines, comments only, a lone BOM, a lone `...` |
| Start and end markers | `accept-bare-document-start-marker`, `accept-document-start-marker`, `accept-document-start-marker-comment`, `accept-document-end-marker`, `reject-two-documents`, `reject-trailing-document-marker` | `---` alone is one document whose content is `null` (`.json` is `null`); a second document is rejected |
| Directives | `reject-directive-*` | `%` as the first non-comment line |
| BOM, line ends, characters, encoding | `accept-bom`, `accept-crlf`, `accept-no-final-newline`, `reject-bom-*`, `reject-lone-cr`, `reject-nel-*`, `reject-line-separator-*`, `reject-paragraph-separator-*`, `reject-form-feed`, `reject-nul`, `reject-delete-char`, `reject-invalid-utf8`, `reject-surrogate-utf8` | one leading BOM, CRLF and a missing final line break are fine; a second BOM, a lone CR, NEL, LS, PS, control characters and malformed UTF-8 are rejected (also where they would hide a `%YAML` line) |
| Tabs | `reject-tab-*` | a tab in the leading whitespace of a line |
| Scalar typing | `accept-*` for booleans, nulls, integers, look-alikes and words; `reject-ambiguous-*`, `reject-out-of-range-*`; `accept-long-s-*`, `accept-fullwidth-true` | the classifier rules R1 to R5; ASCII-only case folding (`falſe`, `yeſ`, `ＴＲＵＥ` are strings) |
| Quoted and block scalars | `accept-quoted-*`, `accept-literal-*`, `accept-folded-block`, `accept-block-scalar-chomping`, `reject-block-indicator*` | quoted and block scalars are always strings; no indentation indicator |
| Tags, anchors, aliases | `reject-tag-*`, `reject-anchor*`, `reject-alias` | any explicit tag, anchor or alias on any node, also where the `yaml` package drops it (`{ 'q'` then `&a}` on the next line) |
| Keys | `reject-key-*`, `reject-duplicate-keys*`, `accept-key-plain-word`, `accept-quoted-key-*` | keys must be unique strings and scalars |
| Flow collections and nesting | `accept-flow-*`, `accept-nested-*`, `accept-empty-flow-*`, `reject-flow-scalar-trailing-colon`, `reject-too-deep`, `accept-depth-100` | `[a:]` is rejected; a quoted key may be followed directly by its value; 100 levels are fine, 101 are not (the root collection is level 1) |

The remaining probes (comments, multi-line plain and quoted scalars, folded and
literal blocks) follow the naming convention above and need no special bytes.
