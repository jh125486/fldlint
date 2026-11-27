# Releasing fldlint

The maintainer owns registry setup, release PR merges, and publication approval.
Complete the public-repository prerequisites in [#4] before the first release.

| Release group | Tags | Distribution |
| --- | --- | --- |
| npm | `spec-vX.Y.Z`, `core-vX.Y.Z`, `eslint-plugin-fldlint-vX.Y.Z` | `@fldlint/spec`, `@fldlint/core`, `eslint-plugin-fldlint` on npm |
| Go | `go/vX.Y.Z`, `go/golangci/vX.Y.Z` | Analyzer and golangci-lint module on the Go proxy |
| Java | `java/vX.Y.Z` | Maven Central publishing and signing are tracked in [#12] |

All groups start at `0.1.0`. The npm packages are linked and published in
dependency order. Go and Java version independently. Codemod uses the npm
version from the ESLint plugin release. Registry publishing from that tag is
deferred until upstream pins the actions and CLI used by its publish action.
The standalone JS CLI was deferred in [#16].

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

- Make the repository public after completing [#4]. Configure the GitHub
  `release` environment with maintainer approval and allow deployments from
  `main` and from `eslint-plugin-fldlint-v*` tags for npm recovery. The
  tagging and npm jobs both use this environment.
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
   npm exec --yes --package=release-please@17.6.0 -- \
     node .github/scripts/prepare-release-pr.cjs \
     "$(find "$(npm config get cache)/_npx" -path '*/release-please/build/src/bin/release-please.js' -print -quit)" \
     "$FLDLINT_RELEASE_METADATA"
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
   reviews before the maintainer squash-merges. After a Java release, the
   Maven strategy proposes a separate snapshot PR when you next run Release
   Please. Merge the snapshot PR before preparing a later stable Java release.
4. For the first golangci module tag, first release the analyzer module. Then update
   `go/golangci/go.mod` to require the tagged analyzer version and remove its local
   `replace`. Run Release Please again to create `go/golangci/v0.1.0`. This sequencing
   makes the published plugin build from a resolvable Go module.
5. After merge, check the Release Please run and approve the protected jobs
   when ready. Use `git fetch --tags` and `git tag -l 'go/*' 'java/*' '*-v*'`
   to inspect the tags. If only the npm job fails, re-run the failed jobs.
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
