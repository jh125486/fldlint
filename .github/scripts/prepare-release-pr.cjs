// Materialize one Release Please candidate locally so the maintainer can sign
// the commit with Git. This script reads GitHub and writes local files only.
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const { mkdirSync, realpathSync, writeFileSync } = require('node:fs');
const { createRequire } = require('node:module');
const { dirname, join, relative, resolve, sep } = require('node:path');

const repository = 'jh125486/fldlint';

const run = (command, args) => execFileSync(command, args, { encoding: 'utf8' }).trim();
const git = (...args) => run('git', args);
const mainSha = () => run('gh', ['api', `repos/${repository}/git/ref/heads/main`, '--jq', '.object.sha']);

async function prepareRelease() {
  const [releasePleaseBin, metadataDirectory] = process.argv.slice(2);
  assert.ok(releasePleaseBin && metadataDirectory, 'Expected the release-please executable and a metadata directory');
  const root = realpathSync(git('rev-parse', '--show-toplevel'));
  assert.equal(realpathSync('.'), root, 'Run from the release worktree root');
  assert.equal(git('status', '--porcelain'), '', 'The release worktree must be clean');
  const metadata = realpathSync(metadataDirectory);
  assert.ok(relative(root, metadata).startsWith(`..${sep}`), 'Use a metadata directory outside the worktree');
  const branch = git('branch', '--show-current');
  const sha = git('rev-parse', 'HEAD');
  assert.equal(sha, mainSha(), 'Start the release branch from the current origin/main');

  const { GitHub, Manifest } = createRequire(realpathSync(releasePleaseBin))('release-please');
  const [owner, repo] = repository.split('/');
  const github = await GitHub.create({ owner, repo, token: run('gh', ['auth', 'token']) });
  const manifest = await Manifest.fromManifest(github, 'main');
  const candidates = await manifest.buildPullRequests();
  const candidate = candidates.find(pr => pr.headRefName === branch);
  assert.ok(candidate, `No release candidate for ${branch}. Run release-pr --dry-run for candidate branch names.`);

  const changes = await github.buildChangeSet(candidate.updates, 'main');
  assert.ok(changes.size, 'The release candidate has no file changes');
  assert.equal(sha, mainSha(), 'origin/main moved while preparing the candidate');
  for (const [path, change] of changes) {
    const file = resolve(root, path);
    assert.ok(file.startsWith(root + sep), `Invalid release path: ${path}`);
    mkdirSync(dirname(file), { recursive: true });
    writeFileSync(file, change.content);
  }

  writeFileSync(join(metadata, 'title'), candidate.title.toString() + '\n');
  writeFileSync(join(metadata, 'body.md'), candidate.body.toString() + '\n');
  writeFileSync(join(metadata, 'labels'), candidate.labels.join(',') + '\n');
}

prepareRelease().catch(error => {
  console.error(error.message);
  process.exitCode = 1;
});
