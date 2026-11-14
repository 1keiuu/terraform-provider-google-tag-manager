# Vendored GitHub Actions

The workflows use rebuilt copies of checkout, setup-go, create-github-app-token,
ghaction-import-gpg, codeql-action, setup-terraform, and goreleaser-action.
Their upstream releases contain vulnerable dependencies. Each copy keeps the
upstream implementation and replaces dependencies through its package manifest
and lockfile. Both runtime and build dependencies must pass `npm audit`.

`sources.json` records the official upstream repository, full commit SHA, source
archive SHA-256, and runtime files to copy. Builds download and verify that source,
install the checked-in lockfile with lifecycle scripts disabled, and execute the
explicit build command in the checked-in package manifest. TypeScript builds
check the runtime entry points and their imports; upstream test harnesses and
documentation generators are not included. Upstream and dependency licenses are
preserved alongside the generated files.

## Validation

Use Node.js 24, npm, Python 3.12 or newer, Git, and GnuPG:

```sh
python3 scripts/audit-vendored-actions.py
python3 scripts/build-vendored-actions.py --check
node --test scripts/action-smoke.test.mjs
```

The audit checks every vendored runtime and build dependency, artifact checksums,
and every workflow Action reference. The rebuild check compares generated files
with the checked-in artifacts. The smoke tests create a temporary GitHub App key
and local API server to check repository scope, permissions, and token revocation.
They also import a temporary GPG key, sign and verify a message, and verify post-job
key removal. No repository secrets are used by these tests.

The regular Go, Terraform documentation, CodeQL, and GoReleaser configuration jobs
exercise the rebuilt Actions on GitHub-hosted Ubuntu runners. The protected
release jobs retain their existing permission and environment restrictions.

## Updating dependencies

1. Update exact dependency versions or overrides in an Action's `package.json`.
   Update its lockfile with `npm install --package-lock-only --ignore-scripts` in
   that directory. Keep all dependency resolutions on the official npm registry.
2. Rebuild that Action, for example:

   ```sh
   python3 scripts/build-vendored-actions.py setup-go
   ```

3. Run the validation commands and inspect the manifest, lockfile, and generated
   artifact changes. Do not edit bundled JavaScript directly.
4. Commit the generated files with their manifests. If checkout changed, update
   every workflow's checkout reference to this new commit SHA in a following
   commit. Push both commits together.

Checkout is referenced as
`1keiuu/terraform-provider-google-tag-manager/.github/actions/checkout@<commit>`
because repository files do not exist until checkout runs. All other Actions use
local paths. The audit downloads the pinned checkout's package manifests and
artifact manifest and requires an exact match with the audited local copy.

To change the upstream source, update its commit and archive SHA-256 in
`sources.json`, review the upstream changes, and repeat the rebuild and validation.
CI verifies reproducibility and audits dependencies on pull requests, main pushes,
and a weekly schedule. A new advisory fails the audit even when the lockfile has
not changed.
