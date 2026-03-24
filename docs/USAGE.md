# OpenCode Plugin CLI - 使用指南

## 安装后如何使用插件

### 1. 安装插件

当你安装一个插件后,CLI 会自动:

1. **下载插件** 到缓存目录 (`~/.opencode-plugin-cli/cache/`)
2. **创建符号链接** 到 OpenCode 配置目录 (`~/.config/opencode/`)
3. **记录安装信息** 到 `installed_plugins.json`

### 2. 符号链接结构

安装插件后,符号链接会创建在以下位置:

```
~/.config/opencode/
├── skills/
│   ├── skill-name.md -> ~/.opencode-plugin-cli/cache/market/plugin/version/skills/skill-name.md
│   └── ...
├── commands/
│   ├── command-name.md -> ~/.opencode-plugin-cli/cache/market/plugin/version/commands/command-name.md
│   └── ...
└── agents/
    ├── agent-name.md -> ~/.opencode-plugin-cli/cache/market/plugin/version/agents/agent-name.md
    └── ...
```

### 3. OpenCode 如何发现插件

OpenCode 会自动扫描以下目录:
- `~/.config/opencode/skills/` - 技能
- `~/.config/opencode/commands/` - 命令
- `~/.config/opencode/agents/` - 代理

由于我们创建了符号链接,OpenCode 会:
1. **自动发现** 符号链接指向的文件
2. **加载插件** 的 skills, commands, agents
3. **立即可用** 无需重启

### 4. 实际使用示例

```bash
# 安装 code-simplifier 插件
opencode-plugin plugin install code-simplifier

# 输出:
# ✓ Successfully installed plugin: code-simplifier@1.0.0
#   From marketplace: anthropics/claude-plugins-official
#   Cache: ~/.opencode-plugin-cli/cache/anthropics/claude-plugins-official/code-simplifier/1.0.0
#   Skills: 0, Commands: 0, Agents: 1

# 验证符号链接
ls -la ~/.config/opencode/agents/
# 输出:
# code-simplifier.md -> ~/.opencode-plugin-cli/cache/.../code-simplifier.md

# OpenCode 现在可以使用 code-simplifier agent
# 在 OpenCode 中输入:
# "使用 code-simplifier agent 简化这段代码..."
```

### 5. 插件更新和删除

```bash
# 更新插件
opencode-plugin plugin update code-simplifier

# 删除插件(会自动删除符号链接)
opencode-plugin plugin remove code-simplifier
```

### 6. 验证安装

```bash
# 查看已安装插件
opencode-plugin plugin list

# 输出:
# Installed Plugins:
# 
#   code-simplifier@anthropics/claude-plugins-official
#     Version: 1.0.0
#     Scope: user
#     Path: ~/.opencode-plugin-cli/cache/.../code-simplifier/1.0.0
#     Installed: 2026-03-24 16:13:01
```

## 常见问题

### Q: 插件安装后 OpenCode 没有发现?

**A:** 检查以下几点:
1. 符号链接是否存在: `ls -la ~/.config/opencode/agents/`
2. 符号链接目标是否有效: `readlink ~/.config/opencode/agents/agent-name.md`
3. OpenCode 配置目录是否正确

### Q: 如何知道插件包含哪些组件?

**A:** 使用 `plugin info` 命令:
```bash
opencode-plugin plugin info code-simplifier
```

### Q: 符号链接冲突怎么办?

**A:** 如果已存在同名文件,CLI 会跳过并显示警告:
```
⚠️  Some files already exist and were skipped:
  - ~/.config/opencode/agents/existing-agent.md
```
你可以手动删除旧文件后重新安装。

### Q: 插件的多个版本如何管理?

**A:** 每个版本有自己的缓存目录:
```
~/.opencode-plugin-cli/cache/market/plugin/
├── 1.0.0/  # 版本 1.0.0
├── 1.1.0/  # 版本 1.1.0
└── latest/ # latest 版本
```
符号链接始终指向当前安装的版本。

## 目录结构详解

### 插件缓存结构
```
~/.opencode-plugin-cli/
├── known_marketplaces.json       # 已添加的 marketplaces
├── installed_plugins.json        # 已安装的插件
├── markets/                      # marketplace 本地仓库
│   └── anthropics/
│       └── claude-plugins-official/
│           └── plugins/
│               └── code-simplifier/
│                   ├── .claude-plugin/
│                   │   └── plugin.json
│                   ├── agents/
│                   │   └── code-simplifier.md
│                   ├── skills/
│                   └── commands/
└── cache/                        # 已安装插件的缓存
    └── anthropics/
        └── claude-plugins-official/
            └── code-simplifier/
                └── 1.0.0/
                    ├── agents/
                    │   └── code-simplifier.md
                    ├── skills/
                    └── commands/
```

### OpenCode 配置结构
```
~/.config/opencode/
├── skills/                        # OpenCode 自动扫描
│   └── skill-name.md -> symlink
├── commands/                      # OpenCode 自动扫描
│   └── command-name.md -> symlink
└── agents/                        # OpenCode 自动扫描
    └── agent-name.md -> symlink
```

## 高级用法

### 安装特定版本的插件
```bash
opencode-plugin plugin install code-simplifier --version 1.0.0
```

### 从特定 marketplace 安装
```bash
opencode-plugin plugin install my-plugin@my-market
```

### 查看插件详情
```bash
opencode-plugin plugin info code-simplifier
# 输出:
# Plugin: code-simplifier
# Description: Agent that simplifies and refines code...
# Version: 1.0.0
# Category: productivity
# Author: Anthropic <support@anthropic.com>
# Marketplace: anthropics/claude-plugins-official
# Available versions: 1.0.0, latest
```

## 故障排除

### 重置插件安装

如果遇到问题,可以完全重置:

```bash
# 删除所有插件
rm -rf ~/.opencode-plugin-cli

# 删除符号链接
rm -rf ~/.config/opencode/skills/*
rm -rf ~/.config/opencode/commands/*
rm -rf ~/.config/opencode/agents/*

# 重新添加 marketplace 和安装插件
opencode-plugin market add anthropics/claude-plugins-official
opencode-plugin plugin install code-simplifier
```

### 手动验证符号链接

```bash
# 检查符号链接
find ~/.config/opencode -type l -ls

# 查看符号链接目标
readlink ~/.config/opencode/agents/code-simplifier.md

# 验证目标文件存在
ls -la $(readlink ~/.config/opencode/agents/code-simplifier.md)
```
