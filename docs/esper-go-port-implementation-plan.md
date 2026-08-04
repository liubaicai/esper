# Esper 9.0.0 Go 全量移植规划实施文档

## 1. 文档信息

| 项目 | 内容 |
|---|---|
| 文档状态 | Draft 0.2，二次查漏后的实施前评审稿 |
| Java 对照项目 | D:/Code/soc/esper |
| Java 基线 | Esper 9.0.0，tag release_9.0.0，commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c |
| Java 要求 | Java 17 |
| Go 目标项目 | D:/Code/soc/bigsoc-esper |
| 目标仓库现状 | 当前工作树为空；HEAD 454c9b25e 是在 Java 基线之上清空文件的提交 |
| Go 工具链现状 | 本机 go1.25.5；最低支持版本在阶段 0 固化，候选为 Go 1.25 |
| 核心 API 方向 | Flink DataStream 风格的 Go 链式 API，不以 EPL 字符串作为规则定义方式 |
| 规划原则 | 对照 Java 行为完整移植，采用 Go 架构与习惯，不逐类、逐包机械翻译 |

本文是实施规划，不包含 Go 实现代码。文中出现的方法名和调用链只表示 API 形态，不是已经承诺的最终签名。

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
- regression-lib 静态扫描到约 3,848 个 RegressionExecution 实现。
- regression-run 静态扫描到约 860 个 public test 入口方法。
- examples 下有 17 个示例项目，需要按用例价值转换为 Go 示例或端到端测试。
- 回归标签包含多线程、性能、无效输入、即席查询、序列化、数据流、运行时操作、编译器操作和事件发送器等维度。
- 除 regression-lib 外，common/compiler/runtime/common-avro/common-xmlxsd 共 371 个 Java 单元测试文件、EsperIO 共 58 个测试文件，也必须逐项分类；不能只迁移 RegressionExecution。
- 17 个示例为 autoid、benchmark、cycledetect、marketdatafeed、matchmaker、namedwinquery、ohlcpluginview、qos_sla、rfidassetzone、runtimeconfig、servershell、stockticker、terminalsvc、terminalsvc-jse、transaction、trivia、virtualdw。

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
- distinct、聚合、访问聚合、插件聚合、局部分组。
- count/sum/avg/min/max、first/last/window、firstever/lastever、nth、rate、median、stddev、avedev、sorted/maxby/minby 和 Count-Min Sketch 等内建聚合族。
- group by、grouping sets、rollup、cube、grouping/grouping_id、having。
- row-per-event、row-per-group、aggregate-all 和非聚合结果模式。
- order by、limit/offset、变量行数限制。
- output first/last/all/snapshot、按事件数/时间/日历/条件输出。
- output after、cron、when/then、默认 stream selector 和 output-limit 优化开关。
- grouped/discrete delivery、条件输出后的变量更新和 update-istream。
- istream、rstream、irstream 以及旧值/新值配对。
- into-table 聚合与表列访问。
- event-precedence 插入/派发顺序及其运行时配置开关。
- 无 from/source 的 select、仅 Context 的 statement 和 FAF 查询。

### 5.5 连接、子查询、历史流和空间能力

必须覆盖：

- 二流到多流连接、自连接、内连接、左/右/全外连接。
- 单向流、保留关键字语义、窗口组合和连接结果顺序。
- 哈希、范围、复合、唯一索引及查询计划选择。
- 相关/非相关子查询、exists/in/quantified、聚合子查询。
- Named Window/Table 子查询和索引复用。
- database/sql 历史查询、方法流/拉取流、缓存和参数绑定。
- 空间点/矩形查询及空间索引。
- 查询计划 hint 和排除策略中平台无关的行为。

database/sql 适配不能只做到“能查询”，还需对照占位符方言、metadata 与列名大小写转换、null 映射、自定义输入/输出转换 hook、连接生命周期、LRU/expiry 缓存、取消，以及 prepared query 的 Close 语义。

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
- 逐项盘点 371 个核心模块单元测试、58 个 EsperIO 测试和 17 个示例，记录 Go 对应测试或 N 级理由。
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
- into-table 的聚合基础。

对应 Java 套件：resultset、expr aggregation、部分 infra table。

### 阶段 4：Join、Subquery、历史流和空间索引

范围：

- 全部连接类型和多流查询计划。
- 子查询、Named Window/Table lookup。
- database/sql 历史流、方法流和缓存。
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
2. 源测试处置覆盖率：371 个核心单元测试文件、58 个 EsperIO 测试文件和 17 个示例均有 Go 测试映射、合并映射或经评审的 N 级理由，发布要求 100%。
3. Go 语句覆盖率：衡量 Go 实现被测试程度，不能代替前两项。

### 10.2 兼容性清单

计划建立一个版本化、机器可读的 traceability manifest，包含 Capability、Case 两类实体及其多对多关联。

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

模块单元测试、EsperIO 测试和示例进入同一数据库的 source-test 条目，记录 one-to-one、many-to-one、replacement 或 N disposition。many-to-one 必须列出覆盖断言，避免用一个宽泛 Go 测试虚假吞并多个 Java 测试。每个 S/G capability 至少关联一个正向和一个适用的边界/无效测试；每个适用 case 必须关联 capability，防止“有功能无测试”和“有测试无范围归属”。

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
