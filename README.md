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

截至 2026-08-26（Draft 4.257），manifest v2 记录：

| 维度 | 数值 |
| --- | --- |
| Java runtime inventory | 4,136 |
| 已关联 runtime | 3,059（74.0%） |
| Differential-verified runtime | 638 |
| Capability / case | 115 / 554 |
| Differential-verified case | 165 |

Runtime 关联率只表示已有处置记录，不是 Java/Go parity 通过率。只有带可重放 scenario、Java/Go trace 和零差异 evidence 的 runtime 才计入 differential-verified。

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
