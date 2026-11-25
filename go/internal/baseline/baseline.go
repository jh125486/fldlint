// Package baseline reads and writes the baseline file defined by
// spec/contracts/baseline.md: {"version": 1, "fingerprints": [...]}. The
// output is byte-identical to the JS implementation.
package baseline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
)

// ErrInvalid wraps every reason a baseline is rejected (malformed JSON, schema
// violation, unsupported version). Callers map it to their usage-error path.
var ErrInvalid = errors.New("invalid baseline")

var fingerprintRE = regexp.MustCompile(`^[0-9a-f]{32}$`)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Format serializes fingerprints sorted ascending and deduplicated, with a
// two-space indent and a trailing newline.
func Format(fingerprints []string) ([]byte, error) {
	list := slices.Clone(fingerprints)
	slices.Sort(list)
	list = slices.Compact(list)
	for _, fp := range list {
		if !fingerprintRE.MatchString(fp) {
			return nil, invalid("fingerprint %q", fp)
		}
	}
	if list == nil {
		list = []string{}
	}
	out := struct {
		Version      int      `json:"version"`
		Fingerprints []string `json:"fingerprints"`
	}{1, list}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil { // Encode appends the trailing newline
		return nil, err
	}
	return buf.Bytes(), nil
}

// Parse strictly parses baseline JSON text into a set of fingerprints.
func Parse(data []byte) (map[string]struct{}, error) {
	var obj map[string]json.RawMessage
	if !json.Valid(data) {
		// Unmarshal reports why (syntax error, trailing data, BOM).
		return nil, invalid("not valid JSON: %v", json.Unmarshal(data, &obj))
	}
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return nil, invalid("expected a JSON object")
	}
	for k := range obj {
		if k != "version" && k != "fingerprints" {
			return nil, invalid("unknown field %q", k)
		}
	}
	rawVersion, okV := obj["version"]
	rawFps, okF := obj["fingerprints"]
	if !okV || !okF {
		return nil, invalid(`both "version" and "fingerprints" are required`)
	}
	var version float64
	if string(rawVersion) == "null" {
		return nil, invalid(`"version" must be the number 1`)
	}
	if err := json.Unmarshal(rawVersion, &version); err != nil {
		return nil, invalid(`"version" must be the number 1 (flaglint baseline files are not accepted)`)
	}
	if version != 1 {
		return nil, invalid("unsupported baseline version %v", version)
	}
	var fps []any
	if err := json.Unmarshal(rawFps, &fps); err != nil || fps == nil {
		return nil, invalid(`"fingerprints" must be an array`)
	}
	set := make(map[string]struct{}, len(fps))
	for _, v := range fps {
		s, ok := v.(string)
		if !ok || !fingerprintRE.MatchString(s) {
			return nil, invalid("fingerprint %v: expected 32 lowercase hex characters", v)
		}
		set[s] = struct{}{}
	}
	return set, nil
}

// Read reads and validates a baseline file. A missing or unreadable file is an
// ErrInvalid too.
func Read(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a CLI argument
	if err != nil {
		return nil, invalid("cannot read %s: %v", path, err)
	}
	set, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return set, nil
}

// Write writes the baseline file for fingerprints.
func Write(path string, fingerprints []string) error {
	data, err := Format(fingerprints)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil { //nolint:gosec // a committed, non-secret file
		return invalid("cannot write %s: %v", path, err)
	}
	return nil
}
