# Esper 9.0.0 Go 移植：执行路线图与遗漏检查

> 文档定位：本文件只维护当前阶段、优先级、remaining 和风险。完整范围与架构见 [实施规划](esper-go-port-implementation-plan.md)，日常步骤见 [执行手册](esper-go-port-runbook.md)，差分、合成数据和验收口径见 [质量策略](esper-go-port-quality-strategy.md)，历史见 [CHANGELOG](../CHANGELOG.md)。统计数字以 `testdata/compat/capability-manifest.json` 的已校验 `summary` 为唯一来源。

## 0. 实时状态入口

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

截至 2026-08-21，manifest v2 的已校验摘要为：

| 维度 | 数值 |
| --- | --- |
| Capability | 110 |
| Case | 522 |
| Case differential-verified | 134 |
| Differential-verified runtime | 428 / 4,136 |
| Runtime 已关联 | 2,901 / 4,136（70.2%） |
| Runtime 未关联 | 1,235 |
| Representative scenario | 94 / 94 通过 |
| Intentionally-different case | 18 |
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
- 最新进展：`resultset-aggregate-count-sum` 登记为 differential-verified（`ResultSetAggregateCountOneView`/`ResultSetAggregateCountJoin`/`ResultSetAggregateCountSimple`/`ResultSetAggregateSumNamedWindowRemoveGroup`，4 个 case、34 条 records/0 differences）：分组 irstream count/count-distinct/count 的 null volume、重复值与窗口淘汰 distinct 重算（view 与 join 变体），named window on-delete 组删除 null sum 与 iterator 快照；`CountDistinct` 按 boxed 指向值去重（Java equals 语义）、grouped irstream 行序新事件组在前；
- 最新进展：`resultset-aggregate-limit-snapshot` 登记为 differential-verified（`ResultSetLimitSnapshot`/`ResultSetLimitSnapshotJoin`，2 个 case、6 条 records/0 differences），修复 time(10s) `output snapshot every 1 seconds` 的精确边界语义：snapshot 在同一 tick 包含 deadline == now 的到期事件及其聚合贡献，时钟跳变跨过的 overdue 事件在后续 snapshot 排除，istream 含边界到期不变；join 聚合 snapshot 恢复 row-per-event 形状（`aggregateDefinitionSnapshotRowForEvent` 放开 join）；`resultset-aggregate-group-output` 已登记 `ResultSetLastNoDataWindow`/`ResultSetWildcardRowPerGroup`/`ResultSetUnaggregatedOutputFirst`/`ResultSetFirstSimpleHavingAndNoHaving`，并修复 `applyLastEveryTimeGrouped` 单 grouping-set 只发 new（rollup 保留 old）与新增 `RecordStream.Having` 非聚合 having 公共 API；对账修复：`case.output-policy-iterator` 与 `case.filter-window-aggregate-output` 此前把未实际重放的 Java execution 登记为 differential-verified，现改为 representative-verified 合成场景并移除 runtime 声明；`resultset-aggregate-multikey` 已登记 `ResultSetOutputLastMultikeyWArray`/`ResultSetOutputAllMultikeyWArray`；`resultset-aggregate-join-sort-window` 已登记 `ResultSetJoinSortWindow` 并修复 join 侧“进入即淘汰”同一事件的 tuple/delta 处理、aggregate 先 new 后 old，以及按行身份（`sameEventRow`）的聚合移除匹配（同负载替换事件不误删）；`resultset-having-every-events` 已扩展 `ResultSetHavingJoin`；`resultset-aggregate-join-events` 已登记 `ResultSetJoinDefault/All/Last`；`resultset-aggregate-all-having` 已登记 `ResultSet11/12AllHaving`；`resultset-aggregate-join` 已登记 `ResultSet2/4/6/8/14/16/17/10` join 执行；`resultset-aggregate-all-time-window` 已登记 `ResultSet9AllNoHavingNoJoin`；`resultset-aggregate-max-time-window` 已登记 `ResultSetMaxTimeWindow`；`resultset-aggregate-all-events` 已登记 `ResultSetNoJoinAll`；`resultset-aggregate-snapshot-time-window` 已登记 `ResultSet18SnapshotNoHavingNoJoin`；`resultset-aggregate-first-time-window` 已登记 `ResultSet17FirstNoHavingNoJoin`；`resultset-aggregate-last-time-window` 已登记 `ResultSet13LastNoHavingNoJoin` 与 `ResultSet15LastHavingNoJoin`；`resultset-aggregate-time-window` 已登记 `ResultSet5DefaultNoHavingNoJoin` 与 `ResultSet7DefaultHavingNoJoin`；`resultset-aggregate-no-output`、`resultset-aggregate-last`、`resultset-aggregate-default`、`rollup-output-first-having`、`rollup-output-all-sorted`、`rollup-output-all`、`rollup-output-default-market`、`rollup-output-no-limit-market`、`rollup-output-first-market`、`rollup-output-last-market`、`rollup-output-snapshot`、`rollup-output-snapshot-order-limit`、`rollup-output-first-sorted`、`rollup-output-first`、`rollup-output-every-sorted`、`rollup-output-last` 与 `rollup-output-last-sorted` 亦已登记。
- 最新已提交：`35017ea53`（Add subselect-in differential scenario）
- 最新已提交：`8c43e4856`（Update roadmap latest commit）
- 最新已提交：`9510dcd91`（Add resultset-aggregate-limit-snapshot differential scenario）

### 2.2 对账清单

| 维度 | 数值 |
| --- | --- |
| Capability | 110 个 |
| Case | 442 个 |
| Case implemented（verification） | 442 个（其中 53 个 differential-verified、18 个 intentionally-different） |
| Case differential-verified | 53 个（77 个 runtime） |
| Case intentionally-different | 18 个 |
| Case inventoried-only | 0 个 |
| Java inventory runtime | 4,136 个 `status=ok` runtime |
| Runtime 关联 | 2,759 条 |
| 唯一已关联 runtime | 2,630 个 |
| 未关联 runtime | 1,506 个 |
| 关联覆盖率 | 63.6% |
| Representative scenario | 55/55 通过 |
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
2. 扩展 persisted differential evidence。当前有 133 个 differential-verified case（411 个 runtime）和 94/94 个通过的 representative scenario；下一步优先转换共享 runtime 和高风险状态能力，不在路线图手写完整场景名称列表。
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
- 当前未关联的 1,375 个 runtime 中，需要识别哪些属于平台无关核心语义，哪些属于 JVM 特有机制或性能阈值，并分别建立处置记录。

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
