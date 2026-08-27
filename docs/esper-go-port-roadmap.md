# Esper 9.0.0 Go 移植：执行路线图与遗漏检查

> 文档定位：本文件只维护当前阶段、优先级、remaining 和风险。完整范围与架构见 [实施规划](esper-go-port-implementation-plan.md)，日常步骤见 [执行手册](esper-go-port-runbook.md)，差分、合成数据和验收口径见 [质量策略](esper-go-port-quality-strategy.md)，历史见 [CHANGELOG](../CHANGELOG.md)。统计数字以 `testdata/compat/capability-manifest.json` 的已校验 `summary` 为唯一来源。

## 0. 实时状态入口
> 最新补充：Draft 4.270（2026-08-28），新增 `resultset-aggregate-firstlastwindow-indexed` differential-verified 场景，对照固定 Java `ResultSetAggregateFirstLastWindow.java` 的 `ResultSetAggregateFirstLastIndexed`（1 个 runtime）：Java/Go 各 4 条 records、0 differences。场景覆盖 typed `first`/`last` indexes 0..3 的 length(3) new-only 输出、未满窗口时的 Null、越界 index 3 的 Null 和窗口淘汰后的 FIFO 历史移位；join、dynamic index、prev/nth、wildcard、iterator、FAF、output-rate、invalid 与生命周期变体继续保留在 aggregate-access umbrella。manifest 更新为 563 cases、175 个 differential-verified case、688 个 differential runtime IDs、3311 条 associations（referenced 3102）；capability 118 个（33 DV）。

> 最新补充：Draft 4.268（2026-08-28），新增 `resultset-aggregate-firstlastwindow-current` differential-verified 场景，对照固定 Java `ResultSetAggregateFirstLastWindow.java` 的 `ResultSetAggregateFirstLastWindowNoGroup` 与 `ResultSetAggregateFirstLastWindowGroup`（2 个 runtime）：Java/Go 各 11 条 records、0 differences。场景覆盖 typed first/last/window 的 length(2) ungrouped 与 length(5) grouped current-window 语义、FIFO 淘汰和 order-by 行序；indexed/prev/nth、wildcard、batch/output-rate、join、subquery、invalid、old-stream、iterator、FAF 与生命周期变体继续保留在 aggregate-access umbrella。manifest 更新为 561 cases、173 个 differential-verified case、686 个 differential runtime IDs、3309 条 associations（referenced 3102）；capability 118 个（33 DV）。
> 最新补充：Draft 4.269（2026-08-28），新增 `resultset-aggregate-firstlastwindow-prev-nth` differential-verified 场景，对照固定 Java `ResultSetAggregateFirstLastWindow.java` 的 `ResultSetAggregatePrevNthIndexedFirstLast`（1 个 runtime）：Java/Go 各 4 条 records、0 differences。场景覆盖 typed `prev`/`nth`/`last` indexes 0..2 的 length(3) new-only 输出、窗口淘汰后的 FIFO 历史移位和越界 Null；indexed `first`、join、dynamic index、wildcard、iterator、FAF、output-rate、invalid 与生命周期变体继续保留在 aggregate-access umbrella。manifest 更新为 562 cases、174 个 differential-verified case、687 个 differential runtime IDs、3310 条 associations（referenced 3102）；capability 118 个（33 DV）。
> 最新补充：Draft 4.267（2026-08-28），新增 `resultset-aggregate-filtered-all` differential-verified 场景，对照固定 Java `ResultSetAggregateFiltered.java` 的 `ResultSetAggregateAllAggFunctions`（1 个 runtime）：Java/Go 各 29 条 records、0 differences。六个隔离子场景覆盖 length(3) filtered avedev/avg/fmax/median/fmin/stddev/sum/fmaxever/fminever、primitive-width sums、无窗口 fmax/fmin、BigDecimal/BigInteger exact aggregates，以及 EPL/SODA filtered distinct 七统计聚合；保留 broad `case.aggregate-filtered` 的全量 source inventory umbrella。manifest 更新为 560 cases、172 个 differential-verified case、684 个 differential runtime IDs、3307 条 associations（referenced 3102）；capability 118 个（33 DV）。
> 最新补充：Draft 4.266（2026-08-28），新增 `resultset.aggregate-ever` differential-verified 场景，对照固定 Java `ResultSetAggregateFirstEverLastEver.java` 的 3 个可表示 execution（Java/Go 各 14 条 records、0 differences）：SODA/EPL firstever、lastever、first、last 与 countever 的 length(2) current/ever/filter 轨迹，覆盖 null boxed 值和窗口淘汰；keepall named-window on-delete 删除后保留 ever 历史。ordinal 2 `countever(distinct ...)` 为 Go 类型安全 API 不可表示的 compile-error 边界，保持 implemented-only 并保留 Java 原文。独立 `AggregateEverReview-3` 要求 case 元数据保留该 invalid runtime/name；修复后 manifest 仍为 559 cases、171 个 differential-verified case、683 个 differential runtime IDs，association 更新为 3306（referenced 3102，未关联 1034）；capability 118 个（33 DV）。
> 最新补充：Draft 4.265（2026-08-27），`resultset-aggregate-count-sum` 扩展至 9 个 execution differential-verified（Java/Go 各 55 条 records、0 differences）：count(*) 星号投影、无窗口 sum/count HAVING、SODA grouped count/count-distinct/count 的 null 与窗口淘汰重算、`avg(count(*))` 历史前缀；保留 view/join/named-window 四 execution。manifest 更新为 559 cases、170 DV cases、680 DV runtime IDs、3305 associations（referenced 3101）；capability 118 个（32 DV）。

> 最新补充：Draft 4.264（2026-08-27），`ResultSetQueryTypeWTimeBatch`
> differential-verified：固定 Java `ResultSetQueryTypeWTimeBatch.java` 全部 8 个
> execution（Java/Go 各 16 条 records、0 differences）。覆盖 row-for-all、
> row-per-event、row-per-group、aggregate-grouped 的 no-join/keepall join
> 孪生，time-batch 边界与 irstream old/new 批次。引擎修复按窗口节点拆分
> join expiry delta、保留 row-per-event/grouped aggregate 的事件 lineage，并
> 将 aggregate join 路由到修正后的 per-window expiry；HashMap 行序区域以
> mode=any 规范化而保持批次、数量、字段和值严格。manifest 更新为 559 cases、
> 170 DV cases、675 DV runtime IDs、3300 associations（referenced 3096）；
> capability 118 个（32 DV）。

> 最新补充：Draft 4.263（2026-08-27），`case.insertinto-eventcol-col-rest`
> differential-verified：EPLInsertIntoPopulateEventTypeColumnBean/NonBean
> 剩余 12 个 execution 全部闭环（22 条 records、0 differences）。maxby
> 聚合值→事件数组列包裹、keepall 子查询→单事件列、initiated-by context.sb
> split 路由、new{} 匿名结构 EventRowsOf 物化（objectarray/map/json 三表示）、
> named-window 单列二次投影；两个 compile-invalid 族以 Build 拒绝 + Java
> pinned 文本登记。引擎新增 EventRowsOf/EventRowOf/EventFromAggregate 与
> route 成员身份诊断。manifest 更新为 558 cases、169 DV cases、667 DV
> runtime IDs、3292 associations（referenced 3088）；capability 117（31 DV）。
>
> 最新补充：Draft 4.262（2026-08-26），`EPLInsertIntoPopulateUndStreamSelect`
> 3/4 个 execution differential-verified（47 条 records、0 differences）：
> on-merge insert-select 以子类型实例填充超类型 objectarray 列
> （match 链 cast(w.event.id? as string)）；objectarray/map/avro/json/
> json-provided/default 六表示矩阵的 wildcard transpose 插入，覆盖
> int→long / int→double 宽化、多余投影丢弃、纯透传与位置无关的显式
> 覆盖 wildcard。表示登记：非 map 表示的 transpose+额外列以显式 Alias
> 等价表达（Build gate 维持）；exec1 phase 拆分采用每次调用独立 JVM
> （undeployAll 不释放 @public path 类型）；Invalid execution
> implemented-not-DV（go-unit route-validation pins + Java 原文保留）。
> manifest 更新为 557 cases、168 个 differential-verified case、655 个
> differential runtime IDs、3280 条 associations（referenced 3076）；
> capability 117 个（31 DV）。
>
> 最新补充：Draft 4.261（2026-08-26），`case.variables-use` 收口
> `EPLVariablesUse` 全部可表示 execution（新增 2 个 DV runtime IDs，
> 共 9/11）：EPVariableService 运行时 API 全表面（类型内省引擎级断言、
> 单/批量有序 set、byte/short 宽化接受、Long/String/Double 拒绝驱动
> LinkedHashMap 序回滚证明、create-on-the-fly dummy=40、逐字镜像 Java
> 的 unknown/type-mismatch/constant 三类诊断）与 constant-variable 全表
> （int/short/null 算子真值表、字面量/数组/enum 成员 filter、SODA 重建、
> 编译期 const 拒绝句、三目标 API 写保护、ESPER-653 date 标记、末段非
> 常量 enum on-set）。关键语义钉死：含集合 candidate 的 IN 按"每匹配
> slot 一行"投递——`enumValue in (var_enumarr, var_enumone)` 对 V2 双发
> （flattened slots [V2,V1,V2]）；runtime set 强制转换遵循 Java
> Byte<Short<Integer<Long<Float<Double 宽化链。场景 Java/Go 各 101 条
> records、0 differences；12 个 mutation 用例拒绝篡改。表示登记：
> byte[] boxed/primitive 单一 Go 数组形态、SupportBean[] 声明拒绝无
> Go 构建错误等价物、SODA 双路径单编译路径化、FilterItem 断言为引擎
> 内部。manifest 更新为 556 cases、167 个 differential-verified case、
> 652 个 differential runtime IDs、3277 条 associations（referenced
> 3073）；capability 117 个（31 DV）。DotSeparateThread 与 WVarargs
> 保持未表示并登记于 capability remaining。
>
> 最新补充：Draft 4.260（2026-08-26），扩展 `epl.variable-onset` 的
> `case.variables-use` 差分场景至 7 个 execution（新增 4 个 DV runtime
> IDs）：preconfigured CONSTANT boolean 读取、@public 跨 module 可见性、
> 常量工厂 + preconfigured 实例的点调用（'hello'）、自定义类型常量的
> equals filter（canonical struct-object 渲染）。场景 Java/Go 各 11 条
> records、0 differences，既有 3 cases 逐字节保持。表示登记：Go 变量为
> environment 作用域（two-module 以环境注册建模 @public+path）；变量
> 宿主对象的点调用以读取求值上下文变量的函数表达式表示。manifest 更新
> 为 556 cases、167 个 differential-verified case、650 个 differential
> runtime IDs、3275 条 associations（referenced 3071）；capability 117
> 个（31 DV）。
>
> 最新补充：Draft 4.259（2026-08-26），新增 `expr.datetime-round`
> differential-verified 能力，对照固定 Java ExprDTRound.java 全部 4 个
> execution：五表示 roundCeiling('hour')、七单位 ceiling/floor 向量、
> roundHalf 含 Apache Commons 月长依赖进位（2002-05-30→2002-06-01，日
> 偏移 29 > (31-1)/2）、msec 恒等、:30.000 平局进位、以及 mid-case
> undeploy+recompile round-half-min（per-case 序列计数器存活）。场景
> Java/Go 各 7 条 records、0 differences。引擎新增：
> DateTimeRoundCeiling/Floor/Half 构建器（int64/time.Time 表示保持；
> time.Time 按值自身时区、epoch-millis 按 UTC 求值，对齐 pinned
> -Duser.timezone=UTC harness）。此前 PLANS 记录的 roundHalf('month')
> "oracle 自不一致"经源码裁决为误判：断言与 Apache Commons
> DateUtils.modify(MODIFY_ROUND) 语义自洽。manifest 更新为 556 cases、
> 167 个 differential-verified case、646 个 differential runtime IDs、
> 3271 条 associations（referenced 3067）；capability 117 个（31 DV）。
>
> 最新补充：Draft 4.258（2026-08-26），新增 `event.map-core`
> differential-verified 能力，对照固定 Java EventMapCore.java 5 个
> execution 中的 4 个：三层 map-of-maps 导航至 bean 叶子（含逐字对齐的
> ObjectArray sender 拒收文本）、myMapEvent 元数据内省（marker 记录）、
> beanA fragment 导航（nested.nestedNested 与 indexed[1]）、以及裸
> HashMap 对声明类型的再发送。场景 Java/Go 各 6 条 records、0
> differences。引擎修复：SendObjectArray 错误类别拒收文本逐字镜像
> Java EventSenderObjectArray。InvalidStatement execution
> （da04541dce5715cf9129）保持未表示：Go 链式 API 对未解析属性求值为 Missing 且无构建期操作数类型检查，
> 三种非法形态（未解析属性、String 算术、静态误用）均无等价 engine
> build-error 表面，登记于 capability remaining。
> manifest 更新为 555 cases、166 个 differential-verified case、642 个
> differential runtime IDs、3267 条 associations（referenced 3063）；
> capability 116 个（30 DV）。
>
> 最新补充：Draft 4.257（2026-08-26），新增
> `resultset.querytype-aggregate-grouped` differential-verified 能力，对照固定
> Java ResultSetQueryTypeAggregateGrouped.java 全部 9 个 execution：dot-method
> 与 int[] multikey 分组键、unbound 迭代器组更新、阈值边界 having、
> wildcard+min 全表面行、ESPER-185 双键 irstream（计数递减 old 行与空组
> count=0 old 行、有序 per-member 快照）、跨组 IR pair（view 与 join 孪生、
> join 快照 per-tuple）、mid-case insert-into 部署、数组键累计。场景
> Java/Go 各 53 条 records、0 differences。引擎修复：(1) grouped
> row-per-event（AggregateGroupedImpl）按事件发新行并按事件绑定普通列，
> leaving-only 组不再发新行，row-per-group（RowPerGroupImpl）保持每组
> new+old；(2) grouped join 迭代器按 AggregateGroupedImpl 形态走 per-tuple
> 行；(3) 语句投影的 context 分区维按 group 键处理；(4) differ 对
> mode=any 快照行做无序规范化（Java grouped join 迭代器按 HashMap 组序）。
> manifest 更新为 554 cases、165 个 differential-verified case、638 个
> differential runtime IDs、3263 条 associations（referenced 3059）；
> capability 115 个（29 DV）。
>
> 最新补充：Draft 4.256（2026-08-26），新增
> `resultset.orderby-row-for-all` differential-verified 能力，对照固定
> Java ResultSetOrderByRowForAll.java 全部 3 个 execution：
> NoOutputRateJoin（MD×SBS 连续 join 仅 iterator snapshot 观测，
> sumPrice 214→289）与 OutputDefault 无 join/keepall 叉积孪生（每 3
> 个事件一次 irstream 批次，new 行按各自冻结的 sum 快照降序、old 行
> 配先前状态链并以 null 对收尾）。场景 Java/Go 各 4 条 records、
> 0 differences。引擎修复：order-by 键与投影 alias 表达式按规范描述
> 一致时改读行自身投影列（orderResultsWithSelections +
> resolveOrderByKeyColumns），finishOutput 批次投递传 aggregate
> selections——此前 row-for-all/join 聚合在 output-rate 批次内对共享
> 活组重估导致全部比较并列、排序退化为插入序且 Descending 失效。
> manifest 更新为 553 cases、164 个 differential-verified case、
> 629 个 differential runtime IDs、3254 条 associations（referenced
> 3050）；capability 114 个（28 DV）。
>
> 最新补充：Draft 4.255（2026-08-25），登记 `resultset.aggregate-having`
> 能力并扩展 `resultset-query-type-having` 场景至 7 个可观测 runtime：
> length_batch 通配 select 的 where+count(*) 批次门控、text/OM avg-HAVING
> 孪生全 irstream 向量、volume<avg(price) 编译接受、insert-into 子流
> avg>=3 门控、以及无界 sum=2 的退役行携带先前投影。引擎修复：无分组
> irstream 首更新 null-prior old 行在 having 拒绝空组先验态时不再投递。
> join 家族三 execution 因滑动窗口 old/new 分类分歧保持未登记；4.241 的
> manifest 登记缺口同轮闭合。manifest 更新为 552 cases、163 个
> differential-verified case、626 个 differential runtime IDs、3251 条
> associations（referenced 3047）；capability 113 个（27 DV）。
>
> 最新补充：Draft 4.254（2026-08-25），新增 `infra-table-into-table`
> differential-verified 场景，对照固定 Java InfraTableIntoTable.java
> 全部 9 个 execution（BoundUnbound 三个子阶段共享单一 runtime ID，共
> 11 个 case）。场景 Java/Go 各 59 条 records、0 differences。覆盖无键
> count(*) 单/双模块累积、bound/unbound 绑定矩阵与六条精确编译拒绝文案、
> join 喂养的 window/sorted 列 FAF 观察、相关子查询读取的无键/按键 sum、
> #lastevent 流上 exact 大数 avg/sum 的逐事件替换语义，以及 int[] 内容
> 等值主键任意序行集。运行时新增标量 MaxEver/MinEver 与
> SortedEventsBy；into-table 编译期聚合兼容性校验逐字复现 Java 文案；
> 无键 into-table 贡献前快照规范化为空集并登记为表示差异。manifest 更新为 551
> cases、162 个 differential-verified case、619 个 differential runtime
> IDs、3244 条 associations（referenced 3040）；capability 112 个
> （26 DV）。
>
> 最新补充：Draft 4.253（2026-08-25），新增
> `resultset.orderby-row-per-group` differential-verified 能力，对照固定
> Java ResultSetOrderByRowPerGroup.java 全部 9 个 execution：no-having/
> having 无 join 对、no-having/having/having-alias 三个 join 孪生、last
> 与 last-join、iterator-row-per-group（连续 join + 两次迭代器快照）与
> order-by-last（length_batch istream）。场景 Java/Go 各 22 条 records、
> 0 differences。运行时三项修复：(1) order-by 比较器 null-first 语义——
> orderRowRecogResults 改用 compareOrderValues（compareValues 对
> null-vs-value 返回不可比较导致边界重排序退化为符号序）；
> (2) 分组建流的创建 null-prior old 行按 having 门控（pinned
> shortcutEvalGivenKey 对每条生成行求值 having）；(3) grouped
> output-last 批次按组合并——new 取组内末状态、old 取组内首次出现前的
> 先验状态（processOutputLimitedViewLastCodegen 的 put-if-absent 语义），
> pending 累积保留全部片段。runner 以 ResultField 投影表达 order-by 键
> （output-every 边界重排序按投影行求值，直接聚合键在该上下文惰性）。
> manifest 更新为 550 cases、161 个 differential-verified case、610 个
> differential runtime IDs、3235 条 associations（referenced 3031）；
> capability 112 个（26 DV）。
>
> 最新补充：Draft 4.252（2026-08-25），新增 `trigger.table-named-window` 的
> `infra-table-insert-into` differential-verified 场景，对照固定 Java
> InfraTableInsertInto.java（infra/tbl/）的 InfraInsertIntoAndDelete、
> InfraInsertIntoSameModuleUnkeyed、InfraInsertIntoTwoModulesUnkeyed、
> InfraInsertIntoWildcard 与 InfraInsertIntoSameModuleKeyed 共 5 个
> runtime。场景 Java/Go 各 20 条 records、0 differences：复合主键表的
> insert/delete/reinsert 循环以建表语句迭代器快照观察（含两次删除后的
> 空表位置），单/双模块 unkeyed 单行表以 send-error 记录固定编译器的
> "Unique index violation, table 'MyTableIIU' is a declared to hold a
> single un-keyed row" 原文（oracle 侧镜像 pinned runner 的 rethrow
> exception handler；Go 侧 unkeyed 表 insert-only 重复插入对齐同一
> 文案），wildcard map 插入为 pinned 六表示循环的 DEFAULT 迭代子集
> （已登记基础设施差异），keyed 表覆盖 into-table 分组聚合、on-insert
> 建行与 on-merge not-matched 建行。runner 采用 variables-onset 式
> 自定义步骤循环（deploy 隐含于 case 装配、send-error 记录裸消息、
> snapshot 经 FromTable fire-and-forget 计划）。manifest 更新为 549
> cases、160 个 differential-verified case、601 个 differential runtime
> IDs、3226 条 associations（referenced 3022）。
>
> 最新补充：Draft 4.251（2026-08-25），完成 `infra.namedwindow.views` 的
> `infra-named-window-join` 场景收口：新增 InfraNamedWindowJoin.java 的
> InfraUnidirectional（java-runtime-98c12887d1e25c26c101）、
> InfraWindowUnidirectionalJoin（java-runtime-09ca3e1b6ac4f52b0b7e）与
> InfraInnerJoinLateStart（java-runtime-2a245ed9721b6b840746，按
> objectarray/map/json/jsonprovided/default 五种表示各一 case；AVRO 迭代
> 为已登记的基础设施差异）。场景 Java/Go 各 75 条 records、0 differences。
> unidirectional 以单向命名窗口驱动侧 join SupportBean_A#lastevent 验证
> A 侧到达不触发、窗口插入才触发的非对称契约并投影 w.* 全 20 属性行
> （charPrimitive 渲染 "\u0000"）；window-unidirectional-join 以
> JoinStream.Aggregate + WindowValues[Event](JoinEventValue[Event](1)) +
> EnumWhere/EnumToMap 表达 window(win.*) 及其过滤与 toMap 形态（含空 c1
> 数组与空窗口零输出），三个 compile-only window(win.*) 语句仅编译不部署；
> inner-join-late-start 五种表示字节级一致地验证延迟部署 join 命中预填充
> 行。oracle 侧为 window(win.*) 裸底层登记 registered-underlying 行形
> 渲染协议，plain-json case 以单一 population 模块编译规避固定编译器对
> 路径上生成 JSON 底层的重复注册；runner 脚本改为逐 case JVM 执行后按
> 场景顺序合并。manifest 更新为 548 cases、159 个 differential-verified
> case、596 个 differential runtime IDs、3221 条 associations（referenced
> 3017）。capability remaining 移除 executions 7-9。
>
> 最新补充：Draft 4.250（2026-08-24），扩展 `infra.namedwindow.views` 的
> `infra-named-window-join` 场景：新增 InfraNamedWindowJoin.java 的
> InfraJoinNamedAndStream（java-runtime-479eabd476ce403c4ab8）、
> InfraJoinBetweenNamed（java-runtime-342d46139a3313b382b7）、
> InfraJoinBetweenSameNamed（java-runtime-8f4b0394ee32a5524464）与
> InfraJoinSingleInsertOneWindow（java-runtime-e7320476230a3903e2ec）。
> 场景 Java/Go 各 60 条 records、0 differences；覆盖 keepall 命名窗口 ×
> length(10) 流 irstream join 的 on-delete 旧行与 2 行扇出批次、
> boolPrimitive 路由插入 + volume 键控 on-delete 的双窗口 join 及其单插入
> 孪生、同窗口自连接匹配删除仅一行旧行。Go 侧以 Join/JoinMany +
> DeleteFromNamedWindow + Filter 触发流表达，无 internal/esper 改动。
> manifest 更新为 548 cases、159 个 differential-verified case、593 个
> differential runtime IDs、3218 条 associations（referenced 3014）。
>
> 最新补充：Draft 4.249（2026-08-24），新增 `infra.namedwindow.views` 的
> `infra-named-window-join` differential-verified 场景，对照固定 Java
> InfraNamedWindowJoin.java 的 InfraJoinIndexChoice、
> InfraRightOuterJoinLateStart 与 InfraFullOuterJoinNamedAggregationLateStart
> 共 3 个 runtime。场景 Java/Go 各 36 条 records、0 differences；index-choice
> 以五个隔离 Environment 段行为化重放 unique/keepall 数据窗口与 0..2 个
> 索引组合下的单向 join（s2/s1/i1 行 E2,E2,20 与 E1,E1,10，监听器序号跨段
> 连续），right-outer-late-start 填充两个 time(6s) 命名窗口后延迟部署分组
> 右外连接 s1 与镜像左外连接 s2 并快照十行有序结果，full-outer 填充
> groupwin(theString,intPrimitive)#length(3) 十九行后快照 create 迭代器
> （组连续顺序）并部署全外连接聚合快照十组（c3 null-left、c0 匹配 symbol c0、
> c1/c2 未匹配 null symbol 且 c1/int2 cntBool 3）。运行时修复：join 聚合
> 语句迭代器按当前 join 元组组合求值（Esper join 迭代器为实时视图，默认
> count/time 输出视图不再支撑它，输出快照策略的监听器路径保持原状），分组
> 命名窗口快照按组首现顺序连续迭代、组内保持插入顺序。INDEX_CALLBACK_HOOK
> 计划断言与 SERDEREQUIRED 序列化检查保持 approved difference。manifest
> 更新为 548 cases、159 个 differential-verified case、589 个 differential
> runtime IDs、3214 条 associations（referenced 3010）。

> 最新补充：Draft 4.248（2026-08-24），新增 `client.extend.inlined-class` 的
> `expr-class-static-method` differential-verified 场景，对照固定 Java
> ExprClassStaticMethod.java 的 11 个 listener/FAF/compile-only runtime。
> 场景 Java/Go 各 11 条 records、0 differences；覆盖 local 与 created static
> String 调用（各自合并 Java SODA true/false 编译路径）、local 与 path-created
> class 的 keepall named-window FAF 重放、同 module local→created 跨 class 调用、
> recursive/doc 与 empty-class compile-only 成功，以及 package-qualified 2 参数/
> 0 参数调用。Go 以类型化 Func0/Func1 closure 和
> DefineExpression/ExpressionRef 表达，不引入 EPL 或 Java 编译入口。
> CreateCompileVsRuntime 的 deployed-class precedence、compiler inspection hook
> 与 Janino-only invalid diagnostics 保持 approved difference。manifest 更新为
> 547 cases、158 个 differential-verified case、586 个 differential runtime
> IDs、3211 条 associations（referenced 3007）。
>
> 最新补充：Draft 4.247（2026-08-24），新增 `event.property-access-render` 的
> `event-bean-property-fragment` differential-verified 场景，对照固定 Java
> EventBeanPropertyResolutionFragment.java（event/bean）的全部 15 个
> execution。场景 Java/Go 各 16 条 records（native-bean-fragment 两阶段）、
> 0 differences；覆盖标量 map/oa 非 fragment 解析、命名 fragment 包装
>（plusone/mybean 投影）、原生 bean fragment（ComplexProps+CombinedProps
> 两阶段）、命名 fragment 与 fragment 数组嵌套、未命名内联 map 非
> fragment、pattern[one until two] 转置（map+object-array，顺序敏感的
> one[0]/one[1]）、bean fragment 根、3 级命名链（map+oa）、多级 map-multi。
> Java fragment API 断言（getFragmentType/isFragment）为引擎内部；EPL 行
> 输出展示等价的嵌套解析。manifest 更新为 546 cases、157 个
> differential-verified case、575 个 differential runtime IDs、3200 条
> associations（referenced 2996）。
>
> 最新补充：Draft 4.246（2026-08-24），新增 `expr.filter-expressions` 的
> `expr-filter-optimizable-value-limited` differential-verified 场景（扩展），
> 对照固定 Java ExprFilterOptimizableValueLimitedExpr.java 追加 9 个未覆盖
> execution（FromPatternSingle/Multi/Constant/HalfConstant/WithDotMethod、
> ContextWithStart、InSetOfValueWPatternWCoercion、InRangeWCoercion、OrRewrite）。
> 场景 Java/Go 各 19 条 records、0 differences；覆盖模式变量值受限过滤
> （含 every[2] 数组标签、dot-method 值）、context 分区属性、IN-list 与
> 闭区间 int→long 强转、context OR-to-IN 重写；SupportBean 以真实 bean
> 类型注册（a.getTheString() 解析）。Disqualify 登记 intentionally-different
> （compile-time plan forge 断言，无事件流可观测行为）。manifest 更新为
> 545 cases、156 个 differential-verified case、560 个 differential
> runtime IDs、3185 条 associations（referenced 2981）。
>
> 最新补充：Draft 4.245（2026-08-24），新增 `expr.filter-expressions` 的
> `expr-filter-optimizable` differential-verified 场景，对照固定 Java
> ExprFilterOptimizable.java 的 9 个可观测 execution（InAndNotInKeywordMultivalue、
> MethodInvocationContext、TypeOf、VariableAndSeparateThread、OrToInRewrite、
> OrContext、PatternUDF、DeployTimeConstant、RegExManyOr）。场景 Java/Go 各 53 条
> records、0 differences；覆盖多值 in/not in（plain+pattern+context）、
> EPLMethodInvocationContext UDF 观察记录（runtimeURI/functionName/
> statementUserObject/contextPartitionId）、typeof 超类型排除、变量过滤、
> OR-to-IN 字符串交换律、同类型 initiated context OR 过滤、BigDecimal
> compareTo==0 模式 UDF、部署期常量 equals/relop/in/in-array/between
> （substitution 参数与变量、双向、long 强转）、17 重相同 regexp OR。
> 引擎修复：序列模式右腿不再消费触发其 arm 的同一事件（同输入序列语义）。
> InspectFilter 登记 intentionally-different（引擎内部 filter plan 形状断言，
> 无 Go 可观测等价物）。manifest 更新为 544 cases、156 个
> differential-verified case、551 个 differential runtime IDs、3175 条
> associations（referenced 2971）。
>
> 最新补充：Draft 4.244（2026-08-24），新增 `view.basic-windows` 的
> `view-unique` differential-verified 场景，对照固定 Java ViewUnique.java
> 的 5 个 execution（SceneOne、SceneTwo、AnnotationPrefix、
> ExpressionParameter、TwoWindows）。场景 Java/Go 各 33 条 records、
> 0 differences；覆盖 #unique(symbol) irstream 替换对（重复 key 投递
> new+old、新 key 仅 new）、双 key #unique(symbol, feed) 窗口、SupportBean
> 上 theString/intPrimitive 的 c0/c1 投影、#unique(Math.abs(intPrimitive))
> 表达式键窗口（默认 stream：替换不以 remove 投递但窗口状态更新，
> 快照保留每键最新行 {E2,E4}）以及双独立 #unique(intBoxed) 语句的延迟
> s1 部署（s1 只见 E2 为 new-only）。manifest 更新为 543 cases、155 个
> differential-verified case、542 个 differential runtime IDs、3165 条
> associations（referenced 2961）。
>
> 最新补充：Draft 4.243（2026-08-23），新增 `epl.other.select-wildcard-additional`
> differential-verified 场景，对照固定 Java EPLOtherSelectWildcardWAdditional.java
> 的 Single 和 WildcardMapEvent 共两个 runtime。场景 Java/Go 各 3 条 records、
> 0 differences；覆盖 wildcard select + additional concat 投影与 Map event 类型。
> InvalidRepeatedProperties 登记 intentionally-different（compile-only duplicate
> column rejection）。SingleOM/JoinInsertInto/JoinNoCommon/JoinCommon/
> CombinedProperties 暂登记 remaining。manifest 更新为 542 cases、154 个
> differential-verified case、21 个 intentionally-different case、537 个
> differential runtime IDs、3160 条 associations（referenced 2956）。
>
> 最新补充：Draft 4.242（2026-08-23），新增 `view.basic-windows` 的
> `view-length-batch` differential-verified 场景，对照固定 Java ViewLengthBatch.java
> 的 8 个 execution（SceneOne、Size2、Size1、Size3、Delete, Normal{NAMEDWINDOW/GROUPWIN}
> 及 Invalid compile-only approved difference）。场景 Java/Go 各 48 条 records、
> 0 differences；覆盖 length_batch(1/2/3) 批次刷新 old/new 对、wildcard irstream 选择、
> 迭代器部分窗口快照、named-window 消费者与安静删除排除已删行、groupwin(null key)
> 嵌套。Prev 和 Normal{VIEW} 因 prev-on-batch 评估分歧暂登记 remaining。manifest 更新
> 为 540 cases、153 个 differential-verified case、20 个 intentionally-different case、
> 535 个 differential runtime IDs、3156 条 associations（referenced 2953）。
>
> 最新补充：Draft 4.241（2026-08-23），新增 `resultset.aggregate-having` 的
> `resultset-query-type-having` differential-verified 场景，对照固定 Java
> ResultSetQueryTypeHaving.java 的 Statement（text+OM 双 runtime）与
> SumHavingNoAggregatedProp 共三个 runtime。场景 Java/Go 各 9 条 listener records、
> 0 differences；覆盖非分组聚合 having price<avg(price) 的 new-only 投递、离开事件
> having 重评估（失败抑制/通过时以 leaving 绑定投影 old 行）、无 group-by having 中
> 聚合与非聚合属性混用的编译接受。引擎修复：非分组 irstream 聚合 having 查询的
> old 行现在按 leaving 事件重评估 having 后才投递（原先无条件投递 previous 行）；
> rowForEvent 老行投递增加同源 having 门控。StatementJoin 执行因 join-aggregate
> 窗口滑动下 old/new 分类分歧暂登记 remaining。manifest 更新为 538 cases、152 个
> differential-verified case、19 个 intentionally-different case、528 个 differential
> runtime IDs、3148 条 associations（referenced 2945）。
>
> 最新补充：Draft 4.240（2026-08-23），新增 `query.fire-and-forget` 的
> `infra-nwtable-faf-join-matrix` differential-verified 场景，对照固定 Java
> InfraNWTableFAF.java 的 12 个 Infra3StreamInnerJoin execution（representation ×
> namedWindow 全矩阵：OBJECTARRAY/MAP/AVRO/JSON/JSONCLASSPROVIDED/DEFAULT ×
> window/table）。场景 Java/Go 各 48 条 faf records、0 differences；覆盖三流内连接
> （keep-all 窗口与主键表两种 infra、insert 流与 on-merge 两种填充）、显式 ON 连接、
> owner 关联 where、逗号式过滤交叉连接与无 group-by 的 having 四种 fire-and-forget 查询。
> 引擎新增 JoinQuery.Having（FAF join 投影后行过滤）与 fire-and-forget Previous 拒绝
> （InvalidRule 分类）；InfraInvalid/InfraInvalidInsert 六个 compile-only execution 登记
> 为 intentionally-different approved difference（Go 以 ErrorCode 分类同诊断面）。
> manifest 更新为 538 cases、152 个 differential-verified case、19 个
> intentionally-different case、528 个 differential runtime IDs、3148 条 associations
>（referenced 2945）。
>
> 最新补充：Draft 4.239（2026-08-23），完成 `resultset.aggregate-dimensional` 的
> ResultSetQueryTypeRollupDimensionality 12 个新增 execution（累计 20/24 差分验证，
> BoundCube3Dim、ContextPartitionAlsoRollup、Invalid、UnboundGroupingSet2LevelUnenclosed
> 保持未验证登记）：新增 RollupMultikeyWArray 三形态（unbound/bound/join，`java-runtime-c3795d43…/19844b30…/effa54eb…`）、
> RollupMultikeyWArrayGroupingSet（`3f406a35…`）、NamedWindowCube2Dim（`e17c22ce…`）、
> OnSelect（`58abe8e5…`）、OutputWhenTerminated 五变体（`8178e315…`）、BoundGroupingSet2Level
> 两形态（`153edf6a…/23faa930…`）、MixedAccessAggregation（`a4a78ec3…`）、
> NonBoxedTypeWithRollup（`0a3b5f19…`）与 GroupByWithComputation（`3f45c2d3…`）共 12 个
> runtime 的差分验证。场景扩展至 33 case、Java/Go 各 141 条 records、0 differences。
> 引擎新增：`SelectFromNamedWindowRollup` on-select 分组聚合触发器（快照分组、首见组序、
> detail→overall 层级、粗化层非键字段投影 null）；聚合管道补齐「非本层 grouping set 的
> plain 字段投影 null」共享语义。协议新增 types 步骤与数组/EventBean 规范化渲染。
> manifest 更新为 536 cases、151 个 differential-verified case、516 个 differential
> runtime IDs、3130 条 associations（referenced 2927）。
>
> 最新补充：Draft 4.238（2026-08-23），完成 `epl.variable-onset` 全部 17
> 个 execution：新增 EPLVariableOnSetSubqueryMultikeyWArray
>（`java-runtime-d23e6717c5568c62cf50`）、EPLVariableOnSetArrayAtIndex
> {soda=false,soda=true}（`java-runtime-ad6256cf6e3091770906`、
> `java-runtime-f85344604842bf345a7a`）、EPLVariableOnSetArrayBoxed
>（`java-runtime-2788fec53520551b63b8`）、EPLVariableOnSetArrayInvalid
> runtime 行为（`java-runtime-36715ebec5e33bcb2cff`）与
> EPLVariableOnSetExpression（`java-runtime-691fcb9b5f0fe173348f`）共 6 个
> runtime 的差分验证。variables-onset-set 场景扩展至 11 case、Java/Go 各
> 54 条 records、0 differences；覆盖 int[] 内容等值分组标量子查询赋值、
> 数组元素就地写入与共享后备存储别名、Double[] null 装箱、越界索引精确
> 报错、null 索引/null RHS 静默跳过、对象变量 call-form 变换。引擎新增
> `SetVariableIndexExpr`/`SetVariableApply`；数组/call-form on-set 赋值按
> Java 语义不产生输出列。capability remaining 仅保留 Invalid approved
> difference 登记。manifest 更新为 535 cases、150 个 differential-verified
> case、504 个 differential runtime IDs、3118 条 associations（referenced
> 2915）。
>
> 最新补充：Draft 4.237（2026-08-22），扩展 `epl.variable-onset` 至
> 14/15 DV：新增 EPLVariableOnSetSimple/WithFilter/AssignmentOrderNoDup/
> AssignmentOrderDup/RuntimeOrderMultiple/Coercion 与
> EPLVariableUseVariableInFilter/VariableInFilterBoolean/SimpleSameModule
> 共 9 个 runtime 的差分验证。新建独立 v1 场景
> variables-onset-set（6 case，Java/Go 各 27 条 records、0 differences），
> variables-use 场景迁移至 v1 差分协议（Java/Go 各 7 条 records、0
> differences）；legacy variables-onset 三 case 保持原状未迁移。
> 引擎新增 VariableType 选项支持 typed-null 变量声明并为 on-set 触发器
> 补充迭代器快照。capability remaining 保留 subquery/multikey-array/
> array-index/inlined-class 等 6 个未覆盖 execution 及 Invalid approved
> difference 登记。manifest 更新为 534 cases、149 个 differential-verified
> case、498 个 differential runtime IDs、3112 条 associations（referenced
> 2909）。
>
> 最新补充：Draft 4.230（2026-08-22），新增 `epl.other.distinct` 的
> `epl-other-distinct` differential-verified 场景，对照固定 Java
> `EPLOtherDistinct.java` 的 `EPLOtherOutputSimpleColumn`
>（`java-runtime-4b62c52a89cf86a3843a`）、
> `EPLOtherOutputLimitEveryColumn`
>（`java-runtime-2e0f0a1356ea1b8e3299`）与 `EPLOtherBatchWindow`
>（`java-runtime-0318c1bce4d5e806aa11`）的 part-1 行为序列。三个
> case、Java/Go 各 13 条 listener records、0 differences；覆盖 keep-all
> 连续输出重复照发、output every 3 events bundle 首见折叠与跨 delivery
> 重置、length_batch(3) flush 去重保序。引擎修复：`finishOutput` 统一
> 在输出边界对完整交付 bundle 应用 select-distinct 首见去重（对齐
> Java `OutputProcessViewConditionDefault`），移除 OutputEvery/
> EveryTime 分支冗余副本。manifest 更新为 149 个 differential-verified
> case、472 个 differential runtime IDs、3105 条 runtime associations、
> 18 个 differential-verified capabilities，capability 达到 3/20 DV
> runtime。

> 最新补充：Draft 4.229（2026-08-21），新增 `resultset.orderby-simple` 的
> `orderby-simple` differential-verified 场景，对照固定 Java
> `ResultSetOrderBySimple.java` 的三个 execution：
> `ResultSetOrderBySimple`（`java-runtime-d530652f60616c332431`）、
> `ResultSetOrderByDescending`（`java-runtime-9668909b2b2f00769dab`）与
> `ResultSetOrderByMultipleKeys`
>（`java-runtime-92e69b0657b10a656fdc`）。三个 case、Java/Go 各 3 条
> listener records、0 differences；覆盖 length(5/10)+output every 6 events
> 的 asc/desc/multi-key 排序与 stable-tie 语义。Go 侧复用
> `OrderBy`/`Ascending`/`Descending`，无 `internal/esper` 改动。manifest
> 更新为 148 个 differential-verified case、469 个 differential runtime
> IDs、3105 条 runtime associations，`resultset.orderby-simple` capability
> 提升为 differential-verified（3/18 DV runtime）。

> 最新补充：Draft 4.228（2026-08-21），扩展
> `resultset.aggregate-dimensional` 的 `rollup-dimensionality`
> differential-verified 场景（bound/batch 切片），对照固定 Java
> `ResultSetQueryTypeRollupDimensionality.java` 的两个 execution：
> `BoundRollup2Dim`（`java-runtime-3b6467afa76b0966c475`）与
> `UnboundRollup2DimBatchWindow`
>（`java-runtime-274b66386ba625a8b24c`）。场景扩展为 16 个 case，Java/Go
> 各 66 条 listener records、0 differences；覆盖 length(3) 窗口过期空组行
>（键保留+sum=null）、length_batch(4) flush 的 new…old… IR 对、batch 内
> 无事件既有组的 null-sum 行。Go 侧复用 `LengthWindow`/`LengthBatch`/
> `WithOldStream`，无 `internal/esper` 改动。manifest 更新为 147 个
> differential-verified case、466 个 differential runtime IDs、3105 条
> runtime associations，capability 达到 8/24 DV runtime。

> 最新补充：Draft 4.227（2026-08-21），扩展
> `resultset.aggregate-dimensional` 的 `rollup-dimensionality`
> differential-verified 场景（cube 家族切片），对照固定 Java
> `ResultSetQueryTypeRollupDimensionality.java` 的两个 execution：
> `UnboundCubeUnenclosed`
>（`java-runtime-b06640d26b3b63075791`，三种等价语法三 case）与
> `UnboundCube4Dim`（`java-runtime-14c4aecdca8b446299f1`）。场景扩展为
> 14 个 case，Java/Go 各 65 条 listener records、0 differences；覆盖
> cube 位掩码行序（dim0 为最高位、缺维数递增：{0123},{012},{013},{01},
> {023},{02},{03},{0},{123},{12},{13},{1},{23},{2},{3},{}）、被聚合键列
> null 填充、跨键独立累计（R2/R3 的 c4 向量逐行冻结）与三种嵌套/显式
> 语法等价。修复 `internal/esper` 共享语义：`cubeGroupingSets` 的枚举
> 位序改为 dim0 最高位降序（原实现低位在前导致 cube 行序与 Esper 不
> 一致）；`internal/esper` 全量测试通过。manifest 更新为 146 个
> differential-verified case、464 个 differential runtime IDs、3103 条
> runtime associations，capability 达到 6/24 DV runtime。

> 最新补充：Draft 4.226（2026-08-21），新增
> `resultset.aggregate-dimensional` 的 `rollup-dimensionality`
> differential-verified 场景（unbound-rollup 家族第一切片），对照固定 Java
> `ResultSetQueryTypeRollupDimensionality.java` 的四个 execution：
> `UnboundRollup2Dim`（`java-runtime-30499c2e4ff9aece48b2`）、
> `UnboundRollup1Dim`（`java-runtime-b059890b735f776a9e03`，rollup/cube
> 一维退化两 case）、`UnboundRollupUnenclosed`
>（`java-runtime-e60ea25dc87dcfbdcc08`，三种等价语法三 case）与
> `UnboundRollup3Dim`（`java-runtime-f5da6be14e939f2b26cc`，
> rollup/grouping-sets × 非 join/join 四 case）。Java/Go 各 50 条 listener
> records、0 differences；覆盖细化→粗化→总体行序、被聚合键列 null 填充、
> 无界流单调累加、一维 rollup≡cube、嵌套/显式 grouping sets 语法等价
> （Go 以 GroupByGroupingSets 展开表达）、cartesian join priming 与
> JoinField 跨流键。Go 侧复用 `GroupByRollup`/`GroupByCube`/
> `GroupByGroupingSets`，无 `internal/esper` 改动。manifest 更新为 145 个
> differential-verified case、462 个 differential runtime IDs、3101 条
> runtime associations，`resultset.aggregate-dimensional` capability 提升
> 为 differential-verified（4/24 DV runtime）。

> 最新补充：Draft 4.225（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（套件收尾切片），对照固定 Java
> `EPLSubselectFiltered.java` 的四个 execution：`EPLSubselectSelectSceneOne`
>（`java-runtime-b2be42620bdc328d006e`）、
> `EPLSubselectSelectWithWhere2Subqery`
>（`java-runtime-1fcd5cdcc62015389f98`）、`EPLSubselectSubselectMixMax`
>（`java-runtime-9161a695d692feb6902d`）与 `EPLSubselectSubselectPrior`
>（`java-runtime-1072ab9fc42579eed1e5`）。场景扩展为 31 个 case，Java/Go
> 各 103 条 listener records、0 differences；覆盖 irstream 双流投影与逐出行
> 子查询重求值（old 流 {100,null} 判别点）、OR 双 WHERE 相关子查询门控、
> sort(1,measurement desc/asc) 通配符高低列、多语句 insert-into 链 +
> coalesce 去重门控。Go 侧复用 `WithOldStream`/`Or`/`SortWindow`/
> `Coalesce`/`NestedField`/`FromAny` + 多 plan 顺序部署，无 `internal/esper`
> 改动；oracle 以 Map-based SupportMarketDataBean/SupportSensorEvent 规避
> regression-lib classpath 依赖并新增 old 流记录。manifest 更新为 144 个
> differential-verified case、458 个 differential runtime IDs、3097 条
> runtime associations，`epl.subselect.filtered` capability 达到 26/27
> DV runtime（仅剩 SelectWildcardNoName auto-name approved difference）。

> 最新补充：Draft 4.224（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（多流 join 切片），对照固定 Java
> `EPLSubselectFiltered.java` 的三个 execution：
> `EPLSubselectSelectWhereJoined2Streams`
>（`java-runtime-b3ce74a1b603f1dfa644`）、
> `EPLSubselectSelectWhereJoined3Streams`
>（`java-runtime-21f4b30723f3e5256823`）与
> `EPLSubselectSelectWhereJoined3SceneTwo`
>（`java-runtime-71f4714a3f10241083e9`）。场景扩展为 27 个 case，Java/Go
> 各 92 条 listener records、0 differences；覆盖 S1⋈S2 与 S1⋈S2⋈S3 keepall
> join 外层驱动 S0 子查询、部分相关性语义（3-streams 子查询仅相关
> s1.p10/s3.p30，S2 只参与 join 门控——同输入下 R2 99 vs scene-two null
> 的差分对为判别点）、未完成 id 链不触发、窗口跨轮累积（R5 命中 R3 的
> 98）。Go 侧复用 `Join`/`JoinMany`/`OnEqual`/`OnSourcesEqual`/`JoinField`，
> 无 `internal/esper` 改动；oracle 以 Map-based SupportBean_S3 事件类型
> 规避 regression-lib classpath 依赖。manifest 更新为 143 个
> differential-verified case、454 个 differential runtime IDs、3093 条
> runtime associations，`epl.subselect.filtered` capability 达到 22/27
> DV runtime。其余 5 个 filtered execution 保持 implemented-only。

> 最新补充：Draft 4.223（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（wildcard 事件子查询切片），对照固定 Java
> `EPLSubselectFiltered.java` 的四个 execution：`EPLSubselectSameEvent`
>（`java-runtime-4ec1d23f545746f81f20`）、`EPLSubselectSameEventOM`
>（`java-runtime-0626e0d5b878384ba989`）、`EPLSubselectSameEventCompile`
>（`java-runtime-7705828482936493f505`）与 `EPLSubselectSelectWildcard`
>（`java-runtime-7e0f728f635710446f39`）。场景扩展为 24 个 case，Java/Go
> 各 80 条 listener records、0 differences；覆盖 `(select * ...)` 通配符
> 标量子查询的单事件列投影：自流子查询可见触发事件自身（Java assertSame
> 身份断言以 {kind:row,fields:{id,p10,p11}} 字段快照为观测等价物，两侧
> oracle/compat 渲染镜像）与跨流返回窗口先前事件。Go 侧复用
> `EventValue[esper.Event]` + `SubqueryValue`；join-filtered 的拼接谓词
> 改用无类型 `ConcatOf`+`JoinField[any]` 以兼容 S1 指针字段（既有 case
> records 字节级不变）。manifest 更新为 142 个 differential-verified
> case、451 个 differential runtime IDs、3090 条 runtime associations，
> `epl.subselect.filtered` capability 达到 19/27 DV runtime。其余 8 个
> filtered execution 保持 implemented-only。

> 最新补充：Draft 4.222（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（where-previous 切片），对照固定 Java
> `EPLSubselectFiltered.java` 的三个 execution：
> `EPLSubselectWherePrevious`（`java-runtime-78f47a5bcdf6b87c9e68`）、
> `EPLSubselectWherePreviousOM`（`java-runtime-b1c2169572264f9fd646`）与
> `EPLSubselectWherePreviousCompile`
> （`java-runtime-87fd9e194fcd61e0aba5`）——三变体行为逐字等价，各占一个
> case 重放同一序列。场景扩展为 20 个 case，Java/Go 各 76 条 listener
> records、0 differences；覆盖相关门控子查询投影内的 prev(1,id)：WHERE 只
> 筛选候选行、prev 读取未过滤 #length(1000) 窗口的到达序历史（round2
> prev=1 为判别点——若实现为先过滤后 prev 将得 null）、空集→Null。Go 侧
> 复用 `Prev` + `SubqueryValue` + `OuterField`，无 `internal/esper` 改动。
> manifest 更新为 141 个 differential-verified case、447 个 differential
> runtime IDs、3086 条 runtime associations，`epl.subselect.filtered`
> capability 达到 15/27 DV runtime。其余 12 个 filtered execution 保持
> implemented-only。

> 最新补充：Draft 4.221（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（join-gated 切片），对照固定 Java
> `EPLSubselectFiltered.java` 的两个 execution：
> `EPLSubselectJoinFilteredOne`
> （`java-runtime-7474133de58ff180f8fa`）与
> `EPLSubselectJoinFilteredTwo`
> （`java-runtime-0a6686ce79af715278f7`）。场景扩展为 17 个 case，Java/Go
> 各 67 条 listener records、0 differences；覆盖 S0⋈S1 keepall join 的
> WHERE 门控两种形态（标量子查询与 `p00||p10` 拼接比较 / 布尔子查询作
> WHERE 真值）、join 未满足或子查询无匹配时不触发、SELECT 内标量 +
> prior(1)/prev(1) 三列投影——prior/prev 读取未过滤窗口的跨 id 历史
> （round2 Prior=Prev="ab"），以及 #length(1000)/#length(10) 双内层窗口。
> Go 侧复用 `Join`/`OnEqual`/`SelectLeft`/`SelectRight`/`Concat`/
> `Prior`/`Prev` + `SubqueryValue`，无 `internal/esper` 改动。manifest 更新
> 为 140 个 differential-verified case、444 个 differential runtime IDs、
> 3083 条 runtime associations，`epl.subselect.filtered` capability 达到
> 12/27 DV runtime。其余 15 个 filtered execution 保持 implemented-only。

> 最新补充：Draft 4.220（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（Joined4 数值 coercion 切片），对照固定 Java
> `EPLSubselectFiltered.java` 的两个 execution：
> `EPLSubselectSelectWhereJoined4Coercion`
> （`java-runtime-fd19463d0ce39b10f445`，三个谓词排列 case）与
> `EPLSubselectSelectWhereJoined4BackCoercion`
> （`java-runtime-66c3d4d4100cb3f923a0`，两个谓词排列 case）。场景扩展为
> 15 个 case，Java/Go 各 63 条 listener records、0 differences；覆盖三路
> keepall join 外层（`s1.intPrimitive=s2.intPrimitive=s3.intPrimitive`）+
> 跨流 boxed 数值等值 coercion（int==long==double 精确提升）、精确拒绝
> 边界（整数差 1、double 差 0.0001 无容差）、谓词排列不变性、空子查询
> 窗口首轮 Null 与负值 intPrimitive 透传。Go 侧复用
> `JoinMany`/`JoinSource`/`OnSourcesEqual`/`JoinField` +
> `SubqueryValue`，无 `internal/esper` 改动。manifest 更新为 139 个
> differential-verified case、442 个 differential runtime IDs、3081 条
> runtime associations，`epl.subselect.filtered` capability 达到 10/27
> DV runtime。其余 17 个 filtered execution 保持 implemented-only。

> 最新补充：Draft 4.219（2026-08-21），扩展 `subselect-filtered`
> differential-verified 场景（multikey-wArray 切片），对照固定 Java
> `EPLSubselectFiltered.java` 的三个 execution：
> `EPLSubselectWhereClauseMultikeyWArrayPrimitive`
> （`java-runtime-643236df4a8946ab3c24`）、
> `EPLSubselectWhereClauseMultikeyWArray2Field`
> （`java-runtime-9255e3470adb866211bf`）与
> `EPLSubselectWhereClauseMultikeyWArrayComposite`
> （`java-runtime-7fbee5b6cef2f287ee41`）。场景扩展为 10 个 case，Java/Go
> 各 38 条 listener records、0 differences；覆盖 int[] 内容等值相关键
>（空数组==空数组、null 数组==null 数组、长度/元素不等不匹配）、数组等值
> AND 标量等值/严格大于组合、多命中→Null（`SubqueryNullOnMultiple`）。
> Go 侧复用 `Is`/`And`/`OuterField` + `SubqueryValueWithOptions`，无
> `internal/esper` 改动；oracle runner 的 javac 步骤补充编译两个
> regression-lib 事件类源文件。固定 Java oracle、runner、scenario、trace、
> evidence 与 4 个新 trace mutations 已纳入兼容资产；manifest 更新为
> 138 个 differential-verified case、440 个 differential runtime IDs、
> 3079 条 runtime associations，`epl.subselect.filtered` capability 达到
> 8/27 DV runtime。其余 19 个 filtered execution 保持 implemented-only。

> 最新补充：Draft 4.218（2026-08-21），新增 `subselect-filtered`
> differential-verified 场景（第一切片），对照固定 Java
> `EPLSubselectFiltered.java` 的五个 execution：HavingNoAgg 三连
> （`java-runtime-bc6684a32b1cda80e244`、
> `java-runtime-9511e607f74ca4551624`、
> `java-runtime-0ef90e75f854b7845de0`）、`EPLSubselectWhereConstant`
> （`java-runtime-57e3956886ac655d387d`，三个 isolated cases）与
> `EPLSubselectSelectWithWhereJoined`
> （`java-runtime-6034a5785b901739431e`）。七个 case、Java/Go 各 24 条
> listener records、0 differences；覆盖非聚合 having 逐行过滤、where/having
> AND 组合、流 filter 与 where 的可区分排除路径、常量/双列/范围 where、
> 多行标量子查询→Null（`SubqueryNullOnMultiple`）、空集→Null 与
> `p10=s0.p00` 相关匹配。Go 侧复用 `SubqueryValue[WithOptions]` +
> `SubqueryWhere`/`SubqueryHaving`，无 `internal/esper` 改动。固定 Java
> oracle、runner、scenario、trace、evidence 与 value/null/order/mutation
> tests 已纳入兼容资产；manifest 更新为 137 个 differential-verified
> case、437 个 differential runtime IDs、3076 条 runtime associations，
> `epl.subselect.filtered` capability 提升为 differential-verified（5/27
> runtime）。其余 22 个 filtered execution 与 auto-generated column names
> approved difference 保持 implemented-only。

> 最新补充：Draft 4.217（2026-08-21），扩展 `query.subquery` 的
> `subselect-quantified` differential-verified 场景，补齐固定 Java
> `EPLSubselectAllAnySomeExpr` 的两个 null/空集 execution：
> `EPLSubselectRelationalOpNullOrNoRows`
> （`java-runtime-ad44331ab7f0645a4e7a`）与
> `EPLSubselectEqualsInNullOrNoRows`
> （`java-runtime-29c66b744c3754d59ec7`）。场景新增
> `relational-null-no-rows` 与 `equals-in-null-no-rows` case，Java/Go 各
> 40 条 listener records、0 differences；覆盖空集 ALL=true/ANY=false/IN=false、
> `{null}` 全 Null、`{null,1}` 混合集合三值边界与 Integer/Double
> coercion。修复 `internal/esper` 共享语义：relational ANY 的
> false-dominates-unknown（对照 Java
> `SubselectForgeStrategyNRRelOpAnyDefault`）；`case.subquery-empty-quantifiers`
> 提升为 differential-verified。manifest 更新为 136 个
> differential-verified case、432 个 differential runtime IDs、3071 条
> runtime associations；唯一已关联 runtime 仍为 2901，未关联 1235。
> `EPLSubselectInvalid` 保持 compile-time-only approved difference。

> 最新补充：Draft 4.216（2026-08-21），新增 `query.subquery` 的
> `subselect-multirow` differential-verified 场景，对照固定 Java
> `EPLSubselectMultirow.java` 的两个 execution：
> `EPLSubselectMultirowSingleColumn`
> （`java-runtime-29c2087cc4243e9b7a50`）与
> `EPLSubselectMultirowUnderlyingCorrelated`
> （`java-runtime-64eb1701d14bdbcefc86`）。场景覆盖 8 条单列/命名窗口
> 生命周期发送和 6 条相关 underlying 发送，Java/Go 各 6 条 listener
> records、0 differences；`Integer[]` 直接窗口快照为 `[5,10,15,6]`、
> `[10,15,6]`、`[15,6,5]`，相关空匹配为 Null，T1/T2 underlying 行保留
> 字段与顺序归一化。固定 commit 的 oracle、runner、scenario、trace、
> evidence 与 value/window/null/order/type/lifecycle mutation tests 已纳入
> 兼容资产；manifest 更新为 135 个 differential-verified case、430 个
> differential runtime IDs、3069 条 runtime associations；唯一已关联 runtime
> 仍为 2901，未关联 runtime 为 1235。嵌套 `SubqueryRow` 空 map 属性风险仍
> deferred，未纳入本切片。

> 最新补充：Draft 4.215（2026-08-21），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreCast` 的
> `ExprCoreCastDates` execution
> （`java-runtime-2ee2b8ab1bf9bb2c4e90`），三个 isolated cases、3 条
> listener records、0 differences（累计 54 条）。`cast-dates-base`
> 覆盖 date/java.util.Date、long/java.lang.Long、
> calendar/java.util.Calendar 目标加 `.get("month")` 链（epoch
> 1273449600000、月份 4）；`cast-dates-java8` 覆盖 localdate/
> localdatetime/localtime 别名与 FQCN 目标（ISO 字符串渲染）；
> `cast-dates-constant` 覆盖常量输入编译期折叠（1044057600000）。
> Go 侧以 `CastWithLayout[string,time.Time]` + `UnixMillis`/`Month`
> 组合表达，无 internal/esper 改动。zoneddatetime VV 时区、ISO8601
> iso 模式、动态 dateformat、formatter 对象参数与编译诊断 deferred。
> manifest 保持 134 个 differential-verified case，更新为 428 个
> differential runtime IDs。

> 最新补充：Draft 4.214（2026-08-21），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreCast` 的
> `ExprCoreCastGeneric` execution
> （`java-runtime-2fcd2094aaf18ca4ce03`），一个 isolated case、2 条
> listener records、0 differences（累计 51 条）。Go 侧以
> `Cast[any, []any]`/`Cast[any, map[string]any]` 覆盖 8 个擦除泛型
> 目标（`List<String>`、`List<Optional<Integer>>`、
> `Map<String,Integer>`、`List<String>[]`、`List<String[]>`、
> `List<String>[][]`、`List<String[][]>`、`List<Object>`），元素经
> erasure identity 直传；Optional 元素渲染 `Optional[10]` token，List/
> Map 递归 JSON（map 键排序），空 map send 八列全 null。Java EPL/
> SODA/compile entry details、日期 casts 和完整诊断矩阵仍为
> implemented-only。固定 Java oracle、runner、scenario、trace、
> evidence 与 value/null/order/mutation tests 已纳入兼容资产；
> manifest 保持 134 个 differential-verified case，更新为 427 个
> differential runtime IDs。

> 最新补充：Draft 4.213（2026-08-20），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreCast` 的
> `ExprCoreCastWArray` 两个 execution（soda=false
> `java-runtime-53d0455ef4e377c9c2f9`、soda=true
> `java-runtime-58852773df609efe7d69`），两个 isolated case、4 条
> listener records、0 differences（累计 49 条）。Go 侧以
> `Cast[any, []T]` 链式 insert-into（`MyArrayEvent` struct 目标）
> 覆盖 9 个 array cast 目标：`string[]`、`int[primitive]`、
> `Integer[]`（×2）、`Object[]` 及 2/3 维 `int[primitive][]`/
> `Object[][]` 组合，元素逐层递归 coercion，SupportBean 元素渲染为
> 字段级 token `SupportBean(E1,0)`，空 map send 九列全部 null。
> Java EPL/SODA/compile entry details、日期/generic casts 和完整
> 诊断矩阵仍为 implemented-only。固定 Java oracle、runner、
> scenario、trace、evidence 与 value/null/order/mutation tests 已
> 纳入兼容资产；manifest 保持 134 个 differential-verified case，
> 更新为 426 个 differential runtime IDs。

> 最新补充：Draft 4.212（2026-08-20），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreCast` 的
> `ExprCoreCastInterface` execution
> （`java-runtime-2012a048edc6511a33e0`），一个 isolated case、5 条
> listener records、0 differences（累计 45 条）。Go 侧沿用 `Cast[A,B]`
> + 规则 bean/interface marker，覆盖 8 个目标 `cast(item?, T)` 的
> assignable/Implements 鉴别：SupportBeanDynRoot 双包装→t0、
> ISupportDImpl→t5、ISupportBCImpl→t2+t4、
> ISupportAImplSuperGImplPlus→t1/t2/t4/t6/t7、
> ISupportBaseABImpl→t2+t3；每目标 cell 以 Java 简单类名 token
> 确定性渲染，避免身份 hash。修复 `internal/esper` 共享语义：
> `typeOf` 的 interface 类型参数此前解析为 `any`（任意值满足全部
> target），现经 `reflect.TypeOf((*T)(nil)).Elem()` 解析静态 interface
> 类型；全量 `internal/esper` 单测回归通过。Java EPL/SODA/compile
> entry details、日期/array/generic casts 和完整诊断矩阵仍为
> implemented-only。固定 Java oracle、runner、scenario、trace、evidence
> 与 value/null/order/mutation tests 已纳入兼容资产；manifest 保持
> 134 个 differential-verified case，更新为 424 个 differential
> runtime IDs。

> 最新补充：Draft 4.211（2026-08-20），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreCast` 的
> `ExprCoreCastBigDecimalBigInt` execution
> （`java-runtime-f44847213060b8eb3949`），一个 isolated case、8 条
> listener records、0 differences（累计 40 条）。Go 侧复用类型化
> `Cast[A,B]` 覆盖 BigDecimal/BigInteger 动态 cast：int/long 直传、
> double→BigDecimal 经最短十进制往返（`BigDecimal.valueOf(double)`
> 语义，2.4→“2.4”，1.0→“1”）、decimal/bigint 精确文本、float→
> BigInteger 向零截断、2^500500 与 2^500500+0.1 边界向量证明任意
> 精度、以及 null 传播。修复 `internal/esper` 生产语义：
> `castToBigRat` 的 float 分支改用 `strconv.FormatFloat(v,'g',-1,bits)`
> + `big.Rat.SetString`（原 `SetFloat64` 二进制精确与 Java 不符）。
> Java EPL/SODA/compile entry details、日期/interface/array/generic
> casts 和完整诊断矩阵仍为 implemented-only。固定 Java oracle、runner、
> scenario、trace、evidence 与 value/null/order/mutation tests 已纳入
> 兼容资产；manifest 保持 134 个 differential-verified case，更新为
> 423 个 differential runtime IDs。

> 最新补充：Draft 4.210（2026-08-20），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreCast` 的三个可观测
> execution（`ExprCoreCastStringAndNullCompile`
> `java-runtime-9957cb6d9cd9ea836d4d`、`ExprCoreCastBoolean`
> `java-runtime-0b91403db9a899efde99`、`ExprCoreCastWStaticType`
> `java-runtime-9babbbb6f96faf389bb7`），三个 isolated case、10 条 listener
> records、0 differences（累计 32 条）。Go 侧复用类型化 `Cast[A,B]`、bool
> `BitwiseOrOf` 与 `parseStringNumber`，覆盖 dyn-root `item?` cast-to-String
> 的 Java number-to-String 渲染（int/byte/double/long/null/string）、
> SupportBean bool boxed/primitive cast 与 null 传播的 boolean OR、以及
> `StaticTypeMapEvent` 字符串数值 parse（`0x0A` hex→byte、`1.4E-1`→double、
> `1.001`→float、`223`→short）；无 `internal/esper` 生产改动。Java
> EPL/SODA/compile entry details、日期/interface/array/generic/BigDecimal
> casts 和完整诊断矩阵仍为 implemented-only。固定 Java oracle、runner、
> scenario、trace、evidence 与 value/null/order mutation tests 已纳入兼容
> 资产；manifest 保持 134 个 differential-verified case，更新为 422 个
> differential runtime IDs。

> 最新补充：Draft 4.209（2026-08-20），扩展 `expr-core-exists-cast`
> differential-verified 场景，对照固定 Java `ExprCoreExists` 的四个 execution
> 与 `ExprCoreCast` 的四个可观测 execution（`ExprCoreCastSimple`
> `java-runtime-3fc2cde530f321dc3994`、`ExprCoreCastSimpleMoreTypes`
> `java-runtime-5c9fdd17b5480f9d78e2`、`ExprCoreCastAsParse`
> `java-runtime-52fb8d57dd380f5efe6e`、`ExprCoreCastDoubleAndNullOM`
> `java-runtime-45b9d5a2e216f8063b75`），八个 isolated case、22 条 listener
> records、0 differences。Go 侧复用类型化 `Cast[A,B]`，覆盖 numeric/primitive/
> boxed/dynamic cast、char 首字符转换、BigInteger/BigDecimal-equivalent 值、
> string-to-int parse、Null 和 incompatible dynamic values；修复 char cast
> 多字符输入取首字符。Java EPL/SODA/compile entry details、日期/interface/
> array/generic/Boolean casts 和完整诊断矩阵仍为 implemented-only。固定 Java
> oracle、runner、scenario、trace、evidence 与 value/order/null/lifecycle/time
> mutation tests 已纳入兼容资产；manifest 保持 134 个 differential-verified
> case，更新为 419 个 differential runtime IDs。

> 最新补充：Draft 4.207（2026-08-20），新增 `expr-core-type-name`
> differential-verified 场景，对照固定 Java `ExprCoreTypeOfFragment` 的一个
> 可观测 execution（`java-runtime-eeeacbe2c7669e1e1207`），六个
> representation case、18 条 listener records、0 differences。Go 侧复用
> 类型化 `TypeName(Expr)`，新增声明式 fragment metadata 解析，覆盖
> object-array、map、Avro、JSON、JSON-provided、default 的 `InnerSchema`/
> `InnerSchema[]` 名称、Avro 空/Null 元数据和非 Avro Null/Missing；普通 Go
> reflection type name 保持不变。POJO simple-name、invalid compile、dynamic
> wrapper-name 与 variant/match-recognize 仍保持 implemented-only。固定 Java
> oracle、runner、scenario、trace、evidence 与 value/order/case/time mutation
> tests 已纳入兼容资产；manifest 达到 133 个 differential-verified case、411
> 个 differential runtime IDs。

> 最新补充：Draft 4.206（2026-08-20），新增 `expr-core-instanceof`
> differential-verified 场景，对照固定 Java `ExprCoreInstanceOf` 的五个
> execution（`ExprCoreInstanceofSimple`
> `java-runtime-f57ec2f2f04aad28961d`、`ExprCoreInstanceofStringAndNullOM`
> `java-runtime-fc4c87ba7677b72cab9a`、`ExprCoreInstanceofStringAndNullCompile`
> `java-runtime-c5af420270464e5633f7`、`ExprCoreDynamicPropertyJavaTypes`
> `java-runtime-0423d81802ef0e7eb910`、`ExprCoreDynamicSuperTypeAndInterface`
> `java-runtime-f446c95462162907b0c6`），五个 isolated case、17 条 listener
> records、0 differences。Go 侧复用类型化 `InstanceOf[T]` 和 `Or`，覆盖
> primitive/wrapper 数值向量、dynamic property 的 String/Float/Integer/Long
> 与 Null、以及 interface/supertype hierarchy matching 和 fresh listener
> lifecycle；并修复 interface target 错误解析为 `any` 的语义缺陷。Java
> EPL/SODA/compile text forms、primitive-wrapper name aliases、optional
> property/missing 与完整 type-diagnostic matrix 仍在差异边界。固定 Java
> oracle、runner、scenario、trace、evidence 与 value/order/case/time mutation
> tests 已纳入兼容资产；manifest 达到 132 个 differential-verified case、410
> 个 differential runtime IDs。

> 最新补充：Draft 4.205（2026-08-20），新增 `expr-core-case`
> differential-verified 场景，对照固定 Java `ExprCoreCase` 的三个可观测
> execution（`ExprCoreCaseSyntax1WithElse`
> `java-runtime-e9ae8d32b9f6155877ef`、`ExprCoreCaseSyntax1Branches3`
> `java-runtime-725a9999f48d69d792a2`、`ExprCoreCaseSyntax2`
> `java-runtime-0f635579498bb6de3a4c`），三个 isolated case、9 条 listener
> records、0 differences。Go 侧复用类型化 `CaseWhen[T]`/`CaseValue[T]`，
> 覆盖 searched CASE 的 ELSE/多分支顺序、simple CASE 跨 int/long/float/
> double matching 和 fresh deployment lifecycle；existing `JOE` unmatched
> supplemental test 不进入严格 differential vector。固定 Java oracle、runner、
> scenario、trace、evidence 与 value/order/case/time mutation tests 已纳入
> 兼容资产；manifest 达到 131 个 differential-verified case、405 个
> differential runtime IDs，其余 CASE execution 保持 implemented-only。

> 最新补充：Draft 4.204（2026-08-20），新增 `expr-core-equals-is`
> differential-verified 场景，对照固定 Java `ExprCoreEqualsIs` 的四个可
> 观测 execution（`ExprCoreEqualsIsCoercion`
> `java-runtime-fdef3bed6ec0b16d36db`、`ExprCoreEqualsIsCoercionSameType`
> `java-runtime-1eaef3b328c31a863b26`、`ExprCoreEqualsIsMultikeyWArray`
> `java-runtime-2fb582ea3ac2dc026c82`、`ExprCoreEqualsNull`
> `java-runtime-d6084d5b7a191cbde6cd`），四个 isolated case、8 条 listener
> records、0 differences。Go 侧复用类型化 `EqualOf`/`NotEqualOf`、`Is`/
> `IsNot`，覆盖 int/long coercion、same-type string、primitive/boxed/
> two-dimensional/object array 深度与 shape equality，以及 SQL-style Null
> 与 Null-safe `is`；同时修复 interface-typed primitive literal 未保留具体
> 类型导致的 Plan identity 折叠。固定 Java oracle、runner、scenario、trace、
> evidence 与 payload/order/null/lifecycle mutation tests 已纳入兼容资产；
> invalid compile execution 仍保持 implemented-only，manifest 达到 130 个
> differential-verified case、402 个 differential runtime IDs。

> 最新补充：Draft 4.203（2026-08-20），新增 `expr-core-in-between`
> differential-verified 场景，对照固定 Java `ExprCoreInBetween` 的五个
> execution（`ExprCoreInNumeric`
> `java-runtime-791d833152f1266fa139`、`ExprCoreInStringExpr`
> `java-runtime-ffe28bce1e8e2a0dc4a0`、`ExprCoreBetweenStringExpr`
> `java-runtime-143c2fcf5bb1e4e6aa4a`、`ExprCoreBetweenNumericExpr`
> `java-runtime-1145ffc38eea9ce1624e`、`ExprCoreInRange`
> `java-runtime-31934462edbc04c83973`），五个 isolated case、164 条
> listener records、0 differences。Go 侧复用类型化 `InOf`/`NotInOf`、
> `BetweenOf`/`NotBetweenOf` 和 range builders，覆盖 IN Null 传播、BETWEEN
> Null/reversed-bound 语义、数值精度、四种 range endpoint policy、string
> range，以及 `s0`/`s1`/`s2` 的 deploy/undeploy 生命周期；并修复 range
> endpoint policy 未进入 Plan identity 的缺陷。固定 Java oracle、runner、
> scenario、trace、evidence 和 value/order/null/statement/time mutation
> tests 已纳入兼容资产；manifest 达到 129 个 differential-verified case、
> 398 个 differential runtime IDs，其余 IN/BETWEEN inventoried execution
> 保持 implemented-only。

> 最新补充：Draft 4.202（2026-08-20），新增 `expr-core-like-regexp`
> differential-verified 场景，对照固定 Java `ExprCoreLikeRegexp` 的四个
> execution（`ExprCoreLikeWConstants`
> `java-runtime-376f347aa8fc8367fcbc`、`ExprCoreLikeWExprs`
> `java-runtime-09f15b6eb39cbee59b0d`、`ExprCoreRegexpWConstants`
> `java-runtime-7dc8b98263cf5e9e4c3f`、`ExprCoreRegexpWExprs`
> `java-runtime-8a8794063a0b9c4bfd0e`），四个 isolated case、19 条
> listener records、0 differences。Go 侧复用类型化 LIKE/REGEXP builders，
> 覆盖 `%`/`_` 全字符串匹配、动态 pattern、Java 数值文本、REGEXP
> full-match、Null 和 substring discriminator；并修复 REGEXP 原先的
> substring 匹配缺陷。固定 Java oracle、runner、scenario、trace、evidence
> 与 value/order/null/record/time mutation tests 已纳入兼容资产；manifest
> 更新为 128 个 differential-verified case、393 个 differential runtime
> IDs；其余六个 LIKE/REGEXP inventoried execution 仍为 implemented-only。

> 最新补充：Draft 4.201（2026-08-20），新增 `expr-core-relop`
> differential-verified 场景，对照固定 Java `ExprCoreRelOp` 的两个
> execution（`ExprCoreRelOpTypes`
> `java-runtime-615cb125ab25e488f40c`、`ExprCoreRelOpNull`
> `java-runtime-402d95bd69d700735100`），十个 isolated case、30 条
> listener records、0 differences。Go 侧复用类型化
> `GreaterOf`/`GreaterOrEqualOf`/`LessOf`/`LessOrEqualOf`，覆盖字符串、
> int/long/float/double、BigDecimal/BigInteger 混合比较及 boxed/null
> 三值结果；Java EPL/parser/type metadata 继续保留在差异边界。固定
> Java oracle、runner、scenario、trace、evidence 和 value/order/null/
> time mutation tests 已纳入兼容资产；manifest 更新为 127 个
> differential-verified case、389 个 differential runtime IDs。

> 最新补充：Draft 4.200（2026-08-20），新增 `expr-core-coalesce`
> differential-verified 场景，对照固定 Java `ExprCoreCoalesce` 的六个可
> 观测 execution（`ExprCoreCoalesceBeans`
> `java-runtime-6464df255cd4e23a89e7`、`ExprCoreCoalesceLong`
> `java-runtime-0a352fd0c30e1b3a175c`、`ExprCoreCoalesceLongOM`
> `java-runtime-0ee4f2c3f1d9129e2598`、`ExprCoreCoalesceLongCompile`
> `java-runtime-7cac5278b06087f8e7f2`、`ExprCoreCoalesceDouble`
> `java-runtime-9341a1983c1f4fb3fdda`、`ExprCoreCoalesceNull`
> `java-runtime-c9475d47ffbc6275e330`），六个 isolated case、22 条
> listener records、0 differences。Go 侧复用类型化 `Coalesce`/
> `CoalesceOf`，覆盖 bean/event identity、first-non-null、missing/null、
> long/double numeric promotion 和 all-null result；invalid compile
> execution 保留为 implemented-only。固定 Java oracle、runner、scenario、
> trace、evidence 和 value/order/null/record mutation tests 已纳入兼容资产；
> manifest 更新为 126 个 differential-verified case、387 个 differential
> runtime IDs；已关联 runtime 总数保持 2,901，未关联 runtime 保持 1,235。

> 最新补充：Draft 4.199（2026-08-20），新增 `expr-core-logical`
> differential-verified 场景，对照固定 Java `ExprCoreAndOrNot` 的三个
> execution（`ExprCoreAndOrNotCombined`
> `java-runtime-e48bf14356e3aeb838b5`、`ExprCoreNotWithVariable`
> `java-runtime-c63d599754acde7bb4dc`、`ExprCoreAndOrNotNull`
> `java-runtime-b9d938f2dc52682b77c0`），三个 isolated case、13 条
> listener records、0 differences。Go 侧复用类型化 `And`/`Or`/`Not`、
> `Contains`、nullable `Property` 和部署期变量直接更新，覆盖整数组合谓词、
> 变量更新前后 `not contains`、boxed Boolean 三值真值矩阵、投影顺序和
> explicit null。固定 Java oracle、runner、scenario、trace、evidence 和
> value/order/field/time mutation tests 已纳入兼容资产；manifest 更新为
> 125 个 differential-verified case、381 个 differential runtime IDs。

> 最新补充：Draft 4.198（2026-08-20），新增 `expr-dt-between`
> differential-verified 场景，对照固定 Java `ExprDTBetween` 的三个
> execution（`ExprDTBetweenIncludeEndpoints`
> `java-runtime-6593e0f0cc79ed906f52`、`ExprDTBetweenExcludeEndpoints`
> `java-runtime-5e744f4720368eb6585b`、`ExprDTBetweenTypes`
> `java-runtime-c0d2bebfa077f7e51474`），九个 isolated case、68 条
> listener records、0 differences。Go 侧新增 typed
> `DateTimeBetween`/`DateTimeBetweenWithEndpoints`/
> `DateTimeBetweenRangeOf`/`DateTimeAfter` builders，覆盖 virtual-time
> before/at/inside/after 边界、常量 calendar bounds、runtime boolean
> endpoint flags、reversed inclusive bounds，以及 long/Date/Calendar/
> LocalDateTime/ZonedDateTime 的 epoch-millisecond 等价；types 使用
> `SupportDateTime unidirectional, SupportBean#lastevent`，exclude cases
> 共享部署生命周期，并补充 null/missing field 与 null flag。固定 Java
> oracle、scenario、trace、evidence 和 value/order/field/time mutation tests
> 已纳入兼容资产；manifest 更新为 124 个 differential-verified case、378
> 个 differential runtime IDs。

> 最新补充：Draft 4.197（2026-08-20），新增 `expr-core-current-evaluation-context` differential-verified 场景，对照固定 Java `ExprCoreCurrentEvaluationContext` 的两个 execution（`ExprCoreCurrentEvalCtx{soda=false}` `java-runtime-2efbbb4fce55aa6ce513`、`ExprCoreCurrentEvalCtx{soda=true}` `java-runtime-ca0799a6f2d163a49f7f`），两个 isolated case、两条 listener records、0 differences。Go 侧复用类型化 `CurrentEvaluationContext()`、`Property`、`WithRuntimeURI` 和 `WithStatementUserObject`，覆盖重复 context 投影、`getRuntimeURI()` accessor、statement name、user object、非 context partition ID `-1` 与 virtual time 0；Java boxed context、EPL/SODA model 入口和完整 lifecycle payload 继续保持差异边界。固定 commit Java oracle、runner、scenario、trace、evidence 和 metadata/order/field/time mutation tests 已纳入兼容资产；manifest 更新为 123 个 differential-verified case、375 个 differential runtime IDs。

> 最新补充：Draft 4.196（2026-08-20），新增 `expr-core-current-timestamp` differential-verified 场景，对照固定 Java `ExprCoreCurrentTimestamp` 的三个 execution（`ExprCoreCurrentTimestampGet` `java-runtime-c1c1fd3dc31af4864a50`、`ExprCoreCurrentTimestampOM` `java-runtime-96c8b8cb4cf36a523669`、`ExprCoreCurrentTimestampCompile` `java-runtime-5b126fe7fb865be8b293`），三个 isolated case、四条 listener records、0 differences。Go 侧复用类型化 `CurrentTimestamp()` 和虚拟时钟，覆盖未命名 `current_timestamp()` 字段、重复引用、加一运算，以及 100/999/777 毫秒绝对时间；Java boxed Long 元数据和文本编译诊断继续保持差异边界。Java oracle、固定 commit runner、scenario、trace、evidence 和 value/order/field/time mutation tests 已纳入兼容资产；manifest 更新为 122 个 differential-verified case、373 个 differential runtime IDs。

截至 2026-08-25，manifest v2 的已校验摘要为：

| 维度 | 数值 |
| --- | --- |
| Capability | 113 |
| Case | 552 |
| Case differential-verified | 163 |
| Differential-verified runtime | 626 / 4,136 |
| Runtime 已关联 | 3,251 / 4,136（78.6%） |
| Runtime 未关联 | 1,089 |
| Representative scenario | 94 / 94 通过 |
| Intentionally-different case | 23 |
| NFR-verified case | 0 |
| 质量摘要 | Docker passed；stress passed；race passed；performance pending |

该表是阅读便利快照，不应手工推导后继续传播。每次需要最新数字时直接读取 manifest `summary`；只有 manifest 校验通过后才更新本表。

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

## 2. 历史状态快照（2026-08-14，非权威）

本节保留旧阶段背景，数字和“最新提交”不再作为当前事实来源。当前状态使用第 0 节和 manifest summary，逐切片历史使用 CHANGELOG 与 Git。

### 2.1 代码与分支

- 分支：`master`
- 工作树：干净（最近一次提交已完成门禁）。
- 最近切片：ContextNested 提升为 implemented（非 temporal key/category/hash 嵌套、三层 key 隔离、parent context 属性、nested selector、重复 child 拒绝；temporal/initiated/pattern/iterator 仍 open）；ContextKeySegmentedWInitTermPrioritized 七个显式 initiated executions 提升为 implemented（keyed initiated-terminated grouped aggregate、correlated termination、no-term、filter-expr、invalid）；InfraNWTableCreateIndex late-create/drop/recreate/multiple-index/invalid 提升为 implemented（live NamedWindow/Table CreateIndex/DropIndex、Context partition 继承）；ResultSetOutputLimitAggregateGrouped 八个 no-join executions 提升为 implemented（grouped time-window default/last/first/snapshot/having/max/no-output-clause）；ResultSetOutputLimitRowPerGroupRollup 十个 no-join executions 提升为 implemented（rollup default/last/first/snapshot/order-limit/sorted）；ExprFilterOptimizableConditionNegateConfirm 十个 listener executions 提升为 implemented（typed context/pattern boolean filter 矩阵）；InfraNWTableOnSelect 非聚合 executions 提升为 implemented（on-select index/correlation/condition/limit/invalid、trigger order-by+limit 与 aggregate/prev 校验）；EPLOtherCreateSchema 主要 executions 提升为 implemented（typed create-schema、copyfrom/inherit/variant、ObjectArray 单 supertype）；EPLOtherStaticFunctions 主要 executions 提升为 implemented（Go UDF 静态方法对照、chained/nested/pattern/order-by、投影行源事件保留）；EPLDatabaseJoin 主要 join executions 提升为 implemented（2HistoricalStar/Inner 触发历史 lineage、WithPattern pattern 驱动求值、3Stream 无触发替换）；ResultSetQueryTypeIterator 17 个 execution 提升为 implemented（order-by/filter/pattern/aggregate iterator 契约、WithIterableUnbound）；ResultSetQueryTypeRowPerGroup 17 个 execution 提升为 implemented（group reclaim、array/null group key、output snapshot iterator、join/named-window grouped aggregate）；InfraNamedWindowTypes 18 个 execution 提升为 implemented（窗口事件类型形状、嵌套 schema 列、表示矩阵、继承覆盖）；ViewTimeWin 15 个 execution 提升为 implemented（calendar-month 窗口、变量/参数时长、prev 与聚合、flip-timer）；ViewUnion 15 个 execution 提升为 implemented（named-window union retention、batch/sorted/groupwin/pattern/subquery union、child-delta old 流）；EPLInsertInto 20 个 execution 提升为 implemented；statement metrics CPU 采样差异登记为 approved intentional difference（2026-08-14）。
- 最新进展：`subselect-in` 登记为 differential-verified（`EPLSubselectIn` 14 个 execution，14 个 case、52 条 records/0 differences）：IN/NOT IN 在 select/filter/where 的位置形态、nullable/coercion 三值逻辑、空集 `not in` → true、length(2) 淘汰边界、keepall 相关 IN 索引形态；复用既有 SubqueryIn/In/Not 与 boxed 值语义，无新增运行时改动；
- 最新进展：`subselect-aggregated-single-value` 登记为 differential-verified（`EPLSubselectAggregatedSingleValue` 13 个 execution，14 个 case、57 条 records/0 differences）：无窗口累积 sum、OuterField+聚合混合投影、相关 count/where/having、分组 scalar 子查询（单组多事件折叠）、table + into-table having 子查询；`SubquerySum/SubqueryAvg` 包装聚合投影、`SubqueryGroupScalar` 单组判定、table 子查询行投影用 `Field`；
- 最新进展：`subselect-aggregated-in-exists-any-all` 登记为 differential-verified（`EPLSubselectAggregatedInExistsAnyAll` 13 个 execution，13 个 case、47 条 records/0 differences）：IN/EXISTS/ALL/ANY/SOME 聚合子查询的空集 SQL 语义（无分组空输入 null 行、分组空集 false/true 规则）、`last/first(theString)` having 按组过滤、named window + FAF delete-all 复位；公共 API 新增 `SubqueryGroupKey`，compat 回放协议新增 `faf` step；
- 最新进展：`resultset-aggregate-count-sum` 登记为 differential-verified（`ResultSetAggregateCountSum.java` 9 个 execution、9 个 case、55 条 records/0 differences）：count(*) 星号投影的累积计数、无窗口 sum/count HAVING 进入与退出、SODA grouped count/count-distinct/count 的 null 与窗口淘汰重算、`avg(count(*))` 的有界窗口历史前缀语义；保留既有 view/join/named-window 四 execution，`CountDistinct` 按 boxed 指向值去重，grouped irstream 行序和 named-window on-delete iterator 快照均已核对；无效编译 execution 不在本轮范围。
- 最新进展：`resultset-aggregate-ever` 登记为 differential-verified（`ResultSetAggregateFirstEverLastEver.java` 3 个可表示 execution、3 个 runtime、14 条 records/0 differences）：SODA/EPL firstever/lastever/first/last/countever 的 length(2) current/ever/filter 轨迹，null boxed 值与窗口淘汰，以及 keepall named-window on-delete 删除后保留 ever 历史；`countever(distinct ...)` compile-error execution 因 Go 类型安全 API 不可表示保持 implemented-only。
- 最新提交：`35017ea53`（Add subselect-in differential scenario）
- 最新提交：`8c43e4856`（Update roadmap latest commit）
- 最新提交：`9510dcd91`（Add resultset-aggregate-limit-snapshot differential scenario）

### 2.2 对账清单

| 维度 | 数值 |
| Capability | 118 个 |
| Case | 562 个 |
| Case implemented（verification） | 560 个（其中 174 个 differential-verified、23 个 intentionally-different） |
| Case differential-verified | 174 个（687 个 runtime） |
| Case intentionally-different | 23 个 |
| Case inventoried-only | 0 个 |
| Java inventory runtime | 4,136 个 `status=ok` runtime |
| Runtime 关联 | 3,310 条 |
| 唯一已关联 runtime | 3,102 个 |
| 未关联 runtime | 1,034 个 |
| 关联覆盖率 | 75.0% |
| Representative scenario | 101/101 通过 |
| NFR | 0 个已验证 |
| Docker integration | MySQL/Kafka/RabbitMQ round-trips passed（2026-08-14） |
| Stress baseline | 语义不变量通过；`historyByEvent` 按需构建后 42.6s→18.45s（约 660 events/s），仍开放（2026-08-14） |

> 覆盖率 = 唯一已关联 Java runtime / inventory 中 `status=ok` runtime。它表示“已建立 Java runtime 对账/处置证据”的进度，不是“Go 已通过 parity”的比例。Manifest v2 的 `implemented` 只表示 Go 实现和测试登记，只有 `differential-verified` 才有已保存的 Java/Go trace 差分证据。

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

默认工作单元是同一 capability 子域内 1 至 5 个紧密相关 Java executions；capability 子域是里程碑，不强制等于一次提交。具体粒度和流程以 [执行手册](esper-go-port-runbook.md) 为准。每个工作单元：

1. 阅读 Java 测试和对应 runtime，确认行为预言。
2. 用 Go 链式 API 构造等价规则，编写 _parity_test.go 对照测试。
3. 将 Java runtime ID 登记到 testdata/compat/capability-manifest.json 的对应 case。
4. 开发中运行定向测试和相关差分，提交前运行完整工作单元门禁；race、stress、Docker 和 benchmark 在里程碑边界执行。
5. 更新 Manifest v2、证据和必要文档，保持统计由机器可读清单推导。
6. 不宣称“全量完成”，只更新覆盖率与 capability 状态。

### 3.2 测试对账原则

- 行为预言机：Java 测试输出/事件序列/异常类别作为期望。
- JVM 内部 query-plan hook 和 Java 专属微基准不要求逐项 parity；Go 侧性能、资源增长和稳定性仍按质量策略验收。
- 保留关键行为：输出事件、移除流顺序、时间推进、状态快照、异常类别、生命周期。
- 链式 API 唯一：Go 测试不拼接 EPL 字符串；用 From/Join/JoinMany/FromMethodOn/Select/Where/GroupBy/Having/OrderBy/Output/InsertInto/RouteTo 等 builder 表达规则。
- MySQL 按需：SQL/DB 相关测试需要本地 Docker MySQL；核心/窗口/表达式/Context 测试不依赖 MySQL。

### 3.3 代码与清单管理

- 所有 Java runtime 清单在 testdata/compat/java-execution-inventory.jsonl。
- 静态候选清单在 testdata/compat/static-manifest.json（目前几乎为空）。
- 非 Regression 源资产在 testdata/compat/source-test-manifest.json（目前几乎为空）。
- Capability/case 映射在 testdata/compat/capability-manifest.json。
- README 只保留公开摘要；manifest 维护机器统计，roadmap 维护当前优先级，CHANGELOG 维护逐轮历史。

## 4. 阶段与里程碑

### 4.1 Phase 0 — 基础运行时（已完成）

事件模型、schema、表达式核心、窗口、insert into、路由、简单 join、Table/Named Window 基础、FAF 基础、方法源基础。已建立 33%+ runtime 关联证据。

### 4.2 Phase 1 — 核心能力闭合（进行中）

目标：持续把高风险、共享性强的 `implemented` 能力转换为有持久化 evidence 的 `differential-verified`，同时关闭未关联 runtime；不以单一百分比作为阶段退出条件。

重点领域：

- epl 剩余 412 个未关联 runtime，优先 subselect、insertinto、database、dataflow 和方法源。
- infra 剩余 248 个未关联 runtime，优先表、Named Window、mutation 和 transaction。
- join 与 outer join 复杂链、unidirectional、Context Join。
- resultset 聚合高级特性（filtered、math-context、访问聚合、rollup 组合）。
- context 分区 selector、嵌套、生命周期、事务边界。
- infra 表/命名窗口剩余 mutation/merge/transaction 场景。
- expression 剩余类型、函数、脚本、枚举集合高级组合。
- event、expr、resultset、context、view 和 rowrecog 的未关联 runtime 按当前清单继续拆分。
- multithread 并发测试仍有 56 个未关联 runtime；它们优先进入 race/stress 里程碑，而不是仅做静态映射。

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
- `master` 上完成最终验收并发布。
## 5. 剩余工作优先级

### 5.1 P0 — 立即完成

1. 完成全量 `go test`、race、vet、布局和 diff 门禁，并将结果回写 Manifest v2。
2. 扩展 persisted differential evidence。当前有 147 个 differential-verified case（466 个 runtime）和 94/94 个通过的 representative scenario；下一步优先转换共享 runtime 和高风险状态能力，不在路线图手写完整场景名称列表。
3. 按未关联 runtime 和行为风险拆分下一批 epl/infra/expr/resultset/context 切片。
4. 外部服务 fixture 已本地验证（2026-08-14 MySQL/Kafka/RabbitMQ 全部门控 round-trip 通过）；暂不建设 CI，后续按执行手册定期本地 Docker 重放，并保持普通测试中的显式环境型 skip。
5. 已建立环境门控 stress 基线（`ESPER_STRESS=1 go test ./internal/esper -run '^TestStressSyntheticMediumLoad$'`）；已实现 `windowHistoryByEventRequired` 按需构建 `historyByEvent`，基线从 42.6s 降至 18.45s；继续优化剩余 filter/window/aggregate/join 热点后再宣称 NFR。

### 5.2 P1 — 下一批高价值切片

按未覆盖 runtime 数量排序：

| 域 | 未覆盖 runtime | 关键子域/类 |
| --- | --- | --- |
| epl | 412 | subselect、insertinto、database、dataflow、方法源 |
| infra | 248 | 表、Named Window、mutation、transaction |
| resultset | 198 | 聚合、输出、排序、分组 |
| expr | 172 | 表达式函数、类型、脚本、枚举集合 |
| context | 45 | Context 分区、嵌套、生命周期 |
| view | 40 | 视图高级组合 |
| event | 170 | 事件表示和 Serde 完整矩阵 |
| multithread | 56 | 并发回归 |
| rowrecog | 34 | Match Recognize |

### 5.3 P2 — 清单与能力拆分

- 将 epl/expr/resultset 等粗粒度 capability 拆分为更细 case，便于追踪。
- 填充 static-manifest.json 与 source-test-manifest.json。
- 对 intentionally-different case 写出并维护书面差异理由。

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
| epl | 412 | subselect、insertinto、database、dataflow、方法源 |
| infra | 248 | 表、Named Window、mutation、transaction |
| resultset | 198 | 聚合、输出、排序、分组 |
| expr | 172 | 表达式函数、类型、脚本、枚举集合 |
| event | 170 | 事件表示和 Serde 完整矩阵 |
| context | 45 | Context 分区、嵌套、生命周期 |
| view | 40 | 视图高级组合 |
| multithread | 56 | 并发回归 |
| rowrecog | 34 | Match Recognize |

### 6.2 清单与追踪遗漏

- static-manifest.json 目前几乎为空，需要把静态/编译期候选登记进去。
- source-test-manifest.json 目前几乎为空，需要把非 Regression 源资产（单元测试、集成测试）登记进去。
- epl/expr/resultset 等 capability 拆分过粗，需要继续细分为可验收的 case。
- 18 个 intentionally-different case 需要保持书面差异理由和测试证据。
- 当前未关联的 1,235 个 runtime 中，需要识别哪些属于平台无关核心语义，哪些属于 JVM 特有机制或性能阈值，并分别建立处置记录。

### 6.3 能力与边界遗漏

- 事务/并发：当前 FAF mutation 已有单目标 rollback，但跨 statement、跨目标 routed side effect、listener/external resource 完整事务仍待实现。
- Context Table ownership：live insert 的首次 ownership 注册已部分实现，但更广 Context Table live mutation 组合、aggregate-into-table、跨 context row ownership 仍待补充。
- 索引高级语义：FullOuter、右保留或非相邻链式 outer、unidirectional、OR、UDF、Null/Missing、复杂动态表达式、超大多流 probe 安全回退仍待补充。
- EsperIO 完整矩阵：Kafka 高级 group/rebalance/plugin、AMQP Java serialization/高级重连、JVM JMS provider/session/transaction、完整 Dataflow connector 集成仍计划中。
- Avro/JSON/Serde 完整矩阵：Avro binary codec、union/logical/fixed/enum、schema evolution、XML namespace/XSD、完整 dynamic strict-lax 仍未完整。
- Pattern/Match Recognize 高级语义：Pattern guard/observer、复杂 NFA、consumption、Match Recognize prev/interval/after/聚合/窗口删除仍待补充。
- Client 域：runtime 关联已建立，但编译器路径、异常、大用例、模块可见性、class-loader 行为的差分证据仍不足。
- Multithread：仍有 56 个 runtime 未建立关联。
- Performance/timing：不追求 parity，但需要建立 Go 侧性能基准，避免回归。

## 7. 基础设施与依赖

### 7.1 已具备

- Java 17、Maven 3.9 和 Go 工具链（当前 Linux 工作区已验证）。
- Docker 环境（可选，用于 MySQL、Kafka、RabbitMQ 集成测试）。
- Java Esper 9.0.0 源码：`/root/app/esper`（runner 会校验固定 commit）。
- Go 项目：`/root/app/bigsoc-esper`。

### 7.2 按需使用

- MySQL 8.0、Kafka 3.8.1、RabbitMQ：通过 Docker 启动，用于 SQL/DB 和 connector 集成测试；fixture、ready 检查、环境变量和测试命令见 [外部服务集成](integration/external-services.md)。
- Maven：运行 Java 回归测试以生成预言输出，例如：
      mvn -pl regression-run '-Dtest=TestSuiteInfraNWTable' '-DfailIfNoTests=false' '-Dgpg.skip=true' test
- Java ContextHash runner 的 Linux 命令、Windows PowerShell 变体和 trace 差分命令见 [Java oracle runner](../tools/java-oracle/README.md)。

## 8. 门禁与质量标准

完整规则以 [质量策略](esper-go-port-quality-strategy.md) 为准。开发迭代、工作单元提交和里程碑收口使用不同成本的门禁，避免每个小 execution 都重复运行全部高成本测试，也禁止把验证推迟到整个项目结束。

每个工作单元提交前至少执行并全部通过：

    go vet ./...
    go test ./... -count=1 -timeout 180s
    go test ./internal/compat/... -count=1

`go test -race ./...`、域级完整差分、stress、相关 Docker 和 benchmark 在 capability 子域里程碑收口时执行。

质量标准：

1. 每个 Go parity 测试必须对应至少一个 Java runtime ID。
2. 新增代码必须有 go test 覆盖；核心路径不接受 EPL 字符串。
3. 所有 race 测试通过；并发路径使用显式锁/原子操作，避免 data race。
4. 异常类别与 Java 一致（build-time error vs runtime error）。
5. 不引入未使用的 capability 或空壳 case；manifest 必须与代码同步更新。
6. 提交信息清晰：feat(<domain>): port <java-class> <N> runtimes. Coverage X/Y (Z%)。
