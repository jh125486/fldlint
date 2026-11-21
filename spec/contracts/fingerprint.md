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
| `key`    | The flag key if it is a static string, otherwise absent (dynamic key, or a bulk call with no key). |
| `occ`    | 0-based ordinal of this finding among findings in the same file with identical `(ruleId, kind, key)`, ordered by `(line, col)` ascending. |

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

Note: `all-flags-state` is an illustrative rule id for the bulk vector; use the
actual id from `rules.yaml` once it exists. The algorithm, not the id, is what
the vector pins.

## Difference from flaglint

flaglint's fingerprints are readable, provider- and language-specific strings.
fldlint's are opaque hashes keyed by rule id so they are the same across
languages. They are intentionally incompatible; flaglint baselines are not read.
