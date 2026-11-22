## What

<!-- What does this PR change? -->

## Why

<!-- Motivation, linked issue (Fixes #123), or ADR. -->

## Verified

<!-- Tick what you ran; CI calls only these targets. -->

- [ ] `make spec-check`
- [ ] `make js-test`
- [ ] `make go-test`
- [ ] `make lint-go`
- [ ] `make lint-js`

## Contract impact

<!-- Does this touch docs/m0-contract.md, spec/ (rules, schemas, fixtures, contracts), or an ADR?
     If so: spec first, then engines; run `make spec-gen` and commit generated files;
     accepted ADRs are immutable (supersede them with a new one). Write "None" otherwise. -->

## Checklist

- [ ] Every commit is signed off (DCO): `git commit -s`
- [ ] PR title uses a conventional-commit prefix (`feat:`, `fix:`, `docs:`, `chore:`, `test:`, ...)
- [ ] Tests added or updated
- [ ] No new dependency without approval (see CONTRIBUTING.md)
- [ ] Fixtures updated for all languages that have a template
