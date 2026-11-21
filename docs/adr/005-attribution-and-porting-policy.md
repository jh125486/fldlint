# ADR 005: Attribution and porting policy

Date: 2026-11
Status: Accepted

## Context

flaglint-js and flaglint-go are MIT-licensed and contain algorithms (call-site detection, client identity resolution) relevant to fldlint. MIT requires keeping the copyright and license notice in copies or substantial portions. fldlint is a separate project with its own design.

## Decision

Port only targeted algorithms, never whole modules. Every ported file carries an SPDX header (`SPDX-License-Identifier: MIT`) and a `Derived from flaglint-<x> <path> @<sha>` comment. `NOTICE` lists the upstream repositories, the commits referenced and the upstream license text. fldlint's own code is MIT under the repository `LICENSE`. The README states that fldlint is a separate project, derived in part from flaglint, and not affiliated with it.

## Consequences

- License obligations are met and provenance is auditable per file.
- Contributors must record the upstream path and SHA when porting, and add new upstream repos or SHAs to `NOTICE`.
- Design choices (fingerprint, baseline, exit codes) are fldlint's own and are not required to match upstream.
