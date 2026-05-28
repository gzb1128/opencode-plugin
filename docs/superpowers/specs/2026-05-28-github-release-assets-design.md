# GitHub Release Assets Workflow Design

Date: 2026-05-28

## Goal

Add an automated release workflow for `opencode-plugin` that builds tested,
cross-platform CLI binaries and publishes them as GitHub Release assets.

The initial release channel is GitHub Releases only. Package managers such as
Homebrew, Scoop, npm, and container registries are out of scope for this design.

## Current Context

`opencode-plugin` is a Go CLI module at `github.com/opencode/plugin-cli`.
The repository currently has:

- a `Makefile` with local `build`, `test`, and `install` targets;
- no existing `.github/workflows` release automation;
- a Cobra root command with a hardcoded `cmd.version` default of `0.1.0`;
- source install instructions in `README.md`, but no binary release
  installation path.

The release workflow should fit the existing small Go CLI structure and avoid
adding custom release orchestration code.

## External Constraints

GitHub Actions is free for public repositories using standard GitHub-hosted
runners. If the repository is private, usage is subject to the account plan's
included minutes and storage, with charges or blocking after quota exhaustion.
Larger runners are not part of the free public-repository runner behavior.

GitHub Actions can run on tag pushes using `on.push.tags`. GitHub release events
also exist, but a `release: published` workflow starts after the Release object
already exists, which makes partial releases more likely if build or upload
steps fail.

GoReleaser's GitHub Action expects the repository history to be fetched with
`fetch-depth: 0` so it can inspect tags. Publishing archives to GitHub Releases
requires `contents: write` permission and can use the built-in `GITHUB_TOKEN`.

## Recommended Approach

Use a tag-driven GoReleaser workflow:

1. A maintainer creates and pushes a semver tag such as `v1.2.3`.
2. GitHub Actions runs tests on `ubuntu-latest`.
3. If tests pass, GoReleaser builds binaries for the configured target matrix.
4. GoReleaser creates the GitHub Release for that tag and uploads archives plus
   a checksum file.

This keeps the workflow deterministic: the tag is the release input, tests gate
publishes, and GitHub Release assets are produced in one CI run.

## Alternatives Considered

### Manual GitHub Actions Matrix

A hand-written workflow could build a `GOOS` and `GOARCH` matrix, package each
binary, compute checksums, and upload assets with `gh release upload`.

This avoids GoReleaser as a dependency, but it makes the workflow longer and
puts archive naming, checksum generation, and release upload behavior in custom
YAML. That is unnecessary for a Go CLI.

### Release Event Trigger

A workflow could run on `release.published` and upload assets after a maintainer
creates the GitHub Release in the UI.

This is less robust because the user-visible Release can exist before assets are
available. If CI fails, the project has a published but incomplete release.

## Architecture

The release design adds two configuration files:

- `.github/workflows/release.yml`
- `.goreleaser.yaml`

No application runtime code needs to change for the workflow itself. One small
release-readiness issue should be addressed during implementation: keep
`cmd.version` as the local-development fallback, but have release builds inject
the tag version through Go linker flags:

```text
-X github.com/opencode/plugin-cli/cmd.version={{ .Version }}
```

GoReleaser's `{{ .Version }}` value omits the leading `v`, so a `v1.2.3` tag
should produce `opencode-plugin --version` output of `1.2.3`. This keeps CLI
version output conventional while avoiding a source edit for every release.

## Workflow Design

`release.yml` should:

- trigger on `push` tags matching `v*.*.*`;
- include `workflow_dispatch` only with an explicit `tag` input; the job must
  validate that input, check out `refs/tags/<tag>`, and refuse to publish from a
  branch ref;
- set `permissions.contents` to `write`;
- checkout with `fetch-depth: 0`;
- set up Go using the module's Go version;
- validate that the release tag is a SemVer tag in the form
  `vMAJOR.MINOR.PATCH`, with an optional prerelease or build suffix;
- run `go test -short ./...` as the release gate;
- run `goreleaser/goreleaser-action` with GoReleaser v2 and
  `release --clean`;
- pass `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`.

The workflow should not run release publishing on branch pushes or pull
requests. Pull-request CI can be added separately if desired, but it is not part
of this release design.

The release gate intentionally uses `go test -short ./...` instead of
`go test ./...` because this repository has e2e tests that fetch external
marketplaces from GitHub. Release publishing should not depend on that external
network path. Full e2e validation remains a separate maintainer check before
creating a release tag.

## GoReleaser Design

`.goreleaser.yaml` should:

- declare `version: 2`;
- set `project_name: opencode-plugin`;
- build package `.` into binary `opencode-plugin`;
- set `CGO_ENABLED=0` for portable static-ish Go binaries where the dependency
  graph allows it;
- use GoReleaser `targets` syntax with the exact target list:
  - `linux_amd64_v1`
  - `linux_arm64`
  - `darwin_amd64_v1`
  - `darwin_arm64`
  - `windows_amd64_v1`
- use `-trimpath` and release `ldflags` including the version injection;
- produce `tar.gz` archives for Unix targets;
- produce `zip` archives for Windows targets;
- include `README.md` and `LICENSE` in archives;
- generate a SHA-256 checksum file named
  `opencode-plugin_checksums.txt`;
- use GoReleaser's default GitHub Release publisher.

The first version should not sign artifacts. Signing can be added later with
OIDC or GPG once there is a concrete consumer requirement.

## Release Operator Flow

Maintainers should release with:

```bash
go test ./...
git tag v1.2.3
git push origin v1.2.3
```

The pre-tag `go test ./...` check is intentionally outside the release workflow
because it may run e2e tests that fetch external marketplace fixtures. A
maintainer should run it when preparing a release and only create the tag after
the full suite passes in an environment with acceptable network access.

Maintainers should push one release tag at a time. Do not use `git push --tags`
for releases, because bulk tag pushes are easier to misfire and GitHub does not
create push events when more than three tags are pushed at once.

If a tag was pushed incorrectly before publishing assets, the maintainer should
delete or supersede the tag intentionally rather than rerunning an ambiguous
release. Immutable-release settings, if enabled later, must be considered before
adopting any retagging process.

Manual reruns should use the explicit workflow input and a tag ref, for example:

```bash
gh workflow run release.yml --ref v1.2.3 -f tag=v1.2.3
```

The workflow should reject a manual run when the selected ref and the `tag`
input do not name the same tag.

## Failure Handling

Tests fail:

- no Release assets are uploaded;
- if the failed tag should not become a release boundary, delete the local and
  remote tag before creating the corrected tag;
- after fixing the branch, create and push one corrected release tag.

GoReleaser build fails:

- no successful release should be considered complete;
- the maintainer fixes configuration or code and reruns only if the tag still
  points at the intended commit.

Asset upload fails after some artifacts are present:

- treat the release as incomplete;
- inspect the GitHub Release before rerunning;
- delete the incomplete release or partial assets manually before rerunning;
- do not rely on automatic artifact replacement in the first implementation.

## Documentation Updates

Implementation should update `README.md` with a concise binary install section:

- go to the repository's GitHub Releases page;
- download the archive for the user's OS and CPU architecture;
- extract `opencode-plugin`;
- move it into a directory on `PATH`;
- verify with `opencode-plugin --version`.

Source build instructions should remain as the fallback installation path.

## Verification Plan

Before merging release automation:

1. Run `go test -short ./...`.
2. Validate the GoReleaser v2 config with `goreleaser check`.
3. Run a GoReleaser snapshot build with
   `goreleaser release --snapshot --clean --skip=publish`.
4. Inspect generated archive names and checksum file in `dist/`.
5. Confirm a release-built binary reports the injected version.
6. Run `go test ./...` as a maintainer pre-release check when network access to
   external marketplace fixtures is acceptable.

After merging:

1. Push a test tag only when ready to publish a real release.
2. Confirm the GitHub Actions job passes.
3. Confirm the GitHub Release contains all expected archives and checksums.
4. Download one archive and run `opencode-plugin --version`.

## Security and Permissions

Use the built-in `GITHUB_TOKEN` with `contents: write`. Do not introduce a
personal access token for the GitHub Release assets-only scope.

Do not add package-manager publishing secrets in this first version. Keeping the
release surface to GitHub Releases avoids cross-repository write tokens and
reduces blast radius.

## Scope Boundaries

In scope:

- automated Release asset publishing from semver tags;
- cross-platform Go binary archives;
- checksum generation;
- release version injection;
- tag validation and safe manual rerun handling;
- README install instructions.

Out of scope:

- Homebrew tap publishing;
- Scoop manifests;
- npm or container publishing;
- artifact signing;
- automatic changelog curation beyond GoReleaser defaults;
- branch or pull-request CI redesign.

## References

- GitHub Actions billing and usage:
  https://docs.github.com/actions/administering-github-actions/usage-limits-billing-and-administration
- GitHub Actions events:
  https://docs.github.com/actions/reference/events-that-trigger-workflows
- GitHub `GITHUB_TOKEN`:
  https://docs.github.com/actions/concepts/security/github_token
- GoReleaser GitHub Actions:
  https://www.goreleaser.com/ci/actions/
- GoReleaser archives:
  https://goreleaser.com/customization/package/archives/
- GoReleaser checksums:
  https://goreleaser.com/customization/package/checksum/
