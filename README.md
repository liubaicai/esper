# Esper Go

Esper 9.0.0 的 Go 移植正在按 [实施规划](docs/esper-go-port-implementation-plan.md) 进行。规则使用可分析的 Go 链式 Builder 构造，核心路径不接受 EPL 字符串。示例位于 `examples/stage1`。

仓库采用公共根 facade、公开 `connectors` 子包、私有 `internal/esper` 实现和 `testdata` 固定资产的 Go Modules 布局。根目录只保留包文档、生成 facade 和外部 API 测试；详细边界和开发入口见 [项目目录结构](docs/project-layout.md)。

## 移植概况

当前工作树已具备一组可运行的垂直切片：struct/map/JSON/XML/Avro-JSON/ObjectArray 基础事件、显式 typed `EventTypeAutoNameRegistry` 短名解析与别名 schema 注册、PREDEFINED/ANY Variant 成员路由、Missing/Null/Present、字段/变量/常量/比较/逻辑/算术/统计与集合聚合表达式、可分析枚举/集合表达式（元素/索引/size、where/select/arrayOf、any/all/count、first/last、distinct、take/while/reverse、min/max/order、sum/average、except/intersect/union、sequenceEqual、aggregate、groupBy、toMap、most/least frequent）、窗口历史 `Prev`/`Prior`/`Leaving`、Filter、length/time 及批量/表达式窗口/表达式批量/分组/交并/排序/唯一 View、Projection Row、new/old stream、内/外连接和 N-way inner/outer 基础 Join（含 Context 分区）、虚拟时钟、Plan Build/manifest、all-or-nothing typed `CompileBatch` 多错误聚合、catalog-independent typed `ValidateSyntax`、typed `CompilerProvider`/`NativeCompilerProvider` 编译边界、standalone typed `CompileExpression[T]`、Build-time statement-name/user-object resolver、Deploy/DeployWithParameters/DeployWithPositionalParameters/DeployPlans、multi-plan statement-local substitution-parameter resolver、typed private/protected/public Module、泛型 module dependency ordering、`module.Uses(...)`/`UsesNames(...)` visibility path、active deployment `Engine.RuntimePath()` immutable compiler snapshot、typed `CompilePathCache` catalog snapshot cache、ambiguity diagnostics、bus-visible public event type 与 deployment-local Table/Named Window/variable/context/event-type/expression namespace、Send/SendBusRecord/SendObjectArray/Route、可选 bounded inbound/outbound/route/timer worker pools 与 waitable async tasks、虚拟时间 runtime/statement metrics（分组、快照、动态周期、单语句/全局停启、Named Window 计数；Linux CPUTime 为进程 CPU 采样，非 Linux 为显式零，已登记为 approved intentional difference）、14 类 typed statement/dataflow audit、锁外可重入订阅与 Java-compatible formatter、普通流/RecordStream/Join/Aggregate/Pattern 链式 `InsertInto`/`RouteTo`（含输出批处理排序与有界循环失败、EPLInsertInto 20 个 Java execution 的 typed 对照：wildcard 链、Null 类型、部分列、rstream-only、跨表示路由与歧义类型 Send 错误）、`GroupByRollup`/`GroupByCube`/`GroupByGroupingSets` 与 `Grouping`/`GroupingID`、`FilterAggregate`/`CountIf`/`SumIf`/`AvgIf`/`MinIf`/`MaxIf`、`FirstEver`/`LastEver`/`CountEver`、`AdvanceTime`/`AdvanceTimeSpan`/`NextScheduledTime`、Listener/Subscriber/Sink、内存 Table、Named Window、`FromTable`/Context-aware FAF、FAF Named Window/Table Join、`FromNamedWindow`/`FromTable` 的 `OnDemand().Insert/UpdateWhere/DeleteWhere/DeleteAll`、类型化参数化 FAF（含同名参数类型一致性、执行时类型校验、Prepared Query、切片 `IN`、历史源快照和取消检查）、显式 `ExecuteFireAndForgetAndRoute*`/`RouteFireAndForget`（只读 FAF 与路由副作用分离，支持投影顺序和参数绑定）、基础 `SubqueryExists`/`SubqueryValue`/`SubqueryIn` 与 `OuterField`（Named Window/Table 快照、相关过滤、嵌套作用域、参数绑定、非聚合 having 逐行过滤、wildcard 事件负载、Prev/Prior 投影、多流 join 相关与数值 coercion、数组相关键、sort 窗口与 NestedField 片段导航）、on-trigger Table/Named Window mutation（含 `SetVariable`、`SelectFromTable`、`SelectFromTableWhere`、`InsertIntoNamedWindow`、`UpdateNamedWindow`、`SelectFromNamedWindow`、`DeleteFromNamedWindow`、`DeleteAllFromNamedWindow`、条件式 `MergeIntoTableWhen`/`MergeIntoNamedWindow`、Named Window matched/not-matched/delete、无 where matched 语义、`TableField`/`NamedWindowField` 条件批量更新/删除）、Fire-and-Forget/Prepared Query、`FromHistorical`/`HistoricalProvider` 外部拉取源（含 database/sql 参数绑定、LRU/expiry 缓存、列名大小写归一化、占位符/metadata hook、可选 PreparedStatement/事务 queryer 和 MySQL Docker 集成）、参数化 `SQLSink` 基础 DML/Upsert、嵌套 Context/partition selector、Pattern followed-by/every/every-distinct/and/or/not/match-until/until/While/显式 timer-schedule/状态上限、基础 Match Recognize（连续/交替/有限排列/可选/重复/reluctant/固定 interval/分区/skip/DEFINE/MEASURES/IntervalOrTerminated/measure alias OrderBy/Statement Snapshot/长度窗口淘汰/Named Window 删除重算）、output first/last/snapshot/every、`output after` 事件数/固定 duration/日历周期门控、变量/计数/时间戳条件式 when/then 与虚拟时钟 every、Dataflow Beacon/EventBus/EPStatementSource 生命周期、显式分支图、Undeploy 与 deployment 依赖前置条件（typed module-use 边与模块拥有的 Named Window/Table/变量/Context/事件类型/声明表达式/Script 资源引用）、deploy-time path dependency 前置条件（跨模块引用对 active provider deployment 解析，缺失时报 DeployPreconditionError）与注册期 duplicate 前置条件（DuplicateModuleObjectError 对齐 Java 消息文本；Java create-index 独立部署、inlined-class 工件与 preconfig 偏差为 approved difference）、consumed/provided 部署依赖枚举（same-module 跨 deployment 边、nested schema 事件类型依赖、edge 推导 Deployment.Dependencies）与 redefinition 部署/卸载生命周期（同名私有对象按 module 作用域共存）。

连接器切片：`connectors/csv`、`connectors/db`、`connectors/http`、`connectors/socket`、`connectors/kafka`、`connectors/amqp`、`connectors/jms` 已按统一 `OPENED/STARTED/PAUSED/DESTROYED` 生命周期落地 Engine bridge。

当前仍是增量移植，不能宣称完整 Esper 对等；各域剩余项见 [实施规划](docs/esper-go-port-implementation-plan.md) 与 `testdata/compat/capability-manifest.json`。逐切片迁移明细记录在 [CHANGELOG.md](CHANGELOG.md)。

## 大概进度

当前 capability 对账进度为：固定 Java commit 的 inventory 中有 4,136 个 `status=ok` runtime，manifest 记录 2,715 条关联，覆盖 2,630 个唯一 runtime（63.6%），尚有 1,506 个 runtime 未建立关联；410 个 case 中 406 个至少达到 `implemented` verification（其中 23 个 `differential-verified`、18 个 `intentionally-different`），4 个仍为 `inventoried`-only。另有 23 个代表性场景已通过 Java/Go 差分验证（`context-hash-segmented`、`filter-window-aggregate-output`、`join-length-window`、`output-policy-iterator`、`pattern-timer-interval`、`subquery-length-window`、`named-window-mutation`、`table-mutation`、`variable-deploy`、`context-output-termination`、`deployment-restart-window`、`high-cardinality-context`、`time-window-long-running`、`dataflow-connector-output`、`output-after-last`、`rollup-output-every`、`match-recognize-simple`、`unidirectional-aggregate-join`、`output-first-having`、`context-keyed-subquery`、`rowrecog-aggregation`、`resultset-grouped-time-window`、`resultset-row-per-group-simple`）。Docker MySQL/Kafka/RabbitMQ 门控已通过；stress 基线已优化至约 660 events/s，性能/NFR 尚未验证。这里的 63.6% 是"已建立 Java runtime 对账/处置证据"的进度，不是 Java/Go 行为 parity 通过率，也不代表 Esper 全量移植完成。

Java 基线的静态回归候选清单在 testdata/compat/static-manifest.json，运行态 execution 清单在 testdata/compat/java-execution-inventory.jsonl，非 Regression 的源资产盘点在 testdata/compat/source-test-manifest.json，首批 capability/case 映射在 testdata/compat/capability-manifest.json；这些清单都不是全量 Go 映射完成或 Java/Go 行为差分通过的证明。

## 运行测试

```text
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
./scripts/check-layout.sh
git diff --check
```

外部服务集成测试（可选）的 Docker fixture、ready 检查、环境变量和精确测试命令见 [外部服务集成](docs/integration/external-services.md)。例如 MySQL：

```sh
ESPER_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/test?parseTime=true&charset=utf8mb4' \
  go test ./connectors/db ./internal/esper -run 'MySQLDocker$' -count=1
```

2026-08-14 已在本地 Docker fixture 上通过 MySQL、Kafka 和 RabbitMQ 的全部门控 round-trip（命令与结果见上方链接）。

周期 stress 基线（环境门控，普通 `go test` 会显式 skip）：

```sh
ESPER_STRESS=1 go test ./internal/esper -run '^TestStressSyntheticMediumLoad$' -count=1 -timeout 5m
```

基线语义不变量通过；2026-08-14 优化 `historyByEvent` 按需构建后，12,200 事件从 42.6s 降至 18.45s（约 660 events/s），仍有性能/NFR 开放项。
