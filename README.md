# Esper Go

Esper Go 是 Esper 9.0.0 的 Go 移植。规则使用可分析、类型安全的链式 Builder 构造，核心路径不接受 EPL 字符串。当前仍是增量迁移，不能宣称完整 Esper 对等或可直接替换 Java。

仓库采用公共根 facade、公开 `connectors`、私有 `internal/esper` 实现和 `testdata` 固定兼容资产的 Go Modules 布局。示例位于 `examples/stage1`，目录边界见 [项目结构](docs/project-layout.md)。

## 项目文档

- [执行路线图](docs/esper-go-port-roadmap.md)：当前阶段、优先级、remaining 和风险
- [迁移执行手册](docs/esper-go-port-runbook.md)：工作单元、验证、并行和提交方式
- [Codex 工作流](docs/esper-go-port-codex-workflows.md)：Codex ExecPlan、agent 分工和恢复方式
- [OMP 工作流](docs/esper-go-port-omp-workflows.md)：Oh My Pi agent 分工、批处理模板和上下文管理
- [质量策略](docs/esper-go-port-quality-strategy.md)：差分、合成数据、门禁和验收口径
- [实施规划](docs/esper-go-port-implementation-plan.md)：完整范围、架构和语义规范
- [CHANGELOG](CHANGELOG.md)：逐切片历史和验证记录

验证状态和统计的机器事实位于 `testdata/compat/capability-manifest.json`；Java runtime inventory 位于 `testdata/compat/java-execution-inventory.jsonl`。

## 当前进度

截至 2026-09-29（HEAD `1766ba2b2`），`testdata/compat/capability-manifest.json` 的已校验 `summary` 记录：

| 维度 | 数值 |
| --- | --- |
| Java runtime inventory | 4,136 |
| 已关联 runtime | 3,763（91.0%） |
| 未关联 runtime | 373 |
| Differential-verified runtime | 1,766（42.7%） |
| Capability（其中 differential-verified） | 126（50） |
| Case（implemented / differential-verified / intentionally-different） | 804（802 / 433 / 40） |
| Representative scenario 通过 | 122 / 122 |

三个口径必须区分：**关联**表示该 runtime 已有处置记录；**implemented** 表示 Go 侧已有对应实现；**differential-verified** 才表示已有可重放 scenario、Java/Go trace 和零差异 evidence（严格 parity 口径以 runtime 计，42.7% 而非 91.0%）。`nfr-verified` 目前为 0，性能与并发尚未纳入验收。

上表的 4,136 只覆盖 `regression-lib` 的 11 个 suite 域（client、context、epl、event、expr、infra、multithread、pattern、resultset、rowrecog、view）；Dataflow、EsperIO 和连接器矩阵不在该 inventory 内，因此这里的百分比不能外推到那些能力面。

数值随每个工作单元提交变化，需要时直接从 manifest 重算，不要手工维护第二份：

```sh
python3 -c "import json;print(json.load(open('testdata/compat/capability-manifest.json'))['summary'])"
```

## 本地验证

```text
./scripts/check-layout.sh
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
git diff --check
```

外部服务 Docker fixture、环境变量和精确命令见 [外部服务集成](docs/integration/external-services.md)。周期 stress 基线使用：

```sh
ESPER_STRESS=1 go test ./internal/esper \
  -run '^TestStressSyntheticMediumLoad$' -count=1 -timeout 5m
```
