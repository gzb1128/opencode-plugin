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
| MCP 实现 | `docs/MCP.md` |
| Plugin Update 隐式依赖 | `docs/PLUGIN_UPDATE_MARKETPLACE_DEPENDENCY.md` |

## 关键隐式知识

### Plugin Source 的两种模式

Marketplace 中的 plugin 有两种 source 类型，直接影响 `plugin update` 行为：

**Local Source（相对路径）** — plugin 代码在 marketplace 仓库内
```json
{ "source": "./plugins/my-plugin" }
```
- `plugin update` 从 **marketplace 缓存目录** 读取
- **必须先 `market update` 再 `plugin update`**，否则拿到旧代码
- 详见：`docs/PLUGIN_UPDATE_MARKETPLACE_DEPENDENCY.md`

**Remote Source（远程对象）** — plugin 代码在独立仓库
```json
{ "source": { "source": "github", "repo": "owner/my-plugin" } }
```
- `plugin update` 直接 clone 远程仓库，不依赖 marketplace 缓存

### 强制覆盖：--force / -f

```bash
opencode-plugin plugin update superpowers --force
opencode-plugin plugin install my-plugin -f
```
用于强制覆盖已存在的 skills、commands、agents。

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
