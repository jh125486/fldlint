#!/usr/bin/env python3
"""Independent generator for fingerprint-vectors.json (stdlib only).

It does NOT call any engine's normalizePath: each case states the already
normalized path by hand, and only hashes it. Run from anywhere:

    python3 spec/contracts/tools/gen_fingerprint_vectors.py > spec/contracts/fingerprint-vectors.json
    python3 spec/contracts/tools/gen_fingerprint_vectors.py --illformed > spec/contracts/fingerprint-illformed-vectors.json

Preimage: ruleId NUL path NUL kind NUL key NUL occ (UTF-8), SHA-256, first 32
lowercase hex chars. kind is "s" when key is not None, else "-".
"""
import hashlib
import json
import sys
import unicodedata

BV, SV = "bool-variation", "string-variation"
SVC = "src/checkout/service.ts"

# (ruleId, path as given, normalized path, key or None, occ)
CASES = [
    # The original ten vectors from fingerprint.md.
    (BV, SVC, SVC, "checkout-v2", 0),
    (BV, "./" + SVC, SVC, "checkout-v2", 0),
    (BV, SVC.replace("/", "\\"), SVC, "checkout-v2", 0),
    (BV, SVC, SVC, "checkout-v2", 1),
    (SV, SVC, SVC, "checkout-v2", 0),
    (BV, "internal/flags/flags.go", "internal/flags/flags.go", "new-ui", 0),
    (BV, SVC, SVC, None, 0),
    (BV, SVC, SVC, None, 1),
    ("all-flags-state", "svc/main.go", "svc/main.go", None, 0),
    (BV, "src/ünï/a.ts", "src/ünï/a.ts", "clé", 0),
    # normalizePath edge cases; each equals the fingerprint of the normalized form.
    (BV, "a//b", "a/b", "flag", 0),
    (BV, "a/./b", "a/b", "flag", 0),
    (BV, "a/././b", "a/b", "flag", 0),
    (BV, "././a", "a", "flag", 0),
    (BV, "a/b/", "a/b", "flag", 0),
    (BV, ".\\a\\b", "a/b", "flag", 0),
    (BV, "a\\\\b", "a/b", "flag", 0),
    # Key is the decoded string value (JS 'aA', Go "a\x41" and `aA` all give aA).
    (BV, "a.ts", "a.ts", "aA", 0),
    # Astral-plane key proves the UTF-8 (not UTF-16) preimage.
    (BV, "a.ts", "a.ts", "\U0001F600", 0),
    # A static empty key differs from an absent key (kind s vs -).
    (BV, "a.ts", "a.ts", "", 0),
    (BV, "a.ts", "a.ts", None, 0),
    # No case folding, no Unicode normalization: these four are all distinct.
    (BV, "Src/a.ts", "Src/a.ts", "flag", 0),
    (BV, "src/a.ts", "src/a.ts", "flag", 0),
    (BV, unicodedata.normalize("NFC", "src/é.ts"), unicodedata.normalize("NFC", "src/é.ts"), "flag", 0),
    (BV, unicodedata.normalize("NFD", "src/é.ts"), unicodedata.normalize("NFD", "src/é.ts"), "flag", 0),
    # Report-only rules carry a key (rules.yaml keyArg, M0.9): two calls with
    # different literal keys have different fingerprints, both occ 0, so deleting
    # one never re-keys the other.
    ("variation", "src/flags.ts", "src/flags.ts", "flag-a", 0),
    ("variation", "src/flags.ts", "src/flags.ts", "flag-b", 0),
    ("bool-variation-detail", "src/flags.ts", "src/flags.ts", "flag-a", 0),
    ("bool-variation-detail", "src/flags.ts", "src/flags.ts", "flag-b", 0),
    ("migration-variation", "src/flags.ts", "src/flags.ts", "flag-a", 0),
    ("int-variation", "internal/flags/flags.go", "internal/flags/flags.go", "flag-a", 0),
    ("int-variation", "internal/flags/flags.go", "internal/flags/flags.go", "flag-b", 0),
]

# Keys that are not well formed (an unpaired surrogate in JS; invalid UTF-8 in
# Go) are not static: the fingerprint is that of an absent key. Each is given as
# UTF-16 code units (for JS) and as UTF-8 bytes in hex (for Go), since neither
# is representable as a JSON string. (ruleId, path, utf16 units, utf8 hex, occ)
ILLFORMED = [
    (BV, "a.ts", [0x61, 0xD800], "61eda080", 0),
    (BV, "a.ts", [0x61, 0xD801], "61eda081", 0),
    (BV, "a.ts", [0xDC00], "edb080", 0),
    ("variation", "src/flags.ts", [0x61, 0xD800, 0x61], "61eda08061", 0),
]


def fingerprint(rule, path, key, occ):
    kind = "s" if key is not None else "-"
    pre = "\0".join([rule, path, kind, key or "", str(occ)]).encode("utf-8")
    return hashlib.sha256(pre).hexdigest()[:32]


def illformed():
    out = []
    for r, p, units, hexbytes, o in ILLFORMED:
        out.append({"ruleId": r, "path": p, "keyUtf16": units, "keyUtf8Hex": hexbytes, "occ": o,
                    "fingerprint": fingerprint(r, p, None, o)})
    print(json.dumps(out, indent=2))


def main():
    if "--illformed" in sys.argv:
        return illformed()
    out = [
        {"ruleId": r, "path": p, "key": k, "occ": o, "fingerprint": fingerprint(r, n, k, o)}
        for r, p, n, k, o in CASES
    ]
    print(json.dumps(out, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
