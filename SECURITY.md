# Security Policy

## Supported versions

fldlint is pre-1.0. Only the latest release (and `main`) receives security
fixes.

## Reporting a vulnerability

Please do not open a public issue. Use GitHub's private vulnerability
reporting: on the repository's **Security** tab, choose **Report a
vulnerability**.

Include the affected engine (JS, Go or Java), a minimal reproduction, and the
version or commit.

## What counts as a vulnerability

fldlint reads and rewrites source code, so the following are in scope:

- A rewrite that silently changes program behavior (as opposed to a rewrite
  that is merely unwanted or incomplete, which is a normal bug).
- Arbitrary file writes outside the files the user asked to rewrite.
- Path traversal in the CLI (for example via symlinks, `..` segments, or
  baseline and config paths).
- Regular-expression denial of service (ReDoS) or other input that makes a
  run hang or exhaust resources.

## Response

This is a small project maintained on a best-effort basis. We aim to
acknowledge reports within about a week and to share a plan or fix as soon as
we reasonably can. We will credit reporters who want credit.
