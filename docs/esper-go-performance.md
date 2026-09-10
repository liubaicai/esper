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

> 说明：上表为受控微基准与本机数据，不是端到端 EPS 结论，也不构成 `nfr-verified` 登记。

## 4. 后续性能与执行模型工作项（尚未实施）

以下工作项与本移植一起安排：先完成对应 capability 的 Java/Go 语义证据，再做性能改动；任何一项
都不得放宽差分、证据或 Plan 身份约束。

### 4.1 过滤服务索引（Java `FilterService` 等价物）

- 现状：`Engine.send` 对每个事件遍历全部 continuous 语句并逐条求值过滤谓词，复杂度 O(语句数)。
  派发顺序已缓存，但候选选择仍是全量。
- 目标：为可索引谓词（等值、IN、范围）建立事件类型/属性索引，把事件直接映射到可能匹配的语句集合。
- Java 参考：`EventTypeIndex`（精确类型 + 深度父类型）、`FilterParamIndexEquals`（可查找量求值一次 +
  哈希查找）、`FilterHandleSetNode`、`FilterParamIndexBooleanExpr`（残余布尔表达式仍逐个求值）、
  `EPEventServiceImpl`（先过滤服务匹配再处理回调）。
- 必须保持：未索引布尔谓词仍全量求值；命中语句全部执行；priority/drop 与部署序屏障；unmatched
  监听器与指标/审计记账语义；多槽位 IN 投递；深层父类型匹配；变量/上下文对谓词求值的影响。
- 前置证据：候选裁剪等价性（被索引判定为不匹配的语句必须确实不匹配）、unmatched 记录顺序、
  优先级屏障、以及能显示候选数下降的基准。

### 4.2 单次谓词求值（消除重复求值）

- 现状：通用路径对同一事件求值两次——派发循环的 `matchesEventFilter` 与执行内的插入/过滤判定。
  已实测：含用户函数的谓词每次发送被调用 2 次（Java 为 1 次），拒绝事件同样如此。
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

- `orderUpdateStatementsFirst` 在存在 update-istream 语句时每事件重新分配并按 update 优先级排序；
  可并入已缓存的派发顺序（updates 优先、其余保持原序），但必须保持 drop 屏障与 update 优先级语义。

### 4.7 表达式求值的进一步特化

- 当前表达式树以闭包逐节点求值；Java 通过生成类内联谓词并直接访问成员。若共享 API 面稳定，
  可在引擎内部为热点谓词生成特化闭包（不改变 §2.2 的合格性判定与公共语义）。

### 4.8 NFR 登记

- 质量策略要求"可重放命令、阈值和结果"才能登记 `nfr-verified`；本单元只提供受控微基准与常驻基准。
  待共享运行时语义稳定后，应按 §3 的复现方式登记每事件 CPU/分配基线与趋势，并明确目标环境阈值。
- 应用侧 10000 EPS 所需的原生判定执行模型属于应用仓库范围，不在本模块内实施。

## 5. 复现命令

```sh
# 常驻基准（属性访问位置、无状态过滤发送）
go test ./internal/esper -run '^$' -bench 'PropertyAccess|StatelessFilter' -benchmem

# 本轮修复的常驻回归
go test ./internal/esper -run 'TestStructProperty|TestPropertyAccess|TestStatelessFilter' -count=1

# 受影响差分链（Java trace 已在仓库内，无需重新生成）
go run ./cmd/parity -mode event-bean-property-fragment-diff -scenario testdata/parity/event-bean-property-fragment.json -java-trace testdata/parity/event-bean-property-fragment.trace.json
go run ./cmd/parity -mode expr-filter-optimizable-diff -scenario testdata/parity/expr-filter-optimizable.json -java-trace testdata/parity/expr-filter-optimizable.trace.json
go run ./cmd/parity -mode expr-core-logical-diff -scenario testdata/parity/expr-core-logical.json -java-trace testdata/parity/expr-core-logical.trace.json
```
