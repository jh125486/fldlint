# Baseline

A baseline freezes existing findings so CI fails only on new ones. Status: planned.

## CLI

- `--write-baseline <file>`: scan, write the baseline, exit `0` (exit `2` if it cannot be written).
- `--baseline <file>`: scan and compare against the file.

Conventional file name: `fldlint-baseline.json`, committed to source control.

## File format

```json
{
  "version": 1,
  "fingerprints": [
    "0c1ab39ac97767156d8a818304b68367",
    "d1a64654f64fa217171b2841c61090fd"
  ]
}
```

- `version`: integer, currently `1`. Any other value is a usage error (exit `2`).
- `fingerprints`: lowercase hex strings as defined in [fingerprint.md](fingerprint.md);
  written sorted ascending and deduplicated.
- No timestamps or tool versions, so regenerating an unchanged baseline
  produces a byte-identical file. Two-space indent, trailing newline.
- Unknown extra fields are rejected.

## Matching

A current finding is **baselined** if its fingerprint is in the file, and
**new** otherwise. Fingerprints in the file with no current finding are ignored
silently (the debt was paid down). Matching is by exact string equality only;
line numbers play no part.

## Effect on output and exit code

- Baselined findings do not count toward exit `1` or `--max-warnings`
  ([exit-codes.md](exit-codes.md)). New findings count normally.
- `text` output omits baselined findings; the stderr summary reports how many
  were suppressed. `json` includes them with `"baselined": true`; SARIF marks
  them `baselineState: "unchanged"` and new ones `"new"` ([sarif.md](sarif.md)).
- Missing file, malformed JSON, schema violation or unsupported `version`:
  exit `2`, nothing scanned.
- A file move or rename changes fingerprints; regenerate the baseline after
  such refactors.

flaglint baseline files (string `version`, `createdAt`) are not accepted.
