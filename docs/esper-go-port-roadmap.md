# Esper 9.0.0 Go 移植：执行路线图与遗漏检查

> 文档定位：本文件是 docs/esper-go-port-implementation-plan.md 的执行摘要与路线图。它把详细实施文档浓缩成可快速查阅的目标、现状、阶段、剩余工作和遗漏检查表，用于指导后续切片和验收。详细设计、API 形态和逐轮增量说明仍见实施规划文档。

## 1. 项目目标与完成定义

### 1.1 总目标

用 Go 重写 Esper 9.0.0 的复杂事件处理能力，并同时满足：

1. 语义完整：Esper 中与语言无关的事件处理语义全部具备 Go 实现。
2. 链式 API：规则通过可组合、可检查、可复用的 Go builder 构造，不以 EPL 字符串作为核心规则定义方式。
3. Java 为基准：Java Esper 9.0.0（commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c）是行为预言机。
4. Go 风格：小接口、显式 error、context.Context、泛型适度使用、清晰的所有权与并发约定。
5. 可追踪对账：每项功能都有 Java 回归用例到 Go 用例的映射；不能仅以代码覆盖率代替功能完整度。
6. 验收闭环：功能对等后完成并发、性能、内存、连接器、文档和示例验收。

### 1.2 完整移植的严格定义

完整移植指可观察行为和能力对等，不是 Java 类文件一对一翻译。发布完成必须同时满足：

- 功能清单中所有平台无关能力状态为 Passed（或 approved-difference 有书面理由）。
- Java 回归用例清单中所有适用项都映射到至少一个 Go 用例并通过。
- 所有不适用项都有书面理由、替代能力、测试证据和评审记录，不能直接标记跳过。
- 事件输出、移除流、顺序、时间推进、状态快照、异常类别和生命周期等关键行为通过 Java/Go 差分验证。
- Go 代码质量、覆盖率、竞态、模糊测试、性能和文档门禁全部通过。

### 1.3 明确边界

根据“使用链式 API，而不是 EPL 风格”的要求，首个完整版本采用以下边界：

- EPL 文本解析器、EPL 文本模块格式、EPL 与 SODA 字符串兼容接口不作为 Go 公共 API。
- EPL 所表达的查询、窗口、模式、上下文、表、数据流等语义不能缺失；必须由链式 API 和底层逻辑计划完整表达。
- Java 回归测试中的 EPL 仍用于驱动 Java 预言机；对应的 Go 测试使用链式规则构造同一语义。
- 如果未来需要接收历史 EPL，可单独增加 compatibility/epl 包；它是兼容层，不得侵入核心运行时，也不作为当前完成条件。
- Java 字节码、类加载、反射和 Java 序列化格式不兼容；以 Go 的计划、注册表和版本化序列化机制替代。
- “Flink DataStream 风格”仅指规则构造体验：类型化流、具名算子、链式组合和显式 sink。不表示首版要复制 Flink 的分布式集群运行时、并行度、watermark/checkpoint/savepoint、作业恢复或 exactly-once 语义；除非 Esper 9.0.0 本身有对应可观察契约。
- 范围严格限定为固定 commit 中已检入的开源 Esper 9.0.0 模块、公共契约、回归场景、单元测试、EsperIO 和示例。NEsper、EsperHA 及商业/企业能力不在本次完成条件内。

## 2. 当前状态（截至 2026-08-08）

### 2.1 代码与分支

- 分支：codex/faf-index
- 工作树：3 个未提交文件
  - README.md：覆盖率数字更新。
  - testdata/compat/capability-manifest.json：新增 fromclausemethod 基础 parity 映射。
  - fromclausemethod_parity_test.go：新增 TestFromClauseMethodOneStreamTwoHistJoinedKeepallParity。
- 最新已提交：68bd238fb docs(manifest): register fromclausemethod basic parity case and update coverage (1397/4136 = 33.78%)

### 2.2 对账清单

| 维度 | 数值 |
| --- | --- |
| Capability | 36 个 |
| Case | 192 个 |
| Case mapped | 185 个 |
| Case partial | 3 个 |
| Case approved-difference | 4 个 |
| Java inventory runtime | 4,136 个 |
| 已建立 runtime 关联 | 1,396 条（提交前）/ 1,408 条（提交未提交变更后） |
| 唯一已覆盖 runtime | 1,370 个（提交前）/ 1,398 个（提交未提交变更后） |
| 覆盖率 | 33.13%（提交前）/ 33.80%（提交未提交变更后） |

> 覆盖率 = 唯一已覆盖 Java runtime / 全部可执行 Java runtime。它表示“已建立 Java/Go 对账证据”的进度，不是“Go 已通过 parity”的比例，也不代表 Esper 全量移植完成。

### 2.3 已通过的垂直切片（已映射 capability 示例）

- 基础事件模型：struct/map/JSON/XML/ObjectArray/Avro-JSON 事件、PREDEFINED/ANY Variant 路由、Missing/Null/Present 语义。
- 表达式：字段/变量/常量/比较/逻辑/算术/统计与集合聚合、枚举/集合方法、声明式表达式。
- 窗口与历史：Prev/Prior/Leaving、length/time/batch/表达式/分组/交并/排序/唯一 View。
- 流处理：Projection Row、new/old stream、内/外连接、N-way Join、虚拟时钟、Plan Build、Deploy/DeployWithParameters、Send/Route/InsertInto。
- 聚合：基础聚合、FilterAggregate、FirstEver/LastEver/CountEver、GroupByRollup/Cube/GroupingSets、LocalGroupBy、sorted access、自定义聚合函数。
- 基础设施：内存 Table、Named Window、Context partition、FAF（Fire-and-Forget）、子查询、索引（hash/B-tree/shared-index）、on-trigger Table/Named Window mutation、merge/update/delete。
- 历史/方法源：FromHistorical/HistoricalProvider、SQL 拉取源、LRU/expiry 缓存、方法源 FromMethodOn/JoinMany、多方法源依赖。
- 模式：Pattern followed-by/every/every-distinct/and/or/not/match-until/until/While、定时器、cron。
- Match Recognize：基础实现（连续/交替/有限排列/可选/重复/reluctant/固定 interval/分区/skip/DEFINE/MEASURES）。
- 输出：output first/last/snapshot/every、output after、when/then、基础 cron。
- Dataflow：Beacon/EventBus/EPStatementSource、显式分支图、生命周期。
- EsperIO：CSV、DB（database/sql DML/Upsert）、HTTP、Socket、Kafka、AMQP、JMS 连接器。
- 示例：examples/stage1。

## 3. 实施策略

### 3.1 切片推进

每个 Java execution class 作为一个切片。对每一切片：

1. 阅读 Java 测试和对应 runtime，确认行为预言。
2. 用 Go 链式 API 构造等价规则，编写 _parity_test.go 对照测试。
3. 将 Java runtime ID 登记到 testdata/compat/capability-manifest.json 的对应 case。
4. 运行门禁：go vet ./...、go test ./...、go test -race ./...、go test ./internal/compat/...。
5. 提交并推送 origin/codex/faf-index；更新 README 覆盖率。
6. 不宣称“全量完成”，只更新覆盖率与 capability 状态。

### 3.2 测试对账原则

- 行为预言机：Java 测试输出/事件序列/异常类别作为期望。
- 丢弃性能阈值：时间、性能、JVM query-plan hook 不纳入 parity。
- 保留关键行为：输出事件、移除流顺序、时间推进、状态快照、异常类别、生命周期。
- 链式 API 唯一：Go 测试不拼接 EPL 字符串；用 From/Join/JoinMany/FromMethodOn/Select/Where/GroupBy/Having/OrderBy/Output/InsertInto/RouteTo 等 builder 表达规则。
- MySQL 按需：SQL/DB 相关测试需要本地 Docker MySQL；核心/窗口/表达式/Context 测试不依赖 MySQL。

### 3.3 代码与清单管理

- 所有 Java runtime 清单在 testdata/compat/java-execution-inventory.jsonl。
- 静态候选清单在 testdata/compat/static-manifest.json（目前几乎为空）。
- 非 Regression 源资产在 testdata/compat/source-test-manifest.json（目前几乎为空）。
- Capability/case 映射在 testdata/compat/capability-manifest.json。
- README.md 维护覆盖率数字与本轮增量说明。

## 4. 阶段与里程碑

### 4.1 Phase 0 — 基础运行时（已完成）

事件模型、schema、表达式核心、窗口、insert into、路由、简单 join、Table/Named Window 基础、FAF 基础、方法源基础。已建立 33%+ runtime 关联证据。

### 4.2 Phase 1 — 核心能力闭合（进行中）

目标：把现有 partial 与未覆盖的“核心查询能力”补齐，使覆盖率接近 60%。

重点领域：

- fromclausemethod 剩余 50 runtime / 8 个 class。
- subselect 剩余 142 runtime / 18 个 class。
- join 与 outer join 复杂链、unidirectional、Context Join。
- resultset 聚合高级特性（filtered、math-context、访问聚合、rollup 组合）。
- context 分区 selector、嵌套、生命周期、事务边界。
- infra 表/命名窗口剩余 mutation/merge/transaction 场景。
- expression 剩余类型、函数、脚本、枚举集合高级组合。
- client 域编译器/路径/异常/大用例（244 runtime）。
- multithread 并发测试（56 runtime）。

### 4.3 Phase 2 — 高级模式与连接器

目标：模式、Match Recognize、Dataflow、EsperIO 剩余连接器、完整 Context 语义。

重点领域：

- Pattern guard/observer、复杂 NFA、consumption、timer-schedule 高级语义。
- Match Recognize prev/interval/after/聚合/窗口删除/复杂 NFA。
- Dataflow 类型化多端口、信号、背压、完整连接器矩阵。
- EsperIO 高级 Kafka group/rebalance、AMQP 重连/序列化、JMS provider/session/transaction、完整 Dataflow 集成。
- 完整事件格式与 Serde（Avro binary、union、logical type、schema evolution、XML namespace/XSD）。
- 完整 Context（嵌套、initiated、terminated、keyed segmented、hash/category、生命周期）。

### 4.4 Phase 3 — 收尾与验收

目标：100% 适用 Java runtime 映射并通过；所有门禁通过；文档、示例、性能、内存验收。

- 处理 approved-difference 与明确不适用项。
- 补充 static-manifest.json 和 source-test-manifest.json。
- 完整 Java/Go 行为差分审计。
- 竞态、模糊、内存、性能基准。
- 用户文档、API 参考、examples/ 扩展。
- 合并到主分支并发布。
## 5. 剩余工作优先级

### 5.1 P0 — 立即完成

1. 提交并推送当前未提交变更（fromclausemethod 新增 parity）。
2. 关闭 fromclausemethod 剩余 50 runtime：
   - EPLFromClauseMethod（15）
   - EPLFromClauseMethodNStream（11）
   - EPLFromClauseMethodOuterNStream（7）
   - EPLFromClauseMethodVariable（6）
   - EPLFromClauseMethodMultikeyWArray（5）
   - EPLFromClauseMethodJoinPerformance（4）
   - EPLFromClauseMethodCacheExpiry（1）
   - EPLFromClauseMethodCacheLRU（1）

### 5.2 P1 — 下一批高价值切片

按未覆盖 runtime 数量排序：

| 域 | 未覆盖 runtime | 关键子域/类 |
| --- | --- | --- |
| epl | 701 | subselect 142、insertinto 105、spatial 60、variable 53、fromclausemethod 50、database 37、dataflow 23 |
| infra | 419 | nwtable 148、namedwindow 140、tbl 131 |
| expr | 364 | 待按子包细分 |
| resultset | 313 | 聚合、输出、排序、分组 |
| client | 244 | 编译器、路径、异常、大用例 |
| context | 184 | 分区 selector、嵌套、生命周期 |
| view | 176 | 视图高级组合 |
| event | 170 | 事件表示/Serde 完整矩阵 |
| pattern | 106 | 复杂模式 |
| multithread | 56 | 并发回归 |
| rowrecog | 34 | Match Recognize |

### 5.3 P2 — 清单与能力拆分

- 将 epl/expr/resultset 等粗粒度 capability 拆分为更细 case，便于追踪。
- 填充 static-manifest.json 与 source-test-manifest.json。
- 对 partial/approved-difference case 写出书面差异理由。

### 5.4 P3 — 验收与工程化

- 完整 go test -race 与并发测试。
- 模糊测试/属性测试补充。
- 性能基准与内存剖面。
- 文档、示例、API 稳定。

## 6. 遗漏检查表

### 6.1 未覆盖 Java runtime 域分布

按 java-execution-inventory.jsonl 中 sourceFile 的顶层 suite/<domain> 分组：

| 域 | 未覆盖 runtime | 说明 |
| --- | --- | --- |
| epl | 701 | 最大缺口，集中在 subselect/insertinto/spatial/variable/fromclausemethod/database |
| infra | 419 | 表/命名窗口高级 mutation/merge/transaction |
| expr | 364 | 表达式函数、类型、脚本、枚举集合 |
| resultset | 313 | 聚合、输出、排序、分组 |
| client | 244 | 编译器/客户端契约 |
| context | 184 | Context 分区、嵌套、生命周期 |
| view | 176 | 视图高级组合 |
| event | 170 | 事件表示/Serde 完整矩阵 |
| pattern | 106 | 复杂模式 |
| multithread | 56 | 并发回归 |
| rowrecog | 34 | Match Recognize |

### 6.2 清单与追踪遗漏

- static-manifest.json 目前几乎为空，需要把静态/编译期候选登记进去。
- source-test-manifest.json 目前几乎为空，需要把非 Regression 源资产（单元测试、集成测试）登记进去。
- epl/expr/resultset 等 capability 拆分过粗，需要继续细分为可验收的 case。
- approved-difference 的 4 个 case 需要书面差异理由和测试证据。
- partial 的 3 个 case 需要明确剩余项关闭计划。
- 当前未覆盖的 2,767 个 runtime 中，需要识别哪些属于“平台/语言无关核心语义”，哪些属于“JVM 特有机制”或“性能阈值”，分别标记为 mapped/approved-difference/out-of-scope。

### 6.3 能力与边界遗漏

- 事务/并发：当前 FAF mutation 已有单目标 rollback，但跨 statement、跨目标 routed side effect、listener/external resource 完整事务仍待实现。
- Context Table ownership：live insert 的首次 ownership 注册已部分实现，但更广 Context Table live mutation 组合、aggregate-into-table、跨 context row ownership 仍待补充。
- 索引高级语义：FullOuter、右保留或非相邻链式 outer、unidirectional、OR、UDF、Null/Missing、复杂动态表达式、超大多流 probe 安全回退仍待补充。
- EsperIO 完整矩阵：Kafka 高级 group/rebalance/plugin、AMQP Java serialization/高级重连、JVM JMS provider/session/transaction、完整 Dataflow connector 集成仍计划中。
- Avro/JSON/Serde 完整矩阵：Avro binary codec、union/logical/fixed/enum、schema evolution、XML namespace/XSD、完整 dynamic strict-lax 仍 partial。
- Pattern/Match Recognize 高级语义：Pattern guard/observer、复杂 NFA、consumption、Match Recognize prev/interval/after/聚合/窗口删除仍待补充。
- Client 域：编译器路径、异常、大用例、模块可见性、class-loader 行为仍基本未映射。
- Multithread：并发回归 56 runtime 尚未开始映射。
- Performance/timing：不追求 parity，但需要建立 Go 侧性能基准，避免回归。

## 7. 基础设施与依赖

### 7.1 已具备

- Java 17（C:\\Program Files\\Microsoft\\jdk-17.0.20.8-hotspot）
- Maven 3.9.16（C:\\Users\\baicai\\AppData\\Local\\UniGetUI\\Chocolatey\\lib\\maven\\apache-maven-3.9.16）
- Go 1.25.5（本机）
- Docker 环境（可选，用于 MySQL 集成测试）
- Java Esper 9.0.0 源码：D:\\Code\\soc\\esper
- Go 项目：D:\\Code\\soc\\bigsoc-esper

### 7.2 按需使用

- MySQL 8.0：通过 Docker 启动，用于 SQL 历史源、SQL FAF、esperio-db 等测试。
- Maven：运行 Java 回归测试以生成预言输出，例如：
      mvn -pl regression-run '-Dtest=TestSuiteInfraNWTable' '-DfailIfNoTests=false' '-Dgpg.skip=true' test
- Docker 启动示例：
      docker run -d --name esper-java-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test mysql:8.0
  然后设置环境变量：
      ESPER_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/test?parseTime=true&charset=utf8mb4'

## 8. 门禁与质量标准

每完成一个切片必须执行并全部通过：

    go vet ./...
    go test ./... -count=1 -timeout 180s
    go test -race ./...
    go test ./internal/compat/... -count=1

质量标准：

1. 每个 Go parity 测试必须对应至少一个 Java runtime ID。
2. 新增代码必须有 go test 覆盖；核心路径不接受 EPL 字符串。
3. 所有 race 测试通过；并发路径使用显式锁/原子操作，避免 data race。
4. 异常类别与 Java 一致（build-time error vs runtime error）。
5. 不引入未使用的 capability 或空壳 case；manifest 必须与代码同步更新。
6. 提交信息清晰：feat(<domain>): port <java-class> <N> runtimes. Coverage X/Y (Z%)。
