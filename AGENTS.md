# AGENTS.md — opencode-plugin

> 本文档供 AI Agent 阅读，记录项目背景、技术决策和隐式知识。

## 项目概述

opencode-plugin 是一个 **Claude Code 插件生态兼容层**。它的核心目标是让 Claude Code 的 plugin marketplace 生态能在 opencode 上无缝运行——用户可以用和 Claude Code 相同的 `marketplace.json` 格式、相同的 plugin 目录结构、相同的 CLI 命令来安装和管理插件。

### 设计原则

1. **兼容而非替代**：我们不重新实现 Claude Code 已有的能力（如 `plugin init`、`plugin validate`），让 AI agent 自己完成这些操作更原生
2. **只做搬运层**：CLI 负责 marketplace 发现、plugin 下载/缓存、symlink 管理、MCP 配置注入；不负责 plugin 内容的创建或校验
3. **对齐 Claude Code 数据格式**：`marketplace.json`、`plugin.json`、`.mcp.json` 等文件格式完全兼容 Claude Code 规范
4. **渐进式实现**：Hooks、LSP、Monitors、Themes 等组件先解析存储，等 opencode runtime 支持后激活

## 导航

| 主题 | 文档 |
|------|------|
| 架构设计 | `docs/design/ARCHITECTURE.md` |
| Plugin 模块 | `docs/design/PLUGIN.md` |
| Marketplace 模块 | `docs/design/MARKETPLACE.md` |
| CLI 命令设计 | `docs/design/CLI.md` |
| 配置管理 | `docs/design/CONFIGURATION.md` |
| OpenCode 集成 | `docs/design/OPENCODE.md` |
| MCP 实现（用户指南） | `docs/MCP.md` |
| MCP 模块设计 | `docs/design/MCP.md` |
| 使用指南 | `docs/USAGE.md` |

## 关键隐式知识

### Plugin Source 的类型与 update 行为

marketplace.json 中每个 plugin 的 `source` 字段决定 `plugin update` 从哪里取代码。
`parsePluginSource`（`internal/marketplace/parser.go`）支持 7 种类型，按行为分成两组：

**Local Source（相对路径字符串）** — plugin 代码在 marketplace 仓库内
```json
{ "source": "./plugins/my-plugin" }
```
- `plugin update` 从 **marketplace 缓存目录** 读取（即 `market update` 克隆下来的那份）
- **必须先 `market update` 再 `plugin update`**，否则拿到旧代码

**Remote Source（对象形式）** — plugin 代码在独立仓库/包，不依赖 marketplace 缓存
```json
{ "source": { "source": "github", "repo": "owner/my-plugin" } }
```
取代码的方式由 `source` 字段决定（见 `internal/plugin/version.go` 的 `IsRemoteSource` /
`clonePluginSource`）：

| `source` 值 | 必填字段 | 取代码方式 |
|-------------|---------|-----------|
| `github`     | `repo`            | git clone `https://github.com/<repo>.git` |
| `git`        | `url`             | git clone `<url>` |
| `git-subdir` | `url`/`repo`+`path`| git clone 后取子目录 |
| `url`        | `url`             | 当作 git 仓库 clone（同 `git`，**非**任意 URL 下载） |
| `npm`        | `package`         | `npm install` |
| `pip`        | `package`         | ⚠️ 解析已支持，**安装尚未实现**（`version.go:186` 返回 not implemented） |

### 强制覆盖：--force / -f

```bash
opencode-plugin plugin update superpowers --force
opencode-plugin plugin install my-plugin -f
```
用于强制覆盖已存在的 skills、commands、agents。

### orphan symlink：disable/remove/update 的强制清理

`disable` / `remove` / `update` 删除 symlink 时（`opencode.RemoveSymlinks`），
正常只删 **target 落在 plugin cache 目录之内** 的链接。如果某个 symlink
**名字**匹配 plugin 但 **target 指向 cache 之外**（典型场景：开发态直接
`ln -s` 源码仓库留下的孤儿），它属于 orphan：

- **默认**：保留并打印 ⚠️ 列出所有 orphan，命令照常完成（orphan 不是失败，
  只是知情上报）。
- **`-f` / `--force`**：按名字匹配直接删除 orphan，对齐创建侧 `linkComponentDir`
  的按名匹配语义。

```bash
# 看见 ⚠️ orphan 提示后强制清理
opencode-plugin plugin disable opencode-customize@skill-forge -f
opencode-plugin plugin remove  opencode-customize@skill-forge -f
opencode-plugin plugin update  opencode-customize@skill-forge -f
```

`installer.Remove` / `installer.Disable` 签名均带 `force bool` 形参；
`market update` 的自动清理（`cmd/market/update.go`）永远传 `false`
（自动流程不强制删用户手建的软链）。

### `plugin update` 的原子性（two-phase commit）

`plugin update` 走 `installer.Update()`（`internal/plugin/installer.go`），不再
是 `Remove() + Install()` 的串联。流程是：

1. **Stage 1（materialize）**：把旧 cache 目录 rename 成 `.update-backup`，然后
   重新 clone / copy 出新版本。**网络步骤只在这一阶段发生。**
   - 如果失败：rename 回 `.update-backup` → 原路径，旧 plugin 完整保留，用户无感知。
2. **Stage 2（swap）**：删旧 symlinks/MCP、建新 symlinks/MCP、覆盖 install record。
3. **Stage 3（cleanup）**：删 `.update-backup`、`CleanupOldVersions` 清理同 plugin
   其它历史版本。

**重要**：因为 Stage 1 先 rename 旧 cache，`RemoveSymlinks` 在 Stage 2 调用时旧路径
已经不存在。symlink target 创建时经过 `EvalSymlinks`（如 macOS `/var` → `/private/var`），
所以代码里必须用 `EvalSymlinks(oldCachePath)` 解析后的路径传给 `RemoveSymlinks`，
否则词法路径不匹配会漏删旧 symlink。

## 本地开发速查

```bash
make build          # 构建到 bin/
make local-install  # 安装到 ~/.local/bin（无需 sudo）
make test           # 运行所有测试
go test ./...       # 同上
```

## 常见任务

### 更新 marketplace + plugin（正确流程）

```bash
opencode-plugin market update my-marketplace
opencode-plugin plugin update my-plugin@my-marketplace --force
```

### 排查 plugin 未更新的问题

1. 检查 marketplace 缓存是否最新：`cd ~/.opencode-plugin-cli/markets/<market-name> && git log --oneline -5`
2. 如果落后，手动更新：`opencode-plugin market update <market-name>`
3. 再执行 plugin update：`opencode-plugin plugin update <plugin>@<market-name> --force`

## 技术细节

### Symbolic Link 目录

- Skills：`~/.agents/skills/`
- Commands：`~/.agents/commands/`
- Agents：`~/.agents/agents/`

### 缓存目录结构

```
~/.opencode-plugin-cli/
├── cache/<market>/<plugin>/<version>/     # Plugin 缓存
├── markets/<market>/                      # Marketplace 克隆
│   └── .claude-plugin/marketplace.json
├── known_marketplaces.json                # Marketplace 配置
└── installed_plugins.json                 # 安装记录
```
