# OpenCode Plugin CLI - 使用指南

## 安装后如何使用插件

### 1. 安装插件

当你安装一个插件后,CLI 会:

1. **下载插件** 到缓存目录 (`~/.opencode-plugin-cli/cache/`)
2. **如果插件包含 `skills/`**,创建符号链接到 agents 目录 (`~/.agents/skills/`)
3. **记录安装信息** 到 `installed_plugins.json`

### 2. 符号链接结构

如果插件包含 `skills/`,符号链接会创建在以下位置:

```
~/.agents/skills/
├── skill-name.md -> ~/.opencode-plugin-cli/cache/market/plugin/version/skills/skill-name.md
└── ...
```

### 3. 技能如何被发现

符号链接位于 `~/.agents/skills/` 目录下。

已创建符号链接的插件技能会被自动发现和加载。

### 4. 实际使用示例

```bash
# 安装 code-simplifier 插件
opencode-plugin plugin install code-simplifier

# 输出:
# ✓ Successfully installed plugin: code-simplifier@1.0.0
#   From marketplace: claude-plugins-official
#   Cache: ~/.opencode-plugin-cli/cache/claude-plugins-official/code-simplifier/1.0.0
#   Skills: 0

# 如果插件包含 skills/, 可以验证符号链接
ls -la ~/.agents/skills/
# 输出:
# skill-name.md -> ~/.opencode-plugin-cli/cache/.../skill-name.md

# 已链接的插件技能现在可以使用
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
#   code-simplifier@claude-plugins-official
#     Version: 1.0.0
#     Scope: user
#     Path: ~/.opencode-plugin-cli/cache/.../code-simplifier/1.0.0
#     Installed: 2026-03-24 16:13:01
```

## 常见问题

### Q: 插件安装后 OpenCode 没有发现?

**A:** 检查以下几点:
1. 符号链接是否存在: `ls -la ~/.agents/skills/`
2. 符号链接目标是否有效: `readlink ~/.agents/skills/skill-name.md`
3. agents 目录是否正确

### Q: 如何知道插件包含哪些组件?

**A:** 对 marketplace 中的本地插件源,可以使用 `plugin info` 命令:
```bash
opencode-plugin plugin info my-local-plugin
```

### Q: 符号链接冲突怎么办?

**A:** 如果已存在同名文件,CLI 会跳过并显示警告:
```
⚠️  Some files already exist and were skipped:
  - ~/.agents/skills/existing-skill.md
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
缓存按版本保存。安装不同版本时,如果同名符号链接已经存在,CLI 会跳过该链接并提示冲突; 需要先删除旧插件或手动清理冲突链接。

## 目录结构详解

### 插件缓存结构
```
~/.opencode-plugin-cli/
├── known_marketplaces.json       # 已添加的 marketplaces
├── installed_plugins.json        # 已安装的插件
├── markets/                      # marketplace 本地仓库
│   └── claude-plugins-official/
│       └── plugins/
│           └── my-plugin/
│               ├── .claude-plugin/
│               │   └── plugin.json
│               └── skills/
└── cache/                        # 已安装插件的缓存
    └── claude-plugins-official/
        └── my-plugin/
            └── 1.0.0/
                └── skills/
                    └── skill-name.md
```

### Agents 配置结构
```
~/.agents/
└── skills/                        # 技能符号链接
    └── skill-name.md -> symlink
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
`plugin info` 适用于 marketplace 中的本地插件源。远程插件源需要先安装后查看安装记录。

```bash
opencode-plugin plugin info my-local-plugin
# 输出:
# Plugin: my-local-plugin
# Description: Agent that simplifies and refines code...
# Version: 1.0.0
# Category: productivity
# Author: Anthropic <support@anthropic.com>
# Marketplace: my-market
# Available versions: 1.0.0, latest
```

## 故障排除

### 重置插件安装

如果遇到问题,可以重置插件缓存和技能链接:

```bash
# 删除所有插件
rm -rf ~/.opencode-plugin-cli

# 检查并手动删除对应插件的技能符号链接
find ~/.agents/skills -type l -ls

# 如果插件安装过 MCP,还需要清理 ~/.config/opencode/opencode.json 中对应的 mcp 条目

# 重新添加 marketplace 和安装插件
opencode-plugin market add anthropics/claude-plugins-official
opencode-plugin plugin install code-simplifier
```

### 手动验证符号链接

```bash
# 检查符号链接
find ~/.agents -type l -ls

# 查看符号链接目标
readlink ~/.agents/skills/skill-name.md

# 验证目标文件存在
ls -la $(readlink ~/.agents/skills/skill-name.md)
```
