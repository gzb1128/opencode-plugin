# Plugin Disable/Enable Design

Date: 2026-06-08

## Goal

为 `opencode-plugin` 增加插件级别的 `disable` 和 `enable` 能力，让用户可以临时停用一个已安装插件，同时保留插件的安装记录、缓存源码、版本信息和来源信息。

禁用后的插件应继续出现在 `plugin list` 中，用户可以看到这是自己安装过、常用但当前未激活的插件，并可在后续通过 `plugin enable` 恢复。

## Non-goals

本设计不实现单个 skill、command、agent 的细粒度启停。`disable` 和 `enable` 的粒度是整个 plugin。

本设计不改造 opencode runtime，也不依赖 opencode runtime 理解 Claude Code 的 `enabledPlugins` 语义。

本设计不新增插件依赖图的级联禁用能力。禁用被其他插件依赖的插件时，只提供警告，不自动禁用依赖方或被依赖方。

## Current Context

`opencode-plugin` 当前的插件生命周期只有两种状态：

- installed：插件源码已 materialize 到 cache，skills/commands/agents 已通过 symlink 暴露到 `~/.agents/`，MCP server 已注入 `opencode.json`；
- removed：symlink、MCP 配置、cache 目录和 `installed_plugins.json` 记录都被删除。

当前安装记录位于 `~/.opencode-plugin-cli/installed_plugins.json`，数据结构由 `internal/config/types.go` 中的 `InstallRecord` 定义。该文件记录插件是否已安装、安装路径、版本和时间戳，但不记录启用状态。

当前激活机制不是 runtime 级别的加载开关，而是文件系统和配置副作用：

- skills、commands、agents 通过 `internal/opencode/linker.go` 创建 symlink；
- MCP 通过 `internal/mcp/manager.go` 写入 `~/.config/opencode/opencode.json` 的 `mcp` 字段；
- `plugin remove` 通过删除 symlink、删除 MCP 条目、删除 cache 和删除安装记录完成卸载。

因此，`disable` 不能只是写入一个配置字段。为了真正让 opencode 不再看到插件能力，必须让对应的 symlink 失效，并让 MCP server 停用。

## Claude Code Reference

Claude Code 的插件启停是 settings-first 设计。它同时控制插件管理层和 runtime，因此可以将安装状态与启用状态分开存储：

- `installed_plugins.json` 记录插件是否已安装及其 cache 信息；
- `settings.json` 的 `enabledPlugins` 记录插件启用状态，`true` 表示启用，`false` 表示禁用；
- runtime 启动时读取 `enabledPlugins`，只激活值为 `true` 的插件。

该模型在 Claude Code 中成立，是因为 Claude Code runtime 会参与判断插件是否应该被加载。

`opencode-plugin` 不能完整复刻该模型，因为它不控制 opencode runtime。即使在某个配置文件中记录 `enabledPlugins[plugin] = false`，opencode runtime 也不会因此自动忽略已经存在的 symlink 或 MCP 配置。

因此，`opencode-plugin` 应借鉴 Claude Code 的“安装状态与启用状态分离”思想，但启停动作必须落在实际激活载体上：symlink 和 MCP 配置。

## Recommended Approach

采用 plugin-level disabled state：插件仍然保持 installed，但新增 disabled 状态。

`plugin disable` 执行以下动作：

1. 保留 cache 目录；
2. 保留 `installed_plugins.json` 记录；
3. 删除该插件创建的 skills、commands、agents symlink；
4. 将该插件注入的 MCP servers 设置为 `enabled: false`；
5. 在安装记录中标记 `disabled: true`。

`plugin enable` 执行反向动作：

1. 从 cache 目录读取插件 manifest；
2. 重新创建 skills、commands、agents symlink；
3. 将该插件注入的 MCP servers 设置为 `enabled: true`；
4. 在安装记录中标记 `disabled: false`。

这个方案让 `opencode-plugin` 自己拥有明确的插件状态，同时不要求 opencode runtime 支持任何新语义。

## Alternatives Considered

### Only Record State

仅在 `installed_plugins.json` 中记录 `disabled: true`，但不删除 symlink，也不关闭 MCP。

该方案不能真正禁用插件。opencode 仍然可以通过 `~/.agents/` 下的 symlink 看到 skills、commands 和 agents，也可能继续使用已启用的 MCP server。

### Only Remove Symlinks

禁用时只删除 symlink，不记录 disabled 状态。

该方案可以让多数插件能力失效，但 CLI 无法可靠区分“用户手动删除了 symlink”和“插件被正式禁用”。`plugin list`、`plugin install`、`plugin update` 也无法基于状态做正确行为。

### Runtime Settings Emulation

仿照 Claude Code，在某个 settings 文件中写入 `enabledPlugins[plugin] = false`。

该方案不适合当前项目，因为 opencode runtime 不会读取这个字段来决定是否加载插件。它只能成为 CLI 内部状态，无法产生实际禁用效果。

## Data Model

扩展 `InstallRecord`：

```go
type InstallRecord struct {
    Scope        string    `json:"scope"`
    ProjectPath  string    `json:"projectPath,omitempty"`
    InstallPath  string    `json:"installPath"`
    Version      string    `json:"version"`
    InstalledAt  time.Time `json:"installedAt"`
    LastUpdated  time.Time `json:"lastUpdated"`
    GitCommitSHA string    `json:"gitCommitSha,omitempty"`
    Disabled     bool      `json:"disabled,omitempty"`
    DisabledAt   time.Time `json:"disabledAt,omitempty"`
}
```

`disabled` 默认值为 `false`，因此现有 `installed_plugins.json` 无需迁移即可继续读取。

`disabledAt` 用于展示和排查问题。启用插件后应清空该字段或写入零值，保存 JSON 时通过 `omitempty` 省略。

## CLI Design

新增命令：

```bash
opencode-plugin plugin disable <plugin[@market]>
opencode-plugin plugin enable <plugin[@market]>
```

`plugin enable` 支持 `--force` / `-f`，语义与 install 中的 force 一致：当 symlink 目标已存在且不是当前插件的 symlink 时，允许覆盖。

示例输出：

```text
Disabled plugin superpowers@official
Removed 6 symlinks
Disabled 1 MCP server
```

```text
Enabled plugin superpowers@official
Created 6 symlinks
Enabled 1 MCP server
```

幂等行为：

- 禁用已禁用插件时，应返回友好提示，不重复删除；
- 启用已启用插件时，应返回友好提示，不重复创建；
- 对未安装插件执行 enable/disable 时，应返回明确错误。

## Disable Flow

`Installer.Disable(pluginRef)` 的流程如下：

1. 解析 `pluginRef`，找到 `installed_plugins.json` 中对应的安装记录；
2. 如果找不到记录，返回未安装错误；
3. 如果记录已经是 disabled，返回 no-op；
4. 校验 `InstallPath` 位于 plugin cache 目录内；
5. 调用现有 linker 删除指向 `InstallPath` 内部的 symlink；
6. 调用 MCP manager 将 `{pluginName}.` 前缀的 MCP server 设置为 `enabled: false`；
7. 更新安装记录：`Disabled = true`，`DisabledAt = now`；
8. 保留 cache 目录，不删除插件源码；
9. 保留安装记录，不删除 plugin key。

禁用操作应尽量复用现有 `RemoveSymlinks` 的安全逻辑，只删除目标位于插件 cache 目录内的 symlink，不影响用户手写文件或其他插件 symlink。

## Enable Flow

`Installer.Enable(pluginRef, force)` 的流程如下：

1. 解析 `pluginRef`，找到对应安装记录；
2. 如果找不到记录，返回未安装错误；
3. 如果记录已经启用，返回 no-op；
4. 校验 `InstallPath` 位于 plugin cache 目录内且目录存在；
5. 读取 `InstallPath/plugin.json` 或现有安装流程生成的 fallback manifest；
6. 调用现有 linker 重新创建 symlink；
7. 如果存在 symlink 冲突且未传入 `--force`，返回冲突错误，并保持插件 disabled；
8. 调用 MCP manager 将 `{pluginName}.` 前缀的 MCP server 设置为 `enabled: true`；
9. 更新安装记录：`Disabled = false`，清空 `DisabledAt`。

启用操作必须在 symlink 创建成功后再更新安装记录。否则可能出现记录显示 enabled，但实际 symlink 未恢复的状态。

## MCP Behavior

MCP 采用保留配置、切换状态的方案。

新增 MCP manager 方法：

```go
DisableMCPConfig(pluginName string) error
EnableMCPConfig(pluginName string) error
```

这两个方法只处理 `opencode.json` 中 key 以 `{pluginName}.` 开头的 MCP entries，并只修改 `enabled` 字段：

```json
{
  "mcp": {
    "superpowers.memory": {
      "type": "local",
      "command": ["node", "server.js"],
      "enabled": false
    }
  }
}
```

禁用时不删除 MCP entry，原因是用户可能手动调整过 env、headers、command 或 args。保留 entry 可以避免 enable 时覆盖用户修改。

如果插件此前没有 MCP 配置，enable/disable MCP 步骤应作为 no-op。

如果 enable 时没有找到 `{pluginName}.` 前缀的 MCP entries，但插件 cache 中仍声明了 MCP servers，则应重新执行 MCP 注入流程，从插件 cache 恢复默认 MCP entries。该兜底只在 entries 缺失时触发；正常情况下只切换 `enabled` 字段，以保留用户手动修改。

## Plugin List Behavior

`plugin list` 应展示状态字段。

文本输出示例：

```text
NAME          MARKET      VERSION   STATUS
superpowers   official    latest    disabled
foo           official    1.2.0     enabled
```

JSON 输出应包含机器可读状态：

```json
{
  "name": "superpowers",
  "market": "official",
  "version": "latest",
  "status": "disabled",
  "disabled": true
}
```

默认 list 不应隐藏 disabled 插件，因为该功能的核心价值就是让用户知道自己保留了哪些插件。

## Install Behavior

当用户安装一个已经存在但处于 disabled 状态的插件时，推荐行为是将其视为 enable：

```bash
opencode-plugin plugin install superpowers
```

如果 `superpowers` 已安装但 disabled，则恢复 symlink 和 MCP，并将记录设为 enabled，而不是重新下载或创建重复记录。

如果用户传入 `--force`，则 enable 时也允许覆盖 symlink 冲突。

如果插件已安装且 enabled，保持当前已有的已安装提示行为。

## Update Behavior

`plugin update` 应保留插件更新前的 enabled/disabled 状态。

推荐行为：

- 更新前是 enabled，更新后仍然 enabled；
- 更新前是 disabled，更新后仍然 disabled。

对于 disabled 插件，更新流程应更新 cache 和安装记录，但最终不要留下可用 symlink，并应保持 MCP `enabled: false`。

由于当前 update 是 remove-then-install，实现时需要避免 remove 删除 disabled 状态，或者在 update 流程中先记录原状态，安装完成后再恢复 disabled 状态。

## Remove Behavior

`plugin remove` 应同时支持 enabled 和 disabled 插件。

对于 disabled 插件：

- symlink 删除通常是 no-op，但仍可安全调用；
- MCP uninstall 应删除 `{pluginName}.` 前缀的 MCP entries，而不是只设置 `enabled: false`；
- cache 目录应删除；
- `installed_plugins.json` 中的安装记录应删除。

remove 是彻底卸载，不保留 enable 所需状态。

## Dependency Behavior

第一版只提供警告，不阻止禁用。

如果其他已安装插件依赖被禁用插件，CLI 输出警告：

```text
Warning: plugin foo depends on superpowers. Disabling superpowers may affect foo.
```

不做级联禁用，原因是依赖关系的运行时影响取决于具体插件内容。强制级联容易造成用户意外失去更多插件能力。

## Error Handling

`InstallPath` 不存在：

- disable：仍可标记 disabled，并尝试禁用 MCP，因为 symlink 可能已经缺失；
- enable：返回 cache missing 错误，提示用户执行 `plugin update` 或重新安装。

Symlink 冲突：

- enable 未传 `--force` 时返回冲突错误；
- enable 传 `--force` 时复用现有 install force 行为覆盖目标；
- 冲突失败时不得将记录标记为 enabled。

MCP 配置文件不存在：

- disable 应作为 no-op，因为不存在可禁用的 MCP entries；
- enable 如果插件声明了 MCP servers，应通过现有 MCP 注入逻辑重新创建配置；
- 不应因为没有 MCP 配置而阻止 symlink 启停。

部分失败：

- disable：如果 symlink 已删除但 MCP 写入失败，应返回失败并说明 MCP 未完成；记录是否标记 disabled 需要保持保守，优先不标记，避免状态与 MCP 不一致；
- enable：如果 symlink 创建成功但 MCP 写入失败，应返回失败并说明 MCP 未完成；可以保留 symlink，但不应标记 enabled，便于用户重试。

## Testing Strategy

需要覆盖以下测试：

- `InstallRecord` 读取旧 JSON 时 `Disabled` 默认为 `false`；
- disable 会删除指向插件 cache 的 symlink，并保留 cache 和安装记录；
- disable 不会删除非 symlink 文件或指向其他插件的 symlink；
- enable 会从 cache 重建 symlink；
- enable 遇到冲突时未传 `--force` 会失败并保持 disabled；
- MCP disable 只将 `{pluginName}.` 前缀 entries 的 `enabled` 设为 `false`；
- MCP enable 只将对应 entries 的 `enabled` 设为 `true`；
- `plugin list` 文本和 JSON 输出包含 enabled/disabled 状态；
- install 一个 disabled 插件会执行 enable 行为；
- update 一个 disabled 插件后仍保持 disabled；
- remove 一个 disabled 插件会删除 cache、MCP entries 和安装记录。

## Implementation Notes

预计涉及文件：

- `internal/config/types.go`：扩展 `InstallRecord`；
- `internal/config/manager.go`：新增更新单个安装记录的方法；
- `internal/plugin/installer.go`：新增 `Disable`、`Enable`，并调整 install/update/remove/list 交互；
- `internal/mcp/manager.go`：新增 MCP enable/disable 状态切换方法；
- `cmd/plugin/plugin_disable.go`：新增 disable CLI；
- `cmd/plugin/plugin_enable.go`：新增 enable CLI；
- `cmd/plugin/plugin_install.go`：处理 disabled 插件再次 install 的行为；
- `cmd/plugin/plugin_update.go`：保留 update 前的 disabled 状态；
- 相关测试文件：覆盖 config、linker、MCP、installer、CLI 输出。

实现时应优先复用现有 linker 和 MCP manager 的安全逻辑，避免新增第二套 symlink 解析和 opencode config 写入逻辑。

## Deferred Scope

第一版是否需要 `plugin disable --all` 不在本设计范围内。该能力可以后续基于同一状态模型扩展。

第一版是否需要项目级别禁用不在本设计范围内。当前 `opencode-plugin` 的主要安装记录读取逻辑只使用每个 plugin key 的第一条记录，因此项目级别启停需要先明确多 scope 安装记录语义。

## Success Criteria

该功能完成后，用户可以：

1. 安装插件后通过 `plugin disable` 暂时停用它；
2. 在 `plugin list` 中继续看到该插件和 disabled 状态；
3. 确认对应 skills、commands、agents 不再通过 `~/.agents/` 暴露；
4. 确认对应 MCP servers 在 `opencode.json` 中保留但 `enabled: false`；
5. 通过 `plugin enable` 恢复 symlink 和 MCP 状态；
6. 在 disabled 状态下执行 remove 能彻底删除插件；
7. 在 disabled 状态下执行 update 后仍保持 disabled。
