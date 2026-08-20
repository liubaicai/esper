# Esper Go 迁移执行手册

## 1. 文档读取顺序

每个新工作循环先建立最小充分上下文，不要完整重读数百 KB 的实施计划和 CHANGELOG。

1. 读取根 `AGENTS.md`、`goal.txt`；Codex 还需核对并更新根 `PLANS.md`；
2. 读取 [执行路线图](esper-go-port-roadmap.md) 的当前状态、P0/P1 和遗漏表；
3. 读取 `testdata/compat/capability-manifest.json` 的 summary 及目标 capability/case；
4. 读取本文和 [质量策略](esper-go-port-quality-strategy.md)；
5. 使用 Codex 时读取 [Codex 工作流](esper-go-port-codex-workflows.md)，使用 OMP 时读取 [OMP 工作流](esper-go-port-omp-workflows.md)；
6. 使用 `rg` 定位 [实施规划](esper-go-port-implementation-plan.md) 中与目标域相关的章节，只读相关片段；
7. 使用 `git log`、`git show` 和 CHANGELOG 搜索目标域最近历史，不从头读取全部历史；
8. 读取对应 Java suite/execution 和直接相关的 Go 实现、测试、scenario、evidence。

事实来源：

- manifest/evidence：当前可验证事实；
- roadmap：当前优先级与遗漏；
- implementation plan：稳定范围、架构和语义规范；
- ADR：已确定的架构决策；
- CHANGELOG/Git：历史和审计，不作为当前统计来源。

## 2. 工作粒度

### 2.1 工作单元

一次实现和一次提交的默认粒度是一个可闭环工作单元，通常包含同一 capability 子域内 1 至 5 个紧密相关 Java executions。工作单元必须：

- 共享一个明确的可观测语义；
- 主要修改同一组实现文件；
- 可以在一个连续工作循环中实现、差分、回归和提交；
- 失败时可以独立回退；
- 不依赖尚未提交的其他工作单元。

不要把每个很小的 execution 强制拆成单独提交，也不要把整个大型 capability 域堆进一个提交。若需要共享重构，先做一个行为不变、验证完整的独立工作单元。

### 2.2 里程碑

一个里程碑通常是一个 capability 子域，由多个工作单元组成。里程碑收口时运行 race、域级差分、代表场景、相关 Docker 和 NFR 门禁，并更新路线图中的优先级和风险。

### 2.3 全局审计

每完成约 10 个工作单元、一个高风险域或一次公共 API 扩展，暂停新增功能并做一次只读审计：

- manifest 声明是否有真实 evidence；
- API 是否出现重复、漂移或不一致命名；
- 早期能力是否被后续共享改动破坏；
- Null/Missing、时间、顺序、错误阶段和生命周期是否一致；
- race、资源增长和外部服务门禁是否需要补跑；
- roadmap 数字是否仍由 manifest 推导而非手工猜测。

## 3. 单个工作单元流程

### 3.1 选择

1. 从 roadmap 的 P0/P1 和 manifest remaining 中选择高风险或高复用价值目标；
2. 明确 Java source、execution、runtime IDs 和当前 verification；
3. 列出允许修改的实现、测试、scenario/evidence 和文档文件；
4. 确认工作区，保留所有非本任务修改；
5. 记录定向测试和提交前门禁命令。

禁止为了提高百分比优先选择证据弱但容易登记的 runtime。

### 3.2 建立 oracle

1. 先读 Java 测试、关键实现和断言，写出可观测契约；
2. 优先复用 `tools/java-oracle` 和既有 scenario/runner；
3. 使用固定 Java commit 生成或确认 Java trace；
4. 需要扩展协议时先补 runner/normalizer/diff 测试和 mutation；
5. 不从 Go 当前行为反推 Java 期望。

### 3.3 实现

1. 搜索并复用已有 builder、runtime、state 和测试 helper；
2. 用 Go 惯例实现同一可观测语义，不复制 JVM 内部结构；
3. 共享核心修复必须检查所有调用方和既有场景；
4. 公共 API 变更同步检查 facade、生成器、API snapshot 和示例；
5. 不留下 TODO、stub、静默降级或无依据差异。

### 3.4 验证

验证顺序从快到慢：

1. 受影响包和目标 Go 测试；
2. 目标 Java/Go scenario diff；
3. 相关 mutation（质量策略要求时）；
4. manifest/evidence 校验；
5. 提交前全量本地门禁；
6. 达到里程碑边界时追加 race、stress、Docker 和 benchmark。

失败时定位根因并在当前工作单元内修复。除非证据证明测试假设错误，否则不得修改 oracle 使其适配 Go 结果。

### 3.5 更新状态

验证通过后，在同一提交中更新：

- capability manifest 和 summary；
- scenario、trace、evidence 及对应测试；
- roadmap 的当前优先级或风险（只有发生变化时）；
- CHANGELOG 的切片记录；
- README 的机器统计摘要（只有公开摘要发生变化时）。

不要把每次切片的长篇历史追加到实施规划顶部。稳定规范进入实施规划或 ADR；执行历史进入 CHANGELOG；当前状态进入 roadmap。

### 3.6 提交与推送

项目当前直接使用 `master`，暂不建设 GitLab CI，也不保护分支。

1. 确认当前分支和工作区；
2. 检查 diff 中没有无关文件、生成物或秘密；
3. 执行工作单元提交前门禁；
4. 创建一个语义完整、可回退的提交；
5. 提交信息以 capability/subdomain 开头并说明 Java execution；
6. 推送 `master`；
7. 只读验证远端 ref；commit hash 以 Git 为准，不回写 tracked file。

实际验证命令和语义结果必须在第 4 步提交前写入 CHANGELOG/活动计划。提交后
禁止仅为了记录刚生成的 hash 再修改 `PLANS.md` 或创建 checkpoint-only 提交。

禁止提交或推送已知失败状态。未完成工作不得为了保存进度伪装成已验证状态。

## 4. Agent 并行任务

并行的目标是提高单位时间内完成验证并提交的工作单元数，不是最大化活跃 agent 数。Codex 与 OMP 均按最多 4 个并发子 agent 设计；正常工作单元使用 2 至 3 个，4 路仅用于只读调查或完全独立的组件。调度维持“当前工作单元 N + 最多一个只读预取工作单元 N+1”，禁止无限预读造成事实过期。

标准拓扑：

1. 主 agent 选择工作单元 N；必须通过 agent 工具并行运行 Java oracle scout
   和 Go surface scout；并行 shell 命令不满足此门禁；
2. 主 agent 合并调查，冻结 observable contract、文件所有权和定向验证命令；
3. 共享核心只允许主 agent 或一个 `go-slice-worker` 写入；契约和文件边界完全冻结时，可同时启动一个 `parity-asset-worker` 编写互不重叠的 oracle/scenario/test 源文件；
4. 主 agent 集成、生成 trace/evidence、运行定向验证并更新中央事实；
5. 在启动审查前选择 N+1；同时启动 N 的 parity reviewer、N+1 的 Java contract scout 和 Go surface scout；
6. review 运行期间，主 agent 只读整理 N+1 契约和风险；不得开始 N+1 写入，避免污染 N 的集成 diff；
7. N 的 review 返回后，主 agent验证并修复 findings，统一执行全量门禁、提交和推送；
8. N 提交后，使用已经冻结的 N+1 契约立即进入实现阶段。

每次准备等待子 agent 前，主 agent 必须先检查：是否还能选择 N+1、合并 scout 结果、冻结只读契约、检查 N 的 diff 或准备验证命令。只要存在上述安全工作，就继续推进而不是等待。确实没有安全 sibling task 时允许 singleton task，但必须在启动前记录原因。

实现开始前，活动计划必须能回答：collaboration facility 是否可用、两个 scout
的 agent ID 和结论是什么，或为什么没有安全独立任务。不能以未验证的“工具
不可用”为由串行回退。

审查和完整门禁有固定成本。共享同一 runtime surface、oracle harness 和验证命令的紧密 executions，应在风险允许时组成一个自然闭环工作单元，不要人为拆成多个微小提交。worker 或 reviewer 返回 finding 后，使用同一个 agent follow-up；除非任务边界发生实质变化，不重新启动一个丢失上下文的 agent。

适合并行：

- Java execution/runtime ID/可观测契约调查；
- Go helper、调用方、既有 scenario/evidence 和历史调查；
- 相互独立的 connector、事件表示、oracle tooling 或 benchmark harness；
- 契约冻结后，一个 shared-core writer 与一个文件不重叠的 parity asset writer；
- 当前实现或审查期间，对下一个候选工作单元做只读预研；
- 工作单元 N 的集成审查与 N+1 的 Java/Go 双路调查。

默认不并行：

- context、pattern、join、state、runtime 等共享核心语义；
- 多个任务同时修改 manifest、roadmap、CHANGELOG 或 Goal；
- 依赖同一个未提交公共 API 的实现；
- 多个 worker 同时生成或重写同一 scenario/trace/evidence；
- 工作单元 N 审查期间开始 N+1 写入，使 reviewer 看到混合 diff；
- 需要频繁协调才能确定接口的任务。

共享并行约束：

- 不依赖子 agent 自动继承完整父会话；每个任务显式给出目标、上下文、允许/禁改文件和交付物；
- 子 agent 不运行 formatter、lint、build、tests，不 commit/push，不更新中央事实文件；
- 子 agent 不手写生成的 Java/Go trace 或 evidence；主 agent 运行工具并验证来源后生成；
- 主 agent 审查所有自动应用 patch，只格式化受影响文件，再统一运行定向和全量门禁；
- 有两个或更多安全任务时同时启动；singleton task 必须说明为何没有安全 sibling；
- reviewer 启动后不得立即等待；先消耗 N+1 的安全只读工作，预取深度最多一个工作单元；
- 同一 work unit 的修复回传给原 worker/reviewer，避免重复冷启动和重新读取；
- agent 返回或自动文本合并不代表语义安全；共享状态机、顺序和生命周期仍遵守单写者规则。

平台适配：Codex 使用根 `AGENTS.md`、`PLANS.md` 和 [Codex 工作流](esper-go-port-codex-workflows.md) 的任务契约；OMP 使用 `local://`、`# Goal / # Constraints / # Contract` batch context、`# Target / # Change / # Acceptance` task、agent output schema 和 `schemaMode: strict`，具体模板见 [OMP 工作流](esper-go-port-omp-workflows.md)。

## 5. 长任务与上下文管理

- 目标、约束和完成定义进入仓库文档；根 `AGENTS.md` 保存共享规则，Codex 用 `PLANS.md` 保存活动 checkpoint，OMP 用 `.omp/AGENTS.md` 和 `.omp/RULES.md` 保存平台补充。
- `goal.txt` 只负责 Codex/OMP 共用的长期目标入口，不复制 implementation plan、roadmap 或完整质量策略。
- 每轮只加载当前工作单元相关文件；Codex 使用明确仓库路径，OMP 可使用 `local://`；不要读取或粘贴整个仓库、完整实施规划或全部历史。
- 会话 memory 或摘要只是启发式历史，不是当前事实。恢复时必须用 manifest、evidence、Git 和工作树重新确认。
- `checkpoint`/`rewind` 只用于把大规模只读探索压缩为结论；它不恢复文件，不替代 Git diff、隔离 worktree 或提交。
- 用 Agent Hub（`Alt+A`）观察 agent 的活动、上下文和 patch，越界时立即 steer；优先修正已有 worker，不重复启动同类任务。
- 重要设计选择写 ADR；不要只留在对话、memory、提交信息或代码注释中。
- 在上下文压缩或长时间运行前，确保 roadmap、manifest 和工作树反映真实状态。
- 只有验证通过的事实进入完成清单；推测、待验证实现和阻塞保留为 remaining。
- 连续两次没有新证据时停止循环，缩小失败或重新定位阻塞点。

## 6. 恢复与阻塞

遇到失败时按以下顺序恢复：

1. 重现最小失败；
2. 判断是 Java oracle、scenario/normalizer、Go 实现还是测试环境问题；
3. 检查同类既有 evidence 和最近相关提交；
4. 修复根因并补回归；
5. 重跑定向验证和提交前门禁。

只有缺少必要外部输入、固定 oracle 无法取得或存在必须由用户决定的范围冲突时才视为阻塞。实现困难、测试耗时或上下文不足不是把条目标为 intentionally-different 的理由。
