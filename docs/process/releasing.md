# Releasing CDS — Release Process and Versioning

`version.txt` holds the **last released** version. Nothing moves it on a PR merge — the release builder owns it, and a release is the only thing that changes it. The version number therefore describes what was released, not how many PRs were merged.

---

## What a release produces

One source revision produces every supported platform pack, so a release — and
the updates later published from it — carry executables built from the same
code. CDS is released for Linux only:

```
wso2cds-linux-amd64-<version>.zip   + .sha256 + .md5
wso2cds-linux-arm64-<version>.zip   + .sha256 + .md5
wso2cds-<version>-release-metadata.yaml
```

The pack directory, its zip and its update-service product identity are all
the same string, which is why each platform is a separate product. `<version>`
carries no leading `v`.

`make dist` builds them, cross-compiling with `CGO_ENABLED=0` — every
dependency is pure Go, so no per-platform build host is needed.

Three checks run over the result, on every pull request and again before a
release publishes:

| | |
|---|---|
| `distribution/check-pack.sh` | every pack carries the files an installation needs, and a product identity naming itself |
| `distribution/verify-install.sh` | the Identity-Server-first path really installs: a release is served locally and the real `install_cds.sh` is driven against it, through resolution, checksum, extraction and the hand-off to `wso2cds enable` |
| `distribution/artifact-summary.sh` | writes the artifacts and their SHA-256 digests into the run summary |

So a dependency that stops cross-compiling, a pack that loses a file, or a
broken acquisition path is caught on the change that caused it rather than by
the first customer to download it.

The release metadata names the product, the platform artifacts with their
checksums, and the compatibility the pack itself declares in
`repository/conf/compatibility.yaml`. It is what the
[bootstrap installer](../guides/on-prem-installation.md#path-b--identity-server-first)
reads to resolve a CDS release for an installed Identity Server version
without downloading a pack first.

### Filling in the compatibility metadata

`config/repository/conf/compatibility.yaml` carries empty version fields on
`main`, and the release fills them in:

| Field | Set to |
|---|---|
| `identity_server.supported` | the Identity Server releases this release was verified against |
| `extensions.version` | the published CDS extension baseline for those releases |
| `console.release_line` | the identity-apps line carrying the matching CDS Console |

A build that leaves them empty is not broken: `wso2cds enable` warns that the
baseline is unverified and continues, rather than claiming a compatibility
nobody has tested.

---

## Onboarding a release to the update service

The product build stages `updates/product.txt` and nothing else. The update
tool setup script is published by the Updates team rather than built from this
source, which is the same split `product-is` uses — `updates/product.txt` is
in its source tree and packed by its build, and the setup script appears in
nothing it builds.

So attaching it is a separate, on-demand step: Actions → **🔄 Attach Update
Tool**, with the tag of a published release. It downloads that release's
packs, installs `bin/update_tool_setup.sh` into each, rewrites the checksums,
and uploads the result — replacing the release assets when `publish` is set.
It needs the `U2_UPDATE_TOOL_SETUP_URL` secret, pointing at the
`update_tool_setup.zip` the Updates team publishes.

To do the same locally, from an existing build:

```bash
make dist
distribution/u2-onboarding/attach-update-tool.sh target/dist update_tool_setup.zip
```

Keeping this out of `make dist` means the product build has one behaviour for
everyone: it never depends on a file that is not in the repository, and never
silently produces packs that cannot be updated. See
[distribution/u2-onboarding/](../../distribution/u2-onboarding/README.md).

A CDS service fix is published as one update entry per platform product, all
carrying the executable built from the same revision. The fix is not complete
until every platform entry is published, because a customer on the other
platform would otherwise not have it.

The CDS extension bundles and the CDS Console are **not** published this way.
They are deployed into the Identity Server, not into CDS, and are delivered
through the WSO2 Identity Server's own update stream. A logical fix that
changes both the CDS service and an Identity Server-resident artifact is two
updates, under their two product identities.

---

## The model

A release run resolves the version to release from its dispatch inputs, commits it to the release branch, builds from it, and tags that commit.

```
merge PR  → nothing happens to version.txt
merge PR  → nothing happens to version.txt

dispatch release
  → resolve version (bump_type / version / use_existing_version)
  → refuse if that tag already exists
  → write version.txt
  → commit + push it to the release branch
  → make dist
  → tag the bump commit, push the tag
  → create the GitHub release
```

The tag is the release identity. A version that already has a tag is refused, so the same version can never be released twice.

Between releases, every build off `main` reports the same version. That is expected — local dev zips share a filename until the next release.

---

## Inputs

Actions → **🚀 CDS Release Builder** → Run workflow.

| Input | Default | Use it when |
|---|---|---|
| `bump_type` | `minor` | Normal scheduled release. Bumps `version.txt`. `patch` for a maintenance release. |
| `version` | — | You need an exact version — a GA off a prerelease, or a prerelease itself. Overrides `bump_type` and `use_existing_version`. A bare `0.4.0` is accepted. |
| `use_existing_version` | `false` | Retrying a release whose version bump already landed. See [Retrying a failed release](#retrying-a-failed-release). |
| `branch` | `main` | Releasing a maintenance version from a release branch. |
| `commit_version_bump` | `true` | Set `false` to release without moving the branch. |
| `prerelease` | `false` | Marks the GitHub release as a pre-release. |

`version`, `bump_type` and `use_existing_version` are mutually exclusive, in that order of precedence.

---

## Running a release

### Scheduled release

The common case takes no inputs at all:

```bash
gh workflow run release.yml --repo wso2/identity-customer-data-service
```

`v0.3.65` → releases and tags `v0.4.0`, and commits `version.txt` as `v0.4.0`.

### Maintenance release

A third-digit release, run against the relevant release branch:

```bash
gh workflow run release.yml --repo wso2/identity-customer-data-service \
  -f branch=0.3.x -f bump_type=patch
```

`branch` controls what is checked out, built, bumped and pushed, so this reads `0.3.x`'s own `version.txt` and pushes the bump there. `main` is untouched.

Leave `--ref` alone so the canonical workflow definition on `main` is the one that runs, even when releasing another branch.

### Major release

```bash
gh workflow run release.yml --repo wso2/identity-customer-data-service -f bump_type=major
```

### Prerelease, or an exact version

```bash
gh workflow run release.yml --repo wso2/identity-customer-data-service \
  -f version=v0.4.0-beta1 -f prerelease=true
```

Use an explicit `version` to go from a prerelease to GA. A `patch` bump off `v0.4.0-beta1` computes `v0.4.1`, not `v0.4.0`.

### Retrying a failed release

If a run commits the bump to `v0.4.0` and *then* the build fails, the branch says `v0.4.0` but no tag exists. Re-running with the default `minor` would release `v0.5.0` and silently skip `v0.4.0`. Instead, release `version.txt` exactly as it stands:

```bash
gh workflow run release.yml --repo wso2/identity-customer-data-service \
  -f use_existing_version=true
```

This commits nothing and releases the version already on the branch.

---

## What happens after the release

`update-deployment-versions.yml` runs automatically when the release builder completes. It resolves the released tag as the newest product tag reachable from the released branch, and updates `CDS_APPLICATION_TAG` in the deployment repository's CI variables.

Because the tag lands on the version bump commit created *during* the release, the tag is resolved from the branch rather than from the commit the run was dispatched against.

---

## Verifying a release

| Check | Where |
|---|---|
| Version bump commit | `Version bump: vX.Y.Z` on the release branch |
| Tag | Points at that bump commit |
| GitHub release | Tagged `vX.Y.Z`, carries both platform zips, their checksums and the release metadata |
| Pack contents | `distribution/check-pack.sh target/dist` passes |
| Deployment repo | `CDS_APPLICATION_TAG` updated in `ci-pipelines/cds/setup-variables.yaml` |

---

## Helm chart versioning

The Helm chart is versioned independently of the product. `helm-chart-releaser.yml` fires on pushes to `main` that touch `install/helm/confs/**`, `install/helm/templates/**` or `install/helm/values.yaml`, and bumps the chart version in `install/helm/Chart.yaml`. A product release does not move the chart version, and vice versa.

---

## Troubleshooting

| Failure | Cause and fix |
|---|---|
| `Version 'vX.Y.Z' already exists as a git tag` | That version was already released. Pass a new `version`, or a different `bump_type`. |
| `Could not parse version '<x>' from version.txt` | `version.txt` is malformed. It must be `vX.Y.Z`, optionally with a `-PRERELEASE` suffix. |
| `Version 'vX.Y' does not follow vX.Y.Z[-PRERELEASE]` | An explicit `version` input was short a digit. |
| `version.txt not found or empty` | The release branch is missing `version.txt`. |
| `No product tag found on branch` (deployment update) | The release builder did not push its tag. Check the release run before re-dispatching the deployment update manually with its `tag` input. |
