# Esper Go 移植：运行时性能与执行模型记录

> 文档定位：记录已完成的运行时性能修复（证据与复现方式）以及**尚未实施**的性能/执行模型工作项，
> 后者与移植工作单元一起安排，不单独放宽差分、证据或 NFR 门禁。
> 当前状态数字见 `testdata/compat/capability-manifest.json`；工作单元流程见
> [执行手册](esper-go-port-runbook.md)；质量与验收口径见 [质量策略](esper-go-port-quality-strategy.md)。

## 1. 背景

应用侧（`bigsoc-app`）以本模块为核心执行规则，报告了两个运行时性能问题：

1. 事件属性读取按字段名反射扫描结构体且无缓存；
2. 无状态过滤规则处理路径过重。

对照固定 Java oracle（`9e1b9f1cc9117fea4bf33ab043762c045d73839c`，本地检出 `D:/Code/soc/esper`）确认：
**属于移植缺口，不是 Esper 原项目行为**。原版按属性名缓存 getter（`BeanEventType` 的
`propertyGetterCache`）、复用已解析的 `Field`/`Method`（`ReflectionPropFieldGetter` /
`ReflectionPropMethodGetter`），并在可用处生成直接成员访问（`ExprIdentNodeEvaluatorImpl`）；
运行时通过事件类型/过滤索引把事件映射到匹配回调（`EventTypeIndex`、`FilterParamIndexEquals`、
`FilterHandleSetNode`），而不是对全部语句无条件遍历。

## 2. 已实施修复

### 2.1 P0：属性访问元数据缓存（`property_access.go`）

- 以 `reflect.Type` 为键构建**不可变**字段表 `structFieldTable`：按原先"按序深度优先"的候选顺序
  预先记录字段下标路径与选中名称（`esper` tag → `json` tag → Go 字段名），运行时按 canonical 名称
  做一次 map 探测加一次路径行走，替代每次读取都线性扫描字段与解析 tag。
- 语义保持不变：tag 优先级、`esper:"-"` 跳过、匿名嵌入展平与**未解析候选回退**（如匿名指针为 nil
  时继续后续候选）、递归嵌入终止、Go 字段名与 tag 名双匹配、`PropertyCaseInsensitive` /
  `PropertyDistinctCaseInsensitive` 的大小写折叠语义。
- 大小写不敏感索引使用**简单折叠轨道最小值**（`unicode.SimpleFold`）作为键，与
  `strings.EqualFold` 等价；`strings.ToLower` 不等价（例如 U+212A KELVIN SIGN 与 U+1E9E 大小写对，
  ToLower 不映射），因此不能用作索引键。
- 只缓存类型与访问元数据，不缓存事件值、不捕获 `reflect.Value`；键为 `reflect.Type`，增长有界。
- `Schema.get` 对无路径语法的普通属性名直接走单属性读取，跳过 `parsePropertyPath` 与段循环
  （空名、首尾空白、`.`/`[]`/`()`/引号/转义/`?` 一律仍走解析路径）。
- 公共 API、`Fields()`、`Getter()`、`AccessorStyle()` 与 Plan canonical/hash 均未改变。

### 2.2 P1：无状态过滤内部执行特化（`stateless.go`）

- 对可证明为"接受即输出一行通配结果"的语句解析出内部快速计划：单一静态 struct 事件源 +
  纯谓词过滤链 + 通配投影 + 默认输出策略 + 默认 istream 选择器，且不含 view/join/pattern/
  rowrecog/trigger/update/context/route/distinct/order-by/limit/subquery/on-demand/sink/
  output 策略。
- 谓词必须通过显式**纯表达式白名单**（`statelessPureKinds`）：只允许读取事件与字面量的节点；
  `udf` 节点仅当由本包自身的确定性函数构造并带 `pureBuiltin` 标记（`Lower`/`Upper`/`Trim`/
  `StringLength`）时才允许，用户 `Func*`、脚本、插件、方法、变量、参数、tag、join/pattern 作用域、
  history/prev/prior、时钟、输出计数、子查询一律不合格并继续走通用管线。
- 属性读取额外要求：谓词引用的属性名必须是**无路径语法的普通名**（`child.value`、`items[0]` 这类名称会
  切换到嵌套 schema，其 getter 不在本判定范围内），必须由源 schema 声明（未声明名称在运行时会走动态解析，
  可能调用同名 Go 方法），且源 schema **没有注册 getter / JavaBean 访问器**（`WithPropertyGetter`、
  `WithPropertyMethod`、`WithPropertyPath`、`AccessorJavaBean` 发现）——这类读取会执行用户代码，
  本计划不得改变其调用次数。
- 运行时再校验事件 schema 与计划证明时使用的源 schema 是同一个（schema 身份指针）：事件若携带其他
  schema（例如子类型事件），同一属性名可能经注册 getter 或动态方法解析，因此该事件回退到通用管线。
- 多槽位投递（`multiMatch`，即 Java 的逐元素 IN 投递）被显式排除，因此快速路径一个接受事件恰好
  产生一行。
- 命中时直接产出 `{Time, New:[event], outputInserted:1, Sequence}` 批次，不再构建
  `eventDelta`/history 映射/投影/输出策略/变量装配；拒绝时返回与通用管线一致的"空计数批次"，
  顺序号不消耗。
- 派发循环已计算的过滤结果（`matchesEventFilter`，用于 unmatched 监听器与指标/审计记账）被直接复用，
  不再二次求值；在派发路径本就未计算该结果的场景（routed 分发的 `!needsAccepted`），快速路径按
  与通用管线完全相同的函数与变量装配自行求值一次，语义与求值次数均不变。
- 语句停止/销毁状态检查、互斥、输出赋值收集、监听器派发、指标与审计调用点保持不变。

### 2.3 派发顺序缓存

`Engine.dispatchStatementsLocked` 的结果（按 priority/drop/部署序的稳定顺序）在语句目录变化前保持
不变，现缓存于 `Engine.dispatchOrder`，在部署、卸载与 rollout 回滚三处目录变更点失效。消费者只读取
该切片。

### 2.4 P2：派发路径固定分配削减（第二轮）

针对 §2.2 快速路径语句的剩余固定成本（此前 profile：accepted 16 allocs/op，其中快速路径自身
仅约 1 个），按"零可观测语义变化"实施六项削减与两项随之暴露的 nil 安全修复：

- **纯谓词语句跳过变量装配**：`Statement.matchesEventFilter` 在 `statelessPlanResolved` 且计划
  非空时跳过 `statementVariables` + `variablesWithEngineLockState`（后者在快照为 nil 时每次
  `make(map)` 并装箱 engine ref，约占 8% CPU 与 2.5 allocs/op）。计划已证明整条谓词链只读
  事件字段与字面量，变量/参数/子查询/用户代码不可能观测该次求值；字段只在 `s.mu.Lock`
  （`Statement.process`）下写入，此处在 `RLock` 下读取是同步的。无计划语句保持原装配。
- **事件入口空变量快照**：`Engine.send` 改用 `snapshotVariables`（AdvanceTime 入口自
  b8d8780c9 已用），引擎未声明变量时不再克隆出空 map。随之暴露两处 nil 写入并修复：
  `bindParameterValues` 对 nil 快照物化结果 map（此前直接向 nil map 写入 panic，
  `TestClientCompileLargeSubstitutionParamsMatchesEsper` 可复现）；`executeVariableTriggerAction`
  的触发器写回循环对 nil 快照跳过（引擎级变量被写时快照必非 nil，nil 时只有分区变量写入，
  无可写回目标）。
- **栈背衬派发切片**：`routedQueue`（仅被入站事件种子，路由事件走
  `processPendingRoutedEventsLocked`）与 `deferredTriggerDispatches`（上限 2，超出自动堆溢出）
  改栈数组，每发送各省 1 次分配。
- **惰性排空上下文**：`finishExternalRoutes` 的 `context.WithoutCancel` 延迟到弹出首条外部
  路由时构造；空队列退出路径的控制流、draining/dispatching 标志与错误合并完全不变。
- **typeNames 免复制读取**：新增 `Environment.typeNamesShared`（缓存条目只被整体替换、绝无
  原地修改，RLock 下共享读安全），`SendEvent` 瞬态消费（长度检查、首名、错误文本）不再每次
  复制名字切片。其余调用方（`schemaForGoType`、`materializeDataflowEvent`，冷路径）不变。
- **监听器快照缓存**：`Statement.listenerSnapshot` 在 `s.mu` 写锁下于全部 6 个监听器变更点
  （`Subscribe`、`SubscribeWithReplay`、`removeSubscription`、`cleanupPreparedStatementLocked`、
  `markClosedLocked`、构造空表）重建，`dispatchSync` 在 `RLock` 下读取，替代每批次收集 ID、
  排序、物化两个切片。`nextSubID` 永不复位，升序 ID 即订阅顺序，顺序契约不变；
  `metrics.go` 的 `len(listeners)` 读者仍以 map 为准；dispatch 侧保留"快照缺失即地重建"的
  防御回退。
- **字面量预装箱**：`Literal[T]` 在构造期装箱一次 `Value`，求值返回同一不可变值，消除每次
  求值的接口装箱（约 1.6 allocs/op）；Plan 身份仍取自节点描述与子树，不受影响。

追加发现（未在本单元修改）：panic 若发生在持有 `e.mu` 的派发循环内，deferred
`finishExternalRoutes` 的 `e.mu.Lock` 会在栈展开时自死锁（本单元修复的 nil map panic 即触发过，
表现为测试挂起而非报告 panic）。属既有结构问题，留待独立工作单元论证修复。

### 2.5 P3：纯等值过滤共享候选索引

派发阶段新增 Esper 内部共享候选索引（`shared_filter_index.go`）。仅满足单一静态 struct 源、无状态纯过滤
计划，且过滤链中存在字段与字面量必要等值条件的语句进入索引；当前索引值类型限于字符串和布尔值。索引
只负责排除不可能命中的语句，保留的语句仍由原始 Esper 表达式执行，因此不会改变三值逻辑、数值类型强制、
用户函数、子查询或结果投影语义。数值等值和其他复杂谓词继续走原有路径，避免类型强制造成错误候选排除。

索引在部署时注册、卸载时移除，使用事件 schema 身份和可复用 generation 标记，避免每个事件分配候选 map；
路由事件在替换事件后重新计算候选集。现有事件类型接受索引仍先执行，两个索引均只作为安全剪枝。

在本机 Ryzen 7 PRO 6850HS 上，`BenchmarkAcceptIndexMultiStatement` 从基线 **23.6 µs/op** 降至
**4.07 µs/op**（64 条多类型等值 filter，`-benchmem -benchtime=1s`）；命中结果、子查询保护和数字等值回退
测试通过。

## 3. 验证与证据

### 3.1 差分回放（固定 Java trace，未改动）

改动前后对既有重放链运行 `cmd/parity`，全部 `passing`、0 differences：

| 场景 | 覆盖 | Java runtime IDs |
| --- | --- | --- |
| `event-bean-property-fragment` | 原生 bean/map/object-array/wrapper/嵌套/索引/映射片段、null 元素 | 15 |
| `expr-filter-optimizable` | IN/NOT IN 多值、OR→IN 重写、typeOf、UDF、变量、context、部署期常量 | 9 |
| `expr-core-logical` | and/or/not、null 语义、变量 | 3 |

Java 侧 trace 由固定检出重新生成并确认与仓库内 `testdata/parity/*.trace.json` 逐字节一致（未修改
oracle 或场景资产）。**本单元不新增 capability/DV/NFR 状态**；manifest 计数不变。

### 3.2 性能与分配（同一份基准源码，改动前后各编译一次）

环境：Windows 开发机，`GOMAXPROCS=1`，Go 1.25.5。基线为 `7704c1f` 的独立 git worktree。

1. 单条过滤规则、字段位置扫描（本轮临时基准，不在仓库内；末列为分配次数）：

| 结构体字段数 / 命中位置 | 改动前 | 改动后 |
| --- | ---: | ---: |
| 16 / 首字段 | 7.04 µs, 23 allocs | 2.74 µs, 10 allocs |
| 16 / 末字段 | 11.93 µs, 53 allocs | 2.44 µs, 10 allocs |
| 128 / 末字段 | 64.32 µs, 277 allocs | 5.66 µs, 10 allocs |
| 512 / 首字段 | 15.09 µs, 23 allocs | 8.92 µs, 10 allocs |
| 512 / 末字段 | 258.44 µs, 1557 allocs | 9.02 µs, 10 allocs |
| 16 / 首字段（命中） | 11.00 µs, 29 allocs | 5.85 µs, 15 allocs |

2. 应用形态候选集（213 字段事件、278 条双条件规则、每规则一个引擎、逐条 `SendEvent`，本轮临时基准）：

| 规则形态 | 改动前 | 改动后 |
| --- | ---: | ---: |
| `Contains` + `Contains` | 69.55 µs/候选，445 allocs/事件批 | 6.27 µs/候选，10 allocs/事件批 |
| `Lower`+`Contains` / `Lower`+`Equal`（对应应用 icontains/ieq 形态） | 63.05 µs/候选，449 allocs/事件批 | 6.89 µs/候选，12 allocs/事件批 |

上表为改动前后二进制在同一台空闲机器上连续运行所得（`GOMAXPROCS=1`）；早前一轮与 `-race` 全量门禁并发运行时读数偏高，故以本表为准。

3. 仓库内常驻基准（`go test ./internal/esper -bench 'PropertyAccess|StatelessFilter' -run '^$'`）：
   `BenchmarkPropertyAccessByFieldPosition`、`BenchmarkStatelessFilterSend`。

4. 常驻回归测试：
   - `TestPropertyAccessAllocationIsPositionIndependent`（字段位置无关的分配不变式；
     改动前该形态为 22 vs 148 allocs，会失败）
   - `TestStructPropertyFallbackFollowsCandidateOrder`（nil 匿名候选回退与候选顺序）
   - `TestStructPropertyResolutionTerminatesOnRecursiveEmbedding`（注册期拒绝 + 访问器不循环）
   - `TestStructPropertyTagPrecedenceUnchanged`、`TestStructPropertyCaseInsensitiveFoldingMatchesEqualFold`
   - `TestStatelessFilterListenerSemantics`（值/时间/顺序号/停止/重启/卸载）
   - `TestStatelessFilterKeepsUserCodePredicateSemantics`（用户函数谓词仍在通用路径正确判定）
   - `TestStatelessFilterKeepsGetterBackedPropertySemantics`（注册 getter 的属性读取排除在快速路径外，
     用户代码调用次数保持通用管线行为）

### 3.3 第二轮（§2.4）的同机交替 A/B 证据

方法：改动前后源码树交替各跑两轮（`GOMAXPROCS=1`，`BenchmarkStatelessFilterSend -benchmem
-count 1`），对消机器漂移；分配计数四轮全部一致。

| 形态 | 改动前 | 改动后 |
| --- | ---: | ---: |
| rejected（拒绝事件） | 2965/3487 ns，1321 B，10 allocs | 2519/2589 ns，857 B，**4 allocs** |
| accepted（命中并派发） | 3348/4294 ns，2080 B，16 allocs | 2653/3150 ns，1577 B，**8 allocs** |

分配恰好减半；耗时中位约 -20%，字节数 -24% ~ -35%。同轮门禁：`go vet ./...`、全量
`go test ./... -count=1`、定向 `-race`（stateless/statement/engine/subscribe/listener/variable/
ClientCompileLarge）与三条差分链（§3.1 同一命令）全部通过；`TestClientCompileLargeSubstitutionParamsMatchesEsper`
在修复前可稳定复现 nil map panic（并触发上述 deferred 自死锁），修复后 0.02s 通过。

`make check` 中的 `check-layout.sh` 在本机经 WSL 调用且 WSL 内无 gofmt，属环境限制；其等价项
（vet、gofmt -l、全量 test）已分别通过。

### 3.4 第三轮（§4.7 编译谓词 + §4.1 类型级裁剪）的同机 A/B 证据

方法：§4.7 用源码树交替两轮（`git stash -u`）；§4.1 的多语句基准在 HEAD 独立 worktree
运行同一基准源（公共 API 自包含副本）。`GOMAXPROCS=1`。

| 基准 | HEAD（`e77d1880e`） | 本轮 | 备注 |
| --- | ---: | ---: | --- |
| StatelessFilterSend/rejected | 2008/2319 ns，4 allocs | 1402/1607 ns，4 allocs | §4.7 单独效果，-32% |
| StatelessFilterSend/accepted | 3908/4663 ns，8 allocs | 2012/2139 ns，8 allocs | §4.7 单独效果，约 -50% |
| AcceptIndexMultiStatement（64 语句/2 类型） | 84232/85535 ns，68 allocs | 19988/21825 ns，**4 allocs** | §4.7+§4.1 合并，约 4x / 17x |

单语句引擎在裁剪下零回归（>1 语句才计算事件名集合，集合按类型名缓存）。门禁：全量
`go test ./...`、定向 `-race`（含 accept-index/contained/unnest/variant/join/pattern/
insert-into/named-window 族）、`go vet`、gofmt、三条差分链全部通过。

> 说明：上表为受控微基准与本机数据，不是端到端 EPS 结论，也不构成 `nfr-verified` 登记。

## 4. 后续性能与执行模型工作项（尚未实施）

以下工作项与本移植一起安排：先完成对应 capability 的 Java/Go 语义证据，再做性能改动；任何一项
都不得放宽差分、证据或 Plan 身份约束。

### 4.1 过滤服务索引（Java `FilterService` 等价物）

状态：第一阶段已实施（事件类型级候选裁剪，`accept_index.go`）。`EventTypeIndex` 等价物：
语句在 prepare 期解析出**事件类型接受描述符**（全部源为具名普通 schema 时可裁剪；variant、
contained/unnest、historical/method 源，context、update-istream、子查询注册表、输出策略语句
一律不可裁剪），事件侧按类型名计算一次接受名集合（普通事件 = 自身类型名 + `parentNames`
传递闭包，镜像 `acceptsEventType` 的遍历；routed 事件 = 仅 `StreamType` 精确名，镜像
`sourceNodeAcceptsEvent` 的 routed 早退分支），集合按类型名缓存在引擎上（schema 父链不可变），
派发环对可裁剪语句做一次集合探测即跳过——被跳过语句本会贡献 accepted=false/changed=false，
指标采样、计数与审计记录全部以 accepted/changed 为门（metrics.go:655-675、audit.go:393-452），
故跳过无可观测差异。替换事件（update-istream 的 copy-on-write）后重算集合。派发顺序、
priority/drop 屏障、unmatched 记账不变（同序遍历、逐语句跳过）。
第二阶段（`FilterParamIndexEquals` 等值/IN/范围属性索引）尚未实施：同类型多语句仍逐个求值谓词。

- 证据：`accept_index_test.go` 四守护回归（仅非接受语句被跳过且 unmatched 记账不变、子查询
  语句不被裁剪、父类型接受穿越裁剪、variant 语句不裁剪）；contained/unnest/variant 经
  join/pattern 的三处失败驱动修复 `prunable` 单向置位 bug（`addName` 曾把已判不可裁剪的语句
  翻回可裁剪）；`BenchmarkAcceptIndexMultiStatement`（64 语句/2 类型单引擎）对照 HEAD worktree
  同基准：84.2/85.5 µs、68 allocs → 19.9/21.8 µs、**4 allocs**（约 4x 时延、17x 分配，
  与 §4.7 编译谓词的合并效果）。单语句引擎零开销（>1 语句才计算集合，且缓存命中零分配，
  `BenchmarkStatelessFilterSend` 维持 4/8 allocs）。

### 4.2 单次谓词求值（消除重复求值）

- 现状：通用路径对同一事件求值两次——派发循环的 `matchesEventFilter` 与执行内的插入/过滤判定。
  已实测：含用户函数的谓词每次发送被调用 2 次（Java 为 1 次），拒绝事件同样如此。
  §4.9 的追加审查按实际影响把本项提升为第 2 优先级（同时消耗 CPU、闭包调用与临时分配，
  收益覆盖全部非快速路径语句）。
- 快速路径仅在可证明纯谓词上消除第二次求值；非快速路径的语句仍是 2 次。
- 目标：把"过滤结果"作为派发循环的唯一产物向下传递，由 unmatched/指标/审计消费者按需使用，
  使所有语句与 Java 一样只求值一次。
- 风险：语句指标的采样窗口（当前在 process 之前开始）与审计 `accepted` 字段的取值必须保持；
  用户函数/脚本的调用次数是对外可观测的，任何变化都需要单独论证与证据。

### 4.3 单流无 view 语句的 WHERE 下推

- Java 在满足条件时把 `WHERE` 下推到流过滤器（`StatementRawCompiler`：恰好一个 `FilterStreamSpecRaw`、
  无视图、无 on-trigger/子查询/on-demand/表访问，且不含子查询、`DISABLE_WHEREEXPR_MOVETO_FILTER`
  提示、视图资源表达式与 contained 属性求值时）。
- Go 链式 API 由调用方决定 `Filter` 与 `Window` 的相对位置；`From.Window(...).Filter(...)` 目前保留
  视图后再过滤，与 Java 的下推结果不同（状态规模与求值成本更高）。
- 可作为独立工作单元：实现同等的受限下推并给出 old/new 流、窗口状态与错误阶段证据。

### 4.4 结果集处理器特化（HANDTHROUGH / 简单处理器）

- Java 在无分组聚合、无 order-by/having/limit 时选择 HANDTHROUGH，否则走生成的特化简单处理器
  （`ResultSetProcessorFactoryFactory`、`ResultSetProcessorHandThroughImpl`）。
- Go 侧通配无窗口语句已由 P1 覆盖；带视图/选择/去重/排序/限流的语句仍在通用路径构建 delta 与
  history 映射。可按 Java 的形态判定逐步特化，每步配 old/new、输出策略与顺序证据。

### 4.5 属性访问面的其余表示

- P0 覆盖 Go struct 事件；map/JSON/Avro/object-array 事件仍走各自的 canonical 查找与属性路径解析。
  若这些表示进入热路径，可复用同样的"按 schema/类型缓存访问元数据"思路（需保持 canonical 名与
  大小写解析语义）。

### 4.6 派发顺序缓存的收尾

状态：已实施。更新流优先的派发顺序现在与普通派发顺序一起按目录变更缓存，目录失效时同步清除。

- `orderUpdateStatementsFirst` 在存在 update-istream 语句时每事件重新分配并按 update 优先级排序；
  可并入已缓存的派发顺序（updates 优先、其余保持原序），但必须保持 drop 屏障与 update 优先级语义。

### 4.7 表达式求值的进一步特化

状态：已实施（stateless 限定）。`stateless_compile.go` 在计划解析时把已证明纯的谓词链编译为
直线闭包：字段读取预解析 `structFieldTable` 候选路径（与泛型 `Schema.get`→`getOne`→
`structFieldValue` 同一候选选择与回退、同一 Value 包装，Value/Row/Event 壳与非 struct 表示逐分支
镜像或回落泛型 `Get`）；比较/包含/成员/逻辑/空探测节点复用与泛型闭包**完全相同**的操作函数
（`EqualValues`/`compareValues`/`exactNumericCompare`/`As`/`equalValuesUnwrapped`/`strings.*`），
仅消除逐节点闭包分发与名字解析；四个纯内建（`Lower`/`Upper`/`Trim`/`StringLength`，仅这四个
构造器设置 `pureBuiltin`）按描述前缀识别并镜像 `Func1` 的 null 传播。编译集之外的节点使整链
回落泛型（无部分编译）。`matchesEventFilter` 与 `processStatelessEvent` 在 `appliesTo` 成立时
改用编译链，求值次数与结果不变。

- 等价性证据：`TestStatelessCompiledMatchesGenericMatrix`（20 形态 × 6 事件矩阵，含 nil 指针
  字段、null/missing 操作数、大小写、混合数值比较、指针字段与内建组合）与
  `TestStatelessCompiledCaseInsensitiveAndFallback`（大小写不敏感解析、匿名 nil 指针候选回退）
  逐事件断言编译链与泛型 `sourceNodeMatchesEventFilter` 同布尔结果。
- 基准（同机交替 A/B，`GOMAXPROCS=1`，对 HEAD `e77d1880e`）：rejected 2008/2319 → 1402/1607 ns
  （-32%），accepted 3908/4663 → 2012/2139 ns（-50%），分配不变（4/8 allocs）。
- 后续（未实施）：Java 的生成类内联（直接成员访问、无 reflect）不可移植为 Go 代码生成面；
  若需进一步压缩，可对 `field` 读取按字段类型做类型化特化闭包（当前保留一次 `reflect` 路径
  行走 + `Present(Interface())` 装箱）。

### 4.8 NFR 登记

- 质量策略要求"可重放命令、阈值和结果"才能登记 `nfr-verified`；本单元只提供受控微基准与常驻基准。
  待共享运行时语义稳定后，应按 §3 的复现方式登记每事件 CPU/分配基线与趋势，并明确目标环境阈值。
- 应用侧 10000 EPS 所需的原生判定执行模型属于应用仓库范围，不在本模块内实施。

### 4.9 追加审查：新确认的瓶颈与建议优先级

本节记录一次对照实现与 profile 的追加审查结果：Join 全量重算、变量上下文与批次分配、Schema
元数据 fold 索引、Named Window/Table 索引重建与 `typeNames` 入口**此前未登记**；过滤重复求值
虽已列入 §4.2，但本次审查按实测影响提升其优先级。所有条目机制与调用点均已按当前源码核对，
与 §4 其余工作项同等对待——先补齐对应 Java/Go 语义证据，再做性能改动；任何一项都不得放宽差分、
证据或 Plan 身份约束。审查给出的建议修复优先级：

| 优先级 | 工作项 | 条目 |
| ---: | --- | --- |
| 1 | 增量 Join / Join 条件索引 | 4.9.1 |
| 2 | 所有语句统一单次过滤求值 | 4.9.2（即 §4.2） |
| 3 | 变量上下文与 ResultBatch 分配削减 | 4.9.3 |
| 4 | Schema 元数据 fold 索引 | 4.9.4 |
| 5 | Named Window/Table 增量索引维护 | 4.9.5 |
| 6 | `typeNames` 入口缓存 | 4.9.6 |

#### 4.9.1 增量 Join：每次事件重算全量 Join 结果（最高优先级）

- 现状：`statementRuntime.insertJoin`（`runtime.go:12344`）转发到 `updateJoin`（`runtime.go:12349`）。
  事件驱动的单次 update 在更新窗口侧状态**之前**先算 `before := joinKeyedTuples(...)`
  （`runtime.go:12373`），更新后再算 `after := joinKeyedTuples(...)`（`runtime.go:12554`），
  最后 `diffJoinKeyedTuples(before, after)`（`runtime.go:12555`）得出 delta。窗口过期重算路径
  同样是 `before`（`runtime.go:13073`）/`after`（`runtime.go:13107`）全量两次。
- `joinKeyedTuples`（`runtime.go:13301`）对两流 inner join 做左右嵌套全扫描并重跑
  `joinConditionsMatch`；多流 inner join 用 `visit(0)` 递归枚举笛卡尔组合；显式 left-deep join 由
  `joinChainedKeyedTuples` 按边重建全部 partial rows。复杂度约为
  $O(\prod_i |side_i|)$ 次条件求值，且每个事件执行两次（before/after）。
- 与 §4.1 的关系：这比过滤服务索引更严重——即使语句数量不多，只要窗口内事件增长就会迅速恶化；
  §4.1 只降低"事件 → 候选语句"的匹配成本，不降低单条 join 语句内部的 tuple 重算成本。
- 目标：改为增量 join——只用新事件与对侧索引匹配，维护已输出 tuple 集合，diff 只覆盖受影响 tuple；
  join 条件中可等值/IN 的键建立索引（可复用 §4.1 的索引结构思路）。
- 风险与前置证据：输出顺序、old/new 分类、lineage key（`joinStoredTupleLineageKey`）、outer join
  未匹配行、method/table/unidirectional join、trigger lineage 与 evaluate-once 语义都必须逐项钉定；
  需要 before/after delta 等价性证据，以及能显示候选 tuple 数下降的基准。

#### 4.9.2 过滤结果仍被求值两次（§4.2 的优先级提升）

- 现状：`Engine.send` 先计算 `accepted := statement.matchesEventFilter(...)`（`runtime.go:3939`；
  routed 分发在 `runtime.go:5115-5117`，仅当存在 unmatched 监听器、指标或审计类别时才计算），
  随后 `processStatementWithMetricsLocked` 进入 `Statement.process`，通用路径在 `runtime.go:6907`
  起再次装配变量并执行插入/过滤判定。当前只有 stateless 快路径
  （`stateless.go:221` `processStatelessEvent`）在 `acceptedKnown` 时复用派发循环的结果。
- profile：`sourceNodeMatchesEventFilter` 与 `makeBinaryBool.func1` 占据显著 CPU。
- 结论：按实际影响应为本轮第 2 优先级——它同时增加 CPU、闭包调用与临时对象分配，收益覆盖全部
  非快速路径语句（普通语句、带 getter 的语句、Join、pattern、context 等）。
- 目标与风险：见 §4.2（把过滤结果作为派发循环的唯一产物向下传递；语句指标采样窗口、审计
  `accepted` 取值以及用户函数/脚本调用次数的可观测性必须保持）。

#### 4.9.3 变量上下文与 ResultBatch 的分配削减

状态：大部分实施（§2.4）。发送入口空变量快照与纯谓词语句的变量装配跳过已落地；监听器快照缓存与
字面量预装箱消除派发侧每批次分配；accepted/rejected 基准降至 8/4 allocs。仍保留：非空变量快照
复制（隔离语义）、ResultBatch 借用/复用与监听器 `batch.clone()`——待单独验证监听器持有语义。

- 变量上下文：发送路径每轮执行 `cloneValues(e.variables)`（`runtime.go:3898`）→
  `statementVariables(...)` → `variablesWithEngineLockState(...)`（`runtime.go:6907` 起、
  `subquery.go:2700`）。profile 中 `variablesWithEngineLockState` 的累计分配约占总分配的 22%。
  对无变量、无子查询的普通语句，这些 map 复制与引擎引用注入没有实际价值。
- ResultBatch：`processStatelessEvent`（`stateless.go:221`）已避免通用 delta/history 路径，
  但接受事件仍创建 `ResultBatch` 与 `New` slice，并在派发给监听器/订阅者/sink 时再次
  `batch.clone()`（`runtime.go:5515`、`5522`、`5527`），accepted 基准仍为 16 allocs/op。
- 目标：按计划属性预先标记语句是否需要变量上下文——不需要者共享只读视图，或在一个 dispatch
  轮次内构造一次上下文后复用。监听器同步且无 replay/异步隔离需求时，增加内部借用或只读批次路径，
  减少批次与事件 slice 的重复复制。
- 风险与前置证据：需要**单独验证**监听器对 batch 的持有/变更语义——replay 缓冲、threading/
  outbound 池、output 策略的 pending 捕获与 subscriber/sink 是否保留引用；以及变量 map 的对外
  可观测性（用户函数、变量服务、审计）。

#### 4.9.4 Schema 元数据的大小写不敏感 fold 索引

状态：已实施（`f8145e81b`）。Schema 构造阶段为字段、getter、setter 和嵌套 schema 建立基于
`unicode.SimpleFold` 轨道最小值的不可变索引；精确名称、`EqualFold` 确认和歧义错误语义保持不变。

- 现状：`Schema.lookupField`（`schema.go:1657`）、`lookupGetter`（`schema.go:1688`）、
  `lookupSetter`（`schema.go:1714`）在精确名未命中且 resolution 非 `PropertyCaseSensitive` 时，
  分别对 `s.fields` / getter map / setter map 全量执行 `strings.EqualFold`（`lookupNestedSchema`
  同）。§2.1 的缓存只覆盖 Go struct 字段解析，未覆盖 Schema 元数据本身。
- 影响：字段数较多且谓词频繁动态查找的 map/JSON/object-array 事件，每次属性读取仍是
  O(field-count) 成本。
- 目标：在 Schema 构造阶段建立与 struct 缓存同源的 fold 索引（`unicode.SimpleFold` 轨道最小值键，
  不能使用 `strings.ToLower`），同时保留 `PropertyDistinctCaseInsensitive` 的歧义检测与错误文本。
- 前置证据：折叠等价性（如 U+212A KELVIN SIGN 与 U+1E9E 大小写对）与歧义判定点的等价性。

#### 4.9.5 Named Window/Table 索引的全量重建路径

状态：部分实施。Table 索引重建已将排序移出逐行插入循环，并在增量维护和重建时跳过 hash 索引不需要的
排序；Named Window 无索引状态也会跳过重建。位置索引的完整增量维护仍待单独验证。

- 现状：`rebuildNamedWindowIndexesLocked`（`state.go:2045`）重建时遍历全部 `state.entries`，
  对每个索引重建 key map，最后对 entries 做一次 `sort.SliceStable`；
  `rebuildTableIndexesLocked`（`state.go:553`）遍历全部 `state.order`，且在**每行**对 entries
  重新排序（即 O(N² log N) 量级）。两者在多个 mutation/restore 路径被调用，包括单条 insert
  （`state.go:3929`）、retain/delete（`state.go:3277`、`3923`）、restore（`state.go:3014`）
  以及 CreateIndex/DropIndex。
- 影响：对大窗口的批量 insert/delete/merge，本应增量的索引维护退化为 O(N log N)（table 路径更差）
  的尖峰；不是每个普通事件都触发，但在 named-window mutation、rollback、批量删除与 context
  partition 场景中会形成明显尖峰。
- 目标：区分"单条增量维护"和"确实需要重建"的场景——批量 mutation 走增量索引变更，仅在
  rollback/restore/索引定义变化时全量重建。
- 风险与前置证据：B-tree entries 的排序顺序、unique index 首键语义、root/partition 索引共享与
  `keyOrder` 一致性必须保持。

#### 4.9.6 事件类型解析入口缓存

状态：已实施（`f8145e81b`）。Environment 对 `reflect.Type` 缓存注册名称快照，并在成功注册 schema
时刷新；返回值仍复制切片，保持注册顺序和多名称歧义语义。

- 现状：`Environment.typeNames`（`plan.go:171`）每次调用都取 `e.mu.RLock` 并
  `append([]string(nil), e.typeToName[typ]...)` 复制名称切片；`Engine.SendEvent`
  （`runtime.go:4130`）、dataflow 入口（`dataflow.go:3972`）与计划解析（`plan.go:3360`）都会调用。
- 影响：发送频率高或同一 Go 类型绑定多个事件名时，形成稳定的小额分配与读锁竞争。
- 目标：在事件入口缓存 `reflect.Type` → canonical 事件类型/名称，在 schema 注册/卸载时失效；
  注意一个 Go 类型可注册多个事件名（`typeNames` 注释），必须保留 registration order 与歧义判定。
- 前置证据：注册/卸载失效路径与高发送频率下的并发读基准。

## 5. 复现命令

## 6. 尚未实施项目记录

以下项目保留在后续迁移工作中，当前没有直接修改共享语义路径：

- **增量 Join / Join 条件索引（§4.9.1）**：现有实现需要维护重复键、过期、outer join、method/table
  source、trigger lineage 和确定性输出顺序；在完成两流等值 inner join 的 Java/Go 差分场景前不改动。
- **通用单次谓词求值（§4.9.2）**：stateless 纯表达式快路径已经复用派发结果；普通路径同时覆盖 multi-slot
  IN、旧流、窗口、context、partition 和用户函数，继续保持二次求值直到具备逐形态证据。
- **ResultBatch 借用/复用（§4.9.3）**：监听器、subscriber、sink、replay 和异步 threading 可能持有 batch
  引用，需先验证所有持有语义后再减少 clone。
- **Named Window/Table 位置索引增量维护（§4.9.5）**：hash 排序和空索引重建已优化；删除、压缩、unique
  replacement、restore 仍走完整重建，避免位置调整改变查询顺序。
- **WHERE 下推、结果集 HANDTHROUGH、过滤索引第二阶段与 NFR 登记（§4.3、§4.4、§4.1 第二阶段、§4.8）**：
  §4.7 编译谓词已实施（stateless 限定）；这些剩余项目需要独立 capability 证据、old/new 流验证和
  可复现基准，不能仅凭微优化提交状态。
- **派发循环内 panic 的 deferred 自死锁（§2.4 追加发现）**：panic 发生在持有 `e.mu` 的派发循环内时，
  deferred `finishExternalRoutes` 在栈展开中再次 `e.mu.Lock` 导致挂起而非报告 panic；修复需论证
  panic 期间的锁释放顺序，属可观测的故障呈现变化，单独成单元。

保留这些项目是为了让后续 Esper capability 迁移继续使用原有 runtime、Plan identity 和差分证据；每个项目
在进入实现前都必须建立可重放的 Java/Go 场景，并通过受影响包测试、差分回放和完整门禁。

```sh
# 常驻基准（属性访问位置、无状态过滤发送）
go test ./internal/esper -run '^$' -bench 'PropertyAccess|StatelessFilter' -benchmem

# 本轮修复的常驻回归
go test ./internal/esper -run 'TestStructProperty|TestPropertyAccess|TestStatelessFilter' -count=1

# 第二轮（§2.4）的常驻回归与 A/B 方法
go test ./internal/esper -run 'TestClientCompileLarge|TestStatelessFilter' -count=1
# A/B：对 HEAD 与工作树交替运行同一基准，对消机器漂移
GOMAXPROCS=1 go test ./internal/esper -run '^$' -bench BenchmarkStatelessFilterSend -benchmem -count 1

# 第三轮（§4.7/§4.1）的常驻回归与基准
go test ./internal/esper -run 'TestStatelessCompiled|TestAcceptIndex' -count=1
GOMAXPROCS=1 go test ./internal/esper -run '^$' -bench 'StatelessFilterSend|AcceptIndexMultiStatement' -benchmem

# 受影响差分链（Java trace 已在仓库内，无需重新生成）
go run ./cmd/parity -mode event-bean-property-fragment-diff -scenario testdata/parity/event-bean-property-fragment.json -java-trace testdata/parity/event-bean-property-fragment.trace.json
go run ./cmd/parity -mode expr-filter-optimizable-diff -scenario testdata/parity/expr-filter-optimizable.json -java-trace testdata/parity/expr-filter-optimizable.trace.json
go run ./cmd/parity -mode expr-core-logical-diff -scenario testdata/parity/expr-core-logical.json -java-trace testdata/parity/expr-core-logical.trace.json
```


## 7. 应用侧接入指引（bigsoc-app 落地多规则高性能形态）

引擎侧三轮优化（§2.4、§4.7、§4.1）把成本结构变为：**单次 SendEvent 固定成本 ~1.5-2 µs +
每命中类型语句 ~0.4 µs（编译谓词）+ 非命中类型语句 ~0（类型级裁剪）**。要兑现它，应用侧需要
做以下改造（按收益排序）：

1. **合并规则引擎：一引擎多语句，而非每规则一引擎。** 当前形态（278 规则 × 278 引擎 × 逐引擎
   SendEvent）为每个事件支付 278 次完整 SendEvent 固定成本（锁、时钟、事件包装、变量快照、
   派发装配），类型裁剪无从生效。改造为单一 `Engine` + 每规则一个 `Plan`/语句 + 各自
   `Subscribe`：每事件一次 SendEvent，引擎按事件类型裁剪到相关语句（§4.1），同类型规则用编译
   谓词判定（§4.7）。以基准推算：278 条双条件规则、事件命中其中一类的形态，从 ~每事件
   278 × SendEvent 固定成本降到 1 × 固定成本 + 命中类型语句数 × ~0.4 µs。
   - 语义注意：单引擎内语句按 priority/drop/部署序派发（与多引擎各自独立的顺序等价性由规则
     互不依赖保证）；规则间无 insert-into/route 依赖时合并无观测差异。若个别规则需要隔离
     （如独立 undeploy 生命周期），`Deployment` 粒度仍然可用——一引擎可持有多 deployment。
2. **保持规则的快速路径资格**（§2.2 合格性，缺一则整条规则退回通用管线）：
   - 事件用注册 struct（`RegisterStruct`），谓词只读**普通命名字段**（无 `child.value`/`items[0]`
     路径语法）；
   - 不用 `WithPropertyGetter`/`WithPropertyMethod`/JavaBean 访问器（注册 getter 即丧失资格）；
   - 谓词只用纯算子：`Equal/NotEqual/Greater(Less)…/Contains/StartsWith/EndsWith/In/And/Or/Not/
     IsNull/IsMissing` 与 `Lower/Upper/Trim/StringLength`（大小写不敏感形态用 `Lower`+`Equal`/
     `Contains` 即可保资格）；用户自定义 `Func*`、变量、子查询、脚本一律退回通用管线；
   - 无窗口/聚合/输出策略/上下文（纯过滤规则天然满足）。
3. **发送路径**：事件类型固定时用 `Send(ctx, "TypeName", event)`（省去 `SendEvent` 的反射类型
   推断）；同一 Go 类型只注册一个事件名（多注册名会强制 `Send` 显式名）。
4. **监听器**：同步 listener 每批次一次 `batch.clone()`（保留语义必需）；不需要批次历史时不要在
   listener 中持有 `ResultBatch` 引用（为将来批次复用优化保留空间，见 §4.9.3）。
5. **验证方法**：合并前后用相同的规则集与事件流做行为对照（输出行、顺序、unmatched 计数），
   并用 `BenchmarkAcceptIndexMultiStatement` 的形态自测：预期每事件成本 ≈ 固定成本 + 命中类型
   规则数 × ~0.4 µs（单核，本机 2026-09 快照；非承诺阈值）。

上述第 1 项是数量级收益的关键；第 2 项决定单条规则的判定成本档位。引擎侧已无待办阻塞应用改造。
