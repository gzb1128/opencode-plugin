# GitHub Release Assets Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add tag-driven GitHub Actions release automation that publishes tested cross-platform `opencode-plugin` binaries as GitHub Release assets.

**Architecture:** Keep release behavior in repository-level configuration, not application code. Use GoReleaser v2 for build, archive, checksum, and GitHub Release publishing; use GitHub Actions only for trigger, tag validation, checkout, Go setup, tests, and invoking GoReleaser.

**Tech Stack:** Go 1.25 module, Cobra CLI, GitHub Actions, GoReleaser v2, GitHub Releases, built-in `GITHUB_TOKEN`.

---

## Spec Reference

Implement the approved design in:

- `docs/superpowers/specs/2026-05-28-github-release-assets-design.md`

The implementation must stay GitHub Release assets-only. Do not add Homebrew,
Scoop, npm, container, signing, or pull-request CI publishing.

## File Structure

- Create `.goreleaser.yaml`
  - Owns release artifact generation: binary matrix, archive formats, checksum
    filename, and version injection into `cmd.version`.
- Create `.github/workflows/release.yml`
  - Owns release trigger, tag/ref validation, checkout, Go setup, short test
    gate, and GoReleaser invocation.
- Modify `README.md`
  - Adds binary install instructions before source-build instructions.
- Do not modify `cmd/root.go`
  - Existing `version = "0.1.0"` remains the local-development fallback.
  - Release builds inject `cmd.version` with Go linker flags.

## Task 1: Add GoReleaser Config

**Files:**

- Create: `.goreleaser.yaml`
- Read: `cmd/root.go`
- Read: `go.mod`

- [ ] **Step 1: Confirm current version injection target exists**

Run:

```bash
rg -n 'version = "0.1.0"|Version: version' cmd/root.go
```

Expected: output includes both:

```text
14:	version = "0.1.0"
28:	Version: version,
```

- [ ] **Step 2: Write GoReleaser config**

Create `.goreleaser.yaml` with this exact content:

```yaml
version: 2

project_name: opencode-plugin

before:
  hooks:
    - go mod download

builds:
  - id: opencode-plugin
    main: .
    binary: opencode-plugin
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w -X github.com/opencode/plugin-cli/cmd.version={{ .Version }}
    targets:
      - linux_amd64_v1
      - linux_arm64
      - darwin_amd64_v1
      - darwin_arm64
      - windows_amd64_v1

archives:
  - id: opencode-plugin
    ids:
      - opencode-plugin
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}{{ with .Amd64 }}{{ if ne . "v1" }}_{{ . }}{{ end }}{{ end }}
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - README.md
      - LICENSE

checksum:
  name_template: "{{ .ProjectName }}_checksums.txt"
  algorithm: sha256

snapshot:
  version_template: "{{ incpatch .Version }}-snapshot"
```

- [ ] **Step 3: Validate YAML shape locally**

Run:

```bash
goreleaser check
```

Expected: pass with no invalid-option or unsupported-version errors.

If `goreleaser` is missing, install GoReleaser v2 by the project's preferred
local tooling and rerun the same command. Do not skip this validation.

- [ ] **Step 4: Run snapshot build without publishing**

Run:

```bash
goreleaser release --snapshot --clean --skip=publish
```

Expected:

- command exits 0;
- `dist/` contains archives for Linux, macOS, and Windows;
- `dist/` contains `opencode-plugin_checksums.txt`;
- no GitHub Release is created.

- [ ] **Step 5: Confirm snapshot binary version injection**

Run:

```bash
find dist -type f -perm -111 -name opencode-plugin -print -quit
```

Expected: prints one Unix executable path, for example:

```text
dist/opencode-plugin_darwin_arm64_v8.0/opencode-plugin
```

Then run the printed binary path with:

```bash
binary_path="$(find dist -type f -perm -111 -name opencode-plugin -print -quit)"
"${binary_path}" --version
```

Expected: output is not the fallback `0.1.0`; it contains the snapshot version
that GoReleaser injected.

- [ ] **Step 6: Commit GoReleaser config**

Run:

```bash
git add .goreleaser.yaml
git commit -m "build: add goreleaser config"
```

Expected: commit succeeds and includes only `.goreleaser.yaml`.

## Task 2: Add Tag-Driven Release Workflow

**Files:**

- Create: `.github/workflows/release.yml`
- Read: `.goreleaser.yaml`
- Read: `test/e2e/code_simplifier_test.go`
- Read: `test/e2e/mcp_integration_test.go`

- [ ] **Step 1: Confirm e2e tests are skipped in short mode**

Run:

```bash
rg -n 'testing.Short\(\)' test/e2e
```

Expected: output includes:

```text
test/e2e/code_simplifier_test.go:16:	if testing.Short() {
test/e2e/mcp_integration_test.go:221:	if testing.Short() {
```

- [ ] **Step 2: Create workflow directory**

Run:

```bash
mkdir -p .github/workflows
```

Expected: command exits 0.

- [ ] **Step 3: Write release workflow**

Create `.github/workflows/release.yml` with this exact content:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*.*.*'
  workflow_dispatch:
    inputs:
      tag:
        description: 'Release tag to publish, for example v1.2.3'
        required: true
        type: string

permissions:
  contents: read

jobs:
  release:
    name: Build and publish release assets
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Resolve and validate release tag
        id: release_tag
        shell: bash
        env:
          EVENT_NAME: ${{ github.event_name }}
          INPUT_TAG: ${{ inputs.tag }}
          REF_NAME: ${{ github.ref_name }}
          REF_VALUE: ${{ github.ref }}
        run: |
          set -euo pipefail

          if [[ "${EVENT_NAME}" == "workflow_dispatch" ]]; then
            tag="${INPUT_TAG}"
            expected_ref="refs/tags/${tag}"

            if [[ "${REF_VALUE}" != "${expected_ref}" ]]; then
              echo "workflow_dispatch releases must run from the same tag ref as the tag input." >&2
              echo "Use: gh workflow run release.yml --ref ${tag} -f tag=${tag}" >&2
              echo "Got ref=${REF_VALUE} tag=${tag}" >&2
              exit 1
            fi
          else
            tag="${REF_NAME}"
          fi

          semver_re='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
          if [[ ! "${tag}" =~ ${semver_re} ]]; then
            echo "Release tag must be SemVer with a leading v, for example v1.2.3 or v1.2.3-rc.1." >&2
            echo "Got: ${tag}" >&2
            exit 1
          fi

          echo "tag=${tag}" >> "${GITHUB_OUTPUT}"

      - name: Checkout release tag
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
          ref: ${{ steps.release_tag.outputs.tag }}

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run release test gate
        run: go test -short ./...

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 4: Validate workflow shell regex locally**

Run:

```bash
bash -c 'semver_re='\''^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'\''; for tag in v1.2.3 v1.2.3-rc.1 v1.2.3+build.5; do [[ "$tag" =~ $semver_re ]] || exit 1; done; for tag in 1.2.3 v1.2 v01.2.3 v1.02.3 v1.2.03; do if [[ "$tag" =~ $semver_re ]]; then exit 2; fi; done'
```

Expected: command exits 0.

- [ ] **Step 5: Run release test gate locally**

Run:

```bash
go test -short ./...
```

Expected: pass. The e2e tests report skipped tests in short mode.

- [ ] **Step 6: Commit release workflow**

Run:

```bash
git add .github/workflows/release.yml
git commit -m "ci: publish github release assets"
```

Expected: commit succeeds and includes only `.github/workflows/release.yml`.

## Task 3: Document Binary Release Installation

**Files:**

- Modify: `README.md`

- [ ] **Step 1: Locate the existing install section**

Run:

```bash
rg -n '^## Install|Build and install with `make`|go build -o bin/opencode-plugin' README.md
```

Expected: output includes the current `## Install` heading and source build
commands.

- [ ] **Step 2: Update `README.md` install section**

Replace the current `## Install` section body before `## Quick Start` with:

````markdown
## Install

### Download a Release Binary

Download the archive for your OS and CPU architecture from:

```text
https://github.com/gzb1128/opencode-plugin/releases
```

Extract the archive, move `opencode-plugin` into a directory on your `PATH`, and
verify the install:

```bash
opencode-plugin --version
```

### Build from Source

Build and install with `make`:

```bash
make build
make install
```

`make install` copies the binary to `/usr/local/bin/`. If that directory is not
writable, build the binary and copy it to any directory on your `PATH`:

```bash
make build
cp bin/opencode-plugin /path/on/your/PATH/
```

You can also build from source directly:

```bash
git clone https://github.com/gzb1128/opencode-plugin.git
cd opencode-plugin
go build -o bin/opencode-plugin .
```
````

- [ ] **Step 3: Confirm README release install text**

Run:

```bash
rg -n 'Download a Release Binary|github.com/gzb1128/opencode-plugin/releases|opencode-plugin --version|Build from Source' README.md
```

Expected: all four patterns are present.

- [ ] **Step 4: Commit README update**

Run:

```bash
git add README.md
git commit -m "docs: add release binary install instructions"
```

Expected: commit succeeds and includes only `README.md`.

## Task 4: Final Verification and Release Dry Run

**Files:**

- Read: `.goreleaser.yaml`
- Read: `.github/workflows/release.yml`
- Read: `README.md`

- [ ] **Step 1: Check repository status**

Run:

```bash
git status --short
```

Expected: no output.

- [ ] **Step 2: Run short test gate**

Run:

```bash
go test -short ./...
```

Expected: pass.

- [ ] **Step 3: Validate GoReleaser config**

Run:

```bash
goreleaser check
```

Expected: pass with no invalid-option or unsupported-version errors.

- [ ] **Step 4: Run GoReleaser snapshot build**

Run:

```bash
goreleaser release --snapshot --clean --skip=publish
```

Expected:

- command exits 0;
- `dist/` contains Linux, macOS, and Windows archives;
- Unix archives use `.tar.gz`;
- Windows archive uses `.zip`;
- `dist/opencode-plugin_checksums.txt` exists.

- [ ] **Step 5: Inspect artifact list**

Run:

```bash
find dist -maxdepth 1 -type f | sort
```

Expected: output includes files shaped like:

```text
dist/opencode-plugin_checksums.txt
dist/opencode-plugin_0.1.1-snapshot_darwin_amd64.tar.gz
dist/opencode-plugin_0.1.1-snapshot_darwin_arm64.tar.gz
dist/opencode-plugin_0.1.1-snapshot_linux_amd64.tar.gz
dist/opencode-plugin_0.1.1-snapshot_linux_arm64.tar.gz
dist/opencode-plugin_0.1.1-snapshot_windows_amd64.zip
```

If the current repository tag history causes GoReleaser to choose a different
snapshot version, the version segment may differ; the OS, architecture, archive
format, and checksum filename must still match this shape.

- [ ] **Step 6: Verify release-built binary version**

Run:

```bash
binary_path="$(find dist -type f -perm -111 -name opencode-plugin -print -quit)"
"${binary_path}" --version
```

Expected: output is not `0.1.0`. It contains the GoReleaser snapshot version.

- [ ] **Step 7: Run full maintainer pre-release test if network is acceptable**

Run:

```bash
go test ./...
```

Expected: pass in an environment where the external marketplace fixture fetch
is allowed. If the environment blocks network access, record that this full
pre-release check must be run before creating the first real release tag.

- [ ] **Step 8: Review final diff**

Run:

```bash
git log --oneline -4
git status --short
```

Expected:

- the last commits correspond to this plan's release config, workflow, and docs
  tasks;
- `git status --short` has no output.

## Task 5: First Real Release Procedure

**Files:**

- Read: `.github/workflows/release.yml`
- Read: `.goreleaser.yaml`

Run these steps only when maintainers are ready to publish a real GitHub
Release.

- [ ] **Step 1: Run full pre-tag validation**

Run:

```bash
go test ./...
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Expected: all commands pass.

- [ ] **Step 2: Create and push one release tag**

Run:

```bash
git tag v1.2.3
git push origin v1.2.3
```

Expected: GitHub Actions starts the `Release` workflow for tag `v1.2.3`.

Do not use:

```bash
git push --tags
```

- [ ] **Step 3: Verify GitHub Release assets**

After the workflow finishes, inspect the GitHub Release for tag `v1.2.3`.

Expected assets:

```text
opencode-plugin_1.2.3_darwin_amd64.tar.gz
opencode-plugin_1.2.3_darwin_arm64.tar.gz
opencode-plugin_1.2.3_linux_amd64.tar.gz
opencode-plugin_1.2.3_linux_arm64.tar.gz
opencode-plugin_1.2.3_windows_amd64.zip
opencode-plugin_checksums.txt
```

- [ ] **Step 4: Verify downloaded release binary**

Download one archive, extract it, and run:

```bash
./opencode-plugin --version
```

Expected:

```text
opencode-plugin version 1.2.3
```

If Cobra prints only `1.2.3` in this repository's current command format, that
is acceptable as long as the fallback `0.1.0` is not printed.

## Failure Handling During Implementation

- If `go test -short ./...` fails, fix the failing package before continuing.
- If `goreleaser check` fails, fix `.goreleaser.yaml` before touching the
  workflow.
- If snapshot build succeeds but artifact names differ from expected output,
  inspect GoReleaser's `dist/artifacts.json` and update `.goreleaser.yaml`; do
  not update the README to match accidental names.
- If the first real release tag fails after being pushed, decide whether that tag
  should remain a release boundary. If it should not, delete the local and remote
  tag before creating the corrected tag.

## Self-Review Checklist

- Spec coverage:
  - GitHub Release assets-only: Task 1 and Task 2 configure only GoReleaser
    GitHub Release publishing; Task 3 documents only GitHub Releases.
  - SemVer tag trigger and validation: Task 2 includes `v*.*.*` trigger and a
    Bash SemVer validation step.
  - Safe manual rerun: Task 2 requires matching `workflow_dispatch` `tag` input
    and the corresponding tag ref.
  - Short release test gate: Task 2 and Task 4 run `go test -short ./...`.
  - Full e2e pre-release check: Task 5 runs `go test ./...` before tagging.
  - GoReleaser v2 config, target matrix, archives, checksum, version injection:
    Task 1 covers all required fields.
  - README install instructions: Task 3 adds the binary release install path.
- Placeholder scan:
  - No unresolved marker terms or fill-in instructions are intentionally left.
- Scope check:
  - No Homebrew, Scoop, npm, container, signing, or pull-request CI work is in
    this plan.
