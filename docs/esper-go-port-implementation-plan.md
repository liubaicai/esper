# Esper 9.0.0 Go 全量移植规划实施文档

## 1. 文档信息

| 项目 | 内容 |
|---|---|
| 文档状态 | Draft 1.80，补充事件 indexed/mapped/nested 属性访问、JavaBean getter 元数据发现、显式 field/path/method accessor 与 nested explicit schema、有深度上限的 JSON/XML renderer、schema-directed typed JSON parse/write、XML tree/attribute/repeated-element parse、显式 parent schema inheritance、derived/statistical view 链式聚合、multi-key/array-key unique 与 rank window 的稳定快照/历史，以及 aggregate/join/current-state/event-count snapshot output；Context 已补充 multi-key key/hash/initiated-terminated API、flat periodic、fixed daily 与 static single-start/single-end cron temporal Context、`TimeOfDay`/`NewDailyTimeContext`/`NewCronTimeContext`、`ContextStartTime`/`ContextEndTime`、built-in name/id/keyN/label/hash 属性、initiated/terminated boundary-event 表达式、descriptor 的 ID/key/properties 快照、key/ID/category/hash/segmented/nested/descriptor-filter selector、Context created/destroyed replay、statement added/removed、activated/deactivated 生命周期事件、共享分区 ID 与 allocation/deallocation listener、partition-scoped context variable 注册/Get/Set/SetContextVariables/SetContextVariablesByID 原子批量更新、VariableValues/ContextVariableStates 一致快照与 selector 读取、VariableChangeListener old/new 回调、内置 selector-kind mismatch 校验、Statement.SnapshotWithSelector 迭代器式分区过滤、initiated context 的 termination-time pending/snapshot output、事件驱动 pattern-init/pattern-term 的 start-tag/end-tag 关联与 `Every` overlapping 分区、on-trigger/lifecycle、null/array tuple identity 与隔离测试，并登记首批分区/selector/lifecycle/temporal/output/pattern case；纯 TimerAt/TimerInterval/TimerSchedule/TimerCron pattern-root Context、基础 timer-plus-event observer 的序列/And 组合、动态/日历 `Within` guard 与动态/组件化/日历 `TimerInterval` observer、temporal Context termination snapshot、keyed initiated child nested Context、Dataflow、子查询及 EsperIO Kafka/AMQP/Spring JMS bridge contract 仍为部分对等 |
| Java 对照项目 | D:/Code/soc/esper |
| Java 基线 | Esper 9.0.0，tag release_9.0.0，commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c |
| Java 要求 | Java 17 |
| Go 目标项目 | D:/Code/soc/bigsoc-esper |
| 目标仓库现状 | 规划起点为空；当前工作树已出现 Go 垂直切片实现，不能再按“空仓库”判断完成度 |
| Go 工具链现状 | 本机 go1.25.5；最低支持版本在阶段 0 固化，候选为 Go 1.25 |
| 核心 API 方向 | Flink DataStream 风格的 Go 链式 API，不以 EPL 字符串作为规则定义方式 |
| 规划原则 | 对照 Java 行为完整移植，采用 Go 架构与习惯，不逐类、逐包机械翻译 |

本文以实施规划为主，不把当前 Go 原型的签名视为最终稳定 API。文中出现的方法名和调用链只表示目标 API 形态，除第 17 节外不代表对应功能已经完成。

## 2. 目标与完成定义

### 2.1 总目标

用 Go 重写 Esper 9.0.0 的复杂事件处理能力，并同时满足：

1. Esper 中与语言无关的事件处理语义全部具备 Go 实现。
2. 规则通过可组合、可检查、可复用的链式 API 构造，而不是由调用方拼接 EPL。
3. Java Esper 9.0.0 是行为基准和差分测试预言机。
4. 公共 API 符合 Go 习惯：小接口、显式 error、context.Context、泛型适度使用、清晰的所有权与并发约定。
5. 每项功能都有 Java 测试到 Go 测试的可追踪映射；不能仅以代码覆盖率代替功能完整度。
6. 在功能对等后完成并发、性能、内存、连接器、文档和示例验收。

### 2.2 “完整移植”的严格定义

完整移植指“可观察行为和能力对等”，不指 Java 类文件一对一翻译。发布完成必须同时满足：

- 功能清单中的平台无关能力状态全部为 Passed。
- Java 回归用例清单中所有适用项都映射到至少一个 Go 用例并通过。
- 所有不适用项都有书面理由、替代能力、测试证据和评审记录，不能直接标记跳过。
- 事件输出、移除流、顺序、时间推进、状态快照、异常类别和生命周期等关键行为通过 Java/Go 差分验证。
- Go 代码质量、覆盖率、竞态、模糊测试、性能和文档门禁全部通过。

### 2.3 明确边界

根据“使用链式 API，而不是 EPL 风格”的要求，首个完整版本采用以下边界：

- EPL 文本解析器、EPL 文本模块格式、EPL 到 SODA、SODA 到 EPL 的字符串兼容接口不作为 Go 公共 API。
- EPL 所表达的查询、窗口、模式、上下文、表、数据流等语义不能因此缺失；它们必须由链式 API 和底层逻辑计划完整表达。
- Java 回归测试中的 EPL 仍用于驱动 Java 预言机；对应的 Go 测试使用链式规则构造同一语义。
- 如果未来需要接收历史 EPL，可单独增加 compatibility/epl 包；它是兼容层，不得侵入核心运行时，也不作为当前完成条件。
- Java 字节码、Java 类加载、Java 反射和 Java 序列化格式不兼容；以 Go 的计划、注册表和版本化序列化机制替代。

“Flink DataStream 风格”仅指规则构造体验：类型化流、具名算子、链式组合和显式 sink。它不表示首版要复制 Flink 的分布式集群运行时、并行度槽位、watermark/checkpoint/savepoint、作业恢复或 exactly-once 语义；除非 Esper 9.0.0 本身存在对应的可观察契约，否则这些属于后续独立产品能力，不能混入 parity 验收。

首版范围严格限定为固定 commit 中已检入的开源 Esper 9.0.0 模块、公共契约、回归场景、单元测试、EsperIO 和示例：

- NEsper、EsperHA 及未检入该仓库的商业/企业能力不属于本次完成条件。
- 源码中已经存在的 serde、状态管理、编译/运行时扩展契约仍然在范围内；但这不等于承诺分布式持久化、高可用或跨节点恢复。
- OSGi、Maven、JMX、JNDI、Java 类加载和字节码形态按 G/N 级处理：保留平台无关能力，替换 Java 容器机制，不复制 Java API 外形。
- Go 异步入口、背压、进程外扩展等若超出 Esper 契约，标为 G 级增强并默认关闭，不能反向改变同步 parity 路径。

构建系统、Maven/Ant 配置、Checkstyle 规则和 Java 内部类结构不是产品功能，不逐项移植；它们分别由 Go Modules、gofmt、go vet、staticcheck 和 Go CI 规则替代。

### 2.4 完成口径的五个维度

“完整”必须同时沿五条轴验收，避免只迁移查询算子：

| 维度 | 完成证据 |
|---|---|
| 规则表达能力 | Java 9.0.0 平台无关语义均可由 Builder/AST 表达并通过无效规则校验 |
| 运行行为 | 输出、旧流、时间、顺序、状态、并发、部署和资源回收与基线一致 |
| 运维能力 | 配置、指标、诊断、生命周期、依赖检查、升级边界和连接器可操作 |
| 测试证据 | 回归执行、模块单元测试、EsperIO 测试和示例均有机器可追踪处置 |
| 工程质量 | Go API、平台矩阵、性能预算、安全、许可证和版本兼容门禁通过 |

## 3. 基线盘点

### 3.1 Java 模块规模

以下数字来自对固定 commit 的静态扫描，用于评估范围，不直接作为完成指标：

| 模块 | 职责 | 主代码 Java 文件 | 单元/入口测试 Java 文件 |
|---|---|---:|---:|
| common | 类型系统、SODA、表达式、视图、聚合、连接、模式、上下文、数据流等主体 | 5,479 | 319 |
| common-avro | Avro 事件表示 | 55 | 3 |
| common-xmlxsd | XML/XSD 支持 | 3 | 2 |
| compiler | EPL/SODA 编译、验证、依赖和 Java 代码生成 | 127 | 15 |
| runtime | 部署、事件服务、调度、监听器、阶段和运行时服务 | 380 | 32 |
| regression-lib | 回归场景与断言 | 1,380 | 场景代码位于 main |
| regression-run | JUnit 回归入口 | 0 | 82 |
| EsperIO 七个子模块 | AMQP、CSV、DB、HTTP、Kafka、Socket、Spring JMS | 144 | 58 |

额外基线：

- common/client/soda 下有 208 个 Java API 文件，是 Go 规则 AST 的重要结构参考。
- regression-lib 静态扫描到 3,848 条 RegressionExecution 候选；运行时按 801 个外层 suite 枚举得到 4,136 条可构造 execution 记录，另有 4 条明确 ignored，不能把两组数字混为同一覆盖率分母。
- regression-run 静态扫描到约 860 个 public test 入口方法。
- examples 下有 17 个示例项目、34 个 Java 测试源文件和 181 个 Java 主源码文件，需要按用例价值转换为 Go 示例或端到端测试。
- 回归标签包含多线程、性能、无效输入、即席查询、序列化、数据流、运行时操作、编译器操作和事件发送器等维度。
- 除 regression-lib 外，common/compiler/runtime/common-avro/common-xmlxsd 共 371 个 Java 单元测试文件、regression-run 有 82 个入口源文件、EsperIO 共 58 个测试文件，也必须逐项分类；不能只迁移 RegressionExecution。
- 17 个示例为 autoid、benchmark、cycledetect、marketdatafeed、matchmaker、namedwinquery、ohlcpluginview、qos_sla、rfidassetzone、runtimeconfig、servershell、stockticker、terminalsvc、terminalsvc-jse、transaction、trivia、virtualdw。

当前已生成 `compat/source-test-manifest.json`（`esper-source-tests/v1`）作为静态源资产盘点：core-unit 371、regression-entry 82、esperio-unit 58、example-test 34、example-source 181，共 726 个 Java 源文件。另已生成 `compat/java-execution-inventory.jsonl`：静态候选 3,848 条、外层 suite 801 个、运行态记录 4,140 条，其中 4,136 条可枚举且 `runtimeId` 无重复、4 条明确 ignored、0 条探针错误。两类文件仍只证明“发现并固定了源/运行态资产”，尚未证明 Java 测试运行、Go 映射或差分通过；source-test disposition/mapping 仍是阶段 0 的退出条件。

阶段 0 必须同时使用静态扫描和受控运行时枚举生成唯一用例 ID。静态文本计数可能包含参数化或内部实现，而 `executions()` 又可能动态构造多个场景、配置变体和重复名称，两者都不能单独代替最终清单。单元测试中只验证 Java 解析器、字节码生成器或类加载细节的条目可以标为 N，但必须留下逐项处置记录和替代的 Go 测试依据。

### 3.2 Java 回归功能域

现有 regression-lib 的一级功能域和 Java 文件数如下：

| 功能域 | 文件数 | 主要内容 |
|---|---:|---|
| client | 91 | 编译、部署、运行时、扩展、阶段、多租户、观测 |
| context | 20 | 分段、哈希、分类、嵌套、启停、选择器、变量 |
| epl | 181 | 连接、子查询、数据流、数据库、插入流、变量、空间等语义 |
| event | 128 | Bean、Map、Object Array、JSON、Avro、XML、Variant、渲染 |
| expr | 117 | 核心表达式、日期时间、枚举方法、声明表达式、过滤器 |
| infra | 91 | Named Window、Table、索引、触发式增删改查 |
| multithread | 56 | 运行时、窗口、模式、上下文、表和监听器并发 |
| pattern | 33 | every、not、and/or、followed-by、until、guard、observer |
| resultset | 54 | 聚合、查询结果形态、排序、输出限制 |
| rowrecog | 26 | 行模式识别、NFA、间隔、贪婪、重复、状态上限 |
| view | 29 | 长度、时间、批次、唯一、排序、排名、交并、派生视图 |

该分类将成为 Go 兼容性看板的一级目录，不能按实现方便程度重新缩小范围。

## 4. 移植原则与兼容等级

### 4.1 原则

1. 行为优先：以输入、时间、状态和输出行为为真值，不以 Java 内部类名为真值。
2. Go 原生：避免 EP 前缀、Bean 风格 getter/setter、巨型服务接口、异常控制流和类加载器思维。
3. 计划可分析：规则必须生成显式 AST/逻辑计划，优化器不能依赖无法检查的任意闭包。
4. 确定性优先：默认同步处理、显式时钟、稳定顺序和可复现调度；异步吞吐是可配置能力。
5. 先语义后优化：第一版物理执行器以正确性和可观测性为先，热点确认后再做专用算子和内存布局优化。
6. 公共 API 稳定、内部实现可替换：逻辑计划与运行时之间必须有清晰边界。
7. 测试与功能同行：任何功能任务必须同时带对照用例、负例、并发/时间边界和文档。

### 4.2 兼容等级

每个兼容项必须标注以下等级之一：

| 等级 | 含义 | 示例 |
|---|---|---|
| S：语义同等 | API 可不同，可观察语义必须相同 | 窗口、聚合、连接、模式、上下文 |
| G：Go 等价 | Java 机制不可直接搬运，以 Go 习惯提供同能力 | struct 事件、database/sql、函数注册 |
| C：可选兼容层 | 不属于核心，但未来可添加历史兼容入口 | EPL 文本解析 |
| N：非产品能力 | 构建或 Java 实现细节，不移植 | Maven、Janino 字节码类结构 |

S 和 G 都属于“完整移植”。任何从 S/G 改为 C/N 的变更必须通过架构决策记录，不能由开发者在测试中静默跳过。

### 4.3 Java 专属机制映射

| Java Esper 能力/机制 | Go 方案 | 验收重点 |
|---|---|---|
| SODA 对象模型 | 不可变规则 AST + 链式 Builder | 能表达相同语义；Build 时完整验证 |
| ANTLR EPL 编译 | 无 EPL 主入口；Builder 直接形成 AST | 不依赖字符串回解析 |
| Janino/字节码生成 | 逻辑优化 + 物理算子；后续可做类型专用执行器 | 结果一致、性能有基线 |
| EPCompiled | 只读、版本化 Plan/Module 工件 | 依赖、参数、版本和校验和明确 |
| EPRuntime | Engine 及聚焦的小服务接口 | 生命周期、部署、事件、时钟、阶段等价 |
| JavaBean 事件 | Go struct + tag/显式 Schema | 嵌套、可空、动态属性和继承映射明确 |
| Map/Object[] | map 事件和具名 Row 事件 | 类型检查、字段顺序、缺失值语义明确 |
| Java 反射函数 | 稳定名称的函数/聚合/视图注册表 | 编译期签名检查，部署时依赖检查 |
| Nashorn 脚本 | 可插拔 ScriptProvider；默认不绑定具体脚本语言 | 沙箱、确定性、超时、类型边界 |
| 内联 Java class | 预编译 Go 扩展或进程外扩展 | 不能在运行时编译 Go；能力有等价入口 |
| JDBC 历史流 | database/sql 数据源 | 参数绑定、缓存、事务和取消 |
| Java Serialization | 显式版本化的计划/状态编码 | 兼容版本策略、校验、拒绝未知函数 |
| Spring JMS | 通用消息 Source/Sink + 可选 JMS 网桥 | 交付语义、确认、重试、顺序 |
| ClassLoader 插件 | 编译期注册或稳定 RPC 扩展 | 跨平台，不依赖不稳定的 Go plugin |
| Java annotations | Builder 元数据和 functional options | name、priority、audit、hint、visibility 等价 |
| XML Configuration 及 Bean 式配置 | Go Config 结构、校验和 functional options；外部文件解码为可选适配层 | 配置能力对等，不要求 XML 语法兼容 |
| Java 方法重载/泛型工厂 | 不同具名函数、option 或显式类型参数 | 不制造模糊签名，错误仍能定位到原语义 |
| JMX 指标与管理 | Go metrics/exporter 和管理接口 | 指标、启停、分组和查询能力对等，不要求 JMX 协议 |
| JNDI 命名上下文 | 显式服务/数据源注册表 | 名称解析、生命周期和缺失依赖诊断对等 |
| OSGi/Maven 模块元数据 | Go module、Plan module 元数据和依赖图 | 保留 Esper 模块语义，不复制 Java 打包容器 |
| Runtime PluginLoader | 显式注册的 Initializer/ServiceProvider 或进程外服务 | 初始化顺序、配置、关闭和失败回滚对等，不按类名反射加载 |

许可证评审必须覆盖整个目标 Git 历史，而不只是当前空工作树。清空 Java 文件不会消除 GPLv2 来源；测试 fixture、复制/改写的断言、版权声明和第三方依赖也在审查范围内。若目标是非 GPL 发布，必须在编码前完成 EsperTech 商业许可或独立净室实现方案的法律确认。

## 5. 完整功能范围

### 5.1 事件模型与类型系统

必须覆盖：

- Go struct、map、Row、JSON、Avro、XML DOM/XSD 和 Variant 事件。
- 静态、动态、嵌套、索引、映射属性访问，fragment、超类型/接口式兼容和事件别名。
- Schema 注册、推导、继承/组合、可见性、部署作用域、总线事件类型。
- 事件复制、制造、写属性、渲染 JSON/XML、发送器快速路径。
- 缺失字段与显式 null 的区分；类型元数据和运行时查询。
- 数组、集合、时间、枚举、任意对象和用户自定义类型的边界处理。
- Java AccessorStyle 的 JAVABEAN、EXPLICIT、PUBLIC 和 PropertyResolutionStyle 的 CASE_SENSITIVE、CASE_INSENSITIVE、DISTINCT_CASE_INSENSITIVE 都要有明确的 Go 映射和歧义错误。
- struct 事件需固化导出字段、tag、嵌入 struct、指针、接口、显式 getter 注册和方法暴露规则；不得默认把任意方法当属性调用。
- 类型元数据需包含 simple/mapped/indexed property descriptor、缓存 getter、fragment、underlying 对象、start/end timestamp 字段以及可写性。
- Event Type Service 必须能区分预配置、部署作用域和 event-bus 可见类型，并提供按名称/部署查询。
- map 与 Row/Object Array 的字段顺序、复制和写入语义必须稳定；复合键和数组键要贯穿分组、窗口、Context、索引和 Pattern。
- Variant 的 PREDEFINED 与 ANY 模式、事件 identity/equality 以及超类型匹配规则必须单独测试。

各动态格式还需覆盖：

- JSON：严格/宽松解析、数字精度、深度限制、原生表示、provided-underlying 等价能力、自定义 parser hook 和 Schema 演进。
- XML：有/无 XSD、DOM 与 XPath 属性访问、namespace、相对/绝对路径、fragment、根元素校验，以及 XPath 函数/变量 resolver；默认防御 XXE 和实体扩张。
- Avro：Schema 对象/文本、native string、非 null default、supertype、类型映射/拓宽 hook、logical type 与 Schema 演进。

Go 内部需要 tagged Value 模型，至少区分 Missing、Null 和 Present。直接用 nil 无法复现动态属性和三值逻辑。

### 5.2 表达式、函数和过滤

必须覆盖：

- 算术、位运算、比较、逻辑、拼接、in/not-in、between、like、regexp。
- case、coalesce、cast、instance-of/type-of、exists、数组、新对象/行构造。
- any/all/some、数组索引、赋值/更新以及新数组构造。
- prior、prev、窗口访问、当前时间、当前求值上下文。
- 子查询表达式、表访问、变量、替换参数、声明表达式。
- contained-event 选择、嵌套事件拆分、脚本表达式和 Go 等价的预注册扩展表达式。
- 日期时间重格式化、日历操作、区间关系和时区。
- 集合/枚举方法、lambda 形态、链式属性/方法访问；至少逐项覆盖 aggregate、allOf/anyOf、arrayOf、average、countOf、distinctOf、except/intersect/union、firstOf/lastOf、groupBy、min/max、minBy/maxBy、most/least frequent、orderBy/orderByDesc/reverse、selectFrom/where、sequenceEqual、sumOf、take/takeLast/takeWhile/takeWhileLast、toMap 和插件 enum method，以及带 index/size 参数的 lambda。
- 单行函数、聚合函数、多函数聚合、日期时间方法、枚举方法扩展。
- 过滤索引、范围索引、in 索引、布尔表达式和可优化/不可优化路径。
- null 传播、数值提升、溢出、NaN、无穷、字符串排序和错误分类。
- 编译配置中的整数除法、除零返回 null、duck typing、UDF cache、扩展聚合和 MathContext 语义。
- 子查询求值顺序、自引用子查询 pre-evaluation 以及含副作用 UDF 时的调用次数。

默认表达式 API 生成可检查 AST；任意 Go 函数作为显式 UDF 使用，并标记纯度、确定性、线程安全和序列化名称。

### 5.3 流、视图与窗口

必须覆盖：

- keep-all、length、length-batch、time、time-batch、time-length-batch。
- externally-timed、externally-timed-batch、time-order、time-to-live、time-accum。
- first-event、last-event、first-length、first-time、first-unique、unique。
- sort、rank、group、expression window、expression batch。
- size、univariate statistics、weighted average、linear regression、correlation 等 derived/statistical view。
- view 的 union、intersect、组合、参数化上下文。
- previous/prior 访问和插入流/移除流。
- 微秒级时间分辨率、外部时钟和系统时钟。
- 分组状态回收 hint：age、frequency 和 disable，以及 iterable-unbound 行为。
- `leaving()` 等窗口到期指示语义。

窗口实现必须通过统一 StateStore、Clock 和 Scheduler 接口，避免每个窗口自行处理锁和时间。

### 5.4 结果集、聚合和输出

必须覆盖：

- select/projection、通配符、流通配符、别名、嵌套结果。
- distinct、聚合、访问聚合、插件聚合、局部分组；访问聚合至少包括索引化 first/last/nth、窗口事件、sorted/min-by/max-by 和 ever 变体，并支持嵌套在外层分组结果中；表侧还要支持 Go 原生 table sink、live table snapshot join、导航 map/submap 和列访问生命周期。
- count/sum/avg/min/max、first/last/window、firstever/lastever、nth、rate、median、stddev、avedev、sorted/maxby/minby 和 Count-Min Sketch 等内建聚合族。
- group by、grouping sets、rollup、cube、grouping/grouping_id、having。
- row-per-event、row-per-group、aggregate-all 和非聚合结果模式。
- order by、limit/offset、变量行数限制。
- output first/last/all/snapshot、按事件数/时间/日历/条件输出。
- output after、cron、when/then、默认 stream selector 和 output-limit 优化开关。
- grouped/discrete delivery、条件输出后的变量更新和 update-istream。
- istream、rstream、irstream 以及旧值/新值配对。
- into-table 聚合与表列访问；Go 侧支持链式 `AggregateStream.IntoTable`、目标列/类型/主键构建校验、plain/ROLLUP/CUBE/GROUPING SETS 的当前快照物化、窗口淘汰后的原子快照替换和整组移除，以及过滤 scalar/access 列；FAF 明确拒绝该 live side effect。
- event-precedence 插入/派发顺序及其运行时配置开关。
- 无 from/source 的 select、仅 Context 的 statement 和 FAF 查询。

### 5.5 连接、子查询、历史流和空间能力

必须覆盖：

- 二流到多流连接、自连接、内连接、左/右/全外连接。
- 单向流、保留关键字语义、窗口组合和连接结果顺序。
- 哈希、范围、复合、唯一索引及查询计划选择。
- 相关/非相关子查询、exists/in/quantified、聚合子查询。
- Named Window/Table 子查询和索引复用。
- database/sql 历史查询、方法流/拉取流、参数绑定、参数键缓存和取消。
- 空间点/矩形查询及空间索引。
- 查询计划 hint 和排除策略中平台无关的行为。

database/sql 适配不能只做到“能查询”，还需对照占位符方言、metadata-origin 与 SQL 类型映射、列名大小写转换、null 映射、自定义输入/输出转换 hook、连接生命周期、LRU/expiry 缓存、取消，以及 prepared query 的 Close 语义。当前 Go 已实现参数键 LRU/expiry、缓存结果防修改、列名大小写归一化、调用方可插拔的占位符重写/metadata 转换和可选 PreparedStatement 生命周期；完整方言矩阵、metadata-origin/type binding、连接池/事务及 DML 仍以 capability manifest 为准。

### 5.6 CEP Pattern 与 Match Recognize

Pattern 必须覆盖：

- filter/tag、every、every-distinct、not、and、or、followed-by。
- followed-by 最大状态、match-until、重复上下界。
- consuming filter/pattern、死模式回收和子表达式池限制。
- timer:interval、timer:at、timer:schedule。
- within、within-or-max、while guard 和自定义 guard/observer。
- 启停、路由事件、复杂属性、继承事件、组合 select。

Match Recognize 必须覆盖：

- 连接、选择、排列、交替、嵌套、重复、贪婪/非贪婪量词。
- define、measure、partition、prev、聚合、数组访问。
- interval、or terminated、after/skip 策略。
- NFA 状态限制、空分区回收、数据窗口和删除事件。

### 5.7 状态基础设施

必须覆盖：

- Named Window 创建、消费、索引、迭代、更新和删除。
- Table 分组/非分组行、聚合列、普通列、主键和二级索引。
- insert-into、on-select、on-delete、on-update、on-set、on-merge、split stream。
- create schema/index/variable/expression/context/dataflow 的 Go 等价管理 API。
- 变量的配置、读写、原子更新、上下文变量。
- Fire-and-Forget select/insert/update/delete、准备查询和参数化查询。
- 依赖关系、作用域、部署卸载前置条件和资源回收。
- Table 聚合重置、FAF multi-row insert、named/indexed 参数绑定，以及 prepared query 的幂等关闭。
- 多变量和跨 Context 分区更新必须 all-or-nothing；批量读取必须提供一致快照，并覆盖 constant、延迟/版本释放语义。

### 5.8 Context、分区和多租户

必须覆盖：

- category、hash segmented、key segmented、initiated/terminated。
- immediate、never、filter、pattern、time period、crontab 条件。
- overlapping/non-overlapping、distinct、nested context。
- 优先级、context selector、分区枚举、变量和生命周期监听。
- context 中的窗口、表、Named Window、子查询和即席查询。
- runtime URI、stage 独立环境及 stage/unstage。
- Context 管理服务需提供分区数量、ID、descriptor、properties、selector 查询和已有 listener 枚举。

### 5.9 编译、部署与运行时

必须覆盖：

- Builder/Module 到逻辑计划的编译、类型检查、名称解析和语义验证。
- 编译路径、部署路径、公共/保护/私有对象、模块依赖和循环检测。
- Compiler PathCache、module order/uses/imports/URI/archive/user metadata、确定性的模块拓扑排序，以及编译 hook/计划检查和 state-management setting。
- 替换参数、部署选项、rollout 原子性、undeploy 前置条件。
- Statement 名称、编译期/运行时用户对象、properties/metadata、优先级、启动/停止和迭代。
- Listener、Subscriber、Sink、unmatched listener 和事件路由。
- struct/map/Row/JSON/Avro/XML 事件发送。
- 外部时钟、系统时钟、时间推进、调度顺序和统计。
- initialize、destroy、状态监听、异常/条件处理。
- 同步、并发发送、重入路由、动态监听器管理及全局生命周期安全。
- common/compiler/runtime 配置能力：事件类型、缓存、执行策略、线程、过滤、Pattern/Match 状态限制、指标、日志、时间源、异常和条件处理。
- recompile provider/升级路径、部署重定义和版本检查、依赖 provided/consumed 检查、状态 listener 枚举。
- 部署锁策略和超时、原子 rollout 失败回滚，以及卸载/销毁时的 graceful drain。
- safe iterator 的关闭/锁契约、拉取快照一致性和 Event Type Service 查询。

common 配置要逐项覆盖类型/annotation import 的 Go 注册表等价能力、事件类型与默认元数据、auto-name 等价策略、database/method reference 及缓存、变量、Variant stream 和 transient startup object；Java 包扫描和类名反射改为显式注册，但名称解析行为必须测试。

运行时配置及关联 statement annotation 清单至少要展开到：priority/drop/nolock、fair lock/disable locking、filter profile、declared-expression cache、event precedence、subselect pre-evaluation、inbound/outbound/route/timer 线程池及容量、listener/insert-into/Named Window consumer 的 preserve-order 与 dispatch timeout/locking、内部 timer enable/resolution、Pattern/Match 状态上限与 prevent-start、metrics group/JMX 等价出口、变量版本释放、exception/condition/undeploy policy，以及 execution/timer/lock/audit/query/filter plan 日志。配置项必须有默认值、冲突校验、运行时是否可变和测试映射，不能只保留一个笼统的 Config 对象。

编译配置还要覆盖 filter max width/index planning、declared-expression cache 开关、default stream selector、iterable-unbound/output-limit optimization、表达式与脚本设置、扩展注册、serde provider、编译并发/容量和附加 source/plan/debug metadata。JVM 字节码方法/常量池限制标为 N，但对应的 Go 计划大小、递归深度和编译资源上限必须另行定义。

### 5.10 Dataflow、扩展和可观测性

必须覆盖：

- Dataflow 图定义、source/operator/sink、端口、类型、参数和信号。
- 实例化、启动、取消、join/captive 模式、异常处理和统计；对照 INSTANTIATED、RUNNING、COMPLETE、CANCELLED 状态及合法转移。
- 实例名/user object、operator provider、parameter provider、exception handler、统计开关和 captive emitter 的实例化选项。
- 保存/读取/删除实例和 saved configuration，并支持从已保存配置再次实例化。
- 内建 dataflow operators 至少逐项对照 BeaconSource、Emitter、EPStatementSource、EventBusSource、EventBusSink、Filter、Select、LogSink，并提供自定义 operator 接口。
- 自定义聚合、多函数聚合、单行函数、enum/date-time 方法。
- 自定义 view、virtual data window、pattern guard/observer、事件表示和 serde。
- audit、instrumentation、runtime/statement metrics、日志与异常回调。
- 计划说明能力：至少能输出规范化逻辑计划和物理算子/索引选择，便于差分诊断。

Stage 必须拥有独立事件流、时间和对象解析域。由于 Java 侧 Stage API 本身标记为不稳定，Go 对应 API 在完成跨域差分和生命周期压力测试前保留 experimental 标记，不与首批稳定接口一起提前冻结。

### 5.11 EsperIO 与示例

Go 等价连接器范围：

| Java 子模块 | Go 目标 |
|---|---|
| esperio-amqp | AMQP Source/Sink，确认、重连、可注册 codec/JSON 和背压；Java Serializable 不作为线格式兼容目标 |
| esperio-csv | CSV/File Source/Sink、unformatted line source、适配器协调、类型转换和定时回放 |
| esperio-db | database/sql DML/Upsert 输出适配器、参数、事务、执行器与连接池 |
| esperio-http | HTTP client/server Source/Sink、同步/异步模式 |
| esperio-kafka | Kafka Source/Sink、consumer group、offset、确认与错误策略 |
| esperio-socket | TCP Server 输入适配器，覆盖 OBJECT 等价注册 codec、CSV、PROPERTY_ORDERED_CSV、JSON、并发连接和生命周期；不解码 Java Serialization |
| esperio-springjms | 通用消息接口；需要 JVM JMS 互通时提供进程外 bridge |

17 个 Java 示例不要求保持工程结构，但每个独立业务场景都要落为 Go example、教程或端到端测试。示例同时承担 API 易用性验收。

所有 adapter 统一对照 OPENED、STARTED、PAUSED、DESTROYED 状态及 start/pause/resume/stop/destroy 转移。每个连接器必须单独声明 at-most-once/at-least-once、确认、重试、乱序、重复、失败恢复和 drain 契约；没有端到端事务证据时不得宣称 exactly-once。

当前已先实现 `connectors` 公共 `StateManager` 与 `connectors/csv`：`Source` 支持 Path/Reader/Open、标题行/属性顺序、`#` 注释、quoted CSV、String/Int/Int64/Float64/Bool/Time/JSON 和自定义 converter、空值、严格字段、loop/reset、固定速率与 timestamp-delta 回放、EOF/取消、`RunToEngine`；`LineSource` 覆盖无格式行文件；`Sink` 支持固定列/首记录排序列、header、append、flush、Path/Writer/Open 及 Esper Event/Row/TableRow/struct 转换。随后实现 `connectors/db` 的 database/sql DML/Upsert sink（绑定、prepared statement、重试、嵌套属性、生命周期、MySQL Docker round-trip）、`connectors/http` 的 client/server source/sink（GET/POST、query/property URI 模板、JSON body、响应上限、重试、请求采集、暂停/停止/重启、Engine bridge）、`connectors/socket` 的 TCP source（OBJECT/CSV/PROPERTY_ORDERED_CSV/JSON、Java escape、并发连接、暂停/恢复、停止/重启、typed Engine bridge、可注册 object decoder）、`connectors/kafka` 的 Reader/Writer adapter（kafka-go bridge、JSON processor、commit-after-process/immediate、重试、timestamp hook、producer key/header/ordered JSON、Engine bridge、factory restart）、`connectors/amqp` 的 RabbitMQ Consumer/Publisher adapter（amqp091-go bridge、host/port/user/vhost、queue/exchange/binding、prefetch、JSON/GOB codec hook、auto/manual ack、reject/requeue、重试、Engine bridge、factory restart）和 `connectors/jms` 的 provider-neutral Spring JMS bridge contract（Map/Text/Object/Bytes、Java event-type property、DecodeAny、ack-after-process、重试、Engine bridge、channel transport）。Go 侧所有 adapter 统一暴露 `Start/Pause/Resume/Stop/Destroy`，并为每个状态转移、EOF、暂停/恢复、loop/reset、类型错误、回放、网络错误和输出边界建立单测。

本切片的 Java 对照已执行：`mvn -pl esperio/esperio-csv -am -Dgpg.skip=true -DskipITs=true test`，EsperIO CSV 模块 80 个测试全部通过；`mvn -pl esperio/esperio-db -am -Dgpg.skip=true -DskipITs=true test` 运行 4 个测试，其中 Upsert 通过，配置顺序和 DML 数据库断言差异已记录；`mvn -pl esperio/esperio-http -am -Dgpg.skip=true -DskipITs=true test` 运行 4 个测试，3 个通过，`TestHTTPAdapterOutput` 在当前环境的旧订阅/URI 编译与连接路径上失败；`mvn -pl esperio/esperio-socket -am -Dgpg.skip=true -DskipITs=true test` 运行 7/7 通过；Kafka 在单节点 Kafka 3.8.1 KRaft、四个输入 topic 预创建的条件下运行 6/6 通过，Go `TestKafkaDockerRoundTrip` 也通过；AMQP 模块已在 JDK 17/Maven 下编译，首次无 broker 运行是 5/5 连接错误，启动 RabbitMQ 后旧版 `QueueingConsumer` 输出等待未得到可完成的 Maven 计数，已作为 Java 测试 harness 差异保留，Go `TestAMQPDockerRoundTrip` 与 `TestAMQPDockerSinkRoundTrip` 均通过；Spring JMS 模块运行 4 个测试，1 个通过、3 个失败，失败包含 ActiveMQ/JMX 实例冲突和 Map/Text 事件类型标记环境差异，Go `connectors/jms` 的 channel bridge 与 codec/ack/lifecycle 单测通过。当前仍是 `partial`，因为 Java 的 bean population、AdapterCoordinator/Dataflow signal-marker、CSV/File 全量 graph 语义、DB 配置/异步执行器、HTTP XML/classic service、Socket XML/plugin/writable-property cache、Kafka group/rebalance/custom serializer/plugin、AMQP Java serialization/重连恢复、JVM JMS provider/session/transaction/Spring XML 尚未共享 trace 化；不能据此宣称 EsperIO 或全量 Esper 完成。

## 6. Go 链式 API 设计

### 6.1 概念模型

建议的主链路是：

    Environment
      → Source / Pattern / Table / Dataflow
      → Filter
      → KeyBy / Join / Context
      → Window
      → Aggregate / Match
      → Project
      → Output policy
      → Sink
      → Build
      → Compile
      → Deploy

典型语义形态：

- 过滤聚合：From(Trade) → Filter → KeyBy(Symbol) → Window(Time) → Aggregate(Sum) → Having → Project → Emit。
- 时间连接：From(Order) → Join(Payment) → On(OrderID) → Within → Project。
- CEP：Pattern → Begin(A) → Where → FollowedBy(B) → Where → Within → Select。
- 状态更新：CreateTable → From(Event) → KeyBy → Into(Table)，以及 On(Trigger) → Merge(Table)。

这些链最终直接构造 AST，不生成 EPL 再交给解析器。

“链式”不等于所有能力塞进一个万能 Stream：Pattern、Match Recognize、Table/Named Window、Context、Module 和 Dataflow 使用各自的领域 builder，在 Environment/Module 层组合并汇入统一 AST。这样保留流畅风格，同时避免一个接收者暴露数百个在当前状态无意义的方法。

### 6.2 API 家族

| API 家族 | 责任 |
|---|---|
| esper | Engine、Environment、Module、Plan、Deployment、Statement |
| event | Schema、字段、动态事件、Row、Envelope、时间戳 |
| expr | 类型化表达式、聚合、函数引用、日期/集合操作 |
| stream | Source、转换、Join、Projection、Output、Sink |
| window | 窗口和 view 定义 |
| pattern | CEP Pattern Builder |
| match | Match Recognize Builder |
| state | Table、Named Window、Variable、Index、FAF |
| context | Context 和分区定义/选择 |
| dataflow | Dataflow 图和 operator 接口 |
| extension | 函数、聚合、view、guard、observer、serde 注册 |
| connector 子模块 | AMQP、CSV、DB、HTTP、Kafka、Socket、消息 bridge |

最终包数量应通过原型验证控制，避免把 Java 包层次原样搬到 Go。

### 6.3 Go 风格约束

- 只在事件/结果形态和表达式值类型上使用泛型，不建立深层 Java 式泛型继承。
- 同时提供类型化 Stream[T] 与基于显式 Schema 的动态 Stream；两者进入同一逻辑计划。
- 字段表达式必须可解析、可类型检查、可序列化。普通闭包仅作为显式 UDF，不作为默认字段选择方式。
- Builder 采用不可变或逻辑不可变节点，允许安全复用；Build 返回 Plan 和 error。
- 链中间不 panic。配置、验证、编译、部署和运行错误使用可 errors.Is/errors.As 的错误类型。
- 需要取消、超时或 I/O 的方法接收 context.Context；纯规则构造不接收 context。
- functional options 仅用于可选配置；查询语义使用具名链式方法，避免 Option 堆叠成为另一种字符串 DSL。
- 公共接口保持小而聚焦；不复制 EPRuntime 的大服务定位器形态。
- Go 命名使用 ID、URI、HTTP、JSON 等惯用缩写，不保留 EP 前缀。
- nil、零值、关闭顺序、并发安全、回调是否可重入必须写入每个公共 API 的契约。
- Go 没有方法重载；Java/SODA 的重载工厂必须映射为少量具名操作、显式 option 或独立 builder，不能靠 `...any` 模拟重载。

### 6.4 泛型形变与链式 API 可行性门

Go 不支持“方法级新类型参数”：接收者为 `Stream[T]` 的方法不能再自行引入结果类型 `U`。因此 Filter、Window 等保持事件类型的操作可以自然作为方法，但 Project/Map、Join、Aggregate、Match 等改变结果类型的操作无法原样复制 Java/Flink 的泛型方法链。

阶段 1 必须用 builder-only 原型比较并固化一种可持续方案，候选组合包括：

1. 类型保持操作使用 `Stream[T]` 方法，类型变化操作使用顶层泛型组合器。
2. 在第一次动态投影后进入基于显式 Schema 的 RecordStream/RowStream，在 sink 或边界处显式解码成目标 struct。
3. 使用 `go generate` 生成类型化字段描述符、投影结果和适配器；仍保留反射/动态 Schema 兜底。
4. 对少量固定形态使用不同接收者 builder，而不是制造一棵 Java 式泛型继承树。

评审维度包括链式可读性、编译期类型安全、AST 可分析性、错误定位、动态事件、包依赖、生成代码成本和 API 演进。不得用 `any` 全面抹平类型，也不得为了保持“全是点号调用”牺牲表达能力。

在冻结任何 v0 API 前，至少完成 Join、Pattern 标签结果、Aggregate/Projection 类型变化、Table Merge、Context 分区、Dataflow 多端口和动态 JSON/XML/Avro 七类 builder 原型；这些原型只验证公共形态和 AST，不要求提前实现完整运行时。

### 6.5 类型安全与动态能力的平衡

Go 无法从普通闭包中可靠提取字段 AST，也不支持 Java 运行时编译。因此采用双轨表达式：

1. 可分析表达式：字段句柄、常量、操作符和已注册函数组成 AST，用于类型检查、索引、优化和序列化。
2. 不透明 UDF：直接调用 Go 函数，但必须声明输入/输出签名、确定性、线程安全、是否可序列化以及稳定注册名。

能用可分析表达式表示的内建能力不得退化为反射或闭包。动态 map/JSON/Avro/XML 仍通过 Schema 在 Build 阶段检查。

字段访问优先使用可生成、可版本化的 typed descriptor；无生成步骤时允许显式 Schema + 字段句柄和受控反射。两条路径必须形成同一种 AST，并由同一 parity 场景验证，生成代码不得成为运行时语义的另一套实现。

### 6.6 Builder 合法性

不建议用大量阶段接口在编译期编码所有合法链，因为 Esper 语义组合过多，会形成难维护的接口爆炸。采用：

- 泛型保证事件和常用表达式的基本类型安全。
- Builder 记录来源、状态资源、输出和上下文。
- Build 阶段统一执行名称、类型、作用域、依赖和语义验证。
- 错误携带规则名、节点路径、字段、期望类型、实际类型和可修复建议。

### 6.7 生命周期 API 映射

| Java | Go 方向 |
|---|---|
| Compiler.compile | Environment.Build / Compiler.Compile |
| EPCompiled | Plan 或 Module |
| DeploymentService.deploy | Engine.Deploy |
| undeploy / rollout | Engine.Undeploy / Engine.Rollout |
| EPStatement | Statement |
| addListener / subscriber | Statement.Subscribe / To(Sink) |
| sendEvent / routeEvent | Engine.Send / Engine.Route |
| advanceTime | Engine.AdvanceTime |
| fire-and-forget | Engine.Query / Prepare / Exec |
| stage service | Engine.Stage / Stage.Move |

名称全部是候选。阶段 1 完成基础垂直切片和第 6.4 节全部高级域 builder 原型后，只冻结可供后续迁移使用的 provisional v0；Stage、Dataflow 和未完成差分的高级 API 继续标记 experimental。稳定 v1 只能在阶段 10 全量对照通过后冻结。

## 7. 总体架构

### 7.1 分层

    链式公共 API
          │
          ▼
    不可变逻辑 AST ───── Schema / Function / State Catalog
          │
          ▼
    语义分析与规范化
          │
          ▼
    逻辑优化与依赖图
          │
          ▼
    物理计划与算子工厂
          │
          ▼
    部署与生命周期管理
          │
          ▼
    事件分派 → Filter/View/Join/Aggregate/Pattern → 输出
          │                         │
          ├── Clock / Scheduler     ├── State / Index
          ├── Context / Stage       ├── Metrics / Trace
          └── Connector ingress     └── Listener / Sink

### 7.2 编译管线

编译阶段依次执行：

1. 收集模块、事件类型、变量、表、Named Window、Context、函数和规则声明。
2. 解析名称、作用域、可见性和部署依赖。
3. 推导表达式、流和输出 Schema。
4. 校验 null、数值提升、聚合、窗口、连接、子查询和上下文限制。
5. 规范化表达式和规则节点，消除 API 构造顺序差异。
6. 构建过滤索引、连接查询图、聚合策略和 Pattern/Match NFA。
7. 生成物理算子图、状态需求、调度项和生命周期钩子。
8. 输出不可变、确定性编码、带版本、校验和、依赖摘要及可选 source/plan 调试元数据的 Plan；相同输入和注册表版本生成稳定 plan hash。

Plan 不包含任意函数指针的裸序列化值；扩展通过稳定注册名和版本约束解析。
Plan Schema 版本独立于 Go module/API 版本管理，加载前完成版本、校验和、依赖和能力协商；反序列化不得触发任意扩展代码。

### 7.3 运行时模型

- 默认 Send 同步完成本事件触发的计算和输出，便于复现 Esper 顺序。
- 可选异步入口是 G 级增强，必须显式配置队列容量、分区方式、背压和关闭策略；默认 parity 路径保持同步。
- 同一逻辑分区内保持确定顺序；跨分区只承诺文档声明的顺序。
- Route 采用当前处理循环内的有界优先级队列，复现 route/insert event-precedence，并避免无界递归。
- Clock、Scheduler 和 Timer Queue 独立于系统时间，可注入虚拟时钟。
- Stage 拥有独立事件流和时钟域，但共享规则工件的方式必须明确。
- 部署/卸载与发送的互斥范围按资源图设计，不照搬单一 JVM 全局锁。
- Listener/Sink 的同步、异步、错误、阻塞和重入策略均显式配置。

### 7.4 状态与索引

统一 StateStore 抽象承载窗口、聚合、表、Named Window、Pattern 和 Context 状态。首版提供内存实现，并满足：

- 明确 key、partition、row/event identity。
- 哈希、范围、复合、唯一和空间索引。
- 原子更新和一致的读快照。
- 到期、删除、卸载和 context 终止时可验证回收。
- 可观测的状态量、索引命中、调度项和泄漏检测。
- 实现源码已公开的 serde/state-management 契约及版本化边界；持久化介质、分布式状态和高可用不因该抽象自动成为 v1 承诺。

### 7.5 并发策略

并发语义先于性能实现：

- 规则构造与 Plan 是并发只读的。
- Engine、Deployment、Statement、StateStore 分别声明并发安全范围。
- 同 key/partition 使用串行邮箱或细粒度锁，禁止依赖 map 遍历顺序。
- 回调默认不持有引擎核心锁；需要保序时使用事件序号和输出队列。
- 停止、卸载、销毁采用幂等状态机。
- 多变量写入、Table/Named Window 触发操作和 rollout 明确事务边界；失败不得留下部分可见状态。
- safe iterator、prepared query、subscription 和 connector handle 都必须可关闭，定义关闭与并发 Send/Undeploy 的 happens-before 关系。
- 所有并发功能必须在 go test -race 下通过，并包含重复压力运行。

## 8. 必须先固化的语义决策

### 8.1 Null、Missing 与三值逻辑

- Missing 表示字段不存在，Null 表示字段存在但无值。
- 布尔表达式遵循 Esper/SQL 式三值逻辑，而不是直接使用 Go bool 零值。
- 聚合对 null 的计数、忽略和输出行为逐函数对照。
- 动态属性、map、JSON、Avro、XML 使用同一内部语义。

### 8.2 数值

建立 Java 到 Go 的精确数值矩阵，覆盖 byte/short/int/long、float/double、BigInteger、BigDecimal：

- 输入 Schema 类型和表达式提升规则固定。
- 溢出、除零、NaN、正负无穷和比较顺序有明确测试。
- 整数除法、除零返回 null 和 MathContext 逐配置对照。
- decimal 与 big integer 选择稳定依赖或内部接口，不允许不同连接器自行转换；先做精度、舍入、性能、许可证和跨平台 spike。

### 8.3 时间

- 区分处理时间、事件字段时间和外部控制时钟。
- 内部统一精度至少达到 Java 回归要求的微秒级。
- duration、calendar period、时区、DST、crontab 和 timer-at 分开建模。
- 测试一律优先虚拟时钟，不使用 sleep 推进语义。

### 8.4 顺序与输出

- 每个算子定义稳定顺序；无顺序保证的结果在差分层按集合规范化。
- 新流和旧流分别记录，不可只比较最终值。
- 输出批次边界、listener 调用次数、同一批次内顺序均是兼容行为。
- map、索引和并发实现不能改变对外承诺的顺序。
- event-precedence、route 优先级、output after/when/cron 和 grouped delivery 必须进入同一顺序模型。

### 8.5 字符串、正则与排序

- Java 正则能力与 Go 标准库 RE2 不完全相同，尤其是 lookaround、backreference 等；阶段 0 通过源测试清单确定实际用法，再选择兼容引擎或批准差异。
- 若引入回溯正则引擎，必须设置输入、步数/时间和内存上限，防止 ReDoS；不能为了语法兼容取消资源边界。
- 固化 Java UTF-16 与 Go UTF-8/rune 在长度、索引、substring、大小写和 Unicode 边界上的对应规则。
- Java Collator、locale 排序和 sort-collator 配置需要可替换 collation 接口及固定版本数据，不能直接用 Go 字节序冒充。

### 8.6 错误

Go 错误文本不要求与 Java 完全相同，但必须映射到稳定类别：

- InvalidRule、TypeMismatch、UnknownName、Dependency、Deployment、State、Timeout、Canceled、Connector、Internal。
- 差分测试比较阶段、类别、规则节点和关键字段；仅对明确稳定的信息比较文本。
- panic 仅代表内部不变量破坏，必须在引擎边界转为带堆栈的 Internal 错误或按策略终止。

### 8.7 扩展、副作用与求值顺序

- UDF、聚合、数据流 operator 和 sink 必须声明是否确定、线程安全、可重入。
- 影响优化的纯度信息不可由引擎猜测。
- 脚本提供者必须支持取消、资源上限和隔离；脚本异常不得破坏引擎状态。
- UDF cache、declared expression cache、subselect pre-evaluation 和优化器改写不得改变有文档保证的调用次数与副作用顺序。

### 8.8 Oracle 与批准差异

Java 9.0.0 是兼容基准，但可能包含已知缺陷。发现 Java 行为可疑时不得静默“修正”或把 golden 改成 Go 行为：先保留复现场景，再选择“严格 parity”或“有意修正”，记录 ADR、兼容影响、迁移说明和两侧测试。approved-difference 是受审例外，不是跳过测试的别名。

### 8.9 首批 ADR

阶段 0/1 至少产出：

| ADR | 主题 |
|---|---|
| ADR-001 | GPLv2/商业许可、版权和派生代码发布方式 |
| ADR-002 | “完整移植”与 EPL 文本兼容边界 |
| ADR-003 | Go module 路径、最低 Go 版本和平台矩阵 |
| ADR-004 | Schema、Value、Null/Missing 和数值模型 |
| ADR-005 | 链式 API、泛型边界和动态事件 API |
| ADR-006 | Clock、Scheduler、时间精度和时区 |
| ADR-007 | Engine 并发、顺序、背压和回调模型 |
| ADR-008 | Plan 工件、函数注册和版本化序列化 |
| ADR-009 | StateStore、索引和资源回收 |
| ADR-010 | 脚本、插件、JMS 等 Java 专属能力的等价方案 |
| ADR-011 | 正则、Unicode、collation、decimal/big integer 兼容依赖 |
| ADR-012 | JSON/XML/XSD/XPath/Avro 实现与安全边界 |
| ADR-013 | API SemVer、Plan/State 格式版本和升级矩阵 |
| ADR-014 | 资源配额、非可信输入、连接器与扩展威胁模型 |

## 9. 分阶段实施

不在本规划中给出虚假的固定工期。Esper 是成熟的大型引擎，完整重写属于多人员年项目。阶段 1 垂直切片结束后，根据真实的“每个回归执行迁移成本”和性能数据再形成排期。

### 阶段 0：治理、基线与清单

工作：

- 冻结 Esper 9.0.0 commit，不在首版中跟随上游升级。
- 完成包含 Git 历史、测试 fixture 和依赖许可证的评审，恢复目标仓库所需 LICENSE、版权和来源说明。
- 对固定 Java RegressionRunner 加运行时清单探针，生成参数化回归执行、配置 profile 和完整功能标签；再与静态扫描双向核对。
- 基于 `compat/source-test-manifest.json` 逐项盘点 371 个核心单元测试、58 个 EsperIO 测试、34 个示例测试文件和 17 个示例项目；regression-run 的 82 个入口源文件由回归 case manifest 关联，所有条目记录 Go 对应测试或 N 级理由。
- 在受控 Java 17 环境运行基线，记录通过、失败、环境依赖和耗时。
- 建立 Java oracle runner、规范化输出协议和共享场景格式的设计。
- 完成 decimal、Unicode collation、Java-compatible regexp、XSD/XPath、Avro 五类依赖 spike，优先纯 Go、core 无 CGO 的候选方案。
- 对非可信 Plan/State、动态事件、XML、脚本/UDF、SQL 和网络连接器完成初版 threat model 与资源配额设计。
- 固化首批 ADR、Go module、Go API/Plan 版本原则、CI 平台矩阵、工具链 pin 和代码规范。

退出条件：

- 清单能追踪到每个实际执行实例、入口套件、配置 profile、源文件、单元测试和示例；静态与运行时枚举差异为零或有评审记录。
- Java 基线可重复；外部 DB、Kafka 等测试有容器化方案或明确环境标签。
- Java vendor/版本、OS/架构、locale、timezone、charset、随机 seed、JVM 参数和外部服务版本已固定或写入每次结果元数据。
- 没有未决的许可阻断项。
- 关键依赖、安全边界和 Windows/Linux/macOS、amd64/arm64 候选支持矩阵已有结论。
- 功能看板初始状态完整，不能只有大类。

### 阶段 1：API 与端到端垂直切片

范围：

- Engine/Environment/Plan/Deployment 的最小生命周期。
- struct、map、JSON 基本 Schema。
- 字段、常量、比较、逻辑表达式。
- Filter、长度窗口、时间窗口、Projection、Listener/Sink。
- 外部虚拟时钟、同步 Send、基本统计。
- 一组代表性 Java/Go 差分场景。
- 第 6.4 节七类高级域 builder-only 原型和类型形变方案对比。

目的不是先交付“简化版 Esper”，而是验证 API、AST、编译和运行时边界是否能扩展到完整范围。

退出条件：

- 从链式规则到部署、发送、时间推进、输出和卸载全链路可运行。
- 无 EPL 字符串回解析。
- 基础垂直切片及 Join、Pattern、Aggregate/Projection、Table Merge、Context、Dataflow、动态格式 builder 都完成 API 评审。
- 差分框架能同时比较 new/old stream、类型、批次和时钟行为。
- ADR-003 至 ADR-008 的基础决策冻结；未完成运行时 parity 的高级 API 保留 experimental。

### 阶段 2：事件、表达式、过滤和完整 View

范围：

- 完整事件属性模型和 struct/map/Row/JSON。
- 表达式、类型提升、null、日期时间、完整 enum method、UDF、正则、Unicode/collation。
- Filter Service 与全部窗口/view。
- previous/prior、insert/remove stream、属性解析风格、group reclaim 和 `leaving()`。

对应 Java 套件：event 的基础部分、expr、view，以及 client basic/compile 的相关项。

退出条件：对应兼容清单 100% 处理，适用差分场景全部通过；核心包 statement coverage 达阶段门禁。

### 阶段 3：结果集、聚合和输出控制

范围：

- 所有内建聚合与聚合扩展。
- group by、rollup/cube/grouping sets、having。
- 结果集形态、排序、limit、output first/last/all/snapshot/condition。
- source-less/context-only 查询、output after/cron/when/then、event-precedence 和 grouped/discrete delivery。
- into-table 的聚合基础：链式 `AggregateStream.IntoTable`、plain/维度 group-by 目标表映射、subtotal Null 列、窗口增删同步和原子 `Table.Replace`；基础 plugin/Count-Min Sketch 已有 Go 切片，context/属性链等组合继续后置。

对应 Java 套件：resultset、expr aggregation、部分 infra table。

### 阶段 4：Join、Subquery、历史流和空间索引

范围：

- 全部连接类型和多流查询计划。
- 子查询、Named Window/Table lookup。
- database/sql 历史流、方法流、参数键缓存和过期策略。
- SQL 方言占位符、metadata/列名/null/自定义转换、连接与 prepared-query 生命周期。
- 哈希、范围、复合、唯一、空间索引及计划说明。

对应 Java 套件：epl/join、subselect、database、fromclausemethod、spatial。

### 阶段 5：Pattern、Timer 与 Match Recognize

范围：

- Pattern 全部操作符、guard、observer、消费和状态限制。
- timer-at、schedule、时区、微秒级时间。
- Match Recognize NFA、interval、after、贪婪、重复、聚合。

对应 Java 套件：pattern、rowrecog 及相关性能/无效输入测试。

### 阶段 6：Named Window、Table、变量与即席查询

范围：

- Named Window、Table、索引、迭代和一致性。
- on-trigger select/delete/update/set/merge、split stream。
- Table aggregation reset 和 FAF multi-row insert。
- 变量和上下文变量的一致快照及原子批量更新。
- Fire-and-Forget 查询、准备查询、named/indexed 参数化、关闭和并发访问。

对应 Java 套件：infra 全部、epl/insertinto、variable、client FAF。

### 阶段 7：Context、模块、部署、Stage 与完整运行时

范围：

- 全部 Context 类型、selector 和生命周期。
- Context 管理查询、分区 descriptor/properties 和 listener 枚举。
- 模块 metadata/order/uses/imports、PathCache、作用域、依赖、rollout、原子部署/卸载。
- recompile/upgrade、部署重定义/版本检查、锁策略/超时和失败回滚。
- Stage/unstage、多租户、Event Type Service、listener/subscriber、unmatched、safe iterator。
- 系统时钟模式、运行时状态、完整配置矩阵、优先级和 route/event-precedence。
- multithread 全套语义。

对应 Java 套件：context、client、multithread。

### 阶段 8：Dataflow、扩展、Serde 与剩余事件格式

范围：

- Dataflow 服务、明确列出的内建 operators、saved instance/configuration 和生命周期统计。
- 扩展点、审计、instrumentation、metrics、异常/条件处理。
- Avro、XML/XSD、Variant、完整 JSON、事件渲染。
- Plan/State serde 和版本兼容策略。

对应 Java 套件：epl/dataflow、client/extension/instrument、event/avro/xml/variant/render。

### 阶段 9：EsperIO、示例和生态

范围：

- 七类 EsperIO 的 Go 等价实现。
- 统一 adapter 状态机、逐连接器 delivery contract、外部系统集成、容器化测试、错误恢复、drain 和背压。
- 17 个示例场景的 Go 化。
- API 文档、迁移指南、运维和诊断指南。

### 阶段 10：全量收口与发布

范围：

- 全清单差分、race、fuzz、泄漏、长期稳定性和跨平台。
- 性能剖析与物理算子优化。
- 公共 API SemVer 检查、示例编译、v1 冻结，以及 Plan/State 前后向兼容矩阵。
- 可复现构建、工具链/容器 pin、产物校验和和支持平台验证。
- 安全评审、资源配额、漏洞/依赖许可扫描、SBOM、发布包和升级说明。

退出条件见第 12 节的发布 Definition of Done。

### 阶段依赖与排期口径

- 阶段 0 和阶段 1 是大规模实现的串行门；阶段 2 提供类型、表达式、Filter、Window 基础。
- 阶段 2 稳定后，结果集/Join/Pattern 可由不同工作流并行，但共享 Scheduler、State、Value 和差分协议的变更必须统一评审。
- Named Window/Table 依赖状态与索引，Context/Deployment 依赖模块和生命周期，Dataflow/连接器依赖运行时关闭、错误和背压契约；不能仅按目录并行。
- Parity 测试、文档、安全和 benchmark 贯穿每个阶段，不设置“最后补测试”的独立阶段。
- 排期使用校准后的 parity point，而不是 Java 文件数：按语义分支、状态性/时间性、并发、外部依赖、fixture 复杂度和性能门禁加权，并记录每个 point 的实际迁移速率。

## 10. Java 对照测试方案

### 10.1 三重门禁

测试完整性由三个独立指标组成：

1. 功能对照覆盖率：Java 回归清单中已处理项/适用项，发布要求 100%。
2. 源测试处置覆盖率：371 个核心单元测试文件、58 个 EsperIO 测试文件、34 个示例测试文件和 17 个示例项目均有 Go 测试映射、合并映射或经评审的 N 级理由；regression-run 的 82 个入口由回归 case manifest 覆盖，发布要求 100%。
3. Go 语句覆盖率：衡量 Go 实现被测试程度，不能代替前两项。

### 10.2 兼容性清单

当前已建立版本化、机器可读的 `compat/capability-manifest.json`（schema 由 `compat/capability.go` 校验），包含 Capability、Case 两类实体及其多对多关联。它只登记当前已复核的首批映射、基础 rowrecog 映射和明确的 EsperIO/Serde 规划项，不代表 4,140 条 Java execution 已全部映射；新增能力必须先补 manifest，再进入 parity 看板。

Capability 条目至少包含：

- 稳定 capability_id、父功能域、S/G/C/N 等级和 Java 9.0.0 契约摘要。
- Java 公共 API/SODA/config/模块位置，以及适用的 regression/unit/EsperIO/example 证据。
- 目标 Go Builder/AST/运行时/配置/连接器位置、依赖 capability、计划阶段和 owner。
- 验收维度、适用平台、状态、批准差异和 ADR。

Case 条目至少包含：

- 稳定 parity_id。
- Java JUnit 入口、配置 profile、完整外部/内部类名、`RegressionExecution.name()`、同名 ordinal/参数、源码文件和 commit。
- 功能域和标签：correctness、invalid、performance、multithread、serde、dataflow 等。
- 原 Java EPL/SODA 场景引用。
- 对应 Go Builder 测试。
- 输入 fixture、时钟脚本和外部依赖。
- 需要比较的输出维度。
- 状态：unmapped、mapped、passing、blocked、approved-difference。
- 差异原因、ADR 和评审人。

运行时探针必须记录实际构造的 execution，而不是只 grep `implements RegressionExecution`。WConfig 及其他配置变体是不同 manifest case；稳定 ID 不得只使用可能重名的 simple class/name。清单必须完整保留 Java RegressionFlag：EXCLUDEWHENINSTRUMENTED、MULTITHREADED、FIREANDFORGET、INVALIDITY、PERFORMANCE、STATICHOOK、SERDEREQUIRED、OBSERVEROPS、RUNTIMEOPS、COMPILEROPS、ENUMHASHCODEPROCESSDEPENDENT、DATAFLOW、EVENTSENDER，并为每个 flag 定义 Go 执行环境、比较策略和 CI 分组。

模块单元测试、EsperIO 测试和示例进入同一数据库的 source-test 条目，记录 one-to-one、many-to-one、replacement 或 N disposition。当前 `compat/source-test-manifest.json` 只有静态发现字段和分类计数，不能被当作 disposition 已完成；后续必须在不改变稳定 ID 的前提下补充映射表或扩展 schema。many-to-one 必须列出覆盖断言，避免用一个宽泛 Go 测试虚假吞并多个 Java 测试。每个 S/G capability 至少关联一个正向和一个适用的边界/无效测试；每个适用 case 必须关联 capability，防止“有功能无测试”和“有测试无范围归属”。

CI 分别报告 capability coverage、source-test disposition coverage 和 case passing coverage，并禁止无理由删除清单项；源基线变化时清单生成器必须给出差异。

### 10.3 差分测试协议

每个可差分场景执行：

1. 将事件注册、部署、发送、路由、时间推进、变量更新、即席查询和卸载动作表示为版本化中立场景。
2. Java runner 使用原 EPL/SODA 执行 Esper 9.0.0。
3. Go runner 使用对应链式 Builder 构造规则并执行。
4. 两侧输出为规范化记录。
5. 比较结果并输出首个语义差异、计划摘要和事件轨迹。

规范化记录至少包括：

- 事件序号、逻辑时间、statement/deployment/context/partition；随机生成的运行时 ID 通过场景逻辑别名关联，仅在契约要求时比较原值。
- listener 调用批次。
- new events 与 old events。
- 字段名、类型、值、Missing/Null。
- 有序或无序比较标记。
- iterator、Table、Named Window、变量和 FAF 快照。
- 部署/卸载/context/stage 生命周期事件。
- 错误阶段、错误类别和关键上下文。
- underlying/fragment/event-type 元数据、事件 identity（适用时）和 property descriptor。
- callback 次数/顺序、下一调度时间、依赖 provided/consumed 和运行时生命周期状态。
- Filter/Join/Index/Pattern 计划的语义结构不变量；不要求 Java 与 Go 物理类名或内部节点一一相同。

浮点、decimal、时间、XML、Avro、map 顺序和异常文本必须使用统一规范化规则，不能在单个测试里临时放宽。

中立场景不能假设所有载荷都可 JSON 化。Java Bean identity、XML DOM、Avro、typed struct/class、UDF、script 和扩展 fixture 通过版本化 typed codec 与两侧 host setup hook 构造；hook 只能准备宿主对象/扩展，不能绕过被测语义。STATICHOOK、filter/index/query planner 测试主要比较语义结构不变量，单看输出不足以证明覆盖。

### 10.4 测试层次

| 层次 | 目标 |
|---|---|
| 单元测试 | Value、Schema、表达式、索引、调度器、状态机和各算子边界 |
| 组合测试 | 多算子计划、窗口+聚合、Join+Subquery、Pattern+Context 等 |
| Java/Go 差分 | 证明 Esper 语义对等 |
| 公共 API 契约 | 编译失败、零值、取消、并发、关闭、错误分类 |
| Plan/格式契约 | AST 规范化、稳定 hash、优化开关等价、Plan/State 版本读写/升级/拒绝 |
| Race/并发压力 | 所有声明并发安全的 API 和 56 个 multithread 功能域 |
| Fuzz/属性测试 | 表达式、Schema、序列化、窗口边界、Pattern 状态和索引 |
| 变异测试 | 对 Value/表达式/调度/状态/索引等关键包抽样，证明断言能杀死典型逻辑变异 |
| 性能基准 | filter、window、aggregate、join、pattern、table、FAF、connector |
| 长稳/泄漏 | timer、context、deploy/undeploy、listener、连接器重连 |
| 示例/E2E | 真实业务链路和 API 可用性 |
| 安全/鲁棒性 | 畸形 Plan/State、深层 JSON/XML/Avro、正则、SQL 参数、脚本超时、队列和配额 |

### 10.5 Go 覆盖率门禁

候选门禁在阶段 1 用实际数据校准，但不得低于：

- 核心语义包整体 statement coverage 85%。
- 公共 Builder、编译验证、Value/Schema、Clock/Scheduler、State/Index 90%。
- 全部第一方非生成代码整体 80%。
- 新增或修改代码 diff coverage 90%。
- 连接器代码 75%，并必须有容器化集成测试覆盖关键成功和失败路径。
- panic、错误恢复、取消和资源关闭路径要有显式用例，不能靠行覆盖推断。

覆盖率使用跨包 cover profile 统一合并，明确排除生成代码、Java oracle 和测试 harness；排除列表需要代码评审。Go 原生 coverprofile 是语句覆盖率，不应在报告中误称为分支覆盖率。关键分支通过表驱动和变异测试抽查，全部公共 example 必须参加编译测试。

### 10.6 Golden、差异与不稳定测试治理

- Java oracle 结果和共享 golden 只能通过可审查变更更新，禁止失败后自动 re-record。
- 发现 Java 缺陷按第 8.8 节处理；测试必须同时保留原行为证据和批准差异断言。
- flaky case 可以隔离诊断但不得计为 passing；随机 seed、事件轨迹、调度选择和环境版本必须写入失败产物。
- retry 只用于收集诊断和估算不稳定率，不能把“重试后通过”当作 CI 成功。
- PERFORMANCE 用例比较吞吐/延迟/分配的统计分布与预算，不录制逐事件 golden。
- MULTITHREADED 用例比较 Esper 明确保证的原子性、可见性、有序性和无丢失/重复等不变量，不比较未承诺的 goroutine 调度顺序。
- ENUMHASHCODEPROCESSDEPENDENT 等进程相关输出只能按源测试意图规范化，不能全局忽略排序。

### 10.7 CI 分层

每次提交：

- gofmt 检查、go vet、staticcheck、依赖和许可证检查。
- 单元测试、受影响兼容测试、API 合约测试。
- go test -race 覆盖受影响的并发包。
- 覆盖率和 manifest 完整性门禁。

每日：

- 全量 Go 测试。
- 全量或分片 Java/Go 差分。
- 完整 race 分片、重复并发测试。
- DB/Kafka/AMQP/HTTP/Socket 容器集成测试；镜像用 digest 固定，并记录协议端和驱动版本。

每周或发布候选：

- 长时间 fuzz、长稳和泄漏测试。
- 全平台/架构矩阵。
- Java 与 Go 同机性能基准、历史趋势和回归分析。
- Plan/State 跨版本兼容测试。

### 10.8 性能验收

阶段 0 先固定硬件、JVM 参数、Go 参数、数据集、预热和统计方法。阶段 1 建立基线，不提前写一个缺乏依据的单一吞吐数字。

发布至少满足：

- 无随事件数无界增长的非业务状态。
- 无 timer/context/deployment/listener 泄漏。
- 核心场景吞吐、p50/p95/p99 延迟、分配和峰值内存都有预算。
- 相对上一 Go 基线无未经批准的显著回退。
- 与 Java 的差距按场景解释；正确性不得为了追平吞吐而放宽。

## 11. 交付物与仓库规划

建议逐步形成：

| 路径/产物 | 内容 |
|---|---|
| docs/architecture | 架构、语义和 ADR |
| docs/compatibility | Java→Go 功能矩阵和已批准差异 |
| docs/security | threat model、资源配额、安全使用和响应流程 |
| compat/manifest | 机器可读回归执行、源测试、连接器测试和示例映射 |
| testdata/parity | 中立场景、输入、golden 和规范化规则 |
| tools/java-oracle | 固定 Esper 9.0.0 的 Java 测试预言机 |
| cmd/parity | 差分运行和报告工具 |
| 根包及公共子包 | 链式 API 和公共运行时契约 |
| internal/compiler | 分析、规范化、优化、物理计划 |
| internal/runtime | 分派、调度、算子、状态和生命周期 |
| connectors | 可独立依赖和发布的连接器 |
| examples | Go 化业务示例 |
| benchmarks | 固定数据集、配置和结果格式 |
| schemas | 中立场景、Plan、State 和诊断产物的版本化 Schema |
| build/release | 工具链/容器 pin、可复现构建、SBOM 和校验和流程 |

实际目录在 ADR-003 后确定。目标是控制 import 环，避免 common 式超级包，也避免每个 Java 子包变成一个 Go 包。

## 12. Definition of Done

### 12.1 单项功能完成

一项功能只有同时满足以下条件才算完成：

- 行为契约和 Java 参考位置已记录。
- Go 公共 API/内部接口经过评审。
- 正常、边界、无效、时间、null 和资源释放测试齐全。
- 对应 Java regression 项全部映射并通过或有批准差异。
- race、覆盖率和静态检查通过。
- 文档和最小示例同步完成。
- 性能敏感项有 benchmark，状态项有泄漏检查。
- 接收非可信输入或运行扩展的功能有配额、取消、畸形输入和安全负例。

### 12.2 阶段完成

- 本阶段 manifest 没有 unmapped 或无理由 blocked。
- 本阶段所有 S/G 功能为 Passed。
- 没有新增未决高风险 ADR。
- 全量既有测试不回退。
- 阶段演示使用公共 API，不调用 internal 包或测试后门。
- quarantine/flaky、approved-difference 和 N 级条目均不被误计为普通 passing。

### 12.3 v1 发布完成

- Esper 9.0.0 平台无关语义清单 100% 处理，所有适用项通过。
- Java 回归实际执行、核心单元测试、EsperIO 测试和 17 个示例的处置覆盖率均为 100%。
- 所有 approved-difference 都有 Go 等价能力、测试和迁移说明。
- 所有回归套件、单元、差分、race、fuzz、长稳、连接器和示例门禁通过。
- 达到第 10.5 节覆盖率底线。
- 无已知 P0/P1 正确性、竞态、数据丢失、状态泄漏或部署一致性问题。
- API、Plan 格式、错误分类和支持平台已冻结并文档化。
- Go API 遵循 SemVer；experimental 包不伪装为稳定契约；Plan/State 对当前及声明支持的历史格式完成读写/拒绝矩阵测试。
- Windows/Linux/macOS 与 amd64/arm64 的最终支持组合均有 CI 证据，未支持组合明确列出。
- 发布构建可复现，Go/Java/容器/依赖版本固定，产物带校验和。
- 无未处置的发布阻断级安全问题，资源上限和安全默认值有压力/滥用测试。
- 许可证、版权、第三方依赖、漏洞扫描、SBOM 和发布材料评审完成。

“核心差不多可用”“大多数 Java 用例通过”或“Go 覆盖率很高”都不等于全量移植完成。

## 13. 安全、依赖与版本治理

### 13.1 威胁面和强制控制

在开放外部输入、Plan 加载或扩展执行前完成 threat model。至少覆盖：

| 威胁面 | 强制控制与测试 |
|---|---|
| 高基数 key、Pattern/Match 状态爆炸 | Context/window/state/匹配数配额、prevent-start、逐租户指标、回收和过载错误 |
| 入站事件与调度洪泛 | 事件大小/深度、队列、timer、批次和并发上限；背压、限流、健康/就绪信号 |
| Plan/State/serde 反序列化 | 版本、Schema、长度、checksum、能力 allowlist；不解析任意函数指针或触发代码 |
| JSON/XML/XSD/XPath/Avro | 深度/大小/复杂度限制；XXE/外部实体关闭；Schema bomb 和畸形输入 fuzz |
| regexp/like | 输入和执行预算；若使用回溯引擎，设置取消/超时并测试 ReDoS |
| SQL 历史流与 DB sink | 强制参数绑定、方言 adapter、凭据隔离、事务边界、取消和连接池上限 |
| Script/UDF/自定义 operator | panic 隔离、context 取消、时间/内存预算、并发声明；非可信脚本默认禁用或进程隔离 |
| 网络连接器 | TLS/认证、secret redaction、有界重试/退避/队列、防 retry storm、明确交付语义 |
| 停止与故障恢复 | graceful drain、超时后强制关闭策略、幂等 close、部分部署/rollout 回滚测试 |

默认配置以安全有界为原则。需要解除限制的选项必须显式开启、可观测并记录风险，不能让一个事件无限创建状态、goroutine、timer 或回调积压。

### 13.2 依赖、平台和版本

- core 优先纯 Go、无 CGO；若 decimal、collation、regexp、XSD/XPath 或 Avro 需要第三方实现，先隔离在小接口后，再按正确性、性能、维护性、许可证、安全和跨平台评审。
- 依赖和容器必须固定版本/digest，持续执行漏洞与许可证扫描；发布生成 SBOM。
- Go module/API 使用 SemVer；experimental API 明确标记并允许在 v1 前调整。
- Plan Schema、State Schema 和中立 parity 场景各自独立版本化，定义 forward/backward read、write、upgrade 和 reject 矩阵，禁止用 Go struct 的偶然编码作为持久格式。
- 固定 Go 工具链、构建标签、时区/Unicode/collation 数据和生成器版本；发布产物可复现并带校验和。
- 平台策略在 ADR-003 明确 Windows/Linux/macOS、amd64/arm64 和 CGO 状态；跨平台差异只能作为批准差异，不能靠跳过测试隐藏。
- 上游 Esper 版本升级与 v1 parity 分离，采用新的基线清单、格式升级和兼容评审，不在当前范围中滚动追新。

## 14. 风险与控制

| 风险 | 影响 | 控制 |
|---|---|---|
| GPLv2/商业许可不清 | 代码无法按预期方式发布 | 阶段 0 法务门禁；保留来源、版权和许可证 |
| 目标 Git 历史仍含 GPL Java 源码 | 误判为空仓库、发布模式错误 | 审查完整历史与派生 fixture；必要时依法净室重建仓库 |
| 范围被低估 | 长期延期、后期返工 | 自动化功能清单；按回归执行计量，不按类数估算 |
| 静态扫描漏掉动态/参数化场景 | 虚假的 100% parity | Java runner 运行时枚举并与静态清单双向核对 |
| EPL 被去掉后语义遗漏 | API 好看但能力不全 | Java EPL 仍作为 oracle；每项语义必须有 Builder 表达 |
| Go 方法级泛型受限 | 类型变化链无法保持既流畅又安全 | 七类 builder 原型比较顶层组合器、Row 过渡与生成描述符 |
| 过早冻结 API | 后期 Table/Context/Pattern 无法自然表达 | 基础垂直切片和高级域 builder 原型后仅冻结 provisional v0 |
| Go 动态类型与 Java null 差异 | 隐蔽结果错误 | tagged Value、数值矩阵和差分边界测试 |
| regexp/Unicode/collation/decimal 差异 | 边界结果悄然不兼容 | 前置依赖 spike、ADR、固定数据版本和专门差分集 |
| 时间/时区/DST 差异 | Pattern/窗口不稳定 | 注入时钟、固定时区库行为、虚拟时间测试 |
| map/goroutine 导致非确定性 | 差分和生产结果漂移 | 稳定序号、分区串行语义、重复测试和 race |
| 状态/索引内存膨胀 | 长稳失败 | 统一 StateStore、状态指标、泄漏和长期压测 |
| 任意闭包阻碍优化和序列化 | Plan 不可分析 | AST 优先；UDF 必须注册和声明元数据 |
| Go plugin 跨平台限制 | 扩展不可部署 | 编译期注册或进程外 RPC，不依赖标准 Go plugin |
| JMS 无 Go 原生生态 | 无法逐 API 复制 | 通用消息契约 + 明确的 JVM bridge 互通测试 |
| Java 测试依赖外部环境 | 基线不稳定 | 容器化、环境标签、固定数据与可重试诊断 |
| Java oracle 本身有缺陷 | 把已知 bug 永久复制或静默修正 | 双行为复现、ADR 和 approved-difference 评审 |
| 非可信输入导致状态/解析/扩展 DoS | 引擎失稳或安全事件 | threat model、有界默认值、fuzz、取消、隔离和压力门禁 |
| 性能优化改变语义 | 正确性回退 | 优化前后同一差分集；物理计划可关闭/对比 |
| 上游版本漂移 | 范围持续变化 | v1 固定 9.0.0；发布后再做独立升级计划 |

## 15. 规模与组织建议

该项目不是普通库重写。仅核心 Java 主代码就超过六千个文件量级，且有约 3,848 个回归执行实现。建议至少分为四条持续工作流：

1. API/编译器：Builder、AST、类型、验证、计划和扩展契约。
2. Runtime/State：调度、算子、状态、索引、部署、Context 和并发。
3. Parity/Quality：Java oracle、场景迁移、差分、fuzz、race、性能和看板。
4. Integration/Docs：事件格式、连接器、示例、运维和迁移文档。
5. Release/Security：依赖与许可证、threat model、资源配额、平台/格式兼容和可复现发布；可由横向 owner 兼任，但职责不可缺席。

每个功能由实现人员与 parity 人员共同验收，不能让测试迁移成为最后一个独立大阶段。建议阶段 1 后按实际迁移速度估算人月；在此之前只承诺阶段退出条件，不承诺发布日期。

## 16. 实施启动顺序

规划获批后，建议严格按以下顺序启动：

1. 确认 GPLv2/商业许可与目标仓库发布模式。
2. 确认“不提供 EPL 主入口，但完整实现 EPL 语义”的边界。
3. 固化 Go module、最低 Go 版本、候选平台、core/CGO 和外部依赖原则。
4. 通过 Java 运行时探针 + 静态扫描生成并评审回归、单元、EsperIO 和示例 manifest。
5. 跑通 Java 9.0.0 基线及外部依赖环境，固定 runner/容器版本。
6. 完成 Null/Value、时间、并发、Plan、格式版本、扩展、安全配额和关键依赖 ADR/spike。
7. 完成七类高级域 builder-only 原型，确定 Go 类型形变和动态 Schema API。
8. 实施阶段 1 基础垂直切片并用真实差分场景验证公共形态。
9. 根据阶段 1 的 parity point 实际速率形成团队排期，再进入各功能阶段。

在第 1 至 7 项完成前，不建议大规模并行编写算子，否则最容易在许可证、null、时间、输出批次、锁、依赖和 API 形态上发生系统性返工。

## 17. 二次复核后的补充与当前看板

### 17.1 本轮确认的遗漏门禁

前述范围已经覆盖主要 Esper 语义域，但实施中还必须显式保留以下门禁；这些内容不能用“已有 API 雏形”或“静态文件计数”替代：

1. **许可证与来源材料**：目标 Go 仓库当前没有可直接发布的 LICENSE、NOTICE、第三方依赖清单和 SBOM。任何复制 Java 源码、测试 fixture、示例资源或版权头之前，必须完成 GPLv2/商业许可决策、来源隔离和发布目录审查。
2. **Java oracle 的运行态证据**：静态扫描得到的 3,848 条是候选清单，不是最终执行实例清单。当前已在 Java 17 + Maven 3.9.16 上运行 `tools/java-oracle/run-probe.ps1`，记录动态参数、factory 变体、内部类、重复名称、实际 ordinal 和唯一 `runtimeId`；下一步仍需将这些记录与 JUnit 入口、Go 映射和行为 trace 双向对账。
3. **非 Regression 测试资产**：371 个核心模块单元测试、58 个 EsperIO 测试、34 个示例测试文件和 17 个示例项目不能只写在统计数字里；当前 `compat/source-test-manifest.json` 已固定 726 个 Java 源文件的静态分类，但每个文件/场景仍要补充 one-to-one、many-to-one、replacement 或 N disposition，以及 Go 断言映射。regression-run 的 82 个入口源文件必须与回归 case manifest 关联，不能漏算或重复计数。
4. **状态事务边界**：Variable、Table、Named Window、FAF、Context 分区和 `insert-into/on-trigger` 必须共享明确的版本/快照/提交边界。失败、取消、监听器错误或卸载不能留下半更新状态；不能让各子域各自定义一套不可组合的锁语义。
5. **动态注册与计划依赖**：Schema、Variable、Table、Named Window、Function、Serde、Context、Dataflow operator 和 Connector 的注册表版本必须进入 Plan dependency digest；Engine/Plan 绑定、部署卸载前置条件和 recompile/upgrade 需要单独测试。
6. **可观测性不是附加项**：每个 state/index/timer/partition/connector 都要能报告数量、容量、命中/未命中、积压、关闭状态和 owner；日志必须做 secret/PII 脱敏，并能用 statement、deployment、context、partition 和 parity case 关联诊断。
7. **资源配额与取消传播**：事件深度、递归 route、窗口/聚合基数、Pattern 状态、FAF 返回行数、JSON/XML/Avro 深度、正则预算、连接器重试和 goroutine/timer 数量都要有有界默认值，并测试 context 取消后的回收。
8. **发布工程**：必须补充 CI 矩阵、生成代码校验、API/Plan/State 兼容检查、可复现构建、依赖漏洞/许可证扫描、SBOM、CHANGELOG、迁移指南和回滚方案；Windows 当前环境能编译不等于 Linux/macOS/arm64 已支持。
9. **Plan/trace 保真度**：Builder 的分支顺序、条件、主键、端口、历史触发类型、配置和依赖都必须进入规范化 Plan/trace；必须用“语义不同但结构相近”的规则验证 canonical/hash 不碰撞，并在差分报告中保留分支和状态转移。此次条件式 Table Merge 复核已发现并修正该类遗漏，后续每个新 builder 都要复用这条门禁。
10. **on-trigger 结果语义**：Table 与 Named Window 的 insert/upsert/update/delete/delete-all/conditional-merge 不仅要改变状态，还必须按 Java 约定形成 new/old stream；无匹配 update/delete 不得伪造结果或误报错误，Named Window 的 matched/not-matched、无 `where` 与恒假匹配、保留策略驱逐、结果批次和 consumer dispatch 也要进入差分场景；Table schema、主键数值 coercion 和 listener 边界同样必须覆盖。
11. **Variant 事件契约**：PREDEFINED 必须只暴露成员共有且类型可兼容的属性，ANY 必须保留实际成员 identity 并支持动态属性；成员注册、Route、insert-into/derived stream/wrapper、late schema、supertype/interface coercion、Named Window/Join/Pattern/Dataflow/rowrecog/subquery/FAF 传播、identity/equality、metadata/getter/cache 与 Variant 结果类型都必须单独建 case。当前 Go 已完成成员路由、基础字段解析、Route identity，以及普通流/投影到已注册 Schema 或 ANY Variant 的基础链式 InsertInto，但不能将其记为 Variant 全量完成。
12. **InsertInto/Route 事件管道**：必须验证目标 Schema、new-stream-only 语义、无投影时原始 Event identity/underlying 保留、Row 到普通事件格式和 ANY Variant 的转换、Variant 逻辑流与实际成员类型隔离、稳定派发顺序、循环/递归路由上限、取消、监听器错误、部署卸载和 Dataflow 传播；Join、Aggregate、Pattern、Named Window/FAF 等结果路由必须分别定义支持矩阵，不能因普通流路由通过就默认全量支持。当前 Go 已开放 Join/Aggregate/Pattern 的基础结果路由与对应链式入口，补充 Named Window consumer、remove-stream、输出批次排序、循环上限和显式 FAF 结果路由/参数绑定测试；混合 new/old 顺序、完整事件表示/转换、可配置 precedence/loop policy、Named Window/FAF 全矩阵仍未完成。
13. **矩阵完整性与漏域**：`compat/capability-manifest.json` 当前是首批纵向 capability/case 映射，不是 4,136 个 Java runtime execution、726 个源测试文件的全量处置矩阵；在 manifest 扩展完成前，不能用当前 28 个左右 capability 的 `mapped` 状态代表全量完成。必须另行登记 client configuration、Module/PathCache/Stage、Filter Service、完整 view/statistical view、spatial、script/UDF、serde/render、metrics/instrumentation、multithread、example 和剩余 Event/Expr/Infra/EsperIO 入口，并为每项给出 Go test、replacement 或 N 级理由。

### 17.2 当前工作树的事实状态（2026-08-05）

| 项目 | 当前状态 | 结论 |
|---|---|---|
| Java 基线 | 已固定为 release_9.0.0 / `9e1b9f1cc9117fea4bf33ab043762c045d73839c` | 可作为 oracle 输入 |
| Java 源资产清单 | 已生成 `compat/source-test-manifest.json`：726 个源文件，分类计数已校验；首批 capability/case 处置已登记在 `compat/capability-manifest.json` | 仍只有首批映射；全量 source-test disposition、many-to-one 覆盖断言和 N 级理由尚未完成 |
| Java 运行时枚举 | 已接入 `tools/java-oracle/ExecutionInventory.java` + `run-probe.ps1`；3,848 个静态候选经 801 个外层 suite 枚举为 4,140 条记录，4,136 条 ok、4 条 ignored、0 条 probe error，唯一 `runtimeId` 已校验 | 运行态 inventory 已建立，但 Java 测试体尚未全量执行，JUnit 入口/源测试 disposition/Go 行为 trace 仍未完成 |
| Java regression-run 基线 | 已执行 78 个 suite、860 个 JUnit 入口；857 个无失败/错误，2 个性能/数据库诊断差异，1 个多线程上下文用例首跑波动；官方 SQL 夹具已在 MySQL 8.0.46 Docker 中加载，隔离重跑 `TestSuiteMultithread` 为 41/41；汇总固化于 `compat/java-regression-baseline.json` | 仍需将每个 Java execution 与 Go parity case 一一关联；2 个 approved difference 不能计为普通语义 pass，首跑波动必须保留环境/重跑记录 |
| Go 基础垂直切片 | 已有 Value/Schema/Expr/Filter/View/Clock/Plan/Deploy/Listener/Sink；事件模型新增 ObjectArray 与 Variant PREDEFINED/ANY 成员路由、Route 入口、普通流/RecordStream/Join/Aggregate/Pattern/Named Window 链式 InsertInto/RouteTo、new/remove stream 选择、输出排序后的有界路由队列和循环错误 | 仅代表增量实现，不代表阶段退出 |
| Expr/函数 | 已有字段、变量、常量、比较、逻辑、算术、字符串、少量日期/聚合表达式，以及基于窗口历史的基础 `Prev`/`Prior`、`Leaving`、`Cast`、`Exists`、`TypeName`、`InstanceOf`、`ArrayAt`；新增可分析的枚举/集合表达式：元素/索引/size、where/select/arrayOf、any/all/count、first/last、distinct、take/while/reverse、min/max/order、sum/average、except/intersect/union、sequenceEqual、aggregate、groupBy、toMap、most/least frequent | 类型提升、完整 null/三值矩阵、枚举方法的完整 Java 类型/无效规则校验、BigDecimal/任意集合类型、嵌套/子查询/访问聚合组合、UDF、脚本、正则/Unicode/collation、MathContext 和扩展函数仍未完成 |
| View/Window | 已有 length/time、batch、expression、group、union/intersect、sort/rank、unique/first-unique、time-order、TTL、externally-timed 等基础窗口原型；`Prev`/`Prior` 和 `Leaving` 已接入历史/移除流求值，并有 new/old stream 测试；新增 `Window().Aggregate()` 形式的 size/univariate/weighted-average 组合，以及 `Correlation`/`Correl`、`LinearRegression`/`Linest` 的链式统计表达式，长度窗口淘汰会同步重算 new/old 结果；新增 `UniqueBy`/`FirstUniqueBy` 多键唯一窗口、稳定 key 顺序、Snapshot/历史可见性和嵌套 GroupWindow 递归历史；`RankWindowBy(size, uniqueKeys, sortKeys...)` 已覆盖唯一键替换、满窗口 pass-through、最末事件淘汰、primitive/object/二维数组 key、array-key union/intersection 及 Sort/Rank 同分到达顺序差异 | 完整 view 目录、native derived-view event-type properties、组合窗口的全部边界、rank/sort 其余导航、union/intersection 的跨传播组合、group reclaim、窗口索引复用、批/移除/顺序语义及 Java 对照仍未完成 |
| Join | 已有内/外连接、复合/范围条件、N-way inner/left/right/full outer 基础 tuple 差分、Named Window join、Context-partitioned join 和活动 tuple 差分；新增显式 source-indexed `JoinField`/`JoinEventValue` 及 Join-to-Aggregate 的分组、访问聚合、窗口淘汰和 old/new 重算；FAF Named Window/Table Join 已支持一致快照，Context FAF Join 已支持 key/ID partition selector；对应 builder/运行时测试已加入 | 自连接边界、全量外连接顺序、复合/范围索引计划、outer/self/multi-way Join-to-Aggregate、与 Join/Context 的完整相关子查询组合和全量顺序语义仍未完成；基础子查询单独登记为 `query.subquery-basic` |
| Historical/SQL | 新增 `FromHistorical` 外部拉取源、`HistoricalProvider` 请求契约、SQL `database/sql` prepared-query adapter、SQL 字节/数值/null 基础转换；支持独立历史查询、历史源 Join、历史源 FAF 快照和参数透传；新增参数键 LRU/expiry 缓存、缓存事件副本隔离、列名大小写归一化、调用方可插拔的问号占位符重写与 metadata/type 转换 hook、可选 PreparedStatement/Close、`*sql.Tx` caller-owned queryer、可挂到链式 Query `.To(...)` 的参数化 `SQLSink`（new/old 顺序、取消/关闭、基础 DML/Upsert）、fixture 缓存测试和可选 `ESPER_MYSQL_DSN` 的真实 MySQL Docker 测试；有 fake provider 的触发、Join、参数化 FAF 与 DeployWithParameters 测试 | 完整方言 adapter 矩阵、metadata-origin/SQL 类型绑定策略、连接池事务语义、DML/Upsert 的事务/retry/connector 矩阵、SQL 多源 FAF/Context 约束、方法流和完整 Java database trace 仍未完成；EsperIO DB 另有 `connectors/db` 的 DML/Upsert lifecycle sink 与 MySQL round-trip，但 XML/config、异步 executor、连接工厂和完整 Java trace 仍未完成 |
| Aggregate | 已有 count/sum/avg/min/max、first/last/nth/count-distinct/median/stddev、avedev/variance/stddev-pop/weighted-avg/rate（含常量间隔/过滤与虚拟时钟）、`RateByTimestamp`/数量速率、min-by/max-by/window/set/sorted、`first-ever`/`last-ever`/`count-ever`、group-by/having 和 first/last/snapshot/every output 原型；新增 `FilterAggregate`/`CountIf`/`SumIf`/`AvgIf`/`MinIf`/`MaxIf` 的可分析谓词过滤、带可选过滤器的有状态 `Leaving`、`GroupByRollup`、`GroupByCube`、`GroupByGroupingSets`、subtotal Null 字段、`Grouping`/`GroupingID`、增量 old/new 维度状态和重复定义校验，并覆盖 ROLLUP 在 Fire-and-Forget Named Window 快照上的求值；本轮新增可选索引的 `First`/`Last`/`Nth`、`WindowEvents`/`EventValue`、`MinByEver`/`MaxByEver`、多条件 `SortedEvents`、可分析 `LocalGroupBy`，以及投影别名 `ResultField` 驱动的聚合 order-by/having；继续新增可链式 `SortedAccessBy`（重复 key 桶、first/last、lower/floor/ceiling/higher、submap、计数和事件列表）、`PluginAggregate`/`RegisterAggregatePlugin`/`PluginAggregateRef`、`PluginAggregateWithFactory`/`RegisterAggregatePluginFactory`/`PluginAggregateFactoryRef` Go 扩展（按组独立 state、`Enter`/`Leave`/`Value`/`Clear`）、无参数 Event 方法/属性链、`CountMinSketchAdd` 频率与 total 访问、`TableSink` 聚合落表、链式 `AggregateStream.IntoTable`（plain/rollup/cube/grouping-sets 目标快照、subtotal Null、原子 `Table.Replace`、窗口淘汰/整组移除同步、过滤 scalar/access 列、live FAF side effect 拒绝）、live table snapshot join 与 consumer-local filter/window 处理；新增 `OutputAfterEvents`/`OutputAfterTime`/`OutputAfterCalendar` 激活门、事件计数 every、虚拟时钟 `OutputEveryTime`/`OutputSnapshotEvery`、基础五字段 `CronSchedule`/`OutputAt`（含变量/参数字段、严格下一次触发、日月/周 OR），并补充可选秒/毫秒字段；`OutputAt(..., OutputSnapshot())` 在日历触发点读取普通窗口、分组聚合和 Join 的当前快照，`OutputSnapshotEvery(...)` 按虚拟时钟重建分组聚合当前状态；同时保留可组合的 `OutputWhenWith(OutputSnapshot(), ...)` 当前流快照触发和可缓冲的 `OutputWhen`/then 变量提交，支持基础 count_insert/count_remove/total 与 last-output-timestamp 条件，结果查询已有 distinct/order/limit/offset 原型 | 完整聚合族、math-context/decimal、into-table 的 context/属性链生命周期、table/navigable/plugin 全访问方法与属性链、组合/嵌套 grouping specification、Context/Named Window/FAF/output 全矩阵、table sink 批量原子回滚、Count-Min Sketch 的可配置宽深/碰撞策略与更广组合、cron 的微秒精度/特殊日历运算符、动态重排、时区/DST/无效规则矩阵、完整 when-then 上下文与更多条件函数、snapshot-after 的 Context 生命周期、Named Window/Table/custom-access 的高级组合、grouped/discrete delivery、全局结果窗口语义和精确移除语义仍未完成 |
| Variable | 已有全局注册、引用、类型校验、constant、原子快照；新增链式 `OnEvent(...).SetVariable/SetVariables`，支持按声明顺序求值、重复 assignment 最终提交、数字 coercion 和同事件后续 statement 可见；新增 `RegisterContextVariable`/`RegisterVariableInContext`、按 context+partition key 的共享状态、`GetContextVariable`/`SetContextVariable`/`SetContextVariables`/`SetContextVariablesByID`、全局 `VariableValues`、按 selector 返回 `ContextVariableStates`、一致快照、批量校验后一次性提交、`VariableChangeListener` 的 old/new 事件、作用域校验、on-trigger 更新和 initiated 分区释放重置 | 上下文变量的完整 Java iterator、变量版本/延迟可见性、按 deployment/name 的完整管理服务键模型、subquery/array/object assignment、output-then 与 Java 全量 on-set 差分仍未完成 |
| Table/Named Window/FAF | 已有内存 Table/Named Window、基础索引/保留、消费者和 prepared FAF；新增 `FromTable` 表源、Context-aware FAF selector、FAF Named Window/Table Join、类型化 `Parameter[T]`/`Param[T]`、同名参数类型一致性与执行时类型校验、`ExecuteFireAndForgetWithParameters`/`PreparedQuery.ExecuteWithParameters`/`DeployWithParameters`、切片 `InSlice`、`TableField`/`NamedWindowField` 目标行表达式、Table `UpdateTableWhere`/`DeleteFromTableWhere`/`SelectFromTableWhere` 批量触发、Named Window `InsertIntoNamedWindow`/`UpdateNamedWindow`/`SelectFromNamedWindow`/`DeleteFromNamedWindow`/`DeleteAllFromNamedWindow`/`MergeIntoNamedWindow`（含显式 `MergeIntoNamedWindowWhen`），以及 Table/Named Window mutation new/old 结果批次；Named Window 变更已接入 consumer dispatch，并有条件 Merge 的 matched/not-matched/delete、无 where matched、恒假插入、关闭、取消、生命周期、非法规则和 Plan hash 区分测试；新增 `ExecuteFireAndForgetAndRoute*`/`RouteFireAndForget`，显式区分只读 FAF 与路由副作用，覆盖 named-window/table、投影顺序和参数绑定；新增 `SubqueryExists`/`SubqueryValue`/`SubqueryValueWithOptions`/`SubqueryIn`/`SubqueryCount`/`SubquerySum`/`SubqueryAvg` 与 `SubqueryAny`/`SubquerySome`/`SubqueryAll`，支持 Named Window/Table 快照、相关过滤、嵌套作用域、参数绑定、三值量词、标量排序/offset/limit 和多行 cardinality 选项 | 复杂表访问/聚合列、谓词批量操作的原子回滚/索引计划、split stream、完整索引/迭代/持久化、Named Window/Table on-trigger 的全量 Java 差分、SQL/历史源的多源约束、FAF 全结果路由矩阵、分组/having/多行多列子查询和完整 Java 差分仍未完成 |
| Compiler/Deployment | 已有链式 AST、Build 校验、Plan、Deploy/DeployWithParameters、Undeploy 和依赖摘要原型 | Module/PathCache/uses/imports、rollout 原子性、recompile/upgrade、artifact 恢复、完整配置和部署锁策略仍未完成 |
| Pattern / Match Recognize | Pattern 已有单源 tag/filter、followed-by、every/every-distinct、within、`WithinOrMax`、`While` guard、MaxStates、and/or/not、match-until、until、timer-interval/timer-at/显式 timer-schedule，以及基础五字段和可选秒/毫秒精度 `TimerCron`；新增 `RowVar`/`RowSequence`/`RowAlternation`/`RowPermute`、`Optional`/`ZeroOrMore`/`OneOrMore`/`Repeat`（含有限和无限组合重复）、DEFINE、MEASURES、partition、all/first match、三种 skip、固定 duration/日历 `Interval`、`IntervalOrTerminated`、当前窗口 `Prev` DEFINE、typed `PrevTag`/`PriorTag`、`Statement` `Snapshot`、measure-alias `OrderBy`、MaxStates 基础运行时；并有连续序列、可选/重复、交替、有限排列、重复变量、有限和无限组合重复、reluctant、固定/日历 interval、interval-or-terminated、measure alias 排序、当前窗口 Prev、tag-aware previous field、skip 状态快照、长度窗口淘汰、Named Window 乱序删除重算、分区和校验测试；本轮新增 Java `RowRecogDataSet` 的 `A B C* D E* F+` 链式场景，逐事件对照 E7/E8/E9 的 1/3/5 条 listener 输出，并修复 `SKIP TO CURRENT ROW` 对重叠旧起点的裁剪；本轮又新增 Java `RowRecogRegex` 全部 12 组链式组合模式，对照 listener 与 Snapshot 的可选、交替、嵌套重复和重复捕获结果；本轮新增 Java `RowRecogVariantStream` 的 PREDEFINED Variant 成员类型场景，使用 typed `InsertInto` 路由和 concrete member type 谓词；本轮再新增 Java `RowRecogDataSet.RowRecogExampleFinancialWPattern` 的 E1–E60 `A W+ X+ Y+ Z+` 链式数据轨迹，覆盖全部终止 listener batches；本轮新增 Java `RowRecogMultikeyWArray` 的数组内容分区与 int/long 多键分区链式对照，验证 nil/空数组和完整 tuple 隔离；本轮补齐 Java `RowRecogAfter` 的 current/next/past-last 三种 skip 及重复变量、分区场景 listener/Snapshot 矩阵，并修复 `SKIP PAST LAST ROW` 快照保留已发出结果、排除被裁剪新起点的语义；本轮新增 Java `RowRecogInterval` 四个 execution 的 simple/partitioned/multiple-completed/month-scoped 虚拟时间链式对照，覆盖 pending Snapshot、绝对 deadline listener、分区 deadline、多起始匹配和日历月边界；Context 新增事件驱动 pattern-init/pattern-term adapter，支持 tag capture、相关 end pattern、Every overlapping、termination snapshot 和纯 TimerAt/TimerInterval/TimerSchedule/TimerCron root Context 生命周期（部署时锚定虚拟时钟） | Pattern cron 微秒/ISO period 与特殊日历运算符、复杂 timer+event Pattern Context observer、完整时区/DST/动态调度矩阵、guard/observer 的完整组合、`WithinOrMax` 在 Until/Not/MatchUntil/多层组合中的传播、消费语义、复杂状态限制；Match Recognize 的高级嵌套 NFA、Variant/ANY/dynamic-member 高级矩阵、advanced tag-aware prev/prior variants 与高级聚合、`RowRecogIntervalResolution` 精确/微秒边界、`IntervalOrTerminated` 全组合与剩余日历 interval termination parity、完整 out-of-sequence delete 矩阵、完整 iterator/order-by 和完整 Java trace 仍未完成 |
| Context | 已有 key/hash/category/initiated-terminated builder、嵌套 segmented context、普通/Named Window/Join/aggregate/trigger 的分区路由、key/ID/category/hash/segmented/nested/descriptor-filter selector、Context-aware FAF aggregate 与 Context FAF Join、生命周期处理，并有 `ContextPartitionKeys`/count 查询；新增 context-partitioned rollup 聚合隔离和 FAF partition selector 测试，以及 `ContextPartitions` descriptor 快照（ID、key、context properties）、共享分区 ID、allocation/deallocation listener、Environment context 的 created/destroyed 管理、statement added/removed、activated/deactivated 通知、key/category/initiated 生命周期、initiated/terminated boundary-event `Property` 链、partition-scoped context variable 注册/读写、全局与分区变量一致快照和原子批量更新；本轮新增 `NewPatternInitiatedTerminatedContext`/`NewPatternInitiatedContext` 及显式 overlapping 变体，复用 PatternStream matcher 支持 start tag 保留、end pattern 关联、`ContextPatternEvent`/`ContextPatternField`、`Every` 并发分区、pattern termination snapshot 和纯 TimerAt/TimerInterval/TimerSchedule/TimerCron root Context 生命周期（部署时锚定虚拟时钟）和 temporal termination snapshot | selector 的完整 Esper 类型校验、Java hash/segmented/nested identifier 细节与非法组合诊断、复杂 timer+event Pattern Context、initiated parent/pattern child nested Context、ContextDeploymentID/RuntimeURI 与完整 Java 事件 payload 对等、终止/启动事件对象矩阵、变量 iterator/listener 与完整管理服务契约、IntoTable context materialization、跨 Context 事务和多租户仍未完成 |
| Dataflow | 已有保存定义、Beacon/Filter/Select/Emitter、EventBus source/sink、EPStatement source subscription、显式 `Connect/ConnectPorts` 分支图、拓扑校验、实例状态、取消和统计；新增 `FinalMarker`/`WindowMarker`/`CustomSignal`、`OnSignal` handler、图内传播、Emitter 观察、运行态 `SubmitSignal`，以及每个实例独立的 `Custom` factory/runtime、可选 Open/Close/OnSignal 生命周期、named emission port routing、实例 ID/user object、in-process saved configuration、结构化 exception handler、fail/continue policy 和 error/drop 统计，并有对应测试 | typed event/row 多端口约束、自定义 source 和完整 factory 参数/错误策略、并发背压、跨进程持久化 saved instance/configuration、完整 captive/join 和 Java trace parity 仍未完成 |
| EsperIO | 已实现 `connectors` 生命周期、CSV/File source/sink、unformatted line source、DB DML/Upsert sink、HTTP client/server source/sink、Socket TCP source、四种 Socket 协议、Kafka Reader/Writer、Kafka JSON processor/commit/retry/timestamp/key/header、AMQP RabbitMQ Consumer/Publisher、queue/exchange/binding、prefetch、JSON/GOB codec、auto/manual ack、reject/requeue、JMS provider-neutral Consumer/Publisher、Map/Text/Object/Bytes codec、Java event-type property、ack-after-process、重试、类型转换、loop/reset、定时回放、Esper Event/Row bridge、HTTP URI/query/JSON/retry/Engine bridge、Socket 并发/暂停/重启/object decoder；Java `esperio-csv` Maven 80/80 通过，`esperio-db` 4 个测试中 Upsert 通过、2 个环境/断言差异已记录，`esperio-http` 4 个测试中 3 个通过、1 个旧订阅/连接环境差异，`esperio-socket` 7/7 通过，`esperio-kafka` 在固定单节点 broker/topic 环境下 6/6 通过，Spring JMS 4 个测试中 1/4 通过且环境差异已记录，Go Kafka/RabbitMQ/JMS bridge round-trip/单测通过，映射已登记 `integration.esperio`/`case.esperio-csv-file`/`case.esperio-db`/`case.esperio-http`/`case.esperio-socket`/`case.esperio-springjms`/`case.esperio-amqp`/`case.esperio-kafka` | `partial`；DB XML/config、异步 executor/connection factory、HTTP XML/classic service/完整异步 delivery、Kafka group/rebalance/custom serializer/plugin/高级恢复、AMQP Java serialization/高级重连/Dataflow graph、JVM JMS provider/session/transaction/Spring XML、Socket XML/plugin/writable-property cache、Dataflow operator/signal-marker、AdapterCoordinator、bean population 和全量 connector delivery/trace 仍未完成 |
| Variant | 已有 `SchemaVariant`、`NewVariantSchema`/`NewAnyVariantSchema`、`RegisterVariant`/`RegisterVariantAny`、`Engine.Route`，PREDEFINED 共有字段/成员校验与 ANY 动态字段；普通源、Join、Pattern、Named Window consumer、Dataflow EventBus source 的成员路由和 Route identity 已有 Go 测试；新增 `Stream.InsertInto`/`RecordStream.InsertInto`、Join/Aggregate/Pattern 链式结果入口、Named Window consumer、`RouteTo`、Plan 目标校验、有界嵌套路由，以及 Event/Row 到普通 Schema 或 ANY Variant 的基础投影转换 | Java 的完整 insert-into/derived stream/wrapper 转换矩阵、完整 Join/Aggregate/Pattern 结果表示路由、混合 new/old 顺序、循环策略、late schema、supertype/interface coercion、完整 ANY property getter/cache、Variant metadata/fragment/identity/equality、Named Window Variant source、rowrecog/subquery/FAF/Serde 和 Java/Go trace parity 仍未完成 |
| JSON/XML/Avro/Serde | 已有基础 JSON、严格基础 XML scalar/path 解析、Avro JSON datum 解析，以及 `SchemaObjectArray`/`RegisterObjectArray`/`SendObjectArray` 的位置字段访问、长度/类型校验、数值 coercion；本片新增 `array[index]`、`mapped('key')`、转义点路径、嵌套 `Property`、JavaBean getter fallback，以及带标题/缩进/深度上限和 XML attribute 选项的 `RenderJSON`/`RenderXML`，覆盖 schema/map/struct/Row/Event/嵌套数组的 Go 单测；随后补充 schema-directed JSON 对 nested struct/map/slice/array、enum-like string、time.Time、big.Int/big.Rat 的递归转换、大小/尾随数据/未知字段选项和精确数值渲染，并对照 Java `EventInfraGetterIndexed`/`EventInfraGetterMapped`/动态 getter、`EventRenderJSON`/`EventRenderXML`、`EventJsonTypingCoreParse`/`CoreWrite` runtime inventory 登记 case | ObjectArray 的命名嵌套事件类型/属性元数据、完整 EXPLICIT accessor registration、继承和 configuration、JSON provided-underlying/class/list adapter 与全量 laxness/动态值矩阵、XSD/XPath/namespace/DOM、二进制 Avro/logical type/schema evolution、provided-underlying/fragment 和 Java 精确格式 trace、Serde 和格式版本矩阵仍未完成；当前新增 case 仍是 Go 单测映射，不是 Java/Go trace parity |
| 全量差分与覆盖率 | Go 单测、vet、race、首批中立场景和结构化 `DiffTraces` 差异报告可运行；本轮新增 ROLLUP/CUBE/GROUPING SETS、subtotal Null/`grouping_id`、filtered aggregate、ever aggregate、stateful leaving、常量/时间戳/数量 rate、indexed/access aggregate、local group-by、聚合 order-by/having、`SortedAccessBy`、`AggregateStream.IntoTable` 快照替换/删除、维度 subtotal IntoTable、过滤 access 列物化、plugin、无参数 Method、Count-Min Sketch、derived view 的 `Correlation`/`LinearRegression`/`UnivariateStatistics`、multi-key `UniqueBy`/`FirstUniqueBy` 与唯一窗口 Snapshot/历史、primitive/object/二维 array-key、Rank/Sort 同分顺序和 TimeOrder 虚拟时钟到期、Dataflow signal/marker、Dataflow instance options/saved configuration/exception policy、子查询 count/sum/avg、any/all/some 量词和标量 order/offset/limit/cardinality、事件 indexed/mapped/nested getter、JSON/XML renderer、typed JSON parse/write、XML tree/attribute/repeated element parse、事件 property metadata/inheritance、Context created/destroyed replay、statement added/removed、activated/deactivated、descriptor/boundary-event/context variable management snapshot/atomic-batch/key-ID-category/hash/segmented/nested/descriptor-filter selector/shared lifecycle listener、EsperIO CSV/File/DB/HTTP/Socket/Kafka/AMQP/JMS bridge source/sink/lifecycle/replay/retry/Engine bridge 和非法定义测试，并定向执行 Java `TestSuiteResultSetQueryType` 17/17、`TestSuiteResultSetOutputLimit` 14/14、`TestSuiteResultSetAggregate` 14/14、`TestSuiteEPLSubselect` 19/19、`TestSuiteView` 27/27（含 ViewRank/ViewSort/ViewTimeOrder/ViewMultikeyWArray 与 `ViewDerived` 12 个 runtime execution）、`TestSuiteContext` 17/17、选定 EventInfra/EventRender/EventJson/EventXML 86/86、`esperio-csv` 80/80、`esperio-http` 3/4、`esperio-socket` 7/7、`esperio-kafka` 6/6；EsperIO AMQP 已完成 Java 编译/外部 broker 诊断，Spring JMS 已完成 Java 4-test 环境诊断，Go RabbitMQ/JMS bridge source/sink round-trip/单测通过；保留基础子查询相关/非相关/嵌套/参数化/FAF 一致快照、显式 FAF 结果路由/参数绑定、历史 SQL、Variant/InsertInto、Join/Aggregate/Pattern/Named Window、new/remove stream、CronSchedule、ExprEnum、output-after/when-then、OutputAt snapshot、Pattern TimerCron 和基础 Match Recognize 测试；本轮已通过 `go test ./...`、`go test -race ./...`、`go vet ./...`、compat 门禁、格式/JSON/diff 检查、MySQL Docker 集成测试和 Kafka/RabbitMQ Docker round-trip；最近一次 `go test ./... -cover` 核心包覆盖率 75.6%，compat 66.9%，connector 各包覆盖率以 coverprofile 为准 | Java/Go 全量 parity、source-test 100%、发布覆盖率门禁均未达成；当前覆盖率不是最终阈值通过 |

本轮又对照 Java `EventInfraGetterIndexed`、`EventInfraGetterMapped`、`EventInfraGetterDynamicIndexed`、`EventInfraGetterDynamicMapped` 以及 `EventRenderJSON`/`EventRenderXML` 的 runtime inventory，补充 Go 事件属性路径解析：`array[index]`、`mapped('key')`、可选 `?`、转义点和嵌套路径现在统一进入 `Event.Get` 与 `Property`；`Schema`/`Event` 同时提供 root/path property names、声明类型和 indexed/mapped/dynamic descriptor，struct schema 在缺少直接字段时支持安全的零参数 JavaBean 风格 getter fallback。新增 `RenderJSON`/`RenderXML` 及 functional options，支持标题、缩进、XML scalar attribute、嵌套 map/struct/array/Event/Row、Missing/Null 保留和最大深度拒绝，并建立正向、转义、空值、嵌套、属性元数据、属性输出和深度边界测试。固定 Java 17/Maven 3.9.16 环境下选定的 `TestSuiteEventInfra`、`TestSuiteEventRender`、`TestSuiteEventJson`、`TestSuiteEventXML` 共 86/86 通过；这只是 Java oracle 可复现证据，不代表 Go 与 Java trace 相等。该切片已登记为 `event.property-access-render` / `case.event-property-access-render`，仍不覆盖 Java 的 EXPLICIT accessor、fragment/metadata、DOM/XPath、XSD、完整 Avro/JSON provided-underlying 或双方共享 trace。

随后继续对照 Java `EventJsonTypingCoreParse`、`EventJsonTypingCoreWrite`、`EventJsonTypingClassParseWrite` 和 `EventJsonParserLaxness`，将 Go `ParseJSONWithOptions` 扩展为 schema-directed 递归转换：支持嵌套 struct/map/slice/array、string alias、`time.Time`、`big.Int`、`big.Rat`，保留 null pointer，拒绝尾随 JSON，并提供最大深度和 undeclared-field 选项；`RenderJSON` 对 arbitrary-precision 数字保持 JSON number 形态。Java `TestSuiteEventJson` 本次 13/13 通过，Go 新增 typed conversion、精度、深度、未知字段和 render 测试已登记为 `event.json-typed` / `case.event-json-typed`；provided-underlying/class/list adapter、完整 laxness、schema inheritance 和共享 parse/write trace 仍未完成。

随后又将 `ParseXML` 从标量扫描扩展为受深度限制的内存树：属性以 `@name` 暴露、重复子元素形成 `[]any`、嵌套 indexed path 可直接由 `Event.Get`/`Property` 读取，并保持无路径字段的 leaf fallback；`ParseXMLWithOptions` 对非法 XML、无根文档和超深输入返回显式错误。Java `TestSuiteEventXML` 本次 33/33 通过，Go 新增属性/重复元素/index path/深度测试登记为 `event.xml-tree` / `case.event-xml-tree`；XSD/XPath/namespace/DOM/fragment/transpose 和完整安全资源配额仍未完成。

本轮还增加显式 `WithSchemaParent`/`NewSchemaWithOptions`：父字段在子字段之前合并，动态属性策略继承，ObjectArray 位置顺序保持，`Schema.ParentNames`/property metadata 可查询，且父关系进入 Plan canonical hash。该切片关联 Java Map/ObjectArray/JSON inheritance runtime entries，登记为 `event.inheritance` / `case.event-inheritance`；多级/分支/跨模块可见性、supertype/interface coercion、provided-underlying/Avro evolution、fragment/route/query 传播和双方 trace 仍未完成。

随后对照 Java `ViewDerived` 的 12 个 runtime execution（并执行 `TestSuiteView` 27/27），补齐 derived/statistical view 中此前遗漏的线性回归和相关系数：Go 通过 `Window().Aggregate()` 组合 `CountAll`、sum/count/avg/variance/stddev/weighted average，并提供 `Correlation`/`Correl` 与 `LinearRegression`/`Linest(...).Slope()`、`YIntercept()`；长度窗口淘汰时 new/old 统计行都会重算，Go 对 Java 回归中的四组数值采用明确浮点容差。该切片登记为 `view.derived-statistical` / `case.view-derived-statistical`；Esper 原生 derived-view 输出事件类型属性、group/union/intersect/time/batch/iterator/reclaim 组合及 exact NaN/decimal/trace policy 仍未完成。

本轮进一步复核 Java `ViewMultikeyWArray`，修复 Go `Unique`/`FirstUnique` 只写入 keyed map、却没有进入 Snapshot/Prev/Prior 历史的问题：`UniqueBy`/`FirstUniqueBy` 现在支持多个 analyzable key，key identity 保留 Value state/type/content，稳定保存首次 key 顺序，并在 `GroupWindow(..., Unique(...))` 中递归生成分区历史。新增唯一窗口快照、嵌套 group、new/old replacement、多键 first-unique、primitive/object/二维 array-key、array-key GroupWindow 和 array-key union/intersection 测试；同时对照 Java `ViewTimeOrderAndTimeToLive`，补齐 TimeOrder 的外部时间排序、虚拟时钟到期和迟到边界事件的即时 old-stream。继续对照 Java `ViewRank`/`ViewMultiKeyRank`，新增 `RankWindowBy(size, uniqueKeys, sortKeys...)`，实现唯一键替换、满窗口时的 pass-through new+old、按排序键淘汰最末事件和快照顺序，并修正 Sort/Rank 同分时各自不同的到达顺序/淘汰策略，登记为 `view.window-core` / `case.view-window-core`；subquery/named-window/dataflow 传播、更多导航和完整 Java trace 仍未完成。

### 17.3 后续每次提交的强制核对项

每实现一个 capability，提交必须同时更新四类证据：

- Go Builder/AST/Runtime 的位置和能力 ID；
- Java 来源入口、适用 Regression/Unit/EsperIO/Example case 及当前映射状态；
- 正向、无效、边界、取消/关闭、并发或资源配额测试（按适用性）；
- Plan/State/trace 规范化输出、已知差异、性能/内存数据和安全影响。

能力映射统一写入 `compat/capability-manifest.json`：`status=mapped` 只表示 Java 来源与 Go 测试已经关联，不表示 Java/Go trace 已相等；只有存在双方 trace 且差异处置完成时，才允许改为 `passing` 或 `approved-difference`。

在 Java oracle 恢复前，允许继续编写 Go 端单元和中立场景，但不允许把这些结果写成“Java 对照通过”；在许可证材料、完整 manifest 和全量差分恢复前，也不允许宣称“完整移植完成”。

### 17.4 Java 环境恢复后的实测补充

本机已发现 Java 17（`C:\Program Files\Microsoft\jdk-17.0.20.8-hotspot`），Maven 3.9.16 可执行文件位于 `C:\Users\baicai\AppData\Local\UniGetUI\Chocolatey\lib-bad\maven\3.9.16\apache-maven-3.9.16\bin\mvn.cmd`。Chocolatey 因当前会话非管理员无法完成最后的环境变量写入，但不影响显式设置 `JAVA_HOME`/Maven 路径运行构建。随后在官方 `common/etc/regression/create_testdb.sql` 夹具和 MySQL 8.0.46 Docker 环境下，按 UTF-8、UTC、Java 17、Maven 3.9.16 执行 `regression-run` 全量基线：78 个 suite、860 个 JUnit 入口，857 个无失败/错误；`testEPLDatabaseJoin` 只出现 MySQL 诊断文本差异，`testEPLDatabaseJoinPerfNoCache` 触发容器/JDBC 性能阈值，均已记录为 approved difference；`TestSuiteMultithread` 首轮另有 `testMultithreadContextCountSimple` 波动，隔离重跑 41/41 通过，不能把它归入 Esper 语义失败。首轮计数和 disposition 固化在 `compat/java-regression-baseline.json`；Surefire 目录保留最近的全量分片/隔离报告，重现首轮结果以 JSON 基线中的命令和差异记录为准。

外部 connector 验证使用独立 Docker 实例：`esper-java-mysql`（MySQL 8.0，`127.0.0.1:3306`，数据库 `test`）、`esper-go-kafka-test`（Kafka 3.8.1 KRaft，`127.0.0.1:9092`）和 `esper-go-rabbitmq`（RabbitMQ 3 management，`127.0.0.1:5672`）。Go 集成测试分别通过 `ESPER_MYSQL_DSN`、`ESPER_KAFKA_BROKERS` 和 `ESPER_AMQP_URL` 显式启用；这些容器只用于可复现的 connector/对照测试，不替代 Java provider、事务、重连和完整 trace 门禁。

第一轮无数据库环境的 `common` 模块曾是 585 个测试、583 个通过、2 个环境错误；在 MySQL 8.0.46 Docker 和官方夹具下复跑后，`common` 为 585/585 通过，错误来自 `TestDatabaseDMConnFactory` 和 `TestDatabaseDSConnFactory` 的环境缺口已消除。随后 `common-avro` 运行 13 个测试、全部通过，`common-xmlxsd` 运行 10 个测试、全部通过；`compiler` 运行 69 个测试、全部通过，`runtime` 运行 50 个测试、全部通过。`regression-run` 已完成 8 个核心 reactor 模块的跳过测试 install；`TestSuitePattern` 实际运行 28 个测试且全部通过；带数据库的 `TestSuiteContext` 17/17 通过。阶段 0 仍需要把 Java 测试结果固定分为 `pass`、`fail`、`error-environment`、`skip`、`manual`、`N` 六类，并为性能阈值、厂商诊断文本和首跑并发波动留下明确 disposition。

本次分片还确认了两个容易遗漏的构建前置条件：Esper 的 `install` 生命周期默认触发 GPG 签名，本地测试安装必须显式使用 `-Dgpg.skip=true`，而发布构建仍必须保留签名门禁；源码未固定编码时 Maven 会按本机 GBK 编译并产生不可映射字符警告，因此 Java baseline 和 Go CI 都必须固定 UTF-8、locale、timezone 与 charset，不能依赖开发机默认值。

本轮又定向执行了 Java `regression-run` 的 `TestSuiteEPLJoin,TestSuiteEPLSubselect`：61 个 JUnit 入口全部通过。Join 入口实际覆盖了多流、外连接、范围/复合条件、数组多键、无条件/单向、coercion 和 query-plan 变体；Subselect 入口覆盖了相关/非相关、exists/not-exists、聚合、prior、Named Window、pattern、UDF、多子查询和无效规则。Go 当前的基础 Join/FAF 快照能力与显式 FAF 路由已经有中立测试，但尚未把这 61 个 Java execution 转为共享 Java/Go trace，不能据此把 `join.basic` 或子查询能力标记为 parity。

本轮新增的 Go `subquery_test.go` 将其中可先落地的语义拆成独立 case：Named Window 相关 `exists`、Named Window 标量子查询、Table `in`、参数化谓词、普通事件源非法校验、Fire-and-Forget 一致快照、嵌套子查询中 `OuterField` 指向外层子查询候选事件、Named Window `count/sum/avg` 与 `any/some/all` 比较量词，以及标量 `order/offset/limit` 和多行 cardinality 选项。该映射已登记为 `query.subquery-basic` / `case.subquery-basic`；它证明聚合/量词/标量选项的基础 builder/runtime 形态和 null 处理可运行，不覆盖 Java 的分组/having、多行/多列、上下文/模式源、迭代器/错误诊断和完整 query-plan/trace 语义。

随后以固定 Java 17/Maven 3.9.16 环境直接执行 `mvn -pl regression-run -Dtest=TestSuiteEPLSubselect -DfailIfNoTests=false test`，该套件本次 19/19 通过；这只是 Java 侧回归入口可复现证据，Go 端仍按 capability manifest 的逐 case 差分口径推进，不能把整套子查询标记为已完成。

本次又定向执行了 Java `regression-run` 的 `TestSuiteRowRecog`：23 个 JUnit 入口全部通过，覆盖连续序列、reluctant/skip、interval、分区、measure aggregation、repetition、prev、variant stream、窗口和 Named Window delete 等入口。Go 已将其中基础连续序列、交替、可选/重复、有限和无限组合重复、reluctant、固定/日历 interval、Statement snapshot、长度窗口淘汰、Named Window 乱序删除重算、分区、重复变量、基础 measure aggregation 和无效规则切片映射到 `rowrecog_test.go`；这只是 Java case 到 Go 测试的关联证据，尚未执行共享场景 trace，因此 capability manifest 仍使用 `mapped` 或 `partial`，而不是 `passing`。

RowRecog 入口拆分如下，后续必须以此表逐项消项，不能以 `TestSuiteRowRecog` 这个外层 JUnit 名称代替：

| Java 入口/来源 | 当前 Go 证据 | 当前处置 |
|---|---|---|
| `RowRecogOps`、`RowRecogPermute`、`RowRecogRepetition` 的基础连续/交替/排列/可选/重复/分区/重复变量 | `rowrecog_test.go` 的基础序列、交替、有限排列、有限和无限组合重复、可选/重复、分区和 canonical/invalid 测试 | `mapped`，尚无共享 trace |
| `RowRecogAfter` | 已有三种 skip builder、固定 duration `Interval` 和 `Statement.Snapshot`；新增 `TestRowRecogAfterNextRowContinuation`、`TestRowRecogAfterSkipToNextRowDataSet`、`TestRowRecogAfterSkipToNextRowRepeatedVariable`、`TestRowRecogAfterSkipToNextRowPartitioned`、`TestRowRecogAfterSkipPastLastRow`，与既有 current-row 测试共同覆盖 Java 的三种 skip、重复变量、分区 listener/Snapshot 矩阵；`SKIP PAST LAST ROW` 的快照历史保留也已固定。interval 与 termination 的组合生命周期、完整 Java iterator/ordering 差异仍未对照 | `partial` |
| `RowRecogGreedyness` | `Reluctant` API、匹配器分支和 Go 正向用例已覆盖；尚无共享 Java/Go trace 证明全部贪婪/非贪婪结果顺序 | `mapped` |
| `RowRecogInvalid`、`RowRecogClausePresence` | 已覆盖空模式、非法 DEFINE、未知 tag、重复 measure、负 MaxStates、旧流和 snapshot 限制的部分校验 | `partial` |
| `RowRecogEmptyPartition`、`RowRecogMultikeyWArray` | 有普通 `PartitionBy` 运行时切片，空分区回收和多键数组语义未对照 | `partial` |
| `RowRecogInterval`、`RowRecogIntervalResolution`、`RowRecogIntervalOrTerminated` | 新增 `TestRowRecogIntervalSimpleTrace`、`TestRowRecogIntervalPartitionedTrace`、`TestRowRecogIntervalMultipleCompletedTrace`、`TestRowRecogIntervalMonthScopedTrace`，对照 Java `RowRecogInterval` 四个 execution 的 pending `Snapshot`、deadline listener batches、partitioned start times、multiple completed starts 和 calendar month boundary；`RowRecogIntervalResolution` 的 exact flip boundary/microsecond 配置与 `RowRecogIntervalOrTerminated` 的完整 composition matrix 仍未对照 | `partial` |
| `RowRecogIterateOnly`、`RowRecogDataWin`、`RowRecogDelete` | 已有 `Statement.Snapshot` 基础 iterator 视图、长度窗口淘汰重算、time-window 到期重算、time-batch pending batch 迭代器及边界后状态清空、无数据窗原始流“listener 有结果但 Snapshot 为空”、Named Window 乱序删除重算和“删除不产生伪 listener batch”的测试；iterate-only 性能提示、PREV 顺序保持、完整 out-of-sequence delete/NFA 状态库和 Java 输出排序仍未对照 | `partial` |
| `RowRecogEnumMethod`、`RowRecogAggregation`、`RowRecogArrayAccess` | 有基础字段 `CountAll`/`Sum`、`TagFieldAt`，枚举方法、完整聚合和数组属性矩阵未对照 | `partial` |
| `RowRecogPrev`、`RowRecogRegex`、`RowRecogVariantStream`、`RowRecogDataSet`、`RowRecogMultikeyWArray` | 已增加 Go 链式 `Prev`/`Abs`，并为每个 match-recognize 分区保存有限滚动 previous-access 快照；time window 淘汰后仍可按到达顺序取 PREV，已覆盖非分区、分区、双字段分区和 keep-all 场景。新增 `TestRowRecogDataSetFinancialPattern` 对照 Java `A B C* D E* F+`、DEFINE 中的普通 PREV/tag 引用、三段 E7/E8/E9 listener trace 和 `SKIP TO CURRENT ROW` 重叠分支；新增 `TestRowRecogFinancialWPatternDataSet` 对照 Java `A W+ X+ Y+ Z+` 的 E1–E60 无窗口流和全部终止 listener batches；新增 `TestRowRecogPartitionMultikeyWithArrayContent` 对照 Java 数组内容分区，验证 nil、空数组和同内容数组的分区身份；新增 `TestRowRecogPartitionMultikeyPlainTuple` 对照 int/long 完整多键 tuple；新增 `TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch` 对照 `RowRecogAfter` 的 A B* continuation；新增 `TestRowRecogRegexMatrix` 对照 Java `RowRecogRegex` 全部 12 组模式，并同时检查 listener 与 `Statement.Snapshot`；新增 `TestRowRecogVariantStreamPreservesMemberType` 对照 PREDEFINED Variant 的 typed member route、concrete type predicate、listener 和 Snapshot，另有非法成员/source/direct-payload 负例。完整外部 fixture trace、Variant/ANY/dynamic-member 高级矩阵和高级 tag-aware PRIOR/PREV 变体仍未映射 | `partial` |
| `RowRecogPerf` 及 MaxStates engine-wide 变体 | Go 已增加 `WithMatchRecognizeStateLimit`/`WithMatchRecognizeMaxStates`/`WithMatchRecognizePreventStart` 引擎级配置、按 statement.ID 汇总的不可变超限事件、prevent-start 分支拒绝、跨 statement/Context/Named Window 生命周期和 undeploy 释放测试；精确 NFA 状态计数及性能基线仍未完成 | `partial` |

本轮针对 `RowRecogDataWin` 增加了 Go 链式 API 对照场景：`TestRowRecogTimeWindowIteratorAndExpiry` 覆盖虚拟时钟下的 time window 到期、匹配后 iterator 视图和到期后的重算；`TestRowRecogTimeBatchWindowPendingIteratorAndBoundary` 覆盖 partition-by、pending batch 的 iterator、边界 listener 发射、边界后识别状态清空和下一批重新识别。实现将 time-batch 窗口保留的上一批与 match-recognize 当前识别状态分离，Java 的 unbound stream、PREV 到达顺序、Named Window time-batch 变体和完整输出顺序仍需继续对照。

本轮继续对照 Java `RowRecogPrev`：Go 新增 `TestRowRecogPreviousHistorySurvivesTimeWindowEviction`、`TestRowRecogPreviousHistoryIsPartitionLocal`、`TestRowRecogPreviousHistoryForPartitionedSequence`、`TestRowRecogPreviousHistorySupportsMultiFieldPartitions` 和 `TestRowRecogPreviousHistoryOnUnpartitionedKeepAll`。实现对应 Java 的 `RowRecogStateRandomAccess`：新事件保存受最大偏移约束的滚动历史，旧事件从当前匹配输入移除时不回写滚动到达历史；因此匹配状态仍受窗口淘汰控制，而 DEFINE 中的 PREV 可复现 Java 的到达顺序语义。`PRIOR`、tag-aware previous access 和完整 Java/Go trace 仍待继续拆解。

随后补齐 Java `PREV(A.property, n)`/`PRIOR(A.property, n)` 的基础 tag-aware field 语义：Go 新增 `PrevTag` 与 `PriorTag` 链式构造器，并在 previous evaluator 中为嵌套 `TagField`/tag enumeration 重绑定被选中的到达顺序事件，而不是继续读取当前 match tag。`TestRowRecogTagAwarePrevAndPriorEvaluateAgainstPreviousEvent` 在 DEFINE 和 MEASURES 中验证 `PrevTag(1)` 与 `PriorTag(0)`、keep-all 的监听器/快照结果及未知 tag 构建期错误；高级重复 tag、复杂嵌套 NFA 与完整 Java/Go trace 仍未完成。

本轮又对照 Java `RowRecogDataSet.RowRecogExampleWithPREV` 与 `RowRecogAfter.RowRecogAfterCurrentRow`：Go 新增 `TestRowRecogDataSetFinancialPattern`，用链式 `RowSequence(RowVar("A"), RowVar("B"), RowVar("C").ZeroOrMore(), RowVar("D"), RowVar("E").ZeroOrMore(), RowVar("F").OneOrMore())`、`Prev`、`TagField` 和 `SkipToCurrentRow` 复现 E1–E9 价格序列；测试不仅比较最终九条结果，还逐事件检查 E1–E6 无输出、E7/E8/E9 分别产生 1/3/5 条 listener rows。运行时同步修复 current-row skip：该策略保留已经被当前匹配接纳的旧起点，使 E8/E9 能继续产生重叠 NFA 结果；`TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch` 进一步固定 A1 初始结果在 B1 到达后扩展为 A1/B1。该测试中的 `Statement.Snapshot` 仍只表示当前可枚举/活跃识别分支，不被误当作 listener 历史结果全集；完整外部 fixture trace 和高级 NFA 仍需后续逐项移植。

本轮又对照 Java `RowRecogRegex`：Go 新增表驱动 `TestRowRecogRegexMatrix`，覆盖 Java 中全部 12 组 SupportTestCaseHolder，包括可选前缀/整组、交替、嵌套可选分支、`(A B)* C D`、嵌套组合重复、重复前缀/重复尾、嵌套 alternation 以及固定/重复 alternative；每个输入序列都通过链式 `RowSequence`/`RowAlternation`/`Optional`/`ZeroOrMore`/`OneOrMore` 构造，并对 listener 和 `Statement.Snapshot` 做无序结果比较。对照期间发现 current-row + ALL MATCHES 的 iterator 不能只保留每个 start 的最长 end，已修复为保留所有可完成路径；FirstMatch 和其他 skip 策略仍保留最长当前路径。Variant 输入和高级 NFA 仍需继续拆解。

本轮又对照 Java `RowRecogVariantStream`：Go 新增 `TestRowRecogVariantStreamPreservesMemberType`，注册 `SupportBean_S0`/`SupportBean_S1` 两个 struct schema 和 PREDEFINED `MyVariantType`，以两个 typed `From[T](...).InsertInto("MyVariantType")` 规则复现 Java 的成员路由；RowRecog 的 DEFINE 使用 `EventValue[Event]` 加命名 `Func1` 读取 concrete `Event.TypeName()`，因此只有 S0→S1 顺序能生成 A/B 结果。测试同时检查 listener 与 `Statement.Snapshot`，`TestRowRecogVariantStreamRejectsInvalidMemberRoute` 固定非成员 schema、未知源和直接 payload 的 Build/运行时错误。Java 的 Variant multikey-array、ANY/动态成员和完整 trace 仍需后续拆解。

本轮又对照 Java `RowRecogDataSet.RowRecogExampleFinancialWPattern`：Go 新增 `TestRowRecogFinancialWPatternDataSet`，使用无窗口原始流和链式 `A W+ X+ Y+ Z+`，以 `Prev` 在 DEFINE 中表达 W/X/Y/Z 的交替涨跌条件，并逐事件发送 E1–E60；测试对照 Java 的全部终止 listener batches（包括同一终点的多起始分支），同时验证累计输出数量。至此 `RowRecogDataSet` 的两组 Java execution 都有 Go 链式数据轨迹，完整外部 fixture trace、Variant multikey-array 和高级 NFA 仍需继续拆解。

本轮继续对照 Java `RowRecogMultikeyWArray`：Go 新增 `TestRowRecogPartitionMultikeyWithArrayContent` 与 `TestRowRecogPartitionMultikeyPlainTuple`，分别使用链式 `PartitionBy(array)` 和 `PartitionBy(intPrimitive, longPrimitive)`，复现数组分区的同内容/空数组/null 身份以及普通多键 tuple 的独立识别状态；两组测试都逐事件验证 A/B listener 输出。数组与普通多键分区的基础 Java trace 已映射，Variant 的 ANY/动态成员矩阵和更复杂的共享 trace 仍需继续拆解。

本轮继续对照 Java `RowRecogInterval`：Go 新增 `TestRowRecogIntervalSimpleTrace`、`TestRowRecogIntervalPartitionedTrace`、`TestRowRecogIntervalMultipleCompletedTrace` 和 `TestRowRecogIntervalMonthScopedTrace`，用虚拟绝对时间逐事件检查 `Snapshot` 中的 pending rows、deadline 到期的 listener batches、partition-local deadline、多起始匹配和 calendar month boundary。四个 Java `RowRecogInterval` execution 已有链式对照证据；`RowRecogIntervalResolution` 的精确/微秒边界以及 `RowRecogIntervalOrTerminated` 的完整终止组合仍需单独拆解。

本轮继续对照 Java `RowRecogAfter`：Go 新增 `TestRowRecogAfterNextRowContinuation`、`TestRowRecogAfterSkipToNextRowDataSet`、`TestRowRecogAfterSkipToNextRowRepeatedVariable`、`TestRowRecogAfterSkipToNextRowPartitioned` 和 `TestRowRecogAfterSkipPastLastRow`，配合既有 `TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch` 覆盖 `A B*` continuation、`A B` 数值比较、重复变量、partition-by 和 past-last 的逐事件 listener/Snapshot 结果。为保持 Java iterator 语义，运行时新增已发出 match 的快照历史；past-last 新起点被 skip 后不再被 Snapshot 重新计算，而已发出结果继续可见。interval/termination 组合与完整排序仍需后续拆解。

随后补齐 `RowRecogDataWin.RowRecogUnboundStreamNoIterator`：无 `Window` 的原始事件流仍按链式 `MatchRecognize`/`Define`/`Measures` 产生 listener match，但 `Statement.Snapshot` 不再把识别分支当作可枚举数据窗；只有带 data window、Named Window、Table 或 historical source 的输入链才建立 iterator 视图。`TestRowRecogUnboundStreamListenerAndEmptyIterator` 固定复现 Java 的 `s1/s2/s1/s3/s2/s1/s1` 序列，验证最后一条重复字符串产生一条监听结果且快照为空；iterate-only 优化提示和性能差异仍属于后续 trace 工作。

本轮又以固定环境在 `regression-run` 模块专项执行 `mvn -pl regression-run -Dtest=TestSuiteExprEnum -DfailIfNoTests=false test`，Java `TestSuiteExprEnum` 的 28 个 JUnit 入口全部通过。该结果证明 Java oracle 的 ExprEnum 基线可复现，不等于 Go 端已经覆盖这 28 个入口；Go 当前只映射了首批可分析枚举算子和中立/运行态测试，嵌套、子查询、访问聚合、UDF、BigDecimal、完整无效规则与 Java trace 仍需逐项关联。

随后以相同 Java 17/Maven 3.9.16 环境执行 `mvn -pl regression-run -Dtest=TestSuiteResultSetOutputLimit -DfailIfNoTests=false test`，该结果集套件的 14 个 JUnit 入口全部通过，其中包含 `ResultSetOutputLimitAfter` 的事件数 after、持续时间 after、after+every、月份和变量/when-then 场景。Go 当前映射的是固定 `time.Duration` after、非负 years/months/days 日历 after、事件计数 every、变量/基础计数/last-output-timestamp when/then、虚拟时钟 time every，以及来自 `ResultSetOutputLimitCrontabWhen` 的基础五字段 output-at：`CronEvery`/`CronRange`/`CronWildcard`、变量/参数字段、下一次触发和日月/周 OR；本轮又补充了普通窗口、分组聚合、Join、Named Window、Table、Table aggregate/join 和 context-partitioned Named Window 的当前状态快照，以及 `OutputSnapshotEvery`。Java 的 cron 秒/毫秒字段、特殊日历运算符、完整时区/DST/无效规则矩阵、更多 when/then 上下文/计数函数、完整 row-for-all/custom-access snapshot-after 和其他输出组合仍未宣称对等。

同一环境下又执行了 `mvn -pl regression-run -Dtest=TestSuiteResultSetAggregate -DfailIfNoTests=false test`，Java `TestSuiteResultSetAggregate` 的 14 个 JUnit 入口全部通过，覆盖 filtered aggregate、first/last/rate/nth、sorted/window access、table/join、invalid 和 aggregate-on-delete 等分支。Go 当前已映射可复用 `FilterAggregate` 的标量/访问聚合过滤和非法 nil predicate、带过滤器的有状态 `Leaving`、常量间隔速率的 ever-point/过滤语义、时间戳/数量速率的窗口淘汰和过滤语义、sorted/window access 的一组链式导航、Named Window 删除后的空聚合行、Named Window/FAF 的 first/window/last 快照、命名及环境注册式 `PluginAggregate` 扩展、按组隔离并能接收 `Enter`/`Leave`/`Value`/`Clear` 的 `AggregatePluginFactory`、无参数 Event 方法/属性链、确定性 Count-Min Sketch 频率/total 与表触发器访问、`TableSink` 适配器、链式 `AggregateStream.IntoTable` 的目标校验/原子快照替换/窗口淘汰删除以及 live table snapshot join；named filter parameter 的全部 API 形态、factory 的配置/serde/HA 生命周期、math-context/decimal、完整属性链/导航方法、context/rollup/FAF 组合和 table/join 全矩阵仍需继续拆解。

本轮新增的 Go `aggregate_test.go` 已覆盖 filtered aggregate 的 true/null 排除、Count/Sum/Avg/Min/Max/Distinct 包装、`first-ever`/`last-ever`/`count-ever` 跨长度窗口淘汰，以及 dimensional grouping 的 subtotal Null、`Grouping`/`GroupingID`、cube/explicit grouping set、Fire-and-Forget Named Window 快照和非法定义；对应 capability/case 已登记为 `resultset.aggregate-filtered`、`resultset.aggregate-ever`、`resultset.aggregate-dimensional`。

本轮继续拆分 Java `TestSuiteResultSetAggregate` 的 access/local-group 分支：Go 已增加索引化 `First`/`Last`/`Nth`、`WindowEvents`/`EventValue`、current/ever `MinBy`/`MaxBy`、多条件 `SortedEvents`、外层分组内的 `LocalGroupBy`，并实现聚合结果按投影别名 `ResultField` 排序及 unknown alias 构建期校验。随后又加入可链式 `SortedAccessBy`/`WindowAccessBy`、Table 方法链、命名及环境注册式 plugin aggregate、Count-Min Sketch、IntoTable 原子快照、live table snapshot join、Named Window 删除后的空聚合行、常量/时间戳/数量 rate、过滤访问聚合和 Named Window/FAF access 快照；对应 Java runtime execution 已登记为 `case.aggregate-access` / `case.aggregate-local-group` / `case.aggregate-rate` 等。它们是可运行的 mapped 切片，不是完整 Java/Go trace parity，plugin factory/configuration 生命周期、join/FAF/context 全矩阵、row-remove、typed alias/descriptor、math-context/decimal、完整 invalid/iterator/type 和跨模块组合矩阵仍待实现。

本轮进一步把 access 语义推进到表侧：`SortedAccessBy` 提供可分析的导航操作和重复 key 的稳定桶，`FirstEventValue`/`LastEventValue` 补齐无参数 first/last 的 Event 身份和 `Property` 链，`Method` 提供 Go 对象/事件的零参数及参数化方法访问，`PluginAggregate`/`RegisterAggregatePlugin`/`PluginAggregateRef` 提供带名称、注册环境和构建期校验的 Go aggregate/access 扩展，新增 `PluginAggregateWithFactory`/`RegisterAggregatePluginFactory`/`PluginAggregateFactoryRef`，按 aggregate group 隔离 state 并用 `Enter`/`Leave`/`Value`/`Clear` 完成窗口重放，`CountMinSketchAdd` 提供确定性频率/total 聚合及表列读取，`TableSink` 将聚合新流按主键 upsert 到 Go Table，链式 `AggregateStream.IntoTable` 通过 `Table.Replace` 原子同步 plain group-by 的完整当前快照并清理窗口淘汰/整组消失的 stale row，live Join 在触发事件上重建 table snapshot 并通过 tuple diff 形成 old/new 结果。该切片已关联 `ResultSetAggregationMethodSorted`、`ResultSetAggregationMethodWindow`、`ResultSetAggregateFirstLastWindow`、`ResultSetAggregateMethodPlugIn`、`ResultSetAggregateAccessAggPlugIn` 和 `ResultSetAggregateIntoTable{join=false/join=true}` 的 Java runtime；Java 的 factory 类型/命名参数校验、配置/serde/HA、Count-Min Sketch 的可配置/碰撞策略、完整属性链/导航接口、context/rollup、on-demand/FAF 变体、table sink 批量回滚和完整 row-remove 生命周期仍未完成。

本轮再把 IntoTable 的边界推进到维度物化：`validateIntoTable` 不再把目标限定为 plain group-by，`ROLLUP`/其它分组集合现在可以将 subtotal 的 Null 维度和 `GroupingID` 作为普通投影写入带稳定主键的 Table；同时补充了过滤 `sum`、`window`、`sorted` access 列的 IntoTable 快照测试。随后增加了 source-indexed `JoinField`/`JoinEventValue`，Join 结果可以进入同一套 AggregateStream 状态机，覆盖分组、First/Window 访问、窗口淘汰以及 old/new 聚合重算。该切片仍不等于 Java 的 join/context/table access 全矩阵，尤其是分组键设计、批量回滚、FAF/on-demand、outer/self/multi-way Join 聚合和 plugin factory 的配置/serde/HA 生命周期仍需继续对照。

同一日历计划内核已用于 Go `TimerCron` Pattern observer，并覆盖了跨多个 due instant 的有序虚拟时钟输出；这只是对 Java `timer:schedule`/calendar observer 的基础语义映射，不代表 Pattern 的全部 observer、guard 和 consumption 组合已完成。

本轮先修正了 `OutputAt(..., OutputSnapshot())` 的一个容易遗漏的边界：日历触发点现在从普通窗口状态重建当前快照，而不是只输出该触发周期内新增的结果；该行为有 `TestOutputAtSnapshotReadsCurrentWindowAtCalendarTick` 覆盖。随后继续对照 Java `ResultSetOutputLimitAggregateGrouped` 与 `ResultSetOutputLimitRowForAll` 的 snapshot execution，补齐分组聚合、Join/Join-aggregate、普通 Context 窗口、Named Window、Table、Table aggregate/join 和 context-partitioned Named Window 的当前状态重建，并增加 `OutputSnapshotEvery(interval)` 与 `OutputSnapshotEveryEvents(count)` 链式 API，以及 untyped `RecordStream.Select` 投影。`TestOutputAtSnapshotRebuildsAggregateState`、`TestOutputAtSnapshotRebuildsJoinState`、`TestOutputSnapshotEveryRebuildsCurrentAggregateState`、`TestOutputSnapshotEveryEventsRebuildsCurrentAggregateState`、`TestOutputSnapshotEveryRebuildsJoinAggregateState`、`TestOutputAtSnapshotRebuildsNamedWindowState`、`TestOutputAtSnapshotRebuildsTableState`、`TestOutputAtSnapshotRebuildsTableAggregateState`、`TestOutputAtSnapshotRebuildsTableJoinState`、`TestOutputAtSnapshotRebuildsContextNamedWindowState` 和 `TestOutputAtSnapshotRebuildsContextWindowState` 覆盖虚拟时间、事件数阈值、分组 count/sum、Join 投影、当前状态源、上下文分区、空 interval/非法组合校验；相关 Java runtime ID 和差异已登记为 `output.when-basic` / `case.output-snapshot-state`。随后对照 Java `ContextInitTermOutputSnapshotWhenTerminated`、`ContextInitTermOutputAllEvery2AndTerminated`、`ContextInitTermOutputOnlyWhenTerminatedCondition` 和 `ContextInitTermOutputOnlyWhenTerminatedThenSet`，增加 `OutputWhenTerminated`、`OutputAndWhenTerminated`、`OutputSnapshotWhenTerminated`、`OutputWhenTerminatedIf`/`OutputAndWhenTerminatedIf` 链式 API；initiated context 终止时在释放分区前执行终止输出，显式 snapshot 重建当前状态并排除同一终止事件，普通输出冲刷 pending delta，支持 count/time policy 与 termination snapshot 组合、termination 条件和变量 then assignment。`TestInitiatedTerminatedContextSnapshotExcludesTerminatingEvent`、`TestInitiatedTerminatedContextCanCombineEveryOutputWithTerminationSnapshot` 和 `TestInitiatedTerminatedContextCanFlushPendingOutputAtTermination` 已覆盖；对应 Java runtime ID、Go 测试和差异已登记到 `case.context-partitioning` 与 `case.output-when-then-snapshot`。本轮又将同一终止输出边界接到事件驱动 Pattern Context：`TestPatternContextTerminationSnapshotCarriesEndTags` 验证 start/end tag、终止事件属性和排除终止事件的 current-state snapshot。纯 timer-root Pattern Context 的启动/终止、部署时锚定和 temporal termination snapshot 已补齐；复杂 timer+event Pattern Context、temporal termination output、row-for-all/custom-access 全量组合和共享 Java/Go trace 仍保持未完成，不能把这条切片解释为完整 output parity。

本轮还将已有 `context_test.go` 的 Context 纵向证据正式补入兼容清单：`case.context-partitioning` 关联 Java `ContextKeySegmented`、`ContextHashSegmented`、`ContextCategory`、`ContextInitTerm`、`ContextInitTermWithDistinct`、`ContextNested` 的代表 runtime execution，并登记 key/hash/category/initiated-terminated/nested、FAF selector、Named Window consumer、built-in context property、descriptor/boundary-event、partition-scoped context variable 和 snapshot 分区测试。随后将 `NewKeyContext` 扩展为可接收完整 key tuple，增加 `NewHashContextBy`/`CreateHashContextBy` 和 `NewInitiatedTerminatedContextBy`/`CreateInitiatedTerminatedContextBy`，增加 typed `ContextField`/`ContextName`/`ContextID`/`ContextLabel`/`ContextKeyValue`、`ContextInitiatingEvent`/`ContextTerminatingEvent`，并把这些属性注入 live statement、Named Window consumer、FAF partition runtime 和 trigger expression；多键窗口隔离、hash tuple 路由、null/基础 array tuple identity、非重叠生命周期、key/category/nested 属性、descriptor 快照、key/ID/category/descriptor-filter selector、共享分区 ID、allocation/deallocation listener、启动/终止事件的 `Property` 链、共享 context variable 的跨 statement 更新/直接 API/生命周期重置和非法 key/越界作用域构造测试已固定行为，单 key 调用保持兼容。新增事件驱动 Pattern Context API：`NewPatternInitiatedTerminatedContext`/`NewPatternInitiatedContext` 及 overlapping 变体将现有 PatternStream transition 状态接入 Context 分区，保留 start tags、在 end pattern 中按 `TagField` 关联，并通过 `ContextPatternEvent`/`ContextPatternField` 暴露；三项 Go 测试覆盖相关 end、Every overlapping 和 termination snapshot。该映射只表示来源与 Go 测试已关联；Java 的多键 distinct initiated-terminated、重叠生命周期、完整 primitive/object/array-key 规范化、纯 timer-root pattern context、temporal termination snapshot 和 keyed initiated child nested Context 已补齐；复杂 timer+event/temporal context、initiated parent/pattern child nested Context、完整 nested namespace、runtime partition administration、完整 context create/activate/statement listener 事件、变量 iterator/listener/管理服务集成、跨 statement 生命周期边界和事务矩阵仍未完成。

本轮补齐了 EsperIO 的 Kafka、AMQP 与 provider-neutral JMS bridge 外部消息切片，并补上 Dataflow 控制面信号、自定义 operator 和实例控制面纵切片：Go `connectors/csv` 的 source、sink、unformatted line source 和公共生命周期测试已通过，Java `esperio-csv` 模块以 JDK 17/Maven 3.9.16 执行 80/80 通过；随后增加 `connectors/db` 的 DML/Upsert sink，fake executor 与 MySQL Docker round-trip 通过；又增加 `connectors/http` 的 client/server source/sink，覆盖 query/property URI 模板、JSON body、响应上限、重试、暂停/停止/重启和 Engine bridge；再增加 `connectors/socket` 的 TCP source，覆盖四种协议、Java escape、多连接、暂停/恢复、停止/重启、typed Engine bridge 和可注册 object decoder，Java Socket 模块 7/7 通过；随后增加 `connectors/kafka` 的 kafka-go Reader/Writer、JSON processor、commit/retry/timestamp、key/header/有序 JSON 输出、Engine bridge 与 factory restart，在单节点 Kafka 3.8.1 KRaft 和预创建 topics 下 Java 模块 6/6、Go Docker round-trip 均通过；再增加 `connectors/amqp` 的 RabbitMQ Consumer/Publisher、queue/exchange/binding、prefetch、JSON/GOB codec、auto/manual ack、reject/requeue、retry、Engine bridge 与 factory restart，Go RabbitMQ source/sink round-trip 均通过；最后增加 `connectors/jms` 的 Map/Text/Object/Bytes 消息模型、Java event-type marker、ack/retry、pause/stop/restart、Engine bridge 和 channel bridge，Go bridge contract 测试通过；Dataflow 增加 `FinalMarker`/`WindowMarker`/`CustomSignal`、`OnSignal`、图内传播、运行态 `SubmitSignal`、每实例独立 `Custom` factory/runtime、named ports、instance options、in-process saved configuration 和 structured exception/error statistics。`case.esperio-csv-file`、`case.esperio-db`、`case.esperio-http`、`case.esperio-socket`、`case.esperio-amqp`、`case.esperio-kafka` 与 `case.esperio-springjms` 仍标记 `mapped`/capability `partial`，尚未完成共享 Java/Go trace、connector graph 的 Dataflow adapter signal/marker、Java bean population、DB 配置/异步 executor、HTTP/Socket XML 与 plugin、Kafka group/rebalance/custom serializer/plugin、AMQP Java serialization/高级重连、JVM JMS provider/session/transaction/Spring XML bridge；Java DB/HTTP/AMQP/Spring JMS 当前结果差异已在 manifest 记录。

补充复核 Java `ViewMultikeyWArray` 后，Go 又增加了 object-array、二维 array、RankWindow array unique-key 和 array-key union/intersection 的替换/稳定顺序测试，并修复 composite view 读取 Unique 子窗口 keyed 状态时丢失事件以及替换导致 iterator 重排的问题；primitive/object/二维数组 key 及组合窗口的基础 identity 已有可执行证据，但 subquery/named-window/dataflow 传播、更多导航和完整 Java trace 仍列为未完成项。

外部 connector 本轮实际执行了以下 PowerShell 命令：`$env:ESPER_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/test?parseTime=true&charset=utf8mb4'; $env:ESPER_KAFKA_BROKERS='127.0.0.1:9092'; $env:ESPER_AMQP_URL='amqp://guest:guest@127.0.0.1:5672/'; go test ./connectors/db ./connectors/kafka ./connectors/amqp -count=1 -v`。结果为 DB 包 4/4、Kafka 包 6/6、AMQP 包 7/7；其中 MySQL Docker、Kafka Docker round-trip、AMQP source/sink Docker round-trip 分别通过，容器保持运行供后续差分复现。

本轮再对照 Java `ContextControllerSelectorUtil` 及 key/hash/category/nested controller 的 `InvalidContextPartitionSelector` 分支，Go 增加内置 selector 与 context kind 的显式校验：key/hash/category/nested selector 误用于不匹配的 context 时，`ContextPartitionDescriptors`、`ContextVariableStates` 和 `ExecuteFireAndForgetWithSelector` 均返回 `ErrorInvalidRule`，不再静默得到空结果；`ContextPartitionSelectorAll`、ID、filtered/descriptor predicate 及合法 key selector 保持可用。`TestContextSelectorValidationRejectsMismatchedBuiltIns` 覆盖正负路径并登记到 `case.context-partitioning`。这只完成 Go API 层的错误分类和首层 kind 校验；Java 的 `InvalidContextPartitionSelector` 独立异常类型/精确诊断文本、nested per-level selector stack、statement iterator/safe-iterator 及完整 FAF selector 矩阵仍需共享 trace 对照。

本轮继续补齐 statement 侧的 selector 入口：`Statement.SnapshotWithSelector` 在读取当前分区状态前复用 context-kind 校验，支持 key/ID 选择并返回 Go `QueryResult` 快照；对非 Context statement 传入 selector 或传入错误 kind 均返回 `ErrorInvalidRule`，不再把 iterator 选择错误静默降级为空结果。`TestContextSnapshotWithSelectorFiltersIteratorState` 覆盖全量、ID、key、错误 kind 和非 Context 误用。它是 Java iterator/safeIterator 的 Go 风格快照边界，Java 的独立 safe iterator、并发迭代、nested selector stack 和精确异常类型仍待差分。

本轮再对照 Java `ContextInitTermTemporalFixed.ContextStartEndStartAfterEndAfter`（`java-runtime-df709b1c195efb70fa200`）：Go 增加 `NewTimePeriodContext`/`CreateTimePeriodContext` 及表达性别名 `NewPeriodicContext`/`CreatePeriodicContext`，以 `time.Duration` 表达 `start after` 与 `end after`，由同一 Environment/Engine 的虚拟时钟共享周期起点；周期窗口在 5s 开始、15s 精确结束、20s 再次开始，分区 allocation/deallocation、间隔内事件丢弃和 `context.startTime`/`context.endTime` 均有 `TestTimePeriodContextCyclesWithVirtualClock` 覆盖。Go 采用 `time.Time` 作为时间属性并明确拒绝当前切片中的 temporal nesting；Java 的 daily/calendar、cron、pattern 起止、变量/动态时间参数、overlap/distinct、temporal join/subselect/Named Window/Fire-and-Forget 组合仍未完成，不能把 flat periodic temporal slice 解释为完整 temporal-context parity。

本轮继续对照 Java `ContextInitTermTemporalFixed.ContextStartEndNWFireAndForget`（`java-runtime-e7e977b7827820a24082`）中的固定日历窗口：Go 增加 `TimeOfDay`、`NewTimeOfDay`/`NewTimeOfDayN` 与 `NewDailyTimeContext`/`CreateDailyTimeContext`，支持 start-inclusive/end-exclusive 的每日窗口、跨午夜窗口及按 Engine 时区构造本地日期，因此 09:00–17:00 在结束边界精确关闭并于下一日重新打开。`TestDailyTimeContextUsesLocalCalendarBoundaries` 覆盖非法时刻、跨午夜、前一窗口未开始、开始/结束边界、descriptor/result 的时间属性和下一日重启；daily 之外的 cron schedule、pattern 起止、变量开关、DST 全矩阵与 temporal 组合仍待实现。

本轮再对照 Java `ContextInitTermTemporalFixed.ContextStartEndMultiCrontab`（`java-runtime-71fcef5de9b15b0e743a`）的单起止固定日历片段：Go 增加 `NewCronTimeContext`/`CreateCronTimeContext`，复用 `CronSchedule` 的秒/分钟/小时/日历字段，以最近 start occurrence 到其后的第一个 end occurrence 构造 active window，并实现稀疏 schedule 的反向定位，`TestCronTimeContextUsesNextCalendarEnd` 覆盖 08:00–09:00 边界、跨日重启和 dynamic schedule 拒绝。当前只接受静态单起止 schedule；Java 多起止 crontab、start/end overlap、变量动态 schedule、特殊日历运算符、DST/时区矩阵及 pattern temporal 仍保持未完成。

本轮继续对照 Java `ContextStateListener`、`ContextPartitionStateListener` 和 `ContextAdminListen`，在 Go 中补齐 Context 管理事件的可观察契约：`AddContextStateListener` 对 Environment 中已存在的 context 做 created replay，`DestroyContext` 在无活动 statement 时发出 destroyed；按 context 注册的分区 listener 可通过可选 `ContextPartitionLifecycleListener` 接收 statement added/removed 与 activated/deactivated。引擎把这些事件与分区 allocation/deallocation 放进同一有序队列，销毁顺序固定为 statement removed、分区 deallocated、context deactivated；`TestContextStateAndPartitionLifecycleListeners` 已覆盖 replay、事件身份、共享队列顺序和 context 删除。Java 对照仍保留 ContextDeploymentID/RuntimeURI、完整 identifier payload、动态 context create 和 listener 注册时机等差异，不标记为完整 parity。

随后对照 Java `EPVariableService` 的 context-partition setter 和 `VariableChangeCallback` commit 语义，Go 增加 `SetContextVariableByID`/`SetContextVariablesByID`，要求 partition ID 当前仍 active；新增按变量名或 wildcard 注册的 `VariableChangeListener`，事件携带 old/new `Value`、ContextName、PartitionID 和 public partition key，批量赋值全部校验成功后才入队通知，并在 engine lock 外回调；`TestVariableChangeListenerAndContextPartitionIDUpdate` 覆盖 wildcard/精确 listener 去重、old/new、按 ID 更新、失效 ID 和回调重入读取。Java 的版本化读取、deployment/name pair、延迟可见性和内部 callback 注册粒度仍待实现。

本轮再对照 Java `ContextInitTermWithDistinct`（`java-runtime-5dda093c21bd6b075462`、`java-runtime-786b9a60d88de2e10fca`、`java-runtime-133d9b0460ecb38ba1e2`、`java-runtime-48f5615c894006027aa9`、`java-runtime-6aa9bf8995c016259f14`）：Go 增加 `NewDistinctInitiatedTerminatedContext`/`NewDistinctInitiatedTerminatedContextBy` 及对应 `Create` API。distinct tuple 只保留一个 active partition，重复启动不会重复分配；新事件会广播到所有 distinct partition，使 initiating event、terminating event 和 `ContextKeyValue` 在每个 partition 内保持独立；null tuple component、array tuple identity、多键生命周期及不同事件类型的 termination 均有 `TestDistinctInitiatedTerminatedContextBroadcastsAndSuppressesDuplicate`、`TestDistinctInitiatedTerminatedContextKeepsNullTupleStable`、`TestDistinctInitiatedTerminatedContextSupportsArrayTupleIdentity`、`TestInitiatedTerminatedContextTerminatesFromDifferentEventType` 覆盖。为支持 lifecycle source 与 statement source 不同，initiated-terminated 的 key/start/end 表达式现在在构建期只校验表达式树、变量、method/subquery 契约，字段绑定交由 runtime 对 incoming lifecycle event 解析；普通 keyed initiated context 的已有行为保持兼容。Java 的 same-key overlapping instances、pattern initiation/termination、termination output snapshot、完整 context event-type/filter registration、跨 statement transaction 和全部 Java trace 仍未完成。

本轮继续对照 Java `ContextInitTerm.ContextInitTermNoTerminationCondition`（`java-runtime-64de4048398d4829c445`）、`ContextStartEndNoTerminationCondition`（`java-runtime-4ae568a55d9923073521`）和 `ContextInitTermWithTermEvent` 的 overlapping 变体（`java-runtime-b22438598e5a61f21b63`、`java-runtime-ff056690ea7294649b4a`）：Go 增加 `NewInitiatedContext`/`NewOverlappingInitiatedContext` 及带 end 表达式的 `NewOverlappingInitiatedTerminatedContext`，同时提供对应 `Create` API。`initiated` 的 same-key start 会生成独立实例，active statement event 广播到所有实例，termination expression 在每个实例的 initiating-event/context scope 中分别计算；无 termination condition 的实例持续到 undeploy。并补上 overlapping 实例的 `BaseKey` 描述属性及基于 base key 的选择，保留唯一实例 `Key`/ID 用于精确定位。`TestInitiatedContextWithoutTerminationConditionIsNonOverlapping`、`TestOverlappingInitiatedContextCreatesBroadcastInstances` 和 `TestOverlappingInitiatedTerminatedContextCorrelatesEachInstance` 固定了重复 start、广播、base-key selector、唯一内部 partition key、逐实例终止和 boundary properties。随后新增的 initiated termination output slice 已覆盖 termination-time pending/snapshot output 及 count-based composition；pattern-based initiation/termination、跨 statement shared context instance/transaction、复杂 nested context overlap 与完整 Java trace 仍未完成。

因此当前新增三个验收动作：

1. 按 `common → common-avro/common-xmlxsd → compiler/runtime → regression-lib/regression-run → EsperIO → examples` 分片执行 Java 构建，保存每个模块的测试计数、环境依赖和耗时。
2. 已接入并运行 Regression execution inventory 探针；后续以 `compat/java-execution-inventory.jsonl` 的 `runtimeId` 核对 `executions()` 动态生成的参数化/配置变体与静态 `3,848` 候选项，并继续关联 JUnit 入口、源测试条目和实际 Java/Go trace。
3. 为 Go 端每个 capability 同时登记 Java 来源、Go Builder/AST/Runtime、正向/负向/边界测试、差分场景和结果状态；只有 Java/Go 两侧都完成并通过，才能从 prototype 看板进入 parity。

本轮补充修订：事件驱动 Pattern Context 已接入纯 timer root，覆盖 `TimerAt`/`TimerInterval`/`TimerSchedule`/`TimerCron` 的虚拟时钟启动、分区创建、Recurring overlap、分区 end timer、终止 snapshot，并在 statement 部署时锚定 timer 起点，避免首次大幅跳时漏掉已到期 occurrence。随后新增每个活动匹配独立持有 observer 状态的基础 timer-plus-event 组合：`TimerInterval(...).FollowedBy(...)`、`TimerInterval(...).And(event)` 和事件后的 `Then(TimerInterval(...))` 可在普通 Pattern 和 Context start/end 生命周期中按虚拟时钟推进，定时器到期后继续等待事件或在 And/序列分支完成；新增 `TestPatternTimerObserverComposesWithEvents`、`TestPatternTimerObserverAndEventCompletesOnClock`、`TestPatternEventThenTimerObserverCompletesOnClock` 和 `TestPatternTimerContextComposesTimerAndEventLifecycle`。当前仍只承诺这一基础组合子集，复杂 timer/guard/observer/consumption、完整 schedule/cron 组合、时区/DST/动态调度和完整 Java trace 仍待逐项移植。

本轮门禁复核：Go `go test ./...`、`go test ./... -race`、`go vet ./...`、manifest JSON/Go-test-name 校验均通过，核心包 `go test . -covermode=atomic` 为 75.6%；Java `regression-run` 的 `TestSuitePattern` 为 28/28、`TestSuiteContext` 为 17/17。覆盖率与 Java 回归结果只证明当前已登记切片可复现，不改变全量 Esper parity 尚未完成的结论。

本轮继续补齐 temporal Context 与终止输出的组合边界：Build 现在允许 `OutputSnapshotWhenTerminated`/其他 termination policy 作用于 periodic、daily、static cron temporal Context；窗口从 active 切换到 gap 或下一周期时，先在释放旧分区前重建当前状态快照、提交 termination assignments，再完成分区 deallocation。`TestTemporalContextTerminationSnapshotAtWindowBoundary` 以 periodic、daily、cron 三种窗口覆盖 start/end 边界、sum 快照和释放；该切片对照 Java `ContextInitTermTemporalFixed` 的虚拟时间/固定日历生命周期，并复用 Java initiated termination snapshot 的输出边界。复杂 temporal nesting、动态变量 schedule、temporal join/subquery/Named Window/FAF 及完整 Java trace 仍未完成。

本轮再补齐 nested Context 的一个安全子集：非 initiated parent 下的 keyed initiated child 现在保留 child start/end、按 parent key + child key 生成复合分区、隔离启动/终止和 context parent 属性；`TestNestedInitiatedTerminatedChildUsesParentPartition` 覆盖双 parent、child termination、descriptor parent key 与现有 nested selector。initiated parent、pattern child、temporal nested 仍显式拒绝，待后续实现完整 Java nested lifecycle/selector 矩阵。

本轮继续对照 Java `PatternTimerWithinOverDistinct` 与 `PatternEveryDistinctOverTimerWithin`：`EveryDistinct(...).Within(...)` 的 distinct key 现在按 Engine 虚拟时钟清理，新增显式 `EveryDistinctFor(key, expiry)` 链式 API；普通 Pattern 和 Pattern Context 都覆盖重复 key 抑制、时间到期后重新放行及非法零 expiry。对应测试为 `TestPatternEveryDistinctKeyExpiresOnVirtualClock` 和 `TestPatternContextEveryDistinctExpiresOnVirtualClock`。当前仍未覆盖 Java 的 distinct key 多级状态、复杂 guard/observer 消费、完整 optional expiry 组合和共享 trace。

门禁结果随后复核为：核心包 `go test . -covermode=atomic` 75.6%，`go test ./...`、`go test ./... -race`、`go vet ./...`、compat 与 manifest 名称校验通过；Java `TestSuitePattern` 28/28、无失败/错误/跳过。该结果仍只覆盖已登记切片，不代表 Esper 全量移植完成。

本轮补充 Pattern guard 对照：新增链式 `WithinOrMax(duration, maximum)`，将 Java `timer:withinmax` 映射为可嵌入 Pattern AST 的虚拟时钟 guard；它可保留在 `Then`/`And` 组合分支中，对 Every 分支按完成次数关闭，`maximum=0` 拒绝所有完成，且在 deadline 的同刻先关闭 guard 再处理后续事件。`TestPatternWithinOrMaxScopesEveryBranchAndVirtualClock`、`TestPatternWithinOrMaxComposesWithSequenceAndAnd` 与 `TestPatternContextWithinOrMaxStopsOverlappingPartitions` 覆盖普通 Pattern、Pattern Context、Every、序列、And、零上限和精确边界。该切片只补齐可观测的基础组合，EveryDistinct 嵌套 expiry、Until/Not/MatchUntil 的 guard 传播、消费/多父级转移以及完整 Java trace 仍未完成。

本轮再复核 Java `PatternTimerWithinOverDistinct` 与 `PatternEveryDistinctOverTimerWithin` 的操作符顺序：Go `Within` 现在总是生成 `patternWithin` AST guard，`EveryDistinct(...).Within(...)` 表示外层 guard，超时后整个 pattern 终止；`Within(...).EveryDistinct(...)` 表示每个 Every 分支各自持有 guard 和 distinct 状态。运行时补上 AST 根 `Every` 完成后的状态驻留，避免在连续事件间丢失 distinct key，并让 Pattern Context 自动把这类根重复 pattern 视为 overlapping start；固定 duration 的 `Within(...).EveryDistinctFor(...)` 还会把 expiry 保存在 AST 节点并按虚拟时钟清理。新增 `TestPatternEveryDistinctInsideWithinRetainsDistinctState`、`TestPatternEveryDistinctForInsideWithinExpiresNodeState` 与 `TestPatternContextEveryDistinctInsideWithinKeepsOnePartitionPerKey`，同时修正外层 Within 的对照期望。当前仍未完成 distinct expiry 与内层 guard 的完整跨分支生命周期、dynamic/calendar duration、复杂 nested observer/guard、Until/Not/MatchUntil 传播、consumption policy 和 Java/Go 共享事件 trace；这些不能由本轮顺序测试推断为完整 parity。

本轮继续对照 Java `PatternGuardTimerWithin.PatternWithinFromExpression`、`PatternWithinMayMaxMonthScoped` 和 `PatternIntervalPrepared`：Go 增加 `DurationSeconds`/`DurationMilliseconds` 可分析表达式，以及 `PatternStream.WithinExpr`、`WithinOrMaxExpr`、`WithinCalendar` 和 `WithinOrMaxCalendar` 链式入口。动态 duration 在 guard 分支真正 armed 时解析，可读取已捕获的 Pattern tag 或 `DeployWithParameters` 的绑定；calendar guard 使用 `time.Time.AddDate`，不把 month 粗略换算成固定小时。运行时补上序列右分支、Every 重启和 Context seed tag 的传递，并在 deadline 同刻先终止 guard。`TestPatternWithinExpressionUsesCapturedTagDeadline`、`TestPatternWithinExpressionUsesDeploymentParameter`、`TestPatternWithinCalendarUsesMonthBoundary`、`TestPatternContextWithinCalendarStopsNewPartitionsAtMonthBoundary` 覆盖普通 Pattern、参数化、Pattern Context、Every、日历边界和 just-before/exact-deadline。Java 的多组件 duration 表达式、动态 `timer:interval` observer、microsecond/ISO period、完整 nested guard/observer/consumption 与共享 Java/Go trace 仍未完成；本轮 API 是已登记的可运行切片，不代表 Pattern 全量 parity。

本轮继续对照 Java `PatternObserverTimerInterval` 的 `PatternIntervalSpec`、`PatternIntervalSpecVariables`、`PatternIntervalSpecExpression`、`PatternIntervalSpecPreparedStmt`、`PatternMonthScoped` 和 property-array 变体：Go 增加 `DurationDays`/`Hours`/`Minutes`/`Seconds`/`Milliseconds`/`Microseconds`/`Nanoseconds` 与可分析的 `DurationSum`，以及 `TimerIntervalExpr`、`TimerIntervalCalendar`。动态 interval 在 observer branch armed 时解析，可读取前序 Pattern tag、重复 tag 的 `TagFieldAt`、变量或 `DeployWithParameters` 绑定；根 timer、普通 Pattern 组合、Pattern Context 和月周期 recurrence 共用同一虚拟时钟 deadline 内核，calendar occurrence 使用 `time.Time.AddDate`。`TestPatternTimerIntervalExpressionUsesCapturedTag`、`TestPatternTimerIntervalExpressionUsesRepeatedTagValues`、`TestPatternTimerIntervalExpressionUsesComponentParameters`、`TestPatternTimerIntervalCalendarUsesMonthRecurrence`、`TestPatternTimerIntervalCalendarContextCreatesMonthlyPartitions` 覆盖 just-before/exact boundary、组件参数、重复 tag property-array 和 Context 分区。Java 的变量动态重配置、property-array/复杂表达式全矩阵、微秒/ISO period 语法、observer 与 guard/consumption 的嵌套组合和双方共享 trace 仍未完成。

本轮再补上 Java `PatternMicrosecondResolution` 对 timer interval 的纳秒级内部表示验证：`DurationMicroseconds` 在 `TimerIntervalExpr` 中可将 1µs deadline 精确落到 Engine 虚拟时钟，`TestPatternTimerIntervalMicrosecondPrecision` 覆盖 1ns-before 与 exact boundary。该测试只证明 Go clock/kernel 的精度切片；Java 的 ISO period、全部 observer 解析形式、DST/时区和共享 Java/Go trace 仍需单独映射。

本轮还修正了 Context 参数依赖的计划遍历遗漏：`queryParameterTypes` 现在递归扫描 context key/category/start/end、cron 和 start/end Pattern AST，确保 Context 内的 `TimerIntervalExpr`/guard 参数必须通过 `DeployWithParameters` 绑定，并复用同一类型冲突校验。`TestPatternTimerIntervalContextRequiresDeploymentParameters` 覆盖无绑定拒绝、参数化部署、虚拟时钟 deadline 和分区事件派发；Context 的变量动态重配置、跨 statement 参数快照和完整 Java deployment trace 仍未完成。

随后补上 Java `PatternIntervalSpecVariables` 的 Go 对照切片：`TimerIntervalExpr` 的 duration 表达式现在以 `VariableRef` 组合分钟/秒变量，并通过 `TestPatternTimerIntervalExpressionUsesVariables` 验证构建期变量依赖、变量初值、just-before/exact deadline 和虚拟时钟输出。这里仅证明变量表达式在 observer armed 时可求值；Esper Java 的变量版本快照、已排队 observer 对变量修改的重调度语义、跨 Context/statement 的可见性和共享 Java/Go trace 仍需单独对照，不能把一次初值测试解释为动态重配置 parity。

本轮继续对照 Java `EventBeanJavaBeanAccessor`、`EventBeanExplicitOnly` 和 `EventBeanPublicAccessors`：Go 事件 schema 新增 `WithPropertyGetter`/`WithTypedPropertyGetter`、`WithPropertyPath`、`WithPropertyMethod` 与 `WithNestedPropertySchema` 链式选项。`AccessorJavaBean` 会在 schema 构造时把 `GetX`/`IsX`（含 acronym decapitalization 和 `(value,error)` 形式）纳入 `PropertyNames`/`Properties`/`PropertyType`，`AccessorExplicit` 对 `StructSchema` 只发布注册的字段、路径和方法；方法既支持零参数 getter，也支持 indexed/mapped path 直接传参并在失败时回退到返回 slice/map 后再索引。注册的 nested schema 现在同时驱动 `GetFragment`、nested JSON/XML render 和 nested property metadata，显式 accessor/path/method/nested 信息也进入 Plan canonical identity。`TestJavaBeanAccessorPublishesGetterMetadataAndRenders`、`TestExplicitAccessorFieldMethodPathAndNestedSchema`、`TestExplicitAccessorRejectsInvalidRegistration`、`TestPublicAccessorKeepsFieldsAndAddsExplicitMethods` 和 `TestAccessorRegistrationEntersPlanCanonicalIdentity` 覆盖正向元数据、字段/方法/嵌套 schema、索引/映射、fragment、非法注册、未注册属性 Missing 与 plan hash 差异，并已登记为 `event.property-access-render` / `case.event-property-access-render`。Java `TestSuiteEventBean` 与 `TestSuiteEventBeanWConfig` 本轮固定为 19/19 通过。XML/Avro/JSON 全表示的 fragment/metadata、DOM/XPath、Java provided-underlying 和双方共享 render trace 仍未完成；本轮不把 accessor slice 宣称为事件模型完整 parity。

本轮继续对照 Java `RowRecogArrayAccess.RowRecogLambda`、`RowRecogClausePresence`、`RowRecogDataSet`、`RowRecogMultikeyWArray`、`RowRecogAfter`、`RowRecogInterval` 和 `RowRecogRegex`：Go 在链式 Match Recognize 表达式层新增 `TagSum`、`TagAvg`、`TagMin`、`TagMax`、`TagFirst`、`TagLast`、`TagAny`、`TagAll`、`TagEvents` 和 `TagSize`。这些构造器把 tag 名与元素表达式保留在 AST 中，在 DEFINE 候选分支和 MEASURES 结果阶段按重复捕获顺序求值；可选 tag 的聚合返回 Null、`TagSize` 返回 0、`TagAll` 遵循空集合为 true，未知 tag 在 Build 阶段报 `ErrorUnknownName`。`TestRowRecogTagAggregatesAndEnumeration` 覆盖 A* B 的重复/可选捕获、数值聚合、枚举 any/all、事件数组复制、边界值和未知 tag 负例；`TestRowRecogRegexMatrix` 现在覆盖 RowRecogRegex 全部 12 组组合模式及 listener/Snapshot 结果；RowRecogAfter 的三种 skip、重复变量、分区和 past-last listener/Snapshot 矩阵也已新增；`TestRowRecogIntervalSimpleTrace`、`TestRowRecogIntervalPartitionedTrace`、`TestRowRecogIntervalMultipleCompletedTrace` 和 `TestRowRecogIntervalMonthScopedTrace` 已映射 RowRecogInterval 的四个 execution，固定 pending/deadline、分区和月边界语义。Java `TestSuiteRowRecog` 本轮 Maven 运行 23/23 通过。RowRecog 的完整差异仍包括 time/time-batch row window、engine-wide max-states/prevent-start、variant/multikey array、复杂嵌套 NFA、iterate-only、invalid diagnostics、tag-aware PREV/PRIOR、`RowRecogIntervalResolution`、`IntervalOrTerminated` 全组合、性能及共享 Java/Go trace；因此清单中的 `case.rowrecog-inventory` 只提升为 partial，不视为完整回归覆盖。

本轮继续对照 Java `ConfigurationRuntimeMatchRecognize`、`RowRecogStatePoolRuntimeSvc`、`ConditionMatchRecognizeStatesMax` 以及 `TestSuiteRowRecogWConfig`：Go 增加 `MatchRecognizeRuntimeConfig` 与链式 Engine option，默认关闭全局上限并保持 `PreventStart=true`；当状态池超限时，`MatchRecognizeStateLimitEvent` 在 engine lock 外按 registration order 回调，并携带 MaxStates 与 deployment/name 组合键对应的 statement 计数。RowRecog 运行时按 admitted start branch 接入池计数，`preventStart=true` 会保留被拒绝起始行但禁止它成为新的匹配起点，`false` 会记录超限后继续接纳；完成、窗口删除、Context 分区释放和 statement undeploy 都归还计数。`TestRowRecogEngineWideMaxStatesPreventStart`、`TestRowRecogEngineWideMaxStatesNoPreventStart`、`TestRowRecogEngineWideMaxStatesAcrossStatementsAndContext`、`TestRowRecogEngineWideMaxStatesNamedWindowRemoval` 与 `TestRowRecogEngineWideMaxStatesUndeployReleasesPool` 已覆盖这些 Go 边界。Java `TestSuiteRowRecogWConfig` 在 JDK 17/Maven 3.9.16 下运行 4/4 通过；Go 当前仍需补齐 Java NFA 内部每个 transition/重复分支的精确 state-count、microsecond configuration、完整 3/4-instance trace 和性能回归，因此该切片是 engine-wide resource-policy 映射，不宣称完整 RowRecog parity。
