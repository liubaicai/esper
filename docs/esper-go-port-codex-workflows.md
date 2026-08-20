# Codex 迁移工作流

## 1. 定位与启动

根目录的 `AGENTS.md` 是 Codex 自动加载的共享项目指令，`PLANS.md` 是可恢复
的活动 ExecPlan，`goal.txt` 是 Codex 与 OMP 共用的长期目标入口。`.omp/`
只保存 OMP 配置、短规则和专用 agent，不是 Codex 的配置来源。

从仓库根目录新建或继续一个直接使用当前检出的 Codex task。需要让该 task
持续提交到当前 `master` 时，使用本地 checkout；不要在一次性隔离 worktree
中启动整个长期迁移。启动消息保持简短：

```text
按照 goal.txt 继续 Esper 到 Go 的迁移。先读取根 AGENTS.md 和 PLANS.md，
核对当前 Git、roadmap、manifest 与 evidence，再选择并完成下一个可闭环工作单元。
持续更新 PLANS.md；所有门禁、独立 parity review、提交和推送规则均按仓库文档执行。
```

Codex 会自动注入适用的 `AGENTS.md`，但不会把 `goal.txt` 或 `PLANS.md` 当成
隐式待办，因此启动消息仍应明确要求读取它们。恢复旧 task 时也先核对文件和
Git，不把对话摘要当当前事实。

仓库不提供 `.codex/config.toml`，也不限定 model、provider 或 reasoning
级别；这些由用户级 Codex 设置和启动时选择决定。

## 2. 活动 ExecPlan

`PLANS.md` 只维护当前工作单元的可恢复状态：baseline、契约、文件所有权、
进度、发现、决策、验证与下一步。开始写代码前必须填实当前 work-unit
contract；每个重要阶段结束后立即更新，不能等到上下文即将压缩时才补写。

一个工作单元提交后：

1. 把 commit 和实际验证结果记录到 CHANGELOG/Git 对应位置；
2. 在 `PLANS.md` 保留一行近期 outcome；
3. 清空当前契约，基于最新 roadmap/manifest 选择下一单元；
4. 预取最多一个 N+1，只允许只读调查。

路线图维护跨工作单元的当前优先级，manifest/evidence 维护机器事实，
CHANGELOG/Git 维护历史。不要把这些内容完整复制到 `PLANS.md`。

## 3. Codex agent 分工

Codex 不读取 `.omp/agents/*.md` 作为自定义角色。主 agent 应使用根
`AGENTS.md` 的通用约束，把具体角色、范围和交付物写入每个子任务。

| 角色 | 用途 | 写入权限 |
| --- | --- | --- |
| Java oracle scout | Java execution、runtime ID、可观测契约 | 只读 |
| Go surface scout | Go API/helper、调用方、测试和历史定位 | 只读 |
| Go slice worker | 冻结边界内的 runtime/API 实现 | 指定 production 文件 |
| Parity asset worker | oracle/scenario/独立 parity test 源资产 | 指定 asset 文件 |
| Parity reviewer | 集成 diff、Java/Go parity 和证据完整性 | 只读 |

正常使用 2 至 3 个 agent，4 个是上限而非目标。共享 `internal/esper`
状态机、顺序、时间或生命周期语义只能单写。只有契约冻结且文件零重叠时，
shared-core writer 与 parity-asset writer 才能并行。Codex 子 agent 共享当前
文件系统，因此文件边界比角色名称更重要。

每个子任务使用以下契约，不依赖父会话的隐含上下文：

```text
# Target
精确 capability、Java executions/runtime IDs 或 Go symbols。

# Context
必须读取的仓库路径、固定 Java source 和已冻结 observable contract。

# Allowed files
允许读取/修改的精确文件或目录；只读任务写“none”。

# Forbidden actions
不扩域；不运行 formatter/build/test；不改中央事实；不生成 evidence；
不 commit/push；不覆盖用户修改。

# Deliverable
结构化列出契约或 changed files、路径引用、假设、风险和主 agent 验证命令。
```

子 agent 返回后，主 agent 必须逐项核对文件边界和 diff。发现问题时优先把
精确失败回传给原 agent；如果运行环境不能续用原 agent，则由主 agent 接管，
并在 `PLANS.md` 记录上下文损失和复核范围。

## 4. 标准拓扑

1. 主 agent 从 roadmap/manifest 选工作单元 N，先更新 `PLANS.md`。
2. 并行启动只读 Java oracle scout 和 Go surface scout。
3. 主 agent 合并结果并冻结 observable contract、CoreFiles、AssetFiles、
   禁改文件和验证命令。未知项未关闭前不实现。
4. 主 agent 或一个 Go slice worker 单写共享核心。若资产契约已冻结，可同时
   使用一个文件不重叠的 parity asset worker。
5. 主 agent 审查 patch，生成 trace/evidence，按成本从低到高运行定向验证；
   把具体失败回传给原 worker 修复。
6. 定向验证通过后，主 agent 更新中央事实和 `PLANS.md`。
7. 并行运行 N 的只读 parity review 与 N+1 的只读 Java/Go 调查。N+1 在 N
   提交前不得写入。
8. 修复 N 的 findings，运行完整门禁，审查最终 diff，提交并推送 `master`。

没有安全独立子任务时由主 agent 串行完成，不为追求 agent 数量人为拆分工作。
审查与预取期间若用户修改了重叠文件，停止相关写入，重新核对所有权并与现有
修改协作。

## 5. 验证、提交与恢复

主 agent 的标准验证顺序：

```text
审查所有子 agent 修改和完整 git diff
仅 gofmt 当前工作单元改变的 Go 文件
受影响包和目标测试
目标 Java/Go scenario diff 与必要 mutation
go test ./internal/compat ./internal/app/manifest -count=1
make check
git diff --check
```

里程碑边界追加质量策略要求的 race、stress、Docker 与 benchmark。门禁和
独立 review 全部通过后才能提交并推送。Codex task 中断或压缩前，在
`PLANS.md` 写明：当前 Git 状态、已完成阶段、失败原文、未验证假设、精确
下一命令和禁止覆盖的用户修改。新的 task 只信当前文件与可重放证据。

Codex 的 agent/plan 工具只管理任务上下文，不是文件快照。恢复和回退始终
依赖 Git、worktree diff、生成物来源与验证记录。
