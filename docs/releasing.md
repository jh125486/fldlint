# Releasing fldlint

The maintainer owns registry setup, release PR merges, and publication approval.
The repository is public. Complete the release-specific GitHub settings below
before the first release.

| Active Release Please component | Tags | Distribution |
| --- | --- | --- |
| npm | `spec-vX.Y.Z`, `core-vX.Y.Z`, `eslint-plugin-fldlint-vX.Y.Z` | `@fldlint/spec`, `@fldlint/core`, `eslint-plugin-fldlint` on npm |
| Go analyzer | `go/vX.Y.Z` | Analyzer on the Go proxy |

The npm packages are linked, start at `0.1.0`, and publish in dependency
order. The Go analyzer starts at `go/v0.1.0` and versions independently.
Codemod uses the npm version from the ESLint plugin release. Registry
publishing from that tag is deferred until upstream pins the actions and CLI
used by its publish action. The standalone JS CLI was deferred in [#16].

Java is not an active Release Please component. Re-add `java` only in the
Maven Central publishing PR tracked by [#12]. The `go/golangci` module is also
not active: first release `go/v0.1.0`, then update `go/golangci/go.mod` to
require that tag and remove its local `replace`. Re-add `go/golangci` in the
subsequent PR that makes the published plugin resolvable.

Use conventional commits and include the affected package in the scope where
useful, as in `fix(eslint-plugin): preserve comments`. Release Please uses
changed paths to select packages and writes a `CHANGELOG.md` for each. For a
shared spec change, the maintainer chooses which release groups to bump. Include
a release note under an affected package path if the group would otherwise
have no releasable change. Use a `Release-As: X.Y.Z` footer on that conventional
commit if a specific bump is needed. A Codemod-only change intended for release
needs a releasable entry under an npm package path. Keep rule references as
Markdown on GitHub.

## Prepare the registries

- The following are owner-side GitHub settings, not repository files. Protect
  the GitHub `release` environment with required reviewers and allow
  deployments from `main` and the npm release tags (`spec-v*`, `core-v*`, and
  `eslint-plugin-fldlint-v*`). The tagging and npm jobs both use this
  environment.
- Create a tag ruleset for `*-v*`, `go/*`, `go/golangci/*`, and `java/*`,
  restricting creation to maintainers and the explicitly approved release
  automation. The nested Go pattern is needed because `go/*` does not cover
  tags below `go/golangci/`. Include deferred namespaces now. The Release
  Please tagging job creates tags with `GITHUB_TOKEN`; require approval on the
  `release` environment before it runs and keep `contents: write` limited to
  that job. Before enforcing the ruleset, confirm GitHub offers a bypass actor
  that covers this token identity. The app-level bypass can also be used by
  other eligible write-enabled workflows. If the built-in GitHub Actions app
  cannot bypass, leave tag creation restrictions unenforced and ask the owner
  to choose another tag-creation identity before the first release.
- Verify that the npm names and the `@fldlint` scope are available to the
  maintainer. For each missing package, locally stage an artifact under a
  disposable version with `npm stage publish --access public`. Leave it
  unapproved. Verify that the resulting package entry can be configured for
  trusted publishing. [npm staged publishing](https://docs.npmjs.com/staged-publishing/)
- In each npm package's settings, add a GitHub Actions trusted publisher for
  owner `jh125486`, repository `fldlint`, workflow `release-please.yml` (the
  caller of `release-npm.yml`), and environment `release`. Allow direct
  `npm publish`. Complete the first successful OIDC publish within two days
  to validate the configuration. The workflow uses the GitHub runner's
  OIDC identity and requests provenance.
  [npm trusted publishing](https://docs.npmjs.com/trusted-publishers/)

## Release checklist

1. On current `main`, run `task workflows:lint`, `task check` and `task parity`.
   Confirm the fingerprint vectors pass and there are zero pending conformance
   cases for each language being released.
2. With `gh` logged in as the maintainer, preview the release PRs using the
   same version of Release Please as the pinned action.

   ```sh
   (
     set -eu
     token_file=$(mktemp)
     trap 'rm -f "$token_file"' EXIT
     gh auth token > "$token_file"
     npm exec --yes --package=release-please@17.6.0 -- release-please release-pr \
       --dry-run --repo-url=jh125486/fldlint --target-branch=main --token="$token_file"
   )
   ```

3. Prepare one release PR at a time. From a fresh worktree at current
   `origin/main`, use the exact candidate branch printed by the dry run.
   Release Please's PR command uses an unsigned commit API. Prepare the
   same changes locally and create a signed, DCO-signed-off commit with Git.

   ```sh
   metadata=$(mktemp -d)
   export FLDLINT_RELEASE_METADATA="$metadata"
   npm exec --yes --package=release-please@17.6.0 -- sh -c \
     'node .github/scripts/prepare-release-pr.cjs "$(command -v release-please)" "$FLDLINT_RELEASE_METADATA"'
   git diff
   git status --short
   ```

   Review the changelogs, versions, dependency pins and lockfile before
   committing. On the first run, ensure the labels in `"$metadata/labels"`
   exist on GitHub. Preserve the generated title, branch name, body and labels
   so the tagging job can recognize the PR.

   ```sh
   git add --all
   git commit -S -s -F "$metadata/title"
   git push -u origin HEAD
   gh pr create --base main --title "$(cat "$metadata/title")" \
  --body-file "$metadata/body.md" --label "$(cat "$metadata/labels")"
   ```

   Wait for `check (22)`, `check (24)`, `check (26)`, `dco` and the usual PR
   reviews before the maintainer squash-merges.
4. Do not prepare Java or golangci-lint module release PRs yet: neither is an
   active Release Please component. After `go/v0.1.0` exists, update
   `go/golangci/go.mod` to require that tag and remove its local `replace`. In
   the subsequent PR, re-add `go/golangci` to Release Please, then prepare
   `go/golangci/v0.1.0`; this sequencing makes the published plugin build from
   a resolvable Go module. Re-add `java` separately in the Maven Central
   publishing PR tracked by [#12]. After a Java release, the Maven strategy
   proposes a separate snapshot PR on its next run; merge that snapshot PR
   before preparing a later stable Java release.
5. After merge, check the Release Please run and approve the protected jobs
   when ready. Use `git fetch --tags` and `git tag -l 'go/*' '*-v*'` to inspect
   the tags. If only the npm job fails, re-run the failed jobs.
   If all three linked npm tags exist at the same release commit and the original
   run cannot be retried, the maintainer can recover from that tag with
   `gh workflow run release-please.yml --ref eslint-plugin-fldlint-vX.Y.Z`.
   The retry keeps the original package versions and verifies existing tarballs.
6. For npm, check the provenance badge and
   `npm view PACKAGE@VERSION dist.attestations` for each published package.
   For Go, warm the proxy with
   `GOPROXY=https://proxy.golang.org go list -m github.com/jh125486/fldlint/go@vX.Y.Z`.
7. Run the [README quickstarts](../README.md#quickstarts) for the released
   artifacts from a clean project using the public registries. Update the
   README milestone table after confirming the results.

[#4]: https://github.com/jh125486/fldlint/issues/4
[#12]: https://github.com/jh125486/fldlint/issues/12
[#16]: https://github.com/jh125486/fldlint/issues/16
[#18]: https://github.com/jh125486/fldlint/issues/18
