---
name: release
description: Use when the user wants to publish a new release, says "release", "publish release", "cut a release", or "new version". Covers tag creation, CI-driven build, and troubleshooting.
---

# Release

项目使用 **GitHub Actions + GoReleaser** 自动构建发布。Agent 只需创建并推送 tag，CI 负责其余一切。

## Workflow

1. **确认工作区干净**
   ```bash
   git status --porcelain
   ```
   有未提交的变更则先处理，不要带着脏工作区发版。

2. **确定版本号**（遵循 SemVer）
   ```bash
   git tag --sort=-v:refname | head -1       # 最新 tag
   git log <latest-tag>..HEAD --oneline      # 新增 commits
   ```
   - 有新 feature → minor 版本 bump（如 v0.2.0 → v0.3.0）
   - 只有 bugfix → patch bump（如 v0.2.0 → v0.2.1）
   - 重大变更 → major bump

3. **创建并推送 tag**
   ```bash
   git tag -a v0.3.0 -m "v0.3.0"
   git push origin v0.3.0
   ```

4. **监控 CI**
   ```bash
   gh run list --workflow=release.yml --limit 3
   ```
   CI 会自动：运行测试 → GoReleaser 构建多平台二进制 → 创建 GitHub Release 并上传 artifacts + checksums。

## 禁忌

- **禁止本地运行 `goreleaser release`** — 会上传 assets 到 GitHub，导致 CI 因 assets 冲突失败
- **tag 必须打在 HEAD commit 上** — 否则 goreleaser 会报 `tag was not made against commit` 校验失败

## 故障排查

### tag 打错了位置

```bash
git tag -d v0.3.0
git push origin :refs/tags/v0.3.0
git tag -a v0.3.0 -m "v0.3.0"
git push origin v0.3.0
```

### CI 因已有 release 失败

本地 goreleaser 提前创建了 release 导致冲突时：

```bash
gh release delete v0.3.0 --yes --cleanup-tag
git tag -a v0.3.0 -m "v0.3.0"
git push origin v0.3.0
```

### 查看 CI 日志

```bash
gh run view <run-id> --log-failed   # 查看失败日志
gh run watch <run-id> --exit-status # 实时跟踪
```
