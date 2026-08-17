# Oh My Pi 迁移工作流

## 1. 目标

本文把 [迁移执行手册](esper-go-port-runbook.md) 映射为 Oh My Pi（OMP）可直接执行的并行工作流。项目配置位于 `.omp/config.yml`，项目上下文和持久规则位于 `.omp/AGENTS.md`、`.omp/RULES.md`，专用 agent 位于 `.omp/agents/`。

并发上限 4 是资源护栏，不是每轮应达到的目标。正常工作单元使用 2 至 3 个 agent；只有只读调查或依赖完全独立的组件才使用 4 路。共享 `internal/esper` 语义始终只有一个写入者。

从仓库根目录启动长期任务：

```sh
omp @goal.txt
```

OMP 会自动加载最近的 `.omp/AGENTS.md`、`.omp/RULES.md`、项目配置和专用 agent；不要再把这些文件全文拼进启动消息。

项目不限定任何 model 或 provider。主 agent 和全部子 agent 使用 OMP 启动参数、用户级 role 映射或全局配置解析出的模型；更换模型不需要修改仓库文件。

## 2. Agent 分工

| Agent | 用途 | 写入 | 建议并发 |
| --- | --- | --- | --- |
| `java-oracle-scout` | Java execution、runtime ID 和可观测契约 | 否 | 1 至 2 |
| 内置 `scout` | Go API/helper、既有测试/evidence、Git 历史定位 | 否 | 1 至 2 |
| `go-slice-worker` | 明确文件边界内的独立实现或测试资产 | 是 | 共享核心 1；独立组件最多 2 |
| `parity-reviewer` | 集成后的 Java/Go parity 与证据审查 | 否 | 1，且只在集成后运行 |

主 agent 始终拥有：工作单元选择、跨任务契约、共享核心、manifest、evidence 最终状态、roadmap、CHANGELOG、格式化、测试、提交和推送。子 agent 不做这些收口动作。

## 3. 标准执行拓扑

1. 主 agent 从 roadmap/manifest 选择一个闭环工作单元，记录允许修改文件、禁改文件、Java executions、runtime IDs 和定向验证命令。
2. 用一个 `task` batch 同时启动 Java oracle 调查和 Go 现状调查。大上下文只用 `local://<path>` 引用。
3. 主 agent 合并调查结果，冻结输入/output/error/lifecycle 契约和文件所有权；存在冲突或未知项时先解决，不启动实现。
4. 共享核心由主 agent 或一个 `go-slice-worker` 单写。独立组件可用 `isolated: true` 并行，成功 patch 自动应用后仍必须逐文件审查。
5. 子 agent 不运行 formatter、lint、build 或 tests。主 agent 集成后先运行最窄定向测试和差分；失败时把精确错误回传给原 worker 修复，不重新启动一个失去上下文的 worker。
6. 定向验证通过后，主 agent 更新 manifest/evidence 和必要文档，再启动 `parity-reviewer` 审查完整 work-unit diff。
7. 修复所有 P0/P1/P2 parity 或证据问题，统一执行提交前门禁。只有全绿后才提交并推送 `master`。

OMP 可以自动合并并发文本修改，但不能证明共享状态机、顺序、timer 或生命周期的语义正确。不要以“能自动解冲突”为理由并行修改共享核心。

## 4. 可复用 task 模板

### 4.1 并行调查 batch

```json
{
  "context": "# Goal\n为当前工作单元冻结 Java/Go 可观测契约。\n# Constraints\n只读；不运行测试；读取 local://docs/esper-go-port-roadmap.md、local://testdata/compat/capability-manifest.json 和目标相关文件。\n# Contract\nJava 结果必须给出 execution、runtime IDs、输入序列、输出顺序、时间/生命周期和错误阶段；Go 结果必须给出现有 API/helper、受影响调用方和可复用测试资产。",
  "tasks": [
    {
      "name": "JavaContract",
      "agent": "java-oracle-scout",
      "effort": "med",
      "task": "# Target\n调查指定 Java execution 及直接实现；不扩展到其他域。\n# Change\n只读提取可观测契约、runtime IDs、边界和 source references。\n# Acceptance\n返回可直接用于 scenario 和 Go 实现的完整契约，明确所有不确定项。",
      "schemaMode": "strict"
    },
    {
      "name": "GoSurface",
      "agent": "scout",
      "effort": "lo",
      "task": "# Target\n定位目标语义已有 Go builder/runtime/helper/test/evidence 和直接调用方。\n# Change\n只读搜索并说明可复用点、缺口与可能受影响文件。\n# Acceptance\n给出带路径的最小实现面和回归面；不提出无关重构。",
      "schemaMode": "strict"
    }
  ]
}
```

### 4.2 隔离实现 task

只有跨任务接口已冻结、允许文件和禁改文件清楚时才使用。共享核心工作单元一次只派一个 writer。

```json
{
  "context": "# Goal\n实现已冻结的工作单元契约。\n# Constraints\n读取 local://docs/esper-go-port-runbook.md 和当前契约/evidence；不运行 formatter、lint、build、tests；不修改 manifest、roadmap、CHANGELOG 或 Goal；不 commit/push。\n# Contract\n列出输入、输出、ordering、Null/Missing、time、lifecycle、error phase 和允许修改文件。",
  "tasks": [
    {
      "name": "ImplementSlice",
      "agent": "go-slice-worker",
      "effort": "hi",
      "isolated": true,
      "task": "# Target\n列出精确文件和 symbols；列出明确非目标与禁改文件。\n# Change\n按已冻结契约实现 Go API/runtime/scenario/test 资产，复用既有模式。\n# Acceptance\n代码与测试资产完整，无 TODO/stub；结构化返回 changed files、assumptions、risks 和主 agent 应运行的定向测试。",
      "schemaMode": "strict"
    }
  ]
}
```

需要两个实现 task 时，必须在 batch `# Contract` 中先定义双方接口，并保证二者不依赖同一个未提交 API、不写同一状态机、不同时更新中央事实文件。

### 4.3 集成审查 task

```json
{
  "context": "# Goal\n审查当前工作单元集成 diff 的行为 parity 与证据完整性。\n# Constraints\n只读；不运行测试；Java 固定 commit 是 oracle。读取 local://docs/esper-go-port-quality-strategy.md、目标 Java source、Go diff、scenario/traces/evidence 和 manifest 条目。\n# Contract\n只报告可定位、可复现、由本工作单元引入的问题。",
  "tasks": [
    {
      "name": "ParityReview",
      "agent": "parity-reviewer",
      "effort": "hi",
      "task": "# Target\n审查完整工作单元 diff 和对应 Java executions。\n# Change\n核对 producer/consumer、值与顺序、Null/Missing/type、time、lifecycle、error phase、scenario/trace/evidence 和 manifest 状态。\n# Acceptance\nstrict 返回 pass/fail；每个 finding 必须有文件、行、证据和修复要求。",
      "schemaMode": "strict"
    }
  ]
}
```

## 5. 主 agent 验证与提交

子任务结束后主 agent 按成本从低到高执行：

```text
审查每个自动应用的 patch 和 git diff
仅 gofmt 本工作单元改变的 Go 文件
受影响包/目标测试
目标 Java/Go scenario diff 和必要 mutation
go test ./internal/compat ./internal/app/manifest -count=1
make check
git diff --check
```

里程碑边界再运行 `make test-race`、域级完整差分、stress、相关 Docker 和 benchmark。不要让多个子 agent 重复运行全量门禁，也不要把所有验证推迟到项目结束。

门禁和审查通过后，主 agent检查 diff 中没有无关文件或秘密，创建一个语义完整提交，推送 `master`，并记录 commit hash 与实际验证命令。

## 6. 上下文与监督

- OMP 子 agent 没有父会话历史；用 `local://` 传文件，不把实施规划或大段源码复制进 task。
- `.omp/AGENTS.md` 提供启动上下文，`.omp/RULES.md` 保存压缩后仍应生效的硬约束；`goal.txt` 只作为长期任务启动入口。
- 本地 memory 只用于回忆决策和失败经验。每次恢复仍以当前 manifest、evidence、Git 和工作树复核，不能把 memory 当当前事实。
- `checkpoint`/`rewind` 适合把一次大规模只读探索压缩为结论；它只回退会话上下文，不恢复文件或 Git 状态。实现前后仍依赖独立 worktree、Git diff 和提交边界。
- 用 `Alt+A` 打开 Agent Hub 查看耗时、上下文、输出和 patch；发现越界或方向错误时直接 steer，避免等到任务结束后重做。
- 不把 `ultrathink`、`orchestrate` 等 magic keyword 当持久项目规则，它们只影响出现该词的当前轮。
