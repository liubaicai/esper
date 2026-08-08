 # Esper 9.0.0 Go 全量移植总体规划与实施路线图
 
 > 版本：Draft 1.0（2026-08-08）
 > 目标：在不写代码的前提下，先建立一份结构化、可度量的总体规划，指导后续 Go 链式 API 移植工作。
 > 约束：本规划不宣称 Esper 全量移植完成；所有进度数字均为“已建立 Java/Go 对账证据”的度量，不等同于行为 parity 通过率。
 
 ---
 
 ## 1. 项目目标与范围
 
 ### 1.1 总体目标
 
 将 Esper 9.0.0（Java，tag "release_9.0.0", commit "9e1b9f1cc9117fea4bf33ab043762c045d73839c"）完整移植到 Go，使用**可分析的链式 Builder API**构造规则，核心路径不接受 EPL 字符串输入。
 
 - **API 风格**：参照 Apache Flink DataStream 的链式风格，而非 Esper EPL 字符串风格。
 - **实现方式**：对照 Java 行为作为 oracle，用 Go 风格与架构重新实现，不逐类机械翻译 JVM 字节码。
 - **完整度目标**：覆盖 Java regression 运行态清单中全部可执行 runtime，并为每个 runtime 建立 Go 链式 API 对照测试证据。
 - **质量目标**：所有 Go 代码通过 "go vet"、单元测试、compat 清单校验和 race 检测；MySQL 等外部依赖通过 Docker 复现。
 
 ### 1.2 范围边界
 
 | 在范围内 | 不在范围内或明确差异 |
 |---|---|
 | Esper 核心事件处理、窗口、聚合、Join、子查询、Pattern、Match Recognize、Context、Table、Named Window、Dataflow、EsperIO 连接器、方法源、历史 SQL 源 | 逐字节复制 JVM 行为、EPL 字符串解析器、XML 配置、JNDI、JVM 特定的 query-plan hook、注解系统 |
 | 对照 Java 行为编写 Go 测试 | 不追求与 Java 完全一致的性能阈值、日志格式、异常文本 |
 | 可分析的 Go 链式 Builder API | 不支持 EPL 字符串作为规则定义方式（仅作为测试 oracle 或读取参考） |
 
 ---
 
 ## 2. 现状评估（基线）
 
 ### 2.1 项目资产现状
 
 | 资产 | 路径 | 现状 |
 |---|---|---|
 | Java 基线 | D:/Code/soc/esper | 完整 Esper 9.0.0 源码，已装 Java 17，Maven 可安装 |
 | Go 项目 | D:/Code/soc/bigsoc-esper | 分支 codex/faf-index，工作树干净 |
 | Java 运行态清单 | compat/java-execution-inventory.jsonl | 4,136 个 status=ok 的可执行 runtime |
 | Capability 清单 | compat/capability-manifest.json | 35 个顶层 capabilities，1 mapped / 33 partial / 1 planned |
 | 静态清单 | compat/static-manifest.json | 0 entries（待填充） |
 | 源测试映射 | compat/source-test-manifest.json | 0 entries（待填充） |
 | 覆盖率记录 | coverage-current | 当前 Go 覆盖率数据（行覆盖） |
 
 ### 2.2 Java 代码规模
 
 | Java 模块 | 文件数 | 说明 |
 |---|---|---|
 | common/internal | 5,479 | 核心事件、编译、运行时、类型、视图、计划、索引、状态管理 |
 | compiler/internal | 127 | 编译器解析、代码生成、工具 |
 | runtime/internal | 380 | 部署、过滤器、调度器、内核、subscriber、timer |
 | regression-lib/suite | 826 | 对照测试套件（epl/expr/infra/resultset/event/...） |
 | esperio-* | 多个 | CSV、DB、HTTP、Socket、Kafka、AMQP、JMS 连接器 |
 
 ### 2.3 Go 代码现状
 
 | 指标 | 数值 |
 |---|---|
 | 根目录 .go 文件 | 324 个（含测试） |
 | 测试文件 (_test.go) | 274 个 |
 | 非测试源文件 | 86 个 |
 | 测试函数（近似） | 1,238 个 |
 | 连接器子模块 | 7 个（csv, db, http, socket, kafka, amqp, jms） |
 | 兼容工具 | cmd/manifest, cmd/parity, cmd/source-manifest |
 
 ### 2.4 当前覆盖度（硬事实）
 
 | 度量 | 数值 | 说明 |
 |---|---|---|
 | Java 可执行 runtime | 4,136 | 来自 java-execution-inventory.jsonl，status=ok |
 | 已建立对账/处置证据 | 1,391 | 约 33.63% |
 | 顶层 capability | 35 | 1 mapped / 33 partial / 1 planned |
 | 已登记 runtime 关联 | 1,401 | 覆盖 1,391 个唯一 runtime |
 
 > **重要**：33.63% 是“已建立对账/处置证据”的进度，不是 Java/Go 行为 parity 通过率，也不能代表 Esper 全量移植完成。
 
 ### 2.5 各 Java 域 runtime 分布
 
 | 域 | runtime 数 | 当前状态 |
 |---|---|---|
 | epl | 1,018 | 部分实现（database、dataflow、fromclausemethod、join、subselect、contained、insertinto、variable、script、spatial） |
 | expr | 657 | 部分实现（core、filter、enummethod、datetime、define、clazz） |
 | infra | 590 | 部分实现（nwtable、namedwindow、tbl） |
 | resultset | 576 | 部分实现（outputlimit、querytype、aggregate、orderby） |
 | event | 294 | 部分实现（object-array、json、xml、inheritance、variant、contained） |
 | client | 281 | 尚未系统登记 |
 | view | 227 | 部分实现（window-core、derived-statistical） |
 | context | 224 | 部分实现（分区、FAF、子查询索引共享） |
 | pattern | 145 | 部分实现 |
 | rowrecog | 68 | 部分实现 |
 | multithread | 56 | 尚未系统登记 |
 
 ### 2.6 Capability 清单现状
 
 | Domain | capabilities | 状态 |
 |---|---|---|
 | event | 8 | 1 mapped, 7 partial |
 | expression | 4 | partial |
 | resultset.aggregate | 5 | partial |
 | resultset.output | 3 | partial |
 | query | 3 | partial |
 | view | 2 | partial |
 | pattern | 2 | partial |
 | context | 1 | partial |
 | dataflow | 1 | partial |
 | esperio | 1 | partial |
 | historical | 1 | partial |
 | infra | 1 | partial |
 | join | 1 | partial |
 | rowrecog | 1 | partial |
 | event.serde | 1 | planned |
 
 > 注：client 和 multithread 域尚未建立顶层 capability 条目，属于明显遗漏。
 
 ---
 
 ## 3. 移植策略
 
 ### 3.1 API 风格：链式 Builder，拒绝 EPL 字符串
 
 所有规则必须通过 Go 链式 API 构造：
 
 ```go
 ep.From("SupportBean", SchemaFor[SupportBean]()).
   Where("IntPrimitive").Gt(10).
   Select("TheString", "IntPrimitive").
   IntoTable("MyTable", "TheString")
 ```
 
 对应 Java EPL：
 
 ```sql
 insert into MyTable
 select TheString, IntPrimitive from SupportBean where IntPrimitive > 10
 ```
 
 EPL 仅作为理解 Java 行为的参考和测试 oracle，不作为 Go 规则的输入。
 
 ### 3.2 架构映射：对照 Java 行为，Go 风格实现
 
 | Java 模块 | Go 对应方向 | 策略 |
 |---|---|---|
 | common/internal/event | schema.go, event_render.go, json_*.go | 事件类型系统、schema、JSON/Avro/XML/ObjectArray 表示 |
 | common/internal/epl | runtime.go, plan.go, stream.go, trigger.go | 语句计划、运行时、流、触发器、索引 |
 | common/internal/view | view_*.go, aggregate_*.go, group_window_*.go | 窗口保留、派生统计、聚合 |
 | common/internal/compile | faf.go, subquery.go, index_*.go | FAF、子查询、索引规划 |
 | runtime/internal | runtime.go, state.go, schedule.go | 部署、运行时状态、调度 |
 | regression-lib/suite | *_parity_test.go, *_test.go | 对照测试 |
 | esperio-* | connectors/* | 连接器 |
 
 ### 3.3 实现优先级
 
 1. **核心事件与表达式**（Phase 1）：schema、事件类型、表达式、函数、枚举、时间。
 2. **流、窗口与聚合**（Phase 1-2）：length/time/batch/keep-all 窗口、聚合、group-by、输出策略。
 3. **Join、子查询与 Pattern**（Phase 2-3）：inner/outer join、subquery、pattern NFA、match recognize。
 4. **Context、Table、Named Window**（Phase 3）：context 分区、Table 与 Named Window 生命周期、索引共享。
 5. **Dataflow、EsperIO、Client、Multithread**（Phase 4-5）：数据流、连接器、客户端 API、多线程/并发测试。
 
 ### 3.4 代码组织原则
 
 - 根目录保持扁平化：核心类型和测试放在根目录，便于对照阅读。
 - 连接器独立子模块：connectors/csv, db, http, socket, kafka, amqp, jms。
 - 兼容工具独立包：compat/。
 - 不复制 Java 包结构；按 Go 风格组合。
 
 ---
 
 ## 4. 分阶段实施路线图
 
 ### Phase 1：核心事件与表达式（基线能力）
 
 **目标**：建立事件类型系统、核心表达式和窗口聚合基础，覆盖 Java event/expr/view 域的基础 runtime。
 
 **主要工作**：
 - 完成 schema 注册、ObjectArray、JSON、XML、Avro 表示。
 - 完成表达式系统：算术、比较、逻辑、类型转换、case、枚举、时间、过滤。
 - 完成核心窗口：length、time、batch、keep-all、group-by、unique、rank。
 - 完成基础聚合：count/sum/avg/min/max、first/last/nth、window 聚合。
 - 建立 capability 清单的 event/expr/view 域条目。
 
 **退出标准**：
 - event/expr/view 域 capabilities 全部达到 mapped 或 approved-difference。
 - 对应 Java runtime 覆盖率达到 60% 以上。
 - go vet / go test ./... / go test ./compat/... 全部通过。
 
 ### Phase 2：Join、子查询、Pattern、Match Recognize
 
 **目标**：建立多流 Join、子查询、Pattern 和 Match Recognize 能力。
 
 **主要工作**：
 - 完成 inner/outer join（2-stream 到 N-stream）、unidirectional、cartesian、where。
 - 完成子查询：scalar、multi-row、correlated、index sharing、FAF subquery。
 - 完成 Pattern：followed-by、guard、observer、timer、consumption、subquery。
 - 完成 Match Recognize：NFA、partition、measure、define、prev、interval、after。
 - 建立 join/pattern/rowrecog 域 capabilities。
 
 **退出标准**：
 - join/pattern/rowrecog 域 capabilities 全部达到 mapped 或 approved-difference。
 - 对应 Java runtime 覆盖率达到 50% 以上。
 - go test -race . 通过（必要时分子集）。
 
 ### Phase 3：Context、Table、Named Window、高级索引
 
 **目标**：建立 Context 分区、Table、Named Window 生命周期和索引共享。
 
 **主要工作**：
 - 完成 Context：key-segmented、hash-segmented、initiated/terminated、nested、FAF。
 - 完成 Table：primary key、secondary index、insert/update/delete/upsert、into table。
 - 完成 Named Window：创建、索引共享、消费者、merge、FAF、live listener。
 - 完成索引：hash、B-tree、range、equality-prefix、multikey、array key。
 - 建立 context/infra/query 域 capabilities。
 
 **退出标准**：
 - context/infra/query 域 capabilities 全部达到 mapped 或 approved-difference。
 - 对应 Java runtime 覆盖率达到 60% 以上。
 - MySQL Docker 回归测试可复现运行。
 
 ### Phase 4：Dataflow、EsperIO、Client API
 
 **目标**：建立 Dataflow 图、EsperIO 连接器和客户端 API。
 
 **主要工作**：
 - 完成 Dataflow：source/sink/operator、typed ports、feedback、join、beacon、filter、log。
 - 完成 EsperIO：CSV、DB、HTTP、Socket、Kafka、AMQP、JMS source/sink 生命周期。
 - 完成 Client API：deploy、undeploy、send event、iterate、statement management、listener。
 - 建立 dataflow/esperio/client 域 capabilities。
 
 **退出标准**：
 - dataflow/esperio/client 域 capabilities 全部达到 mapped 或 approved-difference。
 - 对应 Java runtime 覆盖率达到 50% 以上。
 - 外部依赖（MySQL、Kafka、RabbitMQ 等）Docker 环境可复现。
 
 ### Phase 5：Multithread、性能、并发、收尾
 
 **目标**：建立多线程/并发测试、性能基准、最终收尾和完整 parity 审计。
 
 **主要工作**：
 - 完成 Multithread 域：并发发送、并发查询、并发部署、并发 listener。
 - 完成性能基准：关键路径性能测试，与 Java 合理对比（不要求阈值一致）。
 - 完成完整 runtime 对账：4,136 个 runtime 全部标记为 mapped/approved-difference/deferred。
 - 完成静态清单和源测试映射填充。
 - 完成最终文档、审计和发布准备。
 
 **退出标准**：
 - 4,136 个 Java runtime 全部处置。
 - 所有 go test 门禁通过，包括 race 检测。
 - 不宣称全量完成，但建立完整可审计证据链。
 
 ---
 
 ## 5. 测试与质量保证策略
 
 ### 5.1 三层测试金字塔
 
 | 层级 | 形式 | 说明 |
 |---|---|---|
 | 单元/组件测试 | *_test.go | 针对单个 Go 类型或函数，不依赖 Java |
 | 对照测试 | *_parity_test.go | 对照 Java regression 具体 execution，用 Go 链式 API 复现 |
 | 兼容清单测试 | compat/*_test.go | 验证 manifest、inventory、runtime 关联一致 |
 
 ### 5.2 Capability 清单驱动
 
 每个 capability 条目包含：
 - id：唯一标识
 - domain：所属域
 - level：G（目标）/S（切片）/C（组件）
 - status：mapped / partial / planned / approved-difference / deferred
 - javaRefs：对应 Java 类/execution
 - goRefs：对应 Go 文件/测试
 - remaining：未完成项
 
 新增 capability 或更新状态时，必须同步修改 capability-manifest.json 并跑 go test ./compat/...。
 
 ### 5.3 Runtime 对账流程
 
 1. 运行 Java regression 生成 java-execution-inventory.jsonl。
 2. 选择目标 Java execution，记录其 runtime ID 和名称。
 3. 在 Go 中编写 *_parity_test.go，用链式 API 复现行为。
 4. 在 capability 条目中登记 javaRuntimeIds 和 goTests。
 5. 运行 go test ./compat/... 验证关联。
 6. 更新 README 和 coverage-current 中的覆盖率数字。
 
 ### 5.4 质量门禁（固定）
 
 每次提交前必须：
 
 ```bash
 go vet .
 go test ./... -count=1 -timeout 180s
 go test ./compat/... -count=1
 go test -race .          # 大 fixture 可分子集或增大超时
 ```
 
 - 不能引入新的编译错误。
 - 不能破坏已有对照测试。
 - 新增 capability 必须清单化。
 
 ### 5.5 外部依赖与 Docker
 
 | 依赖 | 使用场景 | 管理方式 |
 |---|---|---|
 | MySQL | 历史 SQL 源、sink、FAF | Docker 启动本地实例 |
 | Kafka | esperio-kafka | Docker KRaft 3.8.1 |
 | RabbitMQ | esperio-amqp | Docker 启动 |
 | Maven | 运行 Java regression | 本地安装 |
 
 ---
 
 ## 6. 风险与依赖
 
 | 风险 | 影响 | 缓解措施 |
 |---|---|---|
 | Java 部分行为依赖 JVM 特性（注解、反射、类加载、字节码生成） | 高 | 用 Go 等价抽象（接口、函数、schema）替代，不复制 JVM 机制 |
 | 4,136 个 runtime 规模巨大，全面覆盖周期长 | 高 | 按 capability 分阶段推进，每阶段有独立退出标准 |
 | 性能阈值与 Java 不同，导致部分 Java 测试无法直接对照 | 中 | 丢弃时间阈值断言，仅保留功能正确性，标记为 approved-difference |
 | 外部依赖（Kafka/RabbitMQ/MySQL）环境搭建不稳定 | 中 | 统一 Docker 配置，记录环境版本，用 fake driver 覆盖基础路径 |
 | 链式 API 表达能力不足，无法覆盖所有 EPL 语义 | 中 | 必要时扩展 Builder 方法，但保持可分析、非字符串风格 |
 | 多线程/并发测试在 Go 与 Java 语义差异大 | 高 | 保留到 Phase 5，独立评估并发模型和内存序 |
 | 现有文档结构混乱，历史信息过度堆积 | 中 | 本规划作为 master plan，原有文档作为实施日志保留 |
 
 ---
 
 ## 7. 已知遗漏与补充项
 
 ### 7.1 清单/元数据遗漏
 
 | 遗漏 | 说明 | 优先级 |
 |---|---|---|
 | static-manifest.json 0 entries | 未填充静态回归候选清单 | 高 |
 | source-test-manifest.json 0 entries | 未填充非 Regression 源资产映射 | 高 |
 | client 域未建立 capability | 281 个 runtime 未系统登记 | 高 |
 | multithread 域未建立 capability | 56 个 runtime 未系统登记 | 高 |
 | epl 域未按子域拆分 capability | database/dataflow/fromclausemethod/join/... 混在一起 | 中 |
 | expr 域未按子域拆分 capability | core/filter/enummethod/... 混在一起 | 中 |
 
 ### 7.2 功能遗漏（从现有 capability 汇总）
 
 | 域 | 未覆盖项示例 |
 |---|---|
 | event | late schema、完整 fragment/metadata、XML XSD 验证、Avro 完整 trace |
 | expression | 完整 invalid/type 校验、BigDecimal/任意集合类型、Java Optional 精确 generic |
 | view | 完整 view family、rank/sort 导航、union/intersection |
 | pattern | guard/observer timer-schedule/consumption 完整矩阵 |
 | rowrecog | 复杂 NFA、prev、interval、after、聚合、窗口删除语义 |
 | context | selector、嵌套、完整生命周期 |
 | infra | 跨 statement transaction、完整 routed/listener rollback |
 | query | FAF/Join 混合 outer join、Context 约束 |
 | join | 完整多流/子查询 Join、复杂 join 顺序 |
 | resultset.aggregate | ROLLUP/CUBE/GROUPING SETS 全矩阵、filtered math-context |
 | resultset.output | cron 微秒精度、when/then 完整上下文 |
 | esperio | DB XML/config、connection factory、完整 Java trace |
 | dataflow | 类型化多端口/信号/背压、完整 trace parity |
 | client | 完整 deploy/undeploy/listener/iterate API |
 | multithread | 并发发送/查询/部署/ listener |
 
 ### 7.3 数据库域剩余项
 
 - EPLDatabaseJoinInsertInto：pattern + SQL + insert into + time batch + aggregation 组合，需确认 pattern timer + SQL 历史流组合是否支持。
 
 ### 7.4 方法源域（fromclausemethod）
 
 - 56 个 runtime 待覆盖：array/object return、overloaded、dependent/independent join、cache LRU/expiry、multikey、variable、N-stream outer。
 - 已有基础设施：FromMethod/FromMethodOn、MethodProvider/MethodProviderFunc。
 
 ### 7.5 文档与过程遗漏
 
 - 现有 docs/esper-go-port-implementation-plan.md 历史信息过度堆积，结构可读性差，需用本 master plan 作为顶层导航。
 - 缺少每阶段退出标准的明确验收列表。
 - 缺少“approved-difference”的判定标准文档。
 - 缺少 MySQL/Kafka/RabbitMQ Docker 启动脚本的标准化。
 
 ---
 
 ## 8. 下一步行动（立即执行项）
 
 1. **接受本规划**：确认本 master plan 作为后续工作的顶层导航文档，保留现有 implementation-plan.md 作为实施日志。
 2. **填补清单空白**：
    - 填充 static-manifest.json 的静态回归候选清单。
    - 填充 source-test-manifest.json 的非 Regression 源资产映射。
 3. **建立 client 和 multithread 域 capability**：先完成这两个域的顶层拆分和 runtime 关联。
 4. **继续方法源域（fromclausemethod）**：按 5-10 个 runtime 一批，完成 EPLFromClauseMethod 的批量移植。
 5. **数据库收尾**：处理 EPLDatabaseJoinInsertInto，若不支持则标记为后续依赖项。
 6. **标准化 Docker 环境**：为 MySQL、Kafka、RabbitMQ 提供统一启动脚本。
 7. **下一次提交**：完成上述任一切片后，必须提交并推送 origin/codex/faf-index，确保门禁通过。
 
 ---
 
 ## 9. 文档约定
 
 - 规划文档：docs/esper-go-port-master-plan.md（本文档）
 - 详细实施日志：docs/esper-go-port-implementation-plan.md
 - 覆盖率与进度：README.md、compat/capability-manifest.json、compat/java-execution-inventory.jsonl
 - 分支：codex/faf-index
 - 提交前缀：按域使用 feat(event):, feat(expr):, feat(join):, feat(context):, feat(db):, feat(method):, feat(client):, feat(multithread):, docs: 等。
 
 ---
 
 ## 10. 免责声明
 
 - 本文档及其数字**不代表 Esper 全量 Go 移植已完成**。
 - 所有覆盖率数字均为“已建立 Java/Go 对账证据”的度量。
 - 任何 capability 标记为 mapped 或 approved-difference 都必须具备正向/无效/边界/关闭/并发证据。
 - 只有全部 4,136 个 runtime 被处置为 mapped、approved-difference 或经评审的 deferred，并具备完整审计链，才允许讨论“全量完成”。
 
 ---
 
 ## 附录 A：遗漏检查与补充记录（2026-08-08）
 
 基于当前 capability-manifest.json、java-execution-inventory.jsonl 和 Java 源码目录复核，补充以下遗漏项：
 
 ### A.1 未在 capability-manifest 中建立顶层条目的域
 
 | 域 | 未登记 runtime 数 | 说明 | 建议补充 |
 |---|---|---|---|
 | client | 281 | 含 basic/compile/deploy/extension/instrument/multitenancy/runtime/stage 8 个子域 | 拆分为 client.basic、client.compile、client.deploy、client.extension 等 capability |
 | multithread | 56 | 含 context/deploy/determinism/fire-and-forget/insert-into/join/listener/named-window/pattern/stateless/subquery/time-window/update/variables/view 等 58 个类 | 作为独立 multithread 域，按并发场景拆分 |
 | epl.subselect | 169 | 子查询相关 EPL 规则 | 从 epl 大域拆出独立 capability |
 | epl.join | 161 | Join 相关 EPL 规则 | 从 epl 大域拆出独立 capability |
 | epl.insertinto | 112 | Insert into 相关规则 | 从 epl 大域拆出独立 capability |
 | epl.spatial | 60 | 空间地理规则 | 从 epl 大域拆出独立 capability |
 | epl.variable | 53 | 变量相关规则 | 从 epl 大域拆出独立 capability |
 | epl.script | 17 | 脚本相关规则 | 从 epl 大域拆出独立 capability |
 | expr.filter | 142 | 表达式过滤规则 | 从 expression 大域拆出独立 capability |
 | expr.datetime | 57 | 日期时间表达式 | 从 expression 大域拆出独立 capability |
 | expr.clazz | 25 | 类相关表达式 | 从 expression 大域拆出独立 capability |
 | resultset.outputlimit | 245 | 输出限制策略 | 已作为 resultset.output 部分，建议细化 |
 | resultset.querytype | 161 | 查询类型 | 已作为 query 部分，建议细化 |
 | resultset.orderby | 51 | 排序规则 | 已作为 resultset.output 或 query 部分，建议细化 |
 
 ### A.2 已部分实现但需细化的域
 
 | 域/文件 | 当前 Go 覆盖 | 关键未覆盖项 |
 |---|---|---|
 | fromclausemethod | 已有 FromMethod/MethodProvider 基础设施 | array/object return、overloaded、dependent/independent join、cache LRU/expiry、multikey、variable、N-stream outer（56 runtime） |
 | database | 已有 SQLHistoricalProvider、fake driver、MySQL Docker 测试 | EPLDatabaseJoinInsertInto（pattern + SQL + insert into + time batch + aggregation）；方言占位符、metadata-origin、连接池/事务/生命周期 |
 | dataflow | 已有部分 operator/source/sink 测试 | 类型化多端口/信号/背压、完整 trace parity |
 | esperio/db | 已有 DML/Upsert sink | XML/config、connection factory、完整 Java trace |
 | context | 已部分实现分区/FAF/索引共享 | selector、嵌套 context、完整生命周期 |
 | infra | 已实现 Table/Named Window 基础 | 跨 statement transaction、完整 routed/listener rollback |
 | rowrecog | 已部分实现 | 复杂 NFA、prev、interval、after、聚合、窗口删除语义 |
 
 ### A.3 清单与元数据补充项
 
 - static-manifest.json 和 source-test-manifest.json 当前均为 0 entries，需尽快填充，以支持全量审计。
 - 现有 capability-manifest.json 的 35 个顶层条目与 4,136 runtime 差距较大，建议按 Java 子域拆分新增 20+ 个 capability 条目。
 - 缺少 approved-difference 判定标准文档（可单独建 docs/esper-go-port-approved-differences.md）。
 - 缺少统一 Docker 启动脚本（建议 tools/docker-compose.esper-test.yml）。
 
 ### A.4 建议优先级调整
 
 在后续切片中，建议按以下顺序补充：
 
 1. **高优先级**：填充 static-manifest.json 和 source-test-manifest.json；建立 client、multithread 域 capability。
 2. **高优先级**：继续 fromclausemethod 域批量移植（56 runtime），已有基础设施，风险低、产出快。
 3. **中优先级**：细化 epl/expr/resultset 子域 capability，避免大域颗粒度过粗。
 4. **中优先级**：完成数据库 EPLDatabaseJoinInsertInto 收尾或明确标记依赖项。
 5. **低优先级（后续阶段）**：spatial、script、复杂 rowrecog/context 嵌套、完整 multithread 并发测试。
 
 ### A.5 后续检查约定
 
 每次完成一个切片后，应重新执行：
 
 ```powershell
 $inventory = Get-Content -Path compat/java-execution-inventory.jsonl | ConvertFrom-Json
 $manifest = Get-Content -Path compat/capability-manifest.json -Raw | ConvertFrom-Json
 $mappedIds = $manifest.capabilities | ForEach-Object { $_.javaRuntimeIds } | Sort-Object -Unique
 $allOkIds = $inventory | Where-Object { $_.status -eq "ok" } | Select-Object -ExpandProperty id | Sort-Object -Unique
 $missing = Compare-Object $allOkIds $mappedIds | Where-Object { $_.SideIndicator -eq "<=" }
 Write-Host "Unmapped runtimes: $($missing.Count)"
 ```
 
 > 本附录为 Draft 1.0 的补充检查，后续每轮切片应复核并更新。
 
