# Finding fingerprint

A fingerprint is the stable identity of a finding. It is used by the
[baseline](baseline.md) and by SARIF `partialFingerprints`
([sarif.md](sarif.md)). It is language-independent: the same logical finding
yields the same fingerprint from the JS and Go engines. Status: planned.

## Algorithm

Inputs for a finding:

| Input    | Definition |
|----------|------------|
| `ruleId` | The `rules.yaml` rule id (e.g. `bool-variation`). The rule, not the skip category, so fixing a skip reason does not change identity. |
| `path`   | File path relative to the scan root, normalized (below). |
| `key`    | The flag key if it is a static, well-formed string, otherwise absent (dynamic key, a key that is not well formed, or a bulk call with no key). It is the decoded string value of the literal, not its source text: JS `'a\u0041'`, Go `"a\x41"` and Go raw `` `aA` `` all give `aA`. (Contrast `{{key}}` in templates and messages, which is source text, quotes included; see [placeholders.md](placeholders.md).) |
| `occ`    | 0-based ordinal of this finding among findings in the same file with identical `(ruleId, kind, key)`, ordered by `(line, col)` ascending. |

Every per-flag rule (every rule except the bulk `all-flags-state`) has a key argument (`keyArg: 0` in `rules.yaml`), report-only
rules included (`variation`, the `*-variation-detail` rules, `migration-variation`,
`int-variation`, `go-json-variation`). Only the bulk rule `all-flags-state` is
keyless. Without a key, identity would depend on file position alone, and
deleting one call would hand its fingerprint to the next one.

A key is well formed when it is valid Unicode text: JS `key.isWellFormed()`
(no unpaired surrogate), Go `utf8.ValidString`. A key that is not well formed is
treated exactly like a dynamic key: it is absent (`kind` `-`), and the engine
never throws. Otherwise two different keys such as JS `'a\uD800'` and
`'a\uD801'` would both encode with U+FFFD and collide. This changes only the
fingerprint input; whether the call is rewritten is unaffected. `occ` groups
use the same rule, so two ill-formed keys in one file get distinct `occ`
values. `path` is not subject to it: a path is always hashed as given after
normalization (it cannot be dropped, and a file always has one). Engines may
therefore disagree on a path that is not valid Unicode; no vector pins that.

Steps:

1. `kind` = `s` if `key` is present, otherwise `-`. If `key` is absent use the empty string for it.
2. Build the preimage as the UTF-8 encoding of the five fields joined by a
   single NUL (`U+0000`) byte:
   `ruleId NUL path NUL kind NUL key NUL occ` where `occ` is decimal with no padding.
3. Compute SHA-256 of the preimage.
4. The fingerprint is the first 32 characters (128 bits) of the lowercase hex digest.

No version prefix is embedded. If the algorithm ever has to change, it gets a
new contract and a new baseline `version`; it is not silently altered.

### Path normalization

Applied in this order: replace `\` with `/`; collapse runs of `/`; remove
leading `./` (repeatedly) and interior `/./`; remove any trailing `/`. The
result is relative to the scan root and never starts with `/` or `./`. Paths
are not case-folded or Unicode-normalized; the byte sequence is used as is.
Files outside the scan root are not scanned.

### Stability

- Unrelated edits (other lines, comments, reformatting, other files) do not
  change a fingerprint, because line and column are not inputs.
- Adding or removing another call with the same rule and key in the same file
  can shift `occ` for later duplicates. This is accepted; repeated identical
  calls in one file are rare, and a shift only moves identity within that group.
- Renaming or moving a file changes every fingerprint in it (the path is an
  input). Regenerate the baseline after such moves.
- Changing the flag key or the rule (e.g. `boolVariation` to `stringVariation`)
  is a different finding.

## Worked example

`src/checkout/service.ts` contains two calls to `boolVariation("checkout-v2", ...)`
(lines 10 and 40) and one `boolVariation(flagName, ...)` call with a dynamic key.
For the first call: `ruleId=bool-variation`, `path=src/checkout/service.ts`,
`kind=s`, `key=checkout-v2`, `occ=0`. The preimage (`<NUL>` is the single byte 0x00) is:

```
bool-variation<NUL>src/checkout/service.ts<NUL>s<NUL>checkout-v2<NUL>0
```

SHA-256, first 32 hex chars:
`d1a64654f64fa217171b2841c61090fd`. The call on line 40 has `occ=1` and
fingerprint `665e542cff4f57f1be484d06d7934fe5`. The dynamic call has `kind=-`,
empty key, `occ=0`: `bbe22edf1d42d792c2119cd54a764d84`.

## Test vectors

Conformance suites in every language must reproduce these. They were computed
with an independent throwaway script.

| ruleId | path (as given) | key | occ | fingerprint |
|--------|-----------------|-----|-----|-------------|
| bool-variation | `src/checkout/service.ts` | `checkout-v2` | 0 | `d1a64654f64fa217171b2841c61090fd` |
| bool-variation | `./src/checkout/service.ts` | `checkout-v2` | 0 | `d1a64654f64fa217171b2841c61090fd` |
| bool-variation | `src\checkout\service.ts` | `checkout-v2` | 0 | `d1a64654f64fa217171b2841c61090fd` |
| bool-variation | `src/checkout/service.ts` | `checkout-v2` | 1 | `665e542cff4f57f1be484d06d7934fe5` |
| string-variation | `src/checkout/service.ts` | `checkout-v2` | 0 | `206d1b51231868124d10e9cd2842bf72` |
| bool-variation | `internal/flags/flags.go` | `new-ui` | 0 | `c1d9df69df563046fb3dbdcdf5bde64b` |
| bool-variation | `src/checkout/service.ts` | (none) | 0 | `bbe22edf1d42d792c2119cd54a764d84` |
| bool-variation | `src/checkout/service.ts` | (none) | 1 | `0c1ab39ac97767156d8a818304b68367` |
| all-flags-state | `svc/main.go` | (none) | 0 | `990be9e272e7aebbdf1c140532da54e1` |
| bool-variation | `src/ünï/a.ts` | `clé` | 0 | `d323baf0889c2c95e1dea4eeab9485c9` |
| bool-variation | `a//b` | `flag` | 0 | `9a0875cbf32a433a5c53f854bc5cae8b` |
| bool-variation | `a/./b` | `flag` | 0 | `9a0875cbf32a433a5c53f854bc5cae8b` |
| bool-variation | `a/././b` | `flag` | 0 | `9a0875cbf32a433a5c53f854bc5cae8b` |
| bool-variation | `././a` | `flag` | 0 | `34a9ce2a3037bc949209c8e341dbe93c` |
| bool-variation | `a/b/` | `flag` | 0 | `9a0875cbf32a433a5c53f854bc5cae8b` |
| bool-variation | `.\a\b` | `flag` | 0 | `9a0875cbf32a433a5c53f854bc5cae8b` |
| bool-variation | `a\\b` | `flag` | 0 | `9a0875cbf32a433a5c53f854bc5cae8b` |
| bool-variation | `a.ts` | `aA` | 0 | `c2b9a4b7493c4f088396feeceebf56b0` |
| bool-variation | `a.ts` | `😀` | 0 | `6258b6b636174d39b39768f39444c88b` |
| bool-variation | `a.ts` | (empty string) | 0 | `32238f7f244421f8dca4df9a114b5b7c` |
| bool-variation | `a.ts` | (none) | 0 | `d51bead2c60751272a82a139ecfa9eb2` |
| bool-variation | `Src/a.ts` | `flag` | 0 | `ba0d8e621f35e978e2d71023dcc087ea` |
| bool-variation | `src/a.ts` | `flag` | 0 | `f6c5f08066efd25f70ae39048ca0a679` |
| bool-variation | `src/é.ts` | `flag` | 0 | `919507daef37bd442ae0cfe0e433e3c9` |
| bool-variation | `src/é.ts` | `flag` | 0 | `5859735d23d412d60e7983c627a469d7` |
| variation | `src/flags.ts` | `flag-a` | 0 | `f48a19e393bd03280279dc4db8e9e626` |
| variation | `src/flags.ts` | `flag-b` | 0 | `c35dd90bb032cbc492fac013d93ba894` |
| bool-variation-detail | `src/flags.ts` | `flag-a` | 0 | `59613199566bd3365f59dcd67d84a506` |
| bool-variation-detail | `src/flags.ts` | `flag-b` | 0 | `48e72553cc9c49897bfa0fad5fe87035` |
| migration-variation | `src/flags.ts` | `flag-a` | 0 | `aa2028418b80b56f8fa75902628d0d43` |
| int-variation | `internal/flags/flags.go` | `flag-a` | 0 | `513987f491cfbc2743d52c75f996fbe4` |
| int-variation | `internal/flags/flags.go` | `flag-b` | 0 | `460c42d801a686ef1c66d6af23b4a226` |

Machine-readable copy: [`fingerprint-vectors.json`](fingerprint-vectors.json)
(entries `{ruleId, path, key | null, occ, fingerprint}`; `path` is as given,
before normalization). It is the file every language's tests and the parity
check read; a JS test asserts it and this table agree. Rows after the first
ten were generated by [`tools/gen_fingerprint_vectors.py`](tools/gen_fingerprint_vectors.py),
which hashes hand-written normalized paths and does not share code with any
engine. Notes on the newer rows:

- Path rows `a//b`, `a/./b`, `a/././b`, `././a`, `a/b/`, `.\a\b` and `a\\b`
  must equal the fingerprint of their normalized form (`a/b` or `a`).
- `aA` is the decoded value of JS `'a\u0041'`, Go `"a\x41"` and Go raw
  `` `aA` `` (see Inputs).
- A static empty key (kind `s`) and an absent key (kind `-`) differ.
- The `Src/`, `src/` and `é` rows prove no case folding and no Unicode normalization: the
  `é` rows are NFC (U+00E9) then NFD (`e` + U+0301) and yield different values.

- The last seven rows are report-only rules with a literal key. The two keys
  of one rule give different fingerprints, both at `occ` 0, so deleting the
  `flag-a` call leaves `flag-b` unchanged.

Keys that are not well formed cannot be written in JSON, so they live in
[`fingerprint-illformed-vectors.json`](fingerprint-illformed-vectors.json)
(entries `{ruleId, path, keyUtf16, keyUtf8Hex, occ, fingerprint}`): `keyUtf16`
is the key as UTF-16 code units for JS, `keyUtf8Hex` the key's bytes for Go.
Each `fingerprint` is that of the same `(ruleId, path, occ)` with an absent
key, e.g. `a` followed by `U+D800` gives `d51bead2c60751272a82a139ecfa9eb2`,
the `(none)` row above. Same generator, with `--illformed`.

Note: `all-flags-state` is the real bulk rule id in `spec/rules.yaml`. The
algorithm, not the id, is what the vector pins.

## Difference from flaglint

flaglint's fingerprints are readable, provider- and language-specific strings.
fldlint's are opaque hashes keyed by rule id so they are the same across
languages. They are intentionally incompatible; flaglint baselines are not read.
