# Esper Go 迁移执行手册

## 1. 文档读取顺序

每个新工作循环先建立最小充分上下文，不要完整重读数百 KB 的实施计划和 CHANGELOG。

1. 读取 [执行路线图](esper-go-port-roadmap.md) 的当前状态、P0/P1 和遗漏表；
2. 读取 `testdata/compat/capability-manifest.json` 的 summary 及目标 capability/case；
3. 读取本文和 [质量策略](esper-go-port-quality-strategy.md)；
4. 使用 OMP 并行任务时读取 [OMP 工作流](esper-go-port-omp-workflows.md)；
5. 使用 `rg` 定位 [实施规划](esper-go-port-implementation-plan.md) 中与目标域相关的章节，只读相关片段；
6. 使用 `git log`、`git show` 和 CHANGELOG 搜索目标域最近历史，不从头读取全部历史；
7. 读取对应 Java suite/execution 和直接相关的 Go 实现、测试、scenario、evidence。

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
7. 记录 commit hash 和实际执行的验证命令。

禁止提交或推送已知失败状态。未完成工作不得为了保存进度伪装成已验证状态。

## 4. OMP 并行任务

并行的目标是缩短调查和独立组件实现的等待时间，不是最大化活跃 agent 数。项目配置允许最多 4 个并发子 agent；正常工作单元使用 2 至 3 个，4 路仅用于只读调查或完全独立的组件。

标准拓扑：

1. 主 agent 选择工作单元并冻结跨任务契约；
2. 一个 `task` batch 并行运行 `java-oracle-scout` 和内置 `scout`；
3. 主 agent 合并调查，解决不确定项并划定文件所有权；
4. 共享核心只允许主 agent 或一个 `go-slice-worker` 写入；
5. 独立组件才使用 `isolated: true`，最多两个 writer；
6. 主 agent 集成、定向验证并更新中央事实；
7. `parity-reviewer` 只读审查集成 diff；
8. 主 agent 统一执行全量门禁、提交和推送。

适合并行：

- Java execution/runtime ID/可观测契约调查；
- Go helper、调用方、既有 scenario/evidence 和历史调查；
- 相互独立的 connector、事件表示、oracle tooling 或 benchmark harness；
- 当前实现执行期间，对下一个候选工作单元做只读预研。

默认不并行：

- context、pattern、join、state、runtime 等共享核心语义；
- 多个任务同时修改 manifest、roadmap、CHANGELOG 或 Goal；
- 依赖同一个未提交公共 API 的实现；
- 多个 worker 同时生成或重写同一 scenario/trace/evidence；
- 需要频繁协调才能确定接口的任务。

并行约束：

- 子 agent 没有父会话历史；共享背景放在 batch `# Goal / # Constraints / # Contract`，大文件使用 `local://<path>`；
- 每个 task 使用 `# Target / # Change / # Acceptance`，明确 Java executions、允许和禁改文件及 observable acceptance；
- 使用 agent 自带 output schema，并将 `schemaMode` 设为 `strict`；
- 子 agent 不运行 formatter、lint、build、tests，不 commit/push，不更新中央事实文件；
- 主 agent 审查所有自动应用 patch，只格式化受影响文件，再统一运行定向和全量门禁；
- OMP 自动文本合并不代表语义安全；共享状态机、顺序和生命周期仍遵守单写者规则；
- 具体批处理模板见 [OMP 工作流](esper-go-port-omp-workflows.md)。

## 5. 长任务与上下文管理

- 目标、约束和完成定义进入仓库文档；`.omp/AGENTS.md` 提供会话启动上下文，`.omp/RULES.md` 保存必须持续生效的短规则。
- `goal.txt` 只负责启动长期目标，不复制 implementation plan、roadmap 或完整质量策略。
- 每轮只加载当前工作单元相关文件；使用 `rg` 和 `local://`，不读取或粘贴整个仓库、完整实施规划或全部历史。
- OMP memory 是启发式历史，不是当前事实。恢复时必须用 manifest、evidence、Git 和工作树重新确认。
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
