# Oh My Pi 迁移工作流

## 1. 目标

本文把 [迁移执行手册](esper-go-port-runbook.md) 映射为 Oh My Pi（OMP）可直接执行的并行工作流。共享项目约束位于根 `AGENTS.md`；项目配置位于 `.omp/config.yml`，OMP 补充上下文和短规则位于 `.omp/AGENTS.md`、`.omp/RULES.md`，专用 agent 位于 `.omp/agents/`。

并发上限 4 是资源护栏，不是每轮应达到的目标。正常工作单元使用 2 至 3 个 agent；只有只读调查或依赖完全独立的组件才使用 4 路。共享 `internal/esper` 语义始终只有一个写入者。

从仓库根目录启动长期任务：

```sh
omp @goal.txt
```

OMP 会自动加载最近的 `.omp/AGENTS.md`、`.omp/RULES.md`、项目配置和专用 agent；`.omp/AGENTS.md` 会要求读取根 `AGENTS.md`。不要再把这些文件全文拼进启动消息。

项目不限定任何 model 或 provider。主 agent 和全部子 agent 使用 OMP 启动参数、用户级 role 映射或全局配置解析出的模型；更换模型不需要修改仓库文件。

## 2. Agent 分工

| Agent | 用途 | 写入 | 建议并发 |
| --- | --- | --- | --- |
| `java-oracle-scout` | Java execution、runtime ID 和可观测契约 | 否 | 1 至 2 |
| 内置 `scout` | Go API/helper、既有测试/evidence、Git 历史定位 | 否 | 1 至 2 |
| `go-slice-worker` | 明确文件边界内的共享 runtime/API 实现 | 是 | 共享核心最多 1 |
| `parity-asset-worker` | 契约冻结后的 oracle/scenario/独立 parity test 源文件 | 是 | 与 shared-core writer 文件不重叠时最多 1 |
| `parity-reviewer` | 集成后的 Java/Go parity 与证据审查 | 否 | 1，且只在集成后运行 |

主 agent 始终拥有：工作单元选择、跨任务契约、共享核心、manifest、evidence 最终状态、roadmap、CHANGELOG、格式化、测试、提交和推送。子 agent 不做这些收口动作。

## 3. 标准执行拓扑

1. 主 agent 从 roadmap/manifest 选择工作单元 N，记录允许修改文件、禁改文件、Java executions、runtime IDs 和定向验证命令。
2. 用一个 `task` batch 同时启动 Java oracle 调查和 Go 现状调查。大上下文只用 `local://<path>` 引用。
3. 主 agent 合并调查结果，冻结 input/output/error/lifecycle 契约和文件所有权；存在冲突或未知项时先解决，不启动实现。
4. 共享核心由主 agent 或一个 `go-slice-worker` 单写。若 oracle/scenario/test 的接口和文件边界也已冻结，可同时启动一个 `parity-asset-worker`；两个 writer 必须使用 `isolated: true` 且文件零重叠。
5. 子 agent 不运行 formatter、lint、build 或 tests。主 agent 集成后生成 trace/evidence，运行最窄定向测试和差分；失败时把精确错误回传给原 worker 修复，不重新启动失去上下文的 worker。
6. 定向验证通过后，主 agent 更新 manifest/evidence 和必要文档，并选择下一个候选工作单元 N+1。
7. 用一个 batch 同时启动：N 的 `parity-reviewer`、N+1 的 `java-oracle-scout`、N+1 的内置 `scout`。主 agent 在 batch 运行期间只读整理 N+1 契约，不开始 N+1 写入。
8. 修复 N 的所有 P0/P1/P2 finding，判断并处理 P3，统一执行提交前门禁。只有全绿后才提交并推送 `master`，然后立即进入已预取的 N+1。

OMP 可以自动合并并发文本修改，但不能证明共享状态机、顺序、timer 或生命周期的语义正确。不要以“能自动解冲突”为理由并行修改共享核心。

### 3.1 调度不变量

- 并发目标是缩短 work-unit cycle time；`maxConcurrency: 4` 只是上限。
- 有两个安全任务时必须在一个 batch 中启动，不能拆成连续 singleton task。
- singleton task 只允许在没有安全 sibling 时使用，主 agent 先记录原因。
- reviewer 运行时保留 N 的干净 diff；N+1 只读预取，深度最多一个工作单元。
- 调用 `hub wait` 前先完成所有安全只读工作；不得启动 reviewer 后立即进入轮询等待。
- 同时写入最多两路：一个 shared-core writer 和一个文件不重叠的 asset writer。
- 同一 work unit 的 finding 回传给原 worker/reviewer，不重复冷启动同角色 agent。
- 共享 runtime surface 和 oracle harness 的紧密 executions 组成一个自然工作单元，摊薄 review/门禁成本。
- trace/evidence、manifest、roadmap、CHANGELOG、格式化、测试、提交和推送始终由主 agent 收口。

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

### 4.2 并行实现 batch

只有跨任务接口已冻结、允许文件和禁改文件清楚时才使用。共享核心始终只有一个 writer；第二个 writer 仅处理文件不重叠的 parity assets。若没有独立资产工作，则主 agent 说明原因并只启动 core worker，或自己单写共享核心。

```json
{
  "context": "# Goal\n并行实现工作单元 N 的已冻结契约。\n# Constraints\n读取 local://docs/esper-go-port-runbook.md 和当前契约；两个 task 文件零重叠；不运行 formatter、lint、build、tests；不修改 manifest、roadmap、CHANGELOG、Goal、生成 trace/evidence；不 commit/push。\n# Contract\n列出输入、输出、ordering、Null/Missing、time、lifecycle、error phase，并分别列出 CoreFiles 与 AssetFiles。",
  "tasks": [
    {
      "name": "CoreImplementation",
      "agent": "go-slice-worker",
      "effort": "hi",
      "isolated": true,
      "task": "# Target\n只修改 CoreFiles 中的精确 production files 和 symbols；AssetFiles 及中央事实文件禁止修改。\n# Change\n按已冻结契约实现 Go API/runtime 语义，复用既有模式。\n# Acceptance\nproduction 代码完整，无 TODO/stub；结构化返回 changed files、assumptions、risks 和主 agent 应运行的定向测试。",
      "schemaMode": "strict"
    },
    {
      "name": "ParityAssets",
      "agent": "parity-asset-worker",
      "effort": "hi",
      "isolated": true,
      "task": "# Target\n只修改 AssetFiles 中明确分配的 Java oracle、scenario 输入或独立 Go parity test；CoreFiles 禁止修改。\n# Change\n按冻结契约编写可由主 agent 执行和验证的 parity 源资产；不得手写生成 trace/evidence。\n# Acceptance\n资产覆盖指定 executions/runtime IDs；结构化返回 changed files、assumptions、risks 及生成/测试命令。",
      "schemaMode": "strict"
    }
  ]
}
```

需要两个实现 task 时，必须在 batch `# Contract` 中先定义双方接口，并保证二者不依赖同一个未提交 API、不写同一状态机、不同时更新中央事实文件。

### 4.3 审查 N + 预取 N+1 batch

```json
{
  "context": "# Goal\n审查已集成的工作单元 N，同时为候选工作单元 N+1 冻结只读契约。\n# Constraints\n全部只读且不运行测试；N 和 N+1 边界必须明确。Java 固定 commit 是 oracle。N reviewer 读取质量策略、目标 Java source、Go diff、scenario/traces/evidence 和 manifest；N+1 scouts 只读取 roadmap/manifest 与候选相关文件。\n# Contract\nN 只报告可定位、可复现、由 N 引入的问题；N+1 返回 executions/runtime IDs、observable contract、最小 Go 实现面和文件冲突风险。",
  "tasks": [
    {
      "name": "ReviewCurrent",
      "agent": "parity-reviewer",
      "effort": "hi",
      "task": "# Target\n审查工作单元 N 的完整集成 diff 和对应 Java executions。\n# Change\n核对 producer/consumer、值与顺序、Null/Missing/type、time、lifecycle、error phase、scenario/trace/evidence 和 manifest 状态。\n# Acceptance\nstrict 返回 pass/fail；每个 finding 必须有文件、行、证据和修复要求。",
      "schemaMode": "strict"
    },
    {
      "name": "NextJavaContract",
      "agent": "java-oracle-scout",
      "effort": "med",
      "task": "# Target\n只读调查候选工作单元 N+1 的指定 Java executions。\n# Change\n提取 runtime IDs、输入序列、输出顺序、状态/time/lifecycle/error 契约和边界。\n# Acceptance\n返回可冻结的 Java observable contract，明确所有不确定项。",
      "schemaMode": "strict"
    },
    {
      "name": "NextGoSurface",
      "agent": "scout",
      "effort": "lo",
      "task": "# Target\n只读定位候选工作单元 N+1 的 Go API/runtime/helper/tests/evidence 和直接调用方。\n# Change\n给出最小实现面、可复用资产、回归面及与 N 未提交 diff 的文件冲突。\n# Acceptance\n返回带路径的 CoreFiles/AssetFiles 候选边界；不提出无关重构。",
      "schemaMode": "strict"
    }
  ]
}
```

若 roadmap/manifest 暂时没有安全的 N+1，允许只启动 `ReviewCurrent`，但主 agent 必须在调用前记录候选为空或依赖 N 未提交结果的具体原因。reviewer 运行期间仍先完成 N 的只读 diff/验证命令检查，再进入等待。

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
- 根 `AGENTS.md` 保存共享项目规则；`.omp/AGENTS.md` 提供 OMP 启动补充，`.omp/RULES.md` 保存压缩后仍应生效的硬约束；`goal.txt` 是 Codex/OMP 共用的长期任务入口。
- 本地 memory 只用于回忆决策和失败经验。每次恢复仍以当前 manifest、evidence、Git 和工作树复核，不能把 memory 当当前事实。
- `checkpoint`/`rewind` 适合把一次大规模只读探索压缩为结论；它只回退会话上下文，不恢复文件或 Git 状态。实现前后仍依赖独立 worktree、Git diff 和提交边界。
- 用 `Alt+A` 打开 Agent Hub 查看耗时、上下文、输出和 patch；发现越界或方向错误时直接 steer，避免等到任务结束后重做。
- 不把 `ultrathink`、`orchestrate` 等 magic keyword 当持久项目规则，它们只影响出现该词的当前轮。
