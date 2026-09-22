




> 最新补充:Draft 4.503(2026-09-22),`epl.other.select-wildcard-additional` 扩展 `case.epl-other-wildcard-additional` 至全部 9 个 execution differential-verified,覆盖固定 Java `EPLOtherSelectWildcardWAdditional.java` ordinals 0 `EPLOtherSingleOM`(`java-runtime-bcacb282274dc1ea256b`)、2 `EPLOtherSingleInsertInto`(`java-runtime-0fe48a706db1e3cde7f0`)、3 `EPLOtherJoinInsertInto`(`java-runtime-d85b62bf7cff58f9ed71`)、4 `EPLOtherJoinNoCommonProperties`(`java-runtime-948405f7925b1af087bd`)、5 `EPLOtherJoinCommonProperties`(`java-runtime-98f8ad1516096842f30d`)、6 `EPLOtherCombinedProperties`(`java-runtime-e9caf805d44d7a1f0ad1`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 13 条 records、0 differences。oracle 修复:join-no-common 补 s1 where 变体(原 oracle 漏掉 Java 第二 deploy cycle);rows() 对 indexed-only 属性 `indexed` 发 `<unreadable>` 标记(Java get("indexed") 抛 PropertyAccessException,listener 异常被 Esper 吞掉导致 combined-props 零记录)。Go runner 扩展至 9 cases:join wildcard 用 SelectSourceEvent 暴露 eventOne/eventTwo bean underlying,insert-into 目标预注册 map 类型,combined-props 手工渲染嵌套 array 行。manifest 735 cases / 361 DV / 1417 DV runtime IDs,unreferenced 613。
> 最新补充:Draft 4.502(2026-09-22),`expr.enum-collection-methods` 扩展 `case.expr-enum-minmax-sum-avg` 至 differential-verified,覆盖固定 Java `ExprEnumSumOf.java` ordinals 1 `ExprEnumSumEventsPlus`(`java-runtime-8497175e9fc13501285c`)、3 `ExprEnumSumScalarStringValue`(`java-runtime-93ec466957ff19bc38a6`)、4 `ExprEnumSumInvalid`(`java-runtime-bbe116cdf8ad7e13172f`)、5 `ExprEnumSumArray`(`java-runtime-b0455a5d1e4b447f34b1`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 11 条 records、0 differences。场景覆盖 element/index/size lambda footprints、case-when null 分支、UDF-in-lambda(extractNum/extractBigDecimal)、2 个 tryInvalidCompile 探针与常量集合 sumOf(Double/BigInteger/nullable Long)。oracle 修复:deploy 编译改用 `CompilerArguments(configuration)` 携带 compiler-level plug-in functions(ContextHashScenarioOracle 先例)。manifest 735 cases / 361 DV / 1411 DV runtime IDs,unreferenced 619。
> 最新补充:Draft 4.501(2026-09-22),`resultset.outputlimit-simple` 扩展 `case.output-simple-core` 至 ord 0-3 differential-verified,覆盖固定 Java `ResultSetOutputLimitSimple.java` ordinals 0 `ResultSet1NoneNoHavingNoJoin`(`java-runtime-427e3f367e9556c57969`)、1 `ResultSet2NoneNoHavingJoin`(`java-runtime-74646a15e15f49fdd1a1`)、2 `ResultSet3NoneHavingNoJoin`(`java-runtime-eb78eaf610806ed398a6`)、3 `ResultSet4NoneHavingJoin`(`java-runtime-ddd51d27fc7e1de1efb8`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 62 条 records、0 differences。场景覆盖 4 个 none 变体(no-having/having x no-join/join),每个 2 次部署(plain + irstream)。manifest 735 cases / 360 DV / 1407 DV runtime IDs,unreferenced 623。
> 最新补充:Draft 4.500(2026-09-22),`resultset.orderby-simple` 扩展 `case.resultset-orderby-simple` 至 ord 15-17 differential-verified,覆盖固定 Java `ResultSetOrderBySimple.java` ordinals 15 `ResultSetNoOutputClauseView`(`java-runtime-6de2b14776f97a0a69b2`)、16 `ResultSetNoOutputClauseJoin`(`java-runtime-091c77bae73759b7cb3e`)、17 `ResultSetInvalid`(`java-runtime-bbc3ab446f26d0f99488`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 20 条 records、0 differences。场景覆盖 4 个 replayable 变体 + 6 个 build-error probes;新增 plan.go `validateOrderByAggregates` 规则(aggregate in ORDER BY must appear in SELECT)。manifest 735 cases / 359 DV / 1403 DV runtime IDs,unreferenced 623。
> 最新补充:Draft 4.499(2026-09-22),`resultset.orderby-simple` 扩展 `case.resultset-orderby-simple` 至 ord 10-14 differential-verified,覆盖固定 Java `ResultSetOrderBySimple.java` ordinals 10 `ResultSetMultipleKeysJoin`(`java-runtime-6c1c5e581115d83acac7`)、11 `ResultSetSimple`(`java-runtime-2524d06789dd35e36c8e`)、12 `ResultSetSimpleJoin`(`java-runtime-d530652f60616c332431`)、13 `ResultSetWildcard`(`java-runtime-9455a5a72c84baa7bdc2`)、14 `ResultSetWildcardJoin`(`java-runtime-4aaec5e95dbea839ece3`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 19 条 records、0 differences。场景覆盖 19 个 EPL 变体(join multikey/simple/wildcard order-by);wildcard join 的 bean-valued 'one'/'two' 属性验证。manifest 735 cases / 359 DV / 1400 DV runtime IDs,unreferenced 623。
> 最新补充:Draft 4.498(2026-09-22),`resultset.orderby-simple` 扩展 `case.resultset-orderby-simple` 至 ord 5-9 differential-verified,覆盖固定 Java `ResultSetOrderBySimple.java` ordinals 5 `ResultSetExpressions`(`java-runtime-0a7a2ff59087a96fe0e1`)、6 `ResultSetAliasesSimple`(`java-runtime-fc69777a3bedc3ad6c52`)、7 `ResultSetExpressionsJoin`(`java-runtime-b0f39fb300a33fa7ddbf`)、8 `ResultSetMultipleKeys`(`java-runtime-92e69b0657b10a656fdc`)、9 `ResultSetAliases`(`java-runtime-178a1d0412982958a3cc`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 26 条 records、0 differences。场景覆盖 20 个 EPL 变体(expression/alias/multikey/join order-by);ord 9 v5 无 output 子句 insert-only 语义验证。manifest 735 cases / 359 DV / 1396 DV runtime IDs,unreferenced 623。
> 最新补充:Draft 4.497(2026-09-22),`resultset.orderby-simple` 扩展 `case.resultset-orderby-simple` 至 differential-verified,覆盖固定 Java `ResultSetOrderBySimple.java` ordinals 3 `ResultSetDescendingOM`(`java-runtime-df8ea61ff025609ce309`)与 4 `ResultSetDescending`(`java-runtime-9668909b2b2f00769dab`,variants 2-6);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags。Java/Go 各 8 条 records、0 differences。场景覆盖 SODA object-model 部署冒烟 + 5 个 desc/asc 变体;OM text/serialization 断言 unrepresentable。manifest 735 cases / 358 DV / 1414 DV runtime IDs,unreferenced 623。
> 最新补充:Draft 4.496(2026-09-22),新增 capability `resultset.aggregate-invalid-closure` + `case.resultset-aggregate-invalid-closure`(born-DV),覆盖 4 个 invalid-form executions(runtime IDs `java-runtime-98f0ac5299780e2d6656`/`java-runtime-09530eec55a704b01c96`/`java-runtime-fbbdc48ea2da6d4bc426`/`java-runtime-50935b7efcc1fdced314`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`):Java/Go 各 11 records、0 differences,ResultSetAggregateExtInvalid.java 收官。场景覆盖 10 个编译失败探针(7 unrepresentable + 3 build-error)。shared-core:sorted 需 data window、min-by/max-by/sorted same-stream 校验、create-table filter unrepresentable。manifest 735 cases / 359 DV / 1391 DV runtime IDs。

> 最新补充:Draft 4.495(2026-09-22),新增 capability `resultset.output-when-then-closure` + `case.resultset-output-limit-crontab-when-closure`(born-DV),覆盖固定 Java `ResultSetOutputLimitCrontabWhen.java` ordinals 9/10/13(runtime IDs `java-runtime-c8af5811fef1d7f582d6`/`java-runtime-feee4a26544010fc7fed`/`java-runtime-54ed0a4e11d7714b8289`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`):Java/Go 各 15 records、0 differences,ResultSetOutputLimitCrontabWhen.java 全部 14 个 execution 至此全覆盖。场景覆盖 SODA when-then 部署冒烟、same-var-twice output-last-when 与 8 个 invalid 探针。shared-core:output-last-when 归约为每 key 最后一行;新增 aggregate-in-when / aggregate-in-then-set / prev-in-when 校验。manifest 734 cases / 358 DV / 1387 DV runtime IDs。

> 最新补充:Draft 4.494(2026-09-22),新增 capability `view.group-closure` + `case.view-group-closure`(born-DV),覆盖固定 Java `ViewGroup.java` ordinals 7/8/19(runtime IDs `java-runtime-a72aa6eebc4ce8d21115`/`java-runtime-e718af611543b6d40436`/`java-runtime-d9b2e762c318369875e5`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`):Java/Go 各 11 records、0 differences,ViewGroup.java 全部 20 个 execution 至此全覆盖。场景覆盖 5 个 groupwin invalid 探针、grouped weighted_avg 性能冒烟(thin trace)与 escaped mapped-property groupwin。shared-core:WeightedAvg 零权重发 NaN;weighted-avg 加入 groupwin implicit grouping;groupwin first-position + null-key 校验。manifest 733 cases / 357 DV / 1384 DV runtime IDs。

> 最新补充:Draft 4.493(2026-09-22),新增 capability `infra.table-count-min-sketch` + `case.infra-table-count-min-sketch`(born-DV),覆盖固定 Java `InfraTableCountMinSketch.java` ordinals 0-3 全部 4 个 execution(runtime IDs `java-runtime-f09401ff1e3349b3497b`/`java-runtime-87f8d0057f91e1dfa7ec`/`java-runtime-4b4c531bbad3d4e1491d`/`java-runtime-217779fa3c860cc1ea18`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):Java/Go 各 57 records、0 differences。场景覆盖 create-table json 参数(epsOfTotalCount/topk/agent)、into-table countMinSketchAdd、countMinSketchFrequency/countMinSketchTopk 读、module/join/subq 消费、byte[] 键与 15 个 invalid-form 探针。shared-core:CountMinSketchValue 保留精确计数+lastBump,TopK() 惰性推导 strict-> 准入/最低桶末位淘汰;TableAggDecl 增 TopK/Agent;countMinSketchAdd 限定 into-table;SelectFromTableWhere 修复 pattern trigger 丢失并放行 pattern 触发的表读。manifest 732 cases / 356 DV / 1381 DV runtime IDs。

> 最新补充:Draft 4.492(2026-09-22),`trigger.table-named-window` 新增 `case.infra-table-invalid`(born-DV),覆盖固定 Java `InfraTableInvalid.java` 全部 4 个 execution:`InfraInvalidAggMatchSingleFunc`(`java-runtime-70e525638c5fbaf37752`,43 个 into-table 签名不匹配探针:param type/distinct/filter/ignore-nulls/min-max 方向/nth size/rate interval/plugin 名)、`InfraInvalidAggMatchMultiFunc`(`java-runtime-7c97277eedfa76b7e0cf`,6 个 unbound #time(1000) 探针:window(*) @type 事件类型不匹配、sorted() sort-expr 拒绝、se1() plugin 名不匹配)、`InfraInvalidAnnotations`(`java-runtime-fd2a7de8e5606de63b80`,5 个表列注解拒绝)、`InfraInvalid`(`java-runtime-b9b6435f81135ce15040`,50 个探针:PK-on-expression/event-type、与 variables/schemas/tables 的命名冲突、into-table group-by 数量/类型、context 可见性、write-only、unidirectional join、requires-aggregation、keyed-access 数量/类型、未知列/函数、views/retain/on-action/match-recognize/update-istream/context/pattern-atom 误用)。Java/Go 各 104 records、0 differences。shared-core:`TableAggDecl` 扩展签名细节字段 + `WithTableAggDecl`;`validateIntoTableCompatible` 逐项校验;`validateIntoTable` group-by 数量/类型 vs PK、unidirectional join、requires-aggregation;`NewTableDefinition` 拒绝 PK-on-expression/event-type;`RegisterTableInModule` 拒绝与 variables/schemas/named-windows 命名冲突;Window/OnRecord/MatchRecognize 表误用守卫;`RegisterSchema` 拒绝表名冲突。manifest 731 cases / 355 DV / 1377 DV runtime IDs。

> 最新补充:Draft 4.491(2026-09-21),`trigger.table-named-window` 新增 `case.infra-table-context`(born-DV),覆盖固定 Java `InfraTableContext.java` ordinals 0-2:`InfraPartitioned`(`java-runtime-8b5b2d92d108da7e8fb2`,CtxPerString 分区上下文 + unkeyed 表 + into-table sum + s0 读 MyTable.thesum)、`InfraNonOverlapping`(`java-runtime-5c625828c160a26df78b`,start-@now/end-S0 上下文 + keyed 表 + output snapshot when terminated 批次 + mid-run create index + deploy-only join)、`InfraTableContextInvalid`(`java-runtime-df03d93aca6b6b5ccd59`,SimpleCtx + 三个 context-visibility tryInvalidCompile 探针)。Java/Go 各 19 records、0 differences。shared-core 新增 `esper.TableContext` TableOption + `validateTableContext`(覆盖 stream/join/subquery/into-table/trigger 目标)。manifest 730 cases / 354 DV / 1373 DV runtime IDs。

> 最新补充:Draft 4.490(2026-09-21),`trigger.table-named-window` 新增 `case.infra-table-subquery`(born-DV),覆盖固定 Java `InfraTableSubquery.java` ordinals 0-3:`SubqueryAgainstKeyed`(`java-runtime-7b449dd45dd6961c5d61`,关联 PK 标量子查询)、`AgainstUnkeyed`(`java-runtime-9cde668ef5b4781b068a`,unkeyed 全表扫描,子查询先于 feed 部署)、`SecondaryIndex`(`java-runtime-8ece65643b6ec15616f2`,二级索引跨 merge 更新被索引列)、`InFilter`(`java-runtime-a839574d871f88c2fdbf`,流过滤器内 uncorrelated orderBy().firstOf() 子查询)。Java/Go 各 23 records、0 differences。零 shared-core 改动。manifest 729 cases / 353 DV / 1370 DV runtime IDs。

> 最新补充:Draft 4.489(2026-09-21),`trigger.table-named-window` 新增 `case.infra-table-faf-execute-query`(born-DV),覆盖固定 Java `InfraTableFAFExecuteQuery.java` ordinals 0-3:`InfraFAFInsert`(`java-runtime-a79e19dc5f135bb8e628`,unkeyed 表 + 空 FAF 结果数组 + 有序迭代器)、`InfraFAFDelete`(`java-runtime-a68109b2bb91de4ce1cd`,delete-all + iteratorCount 10->0)、`InfraFAFUpdate`(`java-runtime-196ff792f0f739c8d97c`,update-all + @Name 语句名 != 表名)、`InfraFAFSelect`(`java-runtime-b995c40f3c052bcc277e`,select-star FAF 结果数组)。Java/Go 各 12 records、0 differences。零 shared-core 改动。manifest 728 cases / 352 DV / 1366 DV runtime IDs。

> 最新补充:Draft 4.488(2026-09-21),`trigger.table-named-window` 新增 `case.infra-table-update-and-index`(born-DV),覆盖固定 Java `InfraTableUpdateAndIndex.java` ordinals 0-4:`InfraEarlyUniqueIndexViolation`(`java-runtime-878326b2aef272d9ef78`,deploy/FAF/on-update/compile 四阶段唯一索引违例)、`InfraLateUniqueIndexViolation`(`java-runtime-59f3f0884fc9ae9d749f`,create-index 与 on-merge 列冲突 + 中途 undeploy)、`InfraFAFUpdate`(`java-runtime-1118b36d6f38fa1c78a8`,FAF update + 二级索引 select)、`InfraTableKeyUpdateSingleKey`/`MultiKey`(`java-runtime-16e0f7011601678fa5df` / `-0b3580bcd42327b7d2bb`,on-update 主键改名)。Java/Go 各 34 records、0 differences。shared-core:`Table.CreateIndex` 校验既有行唯一性并拒绝 merge-updated 列;send 驱动 trigger update 中途失败原子回滚;`env.Build` 拒绝 merge when-matched 更新唯一键列。manifest 727 cases / 351 DV / 1362 DV runtime IDs。

> 最新补充:Draft 4.487(2026-09-21),`trigger.table-named-window` 新增 `case.infra-table-select-enum-multikey`(born-DV),覆盖固定 Java `InfraTableSelect.java` ordinals 1-4(enum firstOf 迭代器 + 数组/复合多键 join);Java/Go 各 18 records、0 differences;零 shared-core 改动。ord 0 select-shape 矩阵延后。manifest 726 cases / 350 DV / 1357 DV runtime IDs。

> 最新补充:Draft 4.486(2026-09-21),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-insertonly-deletethenupdate`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 46-53:`InfraInsertOnly` 剩余六个 execution(ord 46 nw soda+useColumnNames `java-runtime-5cdc46289e4fac78a0c5`;ords 47-51 全部五个 table 变体 `java-runtime-8e9616eb8385c473d45a` / `-af614186a63cbeb33ae5` / `-eb7754c9e46c8c465c14` / `-f21a6fc889f14608828f` / `-f7a73c74e857ffbdfd15`,useEquivalent where 1=2 / plain / useColumnNames / soda / soda+useColumnNames,经 MergeIntoNamedWindowWhen + MergeIntoTableWhen + WhenNotMatchedAny 回放)与 `InfraDeleteThenUpdate{namedWindow=true/false}`(`java-runtime-5816ec0ef519ec8a48e1` / `-3cca4ced23a6097b5023`,static `java-0acf62afc326a95d7216`,matched delete-then-update 动作链 + FAF 种子,固定 nw-update-wins / table-delete-wins 分歧);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 46 条 records、0 differences。引擎修复:merge listener 派发改为按动作 delta 上报(delete 把被删行记入 old,后续 update 把同一删除前行记入 old 并把更新行记入 new),named window 保留更新行、table 应用净效果。manifest 725 cases / 349 DV / 1353 DV runtime IDs / 3903 associations。

> 最新补充:Draft 4.485(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-invalid-insertonly`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 40-45:`InfraInvalid{namedWindow=true/false}`(`java-runtime-99b2d413519187b56232` / `-33b1e837bc8b373bb198`,13/12 个 tryInvalidCompile 探针经 build-error op 回放,含 nw/table 分歧的 probe 2 与 NW-only probe 4)与 `InfraInsertOnly{namedWindow=true}` 四变体(`java-runtime-651148621e89ec5465b0` / `-e51caa89fbde4ab34197` / `-8900ac7e3d063d8b8842` / `-b581753558f219b16a2a`,useEquivalent where 1=2 / plain / insert(p0,p1) 列名 / soda);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 49 条 records、0 differences。零 shared-core 改动。manifest 724 cases / 348 DV / 1345 DV runtime IDs。

> 最新补充:Draft 4.484(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-flow-itv`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 32-39:`InfraFlow{namedWindow=true/false}`(`java-runtime-403bba8c6b29e32b1a8f` / `-ae05c015767242106de7`,filtered SupportBean insert-into feeder + SupportBean_A delete-all trigger + 4-branch merge + 两轮 runAssertionFlow 跨 undeploy/redeploy + wildcard merge tail + ambiguous-columns module)与 `InfraInnerTypeAndVariable{namedWindow,rep}`(`java-runtime-3303d0922bd2d722fa3c` / `-f20972a347aabfcc0260` / `-484a8e636d3734b87673` / `-76d16e1334c83e6d4002` / `-462d190f20742a0c266d` / `-8495f57749c2105b15a8`,flags OBSERVEROPS,tri-state myvar 在三个 not-matched 分支间选择 + nested-fragment c2 + matched-delete);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 195 条 records、0 differences。零 shared-core 改动;ambiguous-columns unkeyed-table where-merge 不可观测(Java 从不发送 TypeOne 事件),runner 部署 nil-keys 等价物。manifest 723 cases / 347 DV / 1339 DV runtime IDs。

> 最新补充:Draft 4.483(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-pattern-nowhere`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 26-31:`InfraPatternMultimatch{namedWindow=true/false}`(`java-runtime-d28146beedb9cfdf9dbb` / `-ac7258306c8be9b66f8f`,static `java-74cbcb4f6ad520a0f3fd`,every-A-then-B pattern merge 经 route-stream workaround,composite-key where,B1 完成两个 pending A 分支)、`InfraNoWhereClause{namedWindow=true/false}`(`java-runtime-0dc6b8eb05a1d3989b23` / `-c6f6bda40f34d29acb8c`,static `java-2b5b9c17389359bc78f7`,no-where merge 对 keepall nw / unkeyed table,not-matched 仅在 target 为空时触发,matched C% 更新所有行)、`InfraMultipleInsert{namedWindow=true/false}`(`java-runtime-2c4851ea026cc835d39b` / `-f075aa28d64b1ae78d2e`,static `java-227e4a044ce4b9c1de81`,四个有序 not-matched insert clauses,首个匹配生效,merge listener insert-stream records);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 50 条 records、0 differences。零 shared-core 改动。manifest 722 cases / 346 DV / 1331 DV runtime IDs。

> 最新补充:Draft 4.482(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-multiaction`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 20-25:`InfraMultiactionDeleteUpdate{namedWindow=true/false}`(`java-runtime-dfa83c0593d19172be7d` / `-ffbda0563d50878dafd8`,static `java-d718ed89dce189e3cb3b`,六个有序 matched actions,后续 where 看到前面 action 的结果,E5/E6 在尾随 delete 前被 update 救活)、`InfraUpdateOrderOfFields{namedWindow=true/false}`(`java-runtime-3034e5da517c1235da22` / `-cb5eefe84a486b090e53`,static `java-71292c39e6d9067bb77d`,UPDATE SET 左到右求值,intBoxed 读已更新的 intPrimitive 而 initial.intPrimitive 读更新前值)、`InfraSubqueryNotMatched{namedWindow=true/false}`(`java-runtime-905164d98662721e5509` / `-d3e1f3aa50c4375f657f`,static `java-1b30fec400708094c9a1`,not-matched insert 中的关联子查询);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 43 条 records、0 differences。零 shared-core 改动。manifest 721 cases / 345 DV / 1325 DV runtime IDs。

> 最新补充:Draft 4.481(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-insert-other-stream`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 8-19 `InfraInsertOtherStream{namedWindow,rep}`:12 个 execution = namedWindow∈{true,false} × OBJECTARRAY/MAP/AVRO/JSON/JSONCLASSPROVIDED/DEFAULT(static `java-8304a4459a4ea865bf1f`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`)。Java/Go 各 96 条 records、0 differences。pin:merge 进 side stream OtherStreamOne(matched/not-matched 分支);named-window 首个事件 not-matched(status=0d)后匹配前一值(10d/11d,#unique(name) 替换),table feeder insert 先于 trigger 路由故首个事件 matched(status=10d);composite-PK table(name+value)。零 shared-core 改动。manifest 720 cases / 344 DV / 1319 DV runtime IDs。

> 最新补充:Draft 4.480(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-nested-insertstream`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 4-7:`InfraUpdateNestedEvent{namedWindow=true/false}`(`java-runtime-065003de88aca37795b8` / `-2e8d691b5e2c927038d7`,static `java-712b26dbe20bbda50f37`,map+objectarray 双 sub-scenario,FAF `select cflat.c0/carr[0].c0/carr[1].c0` 断言 {1,1,2})与 `InfraOnMergeInsertStream{namedWindow=true/false}`(`java-runtime-2c687a68317caea3c148` / `-081455731ccafbf6847c`,static `java-9bcebf3cac321ec7aee2`,五个有序 not-matched insert actions:StreamOne wildcard、StreamTwo/Three id+key0、key0=K2 过滤 StreamFour、WinOMIS 目标);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 58 条 records、0 differences。零 shared-core 改动。manifest 719 cases / 343 DV / 1307 DV runtime IDs。

> 最新补充:Draft 4.479(2026-09-20),`trigger.table-named-window` 新增 `case.infra-nwtable-on-merge-basic`(born-DV),覆盖固定 Java `InfraNWTableOnMerge.java` ordinals 0-3:`InfraOnMergeSimpleInsert{namedWindow=true/false}`(`java-runtime-dbf13fb6d1ca3a37275a` / `-df3d7a21bdd769f1acce`,static `java-0fdb4e6490ae6d9e9103`)与 `InfraOnMergeMatchNoMatch{namedWindow=true/false}`(`java-runtime-240cb61eabb22eb2a215` / `-aaa45c95e95a919bd0c0`,static `java-5f5d122bcbf66d59da6c`);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。Java/Go 各 39 条 records、0 differences。pin:insert-only merge 每行 new-data、matched-delete/not-matched-insert/matched-update 三分支 IR pair、named-window create consumer 先于 merge listener 看到变更(applyDelta 顺序)、not-matched E1(0) 静默发送、iterator snapshot。零 shared-core 改动;runner 复用 MergeIntoNamedWindowWhen/MergeIntoTableWhen + WhenMatchedDelete/WhenNotMatched(Any)/WhenMatched + CopyMatchingFields。manifest 718 cases / 342 DV / 1303 DV runtime IDs。

> 最新补充:Draft 4.477(2026-09-20),`context.partition` 扩展 `case.context-admin-listen` 至 ord 5 `ContextAdminPartitionAddRemoveListener`(`java-runtime-206c08a7f3d6c239050c`,RUNTIMEOPS;static `java-3a026095a61c4060c91b`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`)。Java/Go 各 55 条 records、0 differences。双 scenario:flat `start S0 end S1` + nested `NeverEndingStory start @now` + `ABSession start S0 as s0 end S1`(4.476 解锁的 initiated-parent 嵌套,leaf-initiated 路径处理 never-ending 父级)。pin:3 个 partition-state listener 注册→S0(1) allocated→移除 l0→S1(1) deallocated 仅 l1/l2→iterator [l1,l2]→remove-all→S0(2)/S1(2) 静默。runner 新增 remove-partition-listener(s) ops + admin:partition-listeners snapshot;scenario validator 放开 name-only snapshot 与两个新 op。manifest 717 cases / 341 DV / 1299 DV runtime IDs。

> 最新补充:Draft 4.476(2026-09-20),`context.partition` 扩展 `case.context-selection-faf` 至 ord 3 `ContextSelectionFAFNestedNamedWindowQuery`(`java-runtime-f51a1493ad61c1f0d0d1`,FIREANDFORGET;static `java-1c493e94eff9beb161b3`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`)。Java/Go 各 8 条 records、0 differences。**引擎扩展**:nested initiated-parent 上下文——放开 `NewNestedContext`/`validateContext` 的 initiated 父级限制;新 `processNestedInitiatedParent` 分发(父生命周期先于 leaf 类型门、overlapping 父分区、eager category leaf 实例化、leaf 事件广播、父终止级联);insert-into 继承目标窗口 context 并按 leaf 分区变量路由;`hasLifecycleLevel` 链式释放门;嵌套 selector 限定 All/ById/Nested;context-clause FAF descriptor 驱动。manifest 717 cases / 341 DV / 1298 DV runtime IDs。

> 最新补充:Draft 4.475(2026-09-20),`context.partition` 新登记 `case.context-selection-faf`(differential-verified),覆盖固定 Java `ContextSelectionAndFireAndForget.java` 的 ord 0 `ContextSelectionAndFireAndForgetInvalid`(`java-runtime-c8c49c4c40e41d383d25`,FIREANDFORGET+INVALIDITY)、ord 1 `ContextSelectionIterateStatement`(`java-runtime-6dd5b9086935002cc50d`)、ord 2 `ContextSelectionAndFireAndForgetNamedWindowQuery`(`java-runtime-bf4cefd62580e2abd2c6`,FIREANDFORGET);Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。ord 3 `ContextSelectionFAFNestedNamedWindowQuery`(`java-runtime-f51a1493ad61c1f0d0d1`)暂缓:需要 nested initiated-parent 上下文的 leaf broadcast 与 eager category 实例化,属另一语义簇。Java/Go 各 35 条 records、0 differences。**共享核心修复**:(1) FAF 拒绝 context-clause join 与 context-bound window join(删除 Go 扩展 executeContextJoinFireAndForget 及 10 个非 oracle 测试);(2) 非 context FAF 对 context-bound window 应用 selector;(3) window 创建的 partition 注册 allocation-order ID/descriptor;(4) SnapshotWithSelector 按 Java 顺序先查 non-context 再查 nil selector;(5) contextKeysForNode 支持 named-window 源。manifest 717 个 case(341 DV)/ 1297 个 differential-verified runtime IDs、3847 个 associations(referenced 3473 / unreferenced 663)。

> 最新补充:Draft 4.474(2026-09-20),`context.partition` 新登记 `case.context-lifecycle`(出生即 DV)+ `case.context-lifecycle-vdw`(intentionally-different),对照固定 Java `ContextLifecycle.java` 全部 5 个 execution(Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`):ord 0 SplitStream `java-runtime-d59fd16257279a2118a7`、ord 2 NWOtherContextOnExpr `java-runtime-a80d326a65c440aa438f`、ord 3 Invalid `java-runtime-f04b99d603d61f3808fb`(INVALIDITY)、ord 4 Simple `java-runtime-ba7774dedca1bd391c8c`(STATICHOOK);ord 1 VirtualDataWindow `java-runtime-38ddfd09bd97e862e438` 登记 intentionally-different(VDW SPI 每分区实例化+undeploy destroy 无 Go 边界)。Java/Go 各 26 条 records、0 differences。**共享核心修复**:(1) split-stream 命名窗口分支插入延迟到所有匹配分支 select 求值之后——Java 先求值全部 insert 子句再应用窗口插入,后分支子查询看到 pre-trigger 窗口状态(ord 0 钉 mymax=null,null,100);(2) `ScheduleCountOverall` 对每个有部署语句的 temporal context 计 1 个待决调度(跨语句共享,ord 4 钉 sched=1)。manifest 716 cases / 340 DV / 1294 DV runtime IDs。

> 最新补充:Draft 4.473(2026-09-20),`context.partition` 扩展 `case.inventory.context-nested` 5 个 execution 至 differential-verified,对照固定 Java `ContextNested.java`(Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):ContextNestedPartitionedWithFilterOverlap(`java-runtime-8598d1eb6dbd61614f4f`)、ContextNestedPartitionedWithFilterNonOverlap(`java-runtime-9539162a80f17616650f`)、ContextNestedPartitionWithMultiPropsAndTerm(`java-runtime-c91629ba5a771e1725db`)、ContextNestedCategoryOverInitTermDistinct(`java-runtime-0c5d14d415a1162de7e4`)、ContextNestedKeySegmentedWInitTermEndEvent(`java-runtime-754484318bed39b8eea2`)。Java/Go 各 18 条 records、0 differences。**共享核心修复**:(1) `NewDistinctInitiatedTerminatedContext` 的 distinct key 表达式对发起事件求值(此前 `Property[int](ContextInitiatingEvent(),"intPrimitive")` 在 initiation 时为 nil,导致 `-4`/`-5` 同 key 抑制);(2) `output last when terminated` 对聚合快照按 output key 取末行(此前 row-for-event 聚合快照逐事件发出,多产一行)。manifest 714 cases / 339 DV / 1290 DV runtime IDs。

> 最新补充:Draft 4.472(2026-09-19),`context.partition` 新登记 `case.context-key-segmented-allocation-time`(出生即 DV),对照固定 Java `ContextKeySegmented.java` ord 25 `ContextKeySegmentedWPatternFireWhenAllocated`(`java-runtime-57199db349abfe7ad70e`)、ord 28 `ContextKeySegmentedRegExFilter`(`java-runtime-9356c517931472be6cad`)、ord 6 `ContextKeySegmentedSubtype`(`java-runtime-820bb6f72b84ad070ce4`,回收 4.471 ord-20 重关联后悬空的 ID;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags)。Java/Go 各 10 条 records、0 differences。**引擎扩展**:`OnPattern(PatternStream)` 触发器源(`on pattern[...] set ...`,pattern NFA 驱动、每 match 执行 action、输出 on-set 行形);fire-on-allocation——`partitionRuntime` 返回 allocation batch,`timer:interval(0)` 在分区创建时同步触发(select 走 `patternTimeBatch`,trigger 走新 `patternTriggerTimeBatch`),`statementHasInputlessPattern` 让无输入 pattern 语句在 context 事件到达时分配分区;`terminated after <duration>` 运行时路径(`expireContext` 按分区年龄退役);subtype 分区键沿 schema parent 链传递解析(`contextKeysForEvent` BFS、`contextKeysForType`/`validateSegmentedContextEventType` 接受祖先类型)。**修复**:合并 allocBatch 时保留 `allocBatch.Time`(timer-only pattern 的 `partition.process` 返回零 Time)。manifest 714 cases / 712 implemented / 338 DV / 1285 DV runtime IDs / 3839 associations;referenced 3465、unreferenced 671。

> 最新补充:Draft 4.471(2026-09-19),`context.partition` 扩展 `ContextKeySegmented` 三个 execution:ord 8 `ContextKeySegmentedSubselectPrevPrior` 出生即 DV(`java-runtime-2afbd86618b752a6dd5b`;static `java-1dbfa1926e47afc1a7b4`;Java/Go 各 12 条 records、0 differences——S0 事件广播进每个已存在分区的子查询 keepall、不分配分区,undeploy+redeploy 重置分区状态;Esper `prior(0,id)` 映射为 Go 普通字段),ord 19 `ContextKeySegmentedInvalid` 登记 `case.context-key-segmented-invalid` 为 intentionally-different(9 个 compile 探针中 7 个钉住 Go 拒绝边界:新增跨流 key 类型不匹配校验、named window 分区标准校验、segmented context 下 named window 类型校验;探针 1/6 不可表达),ord 20 `ContextKeySegmentedTermByFilter` 修正 runtime ID 关联(`820bb6f72b84ad070ce4`→`e64c1b8b8cd2dcd39154`,ord 6 Subtype 转为 unreferenced backlog)。manifest 713 cases / 711 implemented / 337 DV / 1282 DV runtime IDs / 3836 associations;referenced 3462、unreferenced 674。ords 25/28(OnPattern 触发器 + fire-on-allocation、terminated-after)为下一单元候选。

> 最新补充:Draft 4.470(2026-09-19),`eplother.stream-selector` 扩展 `EPLOtherSelectExprStreamSelector` 至 16/17 execution:ords 1/2 出生即 DV(runtime IDs `java-runtime-784378ea15f390e587b2`/`java-runtime-68b49a5d4a1630d85f75`;static `java-5dcc72bb024241a1c003`/`java-587f1eac414f59c3f257`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags),ords 0/16 compile-only invalid 与 ord 3 SODA object-model 登记 `case.epl-other-select-expr-stream-selector-invalid` 为 intentionally-different。**引擎扩展**:pattern-source transpose——`select a.* from pattern [...]` 把 tagged event 的 underlying 路由进 insert-into 目标(validatePattern/resultSchema 放行 unnamed transpose、validateRoute 覆盖 pattern transpose、evaluatePatternMatch 返回 Result 并经 patternTransposeSelection 物化 transpose 路由),并修复了 named Transpose 在 pattern select 中静默错路由的潜在缺陷。Java/Go 各 15 条 records、0 differences。场景覆盖 transpose-nested(nested.* 转置 + nestedValue 消费者)、insert-from-pattern(unguarded 与 timer:within 双 transpose + streamB Pair 伴随列)与五个 invalid 探针。manifest 711 cases / 709 implemented / 336 DV / 1281 DV runtime IDs / 3834 associations;referenced 3460、unreferenced 676。

> 最新补充:Draft 4.469(2026-09-19),新 capability `epl.other.from-clause-optional` 出生即 DV,对照固定 Java `EPLOtherFromClauseOptional.java` ords 0/1/2/4(runtime IDs `java-runtime-0e96acf48376ed71c690`/`java-runtime-f54b77f9c8381d0cc12c`/`java-runtime-00d22c5518b7c57f1ba7`/`java-runtime-4f6e15a0c30a5e95b1f6`;共享 static `java-0c9a8913a4dafbcd91e2`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;flags FIREANDFORGET);ord 5 `EPLOtherFromOptionalInvalid`(`java-runtime-6d948697a80bca6a0dac`,INVALIDITY)登记 `case.epl-other-from-clause-optional-invalid` 为 intentionally-different;ord 3(FAFNoContext,含 JVM-only inlined_class)留待后续。**引擎扩展**:source-less(无 from 子句)语句获得完整生命周期——context 分区 initiation 投递单行投影(s0 监听器)、output-when-terminated 在终止时投递(s1)、iterator/snapshot 每活分区一行、context FAF 按分区评估并支持 selector/distinct/where/having;`Query` 新增 fluent 方法 Named/WithContext/WithOutput/WithDistinct/WithWhere/WithHaving/WithOrderBy,where/having 仅限 source-less。Java/Go 各 34 条 records、0 differences。场景覆盖 context-soda-false/true(逐步监听器+iterator)、no-context(select 1 as value 单行 iterator)、faf-context(全分区/by-id selector/distinct/where/having 六轮 FAF)与 invalid 五个 compile-error 探针。manifest 709 cases / 707 implemented / 335 DV cases / 1279 DV runtime IDs / 3829 associations;referenced 3455、unreferenced 681。

> 最新补充:Draft 4.468(2026-09-19),`epl.insertinto-pattern` 新登记 `case.epl-insert-into-populate-single-col-method-call`(出生即 DV)——EPLInsertIntoPopulateSingleColByMethodCall 唯一 execution(ord 0;runtime ID `java-runtime-abe5e5cbda9667e7e112`;static ID `java-9db09f558176cc93b13e`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags)。Java/Go 各 23 条 records、0 differences。场景覆盖单例 execution 的 9 轮单列方法调用 insert-into 填充:5 轮 implicit(`insert into {P}_Stream select *` 的 s1 静默 + `select SupportStaticMethodLib.{fn}(s0)` 的 s2 路由进预注册 {P}_Stream)与 4 轮 configured(`insert into {target} select {fn}(s0)` + `select * from {target}` 的 s0),跨 bean/map/object-array/Avro/JSON 五种表示;UDF 语义为 one 透传、two 包 "|…|",bean 腿 convertEvent 映射 SupportMarketDataBean{symbol,volume} -> SupportBean{theString,intPrimitive}。语句/投递事件的 JVM 类断言以 schema kind(Struct/Map/ObjectArray/Avro/JSON)value records 固定;implicit-json 的 s1 wildcard 用显式 one/two 列别名(JSON target 的 Transpose 要求 string payload);bean 行渲染投影 {theString,intPrimitive}。无引擎改动。manifest:707 cases、705 implemented、334 DV cases、1275 DV runtime IDs、associations 3824(referenced 3450、unreferenced 686)。

> 最新补充:Draft 4.467(2026-09-19),`epl.insertinto-pattern` 的 `case.epl-insert-into-istream-func` 由 implemented 升级为 differential-verified,对照固定 Java `EPLInsertIntoIRStreamFunc.java` 唯一 execution(ord 0;runtime ID `java-runtime-033d9d0dabb864fd8179`;static ID `java-e3ea88d6389f41414b12`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):Java/Go 各 10 条 records、0 differences。场景覆盖 'insert irstream into MyStream select irstream theString, istream()'(#lastevent 生产者:i0 见 IR pair,MyStream 消费者见 removes-as-inserts 展开的 newData;istream() 插入行 true、路由删除行 false)与双流 join 腿(SupportBean#lastevent + SupportBean_S0#lastevent,plain Query 监听器见 new{E2,10,true} old{E1,10,false})。无引擎改动:复用 4.466 的 routeSelector 拆分(WithOldStream + WithIRStreamRoute);SODA 腿仅断言编译期属性类型 Boolean,无 trace 记录(approved difference)。capability remaining 同步清理:EPLInsertIntoIRStreamFunc 与已 DV 的 EPLInsertIntoTransposePattern suite(Draft 4.189 遗留陈旧条目)一并移除。manifest 更新为 706 cases、333 DV cases、1274 DV runtime IDs、associations 3823(referenced 3449、unreferenced 687)。

> 最新补充:Draft 4.466(2026-09-19),`epl.insertinto-pattern` 新登记 `case.epl-insert-into-wrapper`(出生即 DV),对照固定 Java `EPLInsertIntoWrapper.java` 全部 3 个 execution(ords 0-2;runtime IDs `java-runtime-79356b0865c8de3ace17`/`java-runtime-32434556dcbd672d7cf7`/`java-runtime-581a1f109ff2588c6cde`;static IDs `java-70e97824e1f97c1b9eee`/`java-b3c8191c6b0c0831cb61`/`java-08f6aa413ec9ca217168`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):Java/Go 各 6 条 records、0 differences。场景覆盖 wrapper 类型双生产者(select *, intPrimitive as p0 + select sb 嵌套 bean 属性,未提供列 p0 -> null)、三级 `insert into select irstream *` 链(#length(2) + || concat,e3 触发时 s2 同批 new{e3,e3AB} old{e1,e1AB}),以及 on-trigger 多子句 split/fork/join(transpose(UDF) -> MyEvent,where 分支,output all 双写;T,T,F -> final id=1,T,T,T -> 不触发)。引擎改动:insert-into 路由选择器与 select 子句流关键字解耦——Query.routeSelector 默认 istream-only(对应 Esper 普通 'insert into'),新增 `WithIRStreamRoute()`/`WithRStreamRoute()` QueryOption 表达 'insert irstream into'/'insert rstream into'(removes-as-inserts);既有 'insert irstream/rstream into' 调用点已更新,facade 重新生成。manifest 更新为 706 cases、332 DV cases、1273 DV runtime IDs、associations 3823(referenced 3449、unreferenced 687)。

> 最新补充:Draft 4.465(2026-09-19),`epl.insertinto-pattern` 新登记 `case.epl-insert-into-from-pattern`(出生即 DV),对照固定 Java `EPLInsertIntoFromPattern.java` 全部 4 个 execution(ords 0-3;EPLInsertIntoFromPatternNamedWindow 为 ord 3 内部类;runtime IDs `java-runtime-4dc2b394114fdaf84056`/`java-runtime-98eba92039ca0e871ad6`/`java-runtime-1dd189e49dda82f29078`/`java-runtime-9092d240993543259d6e`;static IDs `java-ca3f03a679a887265f3f`/`java-6ea239f49004ae40f1f7`/`java-9fd8106cb703d660e4d9`/`java-3dc830b6ab1131cd3d36`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):Java/Go 各 7 条 records、0 differences。场景覆盖 wildcard tag 属性投影 + 显式列名(未匹配 OR 分支 tag -> null)、bean 类型 tag 列经 s0.id/s1.id 嵌套读取、默认 tag 名列 + MyStream#length(10),以及 named-window pattern 源(PositionW win:time(1 hour).std:unique(intPrimitive))喂 `insert into Foo select * from pattern[every a = PositionW -> every b = PositionW]`——insert-into 语句自身 listener 恰一行 {a:E1,b:E2}(pin count=1,强于 Java invoked 标志)。无引擎改动:tagged pattern 的 select * 展开为 per-tag Alias(tag, PatternEvent(tag))(零选择 Build 拒绝);列名列表映射 alias 名;Foo 预注册 Event 列 + bean 嵌套 schema。manifest 更新为 705 cases、331 DV cases、1270 DV runtime IDs、associations 3820(referenced 3446、unreferenced 690)。

> 最新补充:Draft 4.464(2026-09-19),`epl.insertinto-pattern` 新登记 `case.epl-insert-into-typed-columns`(出生即 DV),对照固定 Java `EPLInsertIntoEmptyPropType.java` ords 0/1 与 `EPLInsertIntoEventTypedColumnFromProp.java` ords 0/1(runtime IDs `java-runtime-0870abe075dc95308a27`/`java-runtime-d585492dbeef1deee74f`/`java-runtime-2fb237744f8a8b010414`/`java-runtime-700d690d1c5c019ec414`;static IDs `java-00bd26d73c1bd2e1695a`/`java-5eeac65322c142a5ed7c`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):Java/Go 各 17 条 records、0 differences。场景覆盖零列 schema 的 `insert into W() select null` + FAF insert/delete + on-merge/on-insert 触发器(named-window-model-after)、create-schema 三个子轮(map/objectarray/bean,共享 ord-1 runtime ID;soda 双跑为 compile-path-only 差异重放一次)的 s0 listener + objectarray subscriber + 事件类型名 value 记录,以及 event/POJO 类型 lastevent 表列的 on-merge 写入 + 一分钟超时 pattern 删除并离线发出。无引擎改动:select e.* 按属性投影、ThenInsertInto 先于 ThenDelete(Go 表目标 matched 链终止语义,lastevent 读删除前行)、pojo 用断言字段 {theString} 最小 SupportBean schema。manifest 更新为 704 cases、330 DV cases、1266 DV runtime IDs、associations 3816(referenced 3443、unreferenced 693)。

> 最新补充:Draft 4.463(2026-09-19),`infra.table` 扩展 `case.infra-table-insert-into` 至全部 10 个 execution(出生即 DV),对照固定 Java `InfraTableInsertInto.java` ords 3/4/6/8/9(runtime IDs `java-runtime-a58e8a2ac1c172b779f1`/`java-runtime-ad31f07cc8535074a471`/`java-runtime-c141cb04dac6838dc264`/`java-runtime-abf1de4c0349b023da5f`/`java-runtime-f03458884f4e7beaa402`;Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`;无 flags):Java/Go 各 39 条 records、0 differences。**共享核心修复**:trigger 驱动 insert 路径允许缺省列与 null 主键分量(Java lenient insert-into 语义);Go 原生 Table API 保持严格。场景覆盖 self-access 去重、named-window merge 插入无键表、#unique 窗口灌表 + FAF delete、split-stream 两表 + OtherStream listener、lenient null-PK 行。manifest 更新为 703 cases、329 DV cases、1262 DV runtime IDs、associations 3812(referenced 3439、unreferenced 697)。
> 最新补充：Draft 4.462（2026-09-19），`infra.table` 新登记 `case.infra-table-reset-aggregation-state`（出生即 DV）映射 `trigger.table-named-window`，对照固定 Java `InfraTableResetAggregationState.java` 全部 6 个 execution（runtime IDs `java-runtime-5478f21270e0cc6e4d85`/`java-runtime-6215263c68fc3379b070`/`java-runtime-38acd85e3cec943643e7`/`java-runtime-78c737da8c98ca9afafe`/`java-runtime-e0e02833416f4b55ebee`/`java-runtime-21ca203d405fd3a727c8`；共享 static `java-0dce757ee7589b9e5955`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）：Java/Go 各 31 条 records、0 differences。场景覆盖 unkeyed sum 列名/整行 `mt.reset()` 两形态、keyed 选择性 per-column reset（S0 清 avgone+winone、S1 清 avgtwo+wintwo）、12 聚合 unkeyed 表整行 reset 后 `countMinSketchFrequency` 1->0、四个 compile-rejected reset() 探针与 compile-only doc-sample。**共享核心修复**：reset() 按 Java 语义重建聚合状态 cell——reset 路径在 per-column epoch 切片之外删除受影响 selection 子树的 plugin/multi-plugin states，否则 stddev 增量 Welford 累加器经 leave+re-enter 展开旧事件留下 1-ulp 残差；新增 `ResetTableAggregatesWhere(table, predicate, columns...)` 链式 API 覆盖 keyed merge where 形态，scenario validator 登记 `build` compile-only op。manifest：703 cases、701 implemented、329 DV cases、1257 DV runtime IDs、associations 3807（referenced 3434、unreferenced 702）。
> 最新补充：Draft 4.461（2026-09-19），`infra.table` 的 `case.infra-table-join` differential-verified 场景，对照固定 Java `InfraTableJoin.java` ordinals 0/3/4/5（runtime IDs `java-runtime-c7ba930cd8efcf4a0daa`/`java-runtime-bd56535c23535c3e90ce`/`java-runtime-375040b0fa5a7b77c5aa`/`java-runtime-7e0cba3e4899478393ef`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）：Java/Go 各 20 条 records、0 differences。**共享核心修复**：(1) `updateJoin` 表侧刷新提前到 before 快照之前，保留流事件不再因表变更幻影重发（Java 表 join 仅由触发流事件驱动）；(2) `faf.go`/`trigger.go` 表查找改走 `ensureTableLockedInModule`，引擎构造后注册的表对 FAF insert 与 on-trigger 可见。manifest：702 cases、700 implemented、328 DV cases、1251 DV runtime IDs、associations 3801（referenced 3428、unreferenced 708）。

> 最新补充：Draft 4.443（2026-09-18），`context.partition` 的 `case.context-declared-expression` differential-verified 场景，对照固定 Java `ContextWDeclaredExpression.java` ords 0/1/2（`java-runtime-9acc9abebb2846f8b439`/`java-runtime-77b90f33562c2c0f9548`/`java-runtime-1bff58f3130b73dfc99f`；static IDs `java-999bc7e77f2d48538dc3`/`java-33045a26365f5dd7f45b`/`java-f4bff32df9aeaf56d49c`；无 flags）：Java/Go 各 16 条 records、0 differences。共享核心修复：composite context start pattern 由 timer 分支播种时同步 arm 事件过滤器，使 `and(TimerAt, every(event))` 在 timer 腿触发后保持 active。manifest：701 cases、699 implemented、327 DV cases、1247 DV runtime IDs、associations 3797（referenced 3424、unreferenced 712）。

> 最新补充：Draft 4.442（2026-09-17），`view.basic-windows` 的 `view-first-last-event` differential-verified 场景，对照固定 Java `ViewFirstEvent.java`/`ViewFirstLength.java`/`ViewLastEvent.java` ords 0/1（runtime IDs `java-runtime-607d915be914a5dce34f`/`java-runtime-5c32ea97d29ecbe957cb`/`java-runtime-d99cd0eba0b2e2ca64ba`/`java-runtime-2389705da83584b445a1`/`java-runtime-260c9f5af6a4a10d5c80`/`java-runtime-af419392d33ae4b948d8`；static IDs `java-9c592c8d69c0251839f3`/`java-b74c5027c5e9385c91fc`/`java-1c8907d6332b94606e35`；无 flags）：Java/Go 各 38 条 records、0 differences。manifest：684 cases、682 implemented、310 DV cases、1153 DV runtime IDs、associations 3701（referenced 3337、unreferenced 799）。

> 最新补充：Draft 4.441（2026-09-17），`resultset.aggregate-group-by` 的 `resultset-aggregate-method-remainder` differential-verified 场景，对照固定 Java `ResultSetAggregateRate.java` ords 0/1（`java-runtime-d0424628aa4aad2d80bc`/`java-runtime-b90c3df2e444e89e8cb6`）与 `ResultSetAggregateLeaving.java` ord0（`java-runtime-276a60a1f8e8298c32ec`；static IDs `java-5d1774a76d730ac199fb`/`java-f24caa115a51453bcad3`；无 flags）：Java/Go 各 21 条 records、0 differences。manifest：683 cases、681 implemented、309 DV cases、1147 DV runtime IDs、associations 3695（referenced 3337、unreferenced 799）。

> 最新补充：Draft 4.440（2026-09-17），`view.basic-windows` 的 `view-unique` 收尾：ViewUnique ords 0-4 已 DV；修复链式 unique→length/time 窗口的逐出传播（`streamWindow` 对 inner oldEvents 调 `removeFromWindowState`）。manifest 不变（682 cases、680 implemented、308 DV、1144 DV runtime IDs、29 intentionally-different、3692 associations、799 unreferenced）。

> 最新补充：Draft 4.439（2026-09-17），`resultset.aggregate-group-by` 的 `resultset-aggregate-filtered-remainder` 收尾：ResultSetAggregateFiltered ords 0/1/2 已 DV；ord 4 `ResultSetAggregateInvalid`（`java-runtime-619cdf3a6b43200abe95`）登记 `intentionally-different`（类型化 API 不可表达非法形态）。manifest：682 cases、680 implemented、308 DV cases、1144 DV runtime IDs、29 intentionally-different、associations 3692（referenced 3337、unreferenced 799）。

> 最新补充：Draft 4.438（2026-09-17），`resultset.aggregate-group-by` 的 `resultset-aggregate-remainder` differential-verified 场景，对照固定 Java `ResultSetAggregateFiltered.java` ord3 `ResultSetAggregateFirstLastEver`（`java-runtime-47dee40e6fe320005f49`）、`ResultSetAggregateSortedMinMaxBy.java` ord5 `ResultSetAggregateMultipleCriteria`（`java-runtime-dd3414775a8421e08a52`）与 `ResultSetAggregateFilterNamedParameter.java` ord19 `ResultSetAggregateAuditAndReuse`（`java-runtime-c95393b13d253135c833`；static IDs `java-3d2c17d4a26d35a0019b`/`java-553516b9d01c12a13172`/`java-0c29efb6d43971aba5c4`；无 flags）：Java/Go 各 15 条 records、0 differences。共享核心新增 `MinByMulti`/`MaxByMulti`/`MinByEverMulti`/`MaxByEverMulti` 异构多键聚合（修复 `aggregateByEverVariadic` 静默丢键）。manifest：682 cases、680 implemented、308 DV cases、1144 DV runtime IDs、associations 3692（referenced 3337、unreferenced 799）。

> 最新补充：Draft 4.437（2026-09-17），`resultset.aggregate-dimensional` 的 `resultset-rollup-having-orderby` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRollupHavingAndOrderBy.java` ordinals 0/1 `ResultSetQueryTypeHaving{join=false}`（`java-runtime-23e2e441fc8898fe0303`）/`{join=true}`（`java-runtime-5c2e7accf8d815fe3ced`）与 `ResultSetQueryTypeRollupGroupingFuncs.java` ordinal 3 `ResultSetQueryTypeGroupingFuncExpressionUse`（`java-runtime-7c4135247329f06e2e40`；static IDs `java-24e47ca92d533e352474`/`java-1ae66c9985dc53282af0`；无 flags）：Java/Go 各 16 条 records、0 differences。场景覆盖 rollup 分层 having（含 `theString is null` 总计行判别）、`SupportBean_S0#lastevent` multiplicity-1 内连接等价、`grouping sets` 上 `grouping()`/`grouping_id()`/无关联子查询/声明表达式逐层求值、聚合行内 `prev(1)`/`prior(1)` 流锚定。共享核心修复：聚合行 prev/prior 接入语句级流历史并屏蔽 group-by 键替换；`JoinField` 补齐 groupingValues 掩码。manifest：681 cases、679 implemented、307 DV cases、1141 DV runtime IDs、associations 3689（referenced 3334、unreferenced 802）。

> 最新补充：Draft 4.326（2026-09-07），`infra.namedwindow.views` 扩展 `case.infra-nwtable-start-stop` differential-verified 场景，对照固定 Java `InfraNWTableStartStop.java` 全部 4 个 execution：InfraStartStopConsumer{namedWindow=true}（`java-runtime-e14796f7892a57096988`）、InfraStartStopConsumer{namedWindow=false}（`java-runtime-7ef0dbf9f8ea7e311720`）、InfraStartStopInserter{namedWindow=true}（`java-runtime-6c0691c9d5549b937cfc`）、InfraStartStopInserter{namedWindow=false}（`java-runtime-683b282c41f311ed8429`；static Consumer 共用 `java-2fe58125cb5f265d5fd1`、Inserter 共用 `java-c4a5793bec169880d651`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；Consumer 两 execution 携带 OBSERVEROPS flag）。Java/Go 各 24 条 records、0 differences：keepall 命名窗口（`create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean`）与复合主键表（`create table MyInfra(a string primary key, b int primary key)`）双形态下的语句级 start/stop 生命周期——每语句独立部署，中途 `undeploy`/`redeploy` 未命名 insert-into 与命名 select 消费者（inserter 形态 stop/redeploy insert：E2/E4 静默、redeploy 后 select 迭代器为 [E1,E3]；consumer 形态 stop/redeploy select：被停 select 的 listener 静默，redeploy 后新消费者首个迭代器快照立即看到当前窗口 [E1,E2]）。命名窗口 istream listener 逐 insert 触发（create+select 各自 sequence 独立计数），表形态全程零 listener 记录（Java 侧 listener 断言全部包在 if(namedWindow) 内）；每形态的每次 start/stop 转换后读取 iterator 快照（consumer NW/表 5/4 条、inserter 3/2 条，多行快照以 mode any 顺序不敏感比对；E1-E4 发送时间线四 execution 相同，落在停用窗口内的发送仍被 infra 保留并出现在后续快照/流中）。typed Go 每语句一个 deployment + deployment.Undeploy 精确停止单语句（对齐 Java undeployModuleContaining 语义）、Statement.Snapshot 迭代器读取（NW）与 FAF-over-FromTable（表）；场景新增 snapshot op（mode any）与 undeploy(statement) op。严格 scenario/oracle validator 固定 Java 元数据、62 步 case/deploy/undeploy/send/snapshot/undeploy-all 顺序、E1-E4 payload 值、每 case 快照计数（5/4/3/2）与 lifecycle 转换存在性，10 种 raw 畸变 + 6 种 trace 变异全部拒绝。manifest 更新为 617 cases、230 个 differential-verified case、816 个 differential runtime IDs、3439 条 associations（referenced 3198、unreferenced 938）；capability 120 个（37 DV）。

> 最新补充：Draft 4.325（2026-09-07），`infra.namedwindow.views` 扩展 `case.infra-nwtable-subq-correl-join` differential-verified 场景，对照固定 Java `InfraNWTableSubqCorrelJoin.java` 全部 3 个 execution：Assertion{namedWindow=true, enableIndexShareCreate=false}（`java-runtime-a1f99ce62c1931e994d8`）、{namedWindow=true, enableIndexShareCreate=true}（`java-runtime-00bf2cd9669115eb2950`）、{namedWindow=false}（`java-runtime-ffb6fe55a1ecde12a134`；static 共用 `java-0ce6cd8b5317076e9a5a`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 12 条 records、0 differences：相关标量子查询 `(select intPrimitive from MyInfra where theString = s1.p10) as val` 嵌入 2-stream last-event join 的 select 子句——首个 S0 事件静默（join 未完成：S1 lastevent 为空），后续 join 行触发 val=20/20/10/30（子查询经唯一键 MyInfra 查找；第 6 步新 S0 p00=E3 不影响 val——子查询仍读 s1.p10=E2）。NW `#unique(theString) as select *` 与复合主键表双形态时间线相同；index-share hint 仅计划层。typed Go 使用 JoinMany(JoinSource(LastEvent) ×2).Select(SelectLeft("val", SubqueryValueWithOptions[int](inner, Field[any,int]("intPrimitive"), SubqueryWhere(Equal[string](Field[any,string]("theString"), JoinField[string](1,"p10"))))))，inner 为 FromNamedWindow/FromTable；插入经 OnEvent InsertIntoNamedWindow/InsertIntoTable+SetColumn；基础设施在引擎构造前创建。严格 scenario/oracle validator 固定 Java 元数据、39 步 deploy/send 顺序与 payload 值、记录形状及 8 种 raw 畸变 + 6 种 trace 变异全部拒绝。manifest 更新为 616 cases、229 个 differential-verified case、812 个 differential runtime IDs、3435 条 associations（referenced 3194、unreferenced 942）；capability 120 个（37 DV）。

> 最新补充：Draft 4.324（2026-09-07），`infra.namedwindow.views` 扩展 `case.infra-nwtable-subq-at-eventbean` differential-verified 场景，对照固定 Java `InfraNWTableSubqueryAtEventBean.java` 两个 execution：InfraSubSelStar{namedWindow=true}（`java-runtime-59604d7dbcef79bacde1`）与 {namedWindow=false}（`java-runtime-d8e8832ac46005459ffd`；static 共用 `java-0d99efae7b23e8d8b346`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 6 条 records、0 differences：整事件子查询 `(select * from MyInfra) @eventbean as detail`——空 infra 时 getFragment 返回 null（Java assertNull 钉定，规范化为 {"state":"null"} 且键存在），填充后为行数组 [{c0:E1,c1:1}] 与 [{c0:E1,c1:1},{c0:E2,c1:2}]（插入序，c1 为 JSON number），p00 恒为 null（SupportBean_S0(id) 构造器不设 p00）。Go 引擎选择 SubqueryValueWithOptions[[]Event](inner, WindowEvents())——聚合组路径累积命名窗口/表候选，values[0] 为全量快照，空 infra 时 WindowEvents() 返回 Null() 精确匹配 Java getFragment null（SubqueryEvents 会渲染为 present-empty [] 而失配）。typed Go 使用 NewMapSchema/CreateNamedWindow(KeepAll) 与 CreateTable(PrimaryKeyColumn+TableColumnOf)、OnEvent InsertIntoNamedWindow/InsertIntoTable+SetColumn；基础设施在引擎构造前创建。严格 scenario/oracle validator 固定 Java 元数据、20 步 deploy/send 顺序与 payload 值（S0 id=0、SupportBean E1/1 与 E2/2）、记录形状及 7 种 raw 畸变 + 6 种 trace 变异全部拒绝。manifest 更新为 615 cases、228 个 differential-verified case、809 个 differential runtime IDs、3432 条 associations（referenced 3191、unreferenced 945）；capability 120 个（37 DV）。

> 最新补充：Draft 4.323（2026-09-07），新增 `infra.namedwindow.views` 的 `case.infra-nwtable-subq-uncorrel` differential-verified 场景，对照固定 Java `InfraNWTableSubqUncorrel.java` 全部 4 个 execution（`java-runtime-e175cfaf55ce541ca088`、`java-runtime-f03db66c82987b5cef54`、`java-runtime-d521ebd5c45c89c8e842`、`java-runtime-1a498d1407159a6d8c90`；static 共用 `java-5af40542812749ae30e4`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 67 条 records、0 differences：投影命名窗口（`create window MyInfra#keepall as select theString as a, longPrimitive as b, longBoxed as c from SupportBean`）与主键表（`create table MyInfra(a string primary key, b long, c long)`）双形态下，非相关标量子查询 `(select a from MyInfra)` 的 null/单行/多行-null 语义（M1 空→null、S1 后→S1、S2 后多行→null），create 语句 insert 行为 new、delete 行为 old，on-delete 语句将删除行发布为 new 数据（Java insert-stream 语义）；index-share hint 仅计划层。Go 引擎两项 parity 驱动修复：命名窗口 delete trigger 的删除行改发布为 newEvents（FAF delete 保留 oldEvents，经 onDemand 区分）；命名窗口 mutation trigger 语句的自身输出延迟至 consumer wave flush 之后（Java 中 create-old 先于 delete-new 到达 listener）。typed Go 使用 CreateNamedWindow(KeepAll)/CreateTable、OnEvent InsertIntoNamedWindow/InsertIntoTable+SetColumn、SubqueryValueWithOptions[string]+SubqueryCardinalityMode(SubqueryNullOnMultiple)、DeleteFromNamedWindow/DeleteFromTableWhere+NamedWindowField/TableField。严格 scenario/oracle validator 固定 Java 元数据、72 步 deploy/send 顺序与 payload 值、记录形状及 8 种 raw 畸变 + 7 种 trace 变异全部拒绝。manifest 更新为 614 cases、227 个 differential-verified case、807 个 differential runtime IDs、3430 条 associations（referenced 3189、unreferenced 947）；capability 120 个（37 DV）。

> 最新补充：Draft 4.322（2026-09-06），`epl.subselect.filtered` 扩展 `case.epl-subselect-order-of-eval-index` differential-verified 场景，对照固定 Java `EPLSubselectOrderOfEval.java` 两个 execution 与 `EPLSubselectIndex.java` 两个 execution：CorrelatedSubqueryOrder（`java-runtime-3dbb926e23a4521d64d5`；static `java-2ddf3ed58c64fe3674f7`）、OrderOfEvaluationSubselectFirst（`java-runtime-9a68943733f98a1ea1dd`；static `java-eaa8e4f13676cc852f92`）、IndexChoicesOverdefinedWhere（`java-runtime-f220d864166a5650e6f7`；static `java-29ec8e7d3c6e96c8d5aa`）、UniqueIndexCorrelated（`java-runtime-0fbd10b1080bb8e7afa3`；static `java-77258a6e645d2449b31c`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 45 条 records、0 differences：相关子查询窗口序——2 语句模块中 window(tl.*)/window(ts.*) 按 securityID 相关、group by securityID 分组输出行数组（墙钟 bean 时间 pin 为 1000/1010，相关性仅读 securityID 且时钟不推进，结果不变）；preeval 默认开启使两条 not-in filter 语句按契约静默（0 records）；19 轮 #unique(<fields>) 数据窗口矩阵——unique 窗口决定子查询保留行集（多行标量子查询返回 null），where 形态覆盖等值/组合/-between/有序范围（含 cycle 14 的 DISABLE_UNIQUE_IMPLICIT_IDX hint 窗口形态；回归的 @Hook 计划断言因类不在 classpath 剔除，属计划观察不影响行为），36 条 {c0,c1} 记录；unique/firstunique/#time(1)#unique/#groupwin#unique 相关标量子查询 7 条记录（含 longPrimitive 102）。typed Go 使用 WindowEvents()/SubqueryValueWithOptions[[]Event]+SubqueryWhere、Not(SubqueryIn)+Unique、SubqueryCardinalityMode(SubqueryNullOnMultiple) 与 UniqueBy 多键数据窗口。严格 scenario/oracle validator 固定 Java 元数据、170 步 deploy/send 顺序、记录形状及 7 种 raw 畸变 + 7 种 trace 变异全部拒绝。manifest 更新为 613 cases、226 个 differential-verified case、803 个 differential runtime IDs、3426 条 associations（referenced 3185、unreferenced 951）；capability 120 个（37 DV）。

> 最新补充：Draft 4.321（2026-09-06），`epl.subselect.filtered` 扩展 `case.epl-subselect-within-filter-having` differential-verified 场景，对照固定 Java `EPLSubselectWithinFilter.java` 两个 execution 与 `EPLSubselectWithinHaving.java` 两个参数化 execution：ExistsWhereAndUDF（`java-runtime-844ff5e51e235151dccc`；static `java-e0811e7acc03cb5daab2`）、RowWhereAndUDF（`java-runtime-1405e55b4e78331c4ec7`；static `java-09e87f283cdd09c8434f`）、HavingSubselectWithGroupBy{namedWindow=true/false}（`java-runtime-5249f7c19ee8d26d8f30`、`java-runtime-4b4ca324ee2a41b8ccd6`；static `java-d69f172401e51113c81d`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 14 条 records、0 differences：filter 内嵌 exists 与标量子查询（inlined-class UDF compareIt 相关性映射为 Go 类型化字符串相等，Java/Go 各 2 条 {id,p00,p01,p02,p03} 行——select * 仅暴露 5 个属性，Java value 字段无 getter 不成属性，实证 pin）；grouped groupwin#length(2) having 由 MyInfra 相关子查询门控——以 #unique(key) 命名窗口与主键表两种形态各回放一次、时间线相同（{G2,21}、{G3,31}、{G3,33}、{G1,105}、{G1,100}，严格 > 边界：和等于 maxAmount 不触发）。typed Go 复用 SubqueryExists/SubqueryValue+OuterField、GroupWindow+GroupBy+Having(Greater(Sum,…))、CreateNamedWindow(Unique retention)/CreateTable 与 OnEvent InsertIntoNamedWindow/InsertIntoTable 列赋值（routeTargetSchema 仅解析命名窗口/普通 schema，故表形态经 trigger 路由）；基础设施在引擎构造前创建以进入引擎的命名窗口快照。严格 scenario/oracle validator 固定 Java 元数据、63 步 deploy(statement)/send payload 顺序、记录形状及 9 种 raw 畸变 + 7 种 trace 变异全部拒绝。manifest 更新为 612 cases、225 个 differential-verified case、799 个 differential runtime IDs、3422 条 associations（referenced 3181、unreferenced 955）；capability 120 个（37 DV）。

> 最新补充：Draft 4.320（2026-09-06），新增 'epl.variable-output-rate' capability 的 'case.epl-variable-output-rate' differential-verified 场景，对照固定 Java 'EPLVariablesOutputRate.java' 全部 4 个 execution：EventsAll/EventsAllOM/EventsAllCompile（'java-runtime-0b028b6fe8bddbbb6480'、'java-runtime-0539523182c81174c7f6'、'java-runtime-303a9b5ee74d52e62125'；static 'java-b8a2d4466ca4d002fa69'、'java-b08aeecf9371278c24f6'、'java-9ee028b9ce2729673e8e'，三种部署形式共享同一可观测时间线）与 TimeAll（'java-runtime-4909a80e9612b815310e'；static 'java-b3199a3fee2738477788'；Java commit '9e1b9f1cc9117fea4bf33ab043762c045d73839c'；无 flags）。Java/Go 各 24 条 records、0 differences：事件计数时间线 'output last every var_output_limit events' 经部署的 on-set 语句把速率 3→5→2→1→null（null 保持现速率），6 次交付 {cnt 3,8,10,11,12,13}；快照时间线 'output snapshot every var_output_limit seconds' 在 3/4/5/8/12 秒交付 {2,4,4,4,6}，重调度按参考点 0 的整倍数对齐（改变速率在下次事件到达或触发时生效，setter 事件不达 s0），13999 置 null 后 14000 推进在投递排队快照前抛出（对齐 Java rethrow handler 消息 'Unexpected exception in statement 's0': Failed to evaluate time period, received a null value for 'Received null value evaluating time period''）。Go 引擎新增表达式驱动的输出速率：OutputPolicy 增加 CountExpr/IntervalExpr，新构造器 OutputLastEveryEventsExpr 与 OutputSnapshotEveryExpr（facade 再导出、plan 校验常量/表达式互斥与每边界求值、时间路径锚定整倍数重调度与调度错误经 engine.AdvanceTime 传播）。typed Go 侧 on-set 用 OnEvent().SetVariables(SetVariableExpr(Cast[*int64,int64](Field(volume))))，变量经 RegisterVariable(int64)+engine.SetVariable 复位。严格 scenario/oracle validator 固定 Java 元数据、99 步 payload/deploy(statement)/advance 顺序、记录形状及 value/time/case/count/record-count mutation（10 种 raw 畸变 + 8 种 trace 变异全部拒绝）。manifest 更新为 611 cases、224 个 differential-verified case、795 个 differential runtime IDs、3418 条 associations（referenced 3177、unreferenced 959）；capability 120 个（37 DV，新增 epl.variable-output-rate）。

> 最新补充：Draft 4.319（2026-09-06），新增 `output.core` 的 `case.resultset-output-limit-changeset-opt` differential-verified 白盒场景，对照固定 Java `ResultSetOutputLimitChangeSetOpt.java` 单 execution `ResultSetOutputLimitChangeSetOpt`（`java-runtime-9b0f27f829d9820e3792`；static `java-9f9a094322f3c6687f00`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。36 轮 × 2 records = Java/Go 各 72 条 records、0 differences：单一 runtime 累计时钟逐轮 redeploy s0（SupportBean#length(2)），每轮发送 E0..E4 后读取白盒 changeset 计数（仅 9 个 DISABLE_OUTPUTLIMIT_OPT 轮 = plain-all/count-last/count-all/string-count-last/string-count-all/grouped-count-last/grouped-count-all/grouped-key-count-last/grouped-key-count-all 读到 5，其余 default/enable/first/snapshot/having 轮全部为 0——Java 侧 default 视图走 POLICY_LASTALL_UNORDERED/ConditionFirst(内联清空)/快照视图，计数恒 0），推进 1 秒后计数清零；ENABLE_OUTPUTLIMIT_OPT+order-by 编译拒绝在两侧代码内断言、不入 trace。Go 引擎修复（parity 驱动，`internal/esper/runtime.go`）：删除 grouped last-every-time 分支的第二次 changeset 递增——applyOutput 每次更新只计一个 interim pair（对齐 Java delta-set 单层缓冲），修复分组轮计数翻倍。typed Go 36 轮构建表使用 `WithOldStream`(irstream)、`WithStatementHints`、`OutputLastEveryTime`/`OutputAllEveryTime`/`OutputFirstEveryTime`、`Aggregate`/`GroupBy`/`Having`/`OrderBy`/`Ascending`。严格 scenario/oracle validator 固定 Java 元数据、217 步 payload/advance 顺序、记录 case/operation/sequence/time/value 形状及 value/time/case/count/record-count mutation。manifest 更新为 610 cases、223 个 differential-verified case、791 个 differential runtime IDs、3414 条 associations（referenced 3173、unreferenced 963）；capability 119 个（36 DV）。

> 最新补充：Draft 4.317（2026-09-05），新增 `output.core` 的 `case.resultset-output-limit-insert-into` differential-verified 场景，对照固定 Java `ResultSetOutputLimitInsertInto.java` 两个 execution：`ResultSetOutputLimitInsertFirst`（`java-runtime-e57a7555b3a0303c0303`；static `java-00bb626878ef4282b5d2`）与 `ResultSetOutputLimitInsertSnapshot`（`java-runtime-cd524991d69bc898c061`；static `java-460f4166b6f5e1099aae`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 9 条 records、0 differences：insert-first 在区间首个事件即时路由 [E1]、新区间 E2 后路由 [E2]（s0 两次、s1 两次）；insert-snapshot 经 keepall 快照在 1000/2000 交付 [E1] 与 [E1, E2]（s0/s1 各两次），而路由消费端逐行接收 [E1]、[E1]、[E2]。typed Go 使用 `InsertInto`+`WithOutput` 组合、`OutputFirstEveryTime`/`OutputSnapshotEvery`、`FromAny` 目标流消费与双语句部署。manifest 更新为 607 cases、220 个 differential-verified case、788 个 differential runtime IDs、3411 条 associations（referenced 3170）；capability 119 个（36 DV）。

> 最新补充：Draft 4.318（2026-09-05），新增 `output.core` 的 `case.resultset-output-limit-microsecond-resolution` 与 `case.resultset-output-limit-parameterized-context` 两个 differential-verified 场景，对照固定 Java `ResultSetOutputLimitMicrosecondResolution.java`（`java-runtime-fb9601d7d5288b0b7ba5`；static `java-c68fd57c7e6ce42ed2d5`；plain-suite millis 参数 0/"1"/1000/1000 与 789123456789/"0.1"/+100 规范化）与 `ResultSetOutputLimitParameterizedByContext.java`（`java-runtime-3fc747eca88c34b6ab3a`；static `java-3bf9eee607100a7666ab`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）。Java/Go 各 4 与 1 条 records、0 differences：微秒解析——`output every 1 seconds` 在 999 静默、1000 边界含端点触发 [E1]、2000 触发 [E2]；`output every 0.1 seconds`（100ms，锚定 789123456789ms = 1995-01-03T08:57:36.789Z）在 +88/-1 静默、+89/+100 边界触发；参数化上下文——context MyCtx 由 SupportScheduleSimpleEvent(athour=10, atminute=15) 启动，per-partition crontab at(minute=15, hour=10) 在 2002-05-01T10:15:00.000Z 一次交付 {c: 1}（start-only context 无终止交付；被 gate 抑制的 S0 事件使 count 保持 1）。typed Go 使用 `OutputEveryTime`、`CreatePatternInitiatedContext`/`PatternFrom`/`ContextPatternField`/`CronValuesExpr`/`OutputAt`/`OutputAndWhenTerminated`、`Aggregate(Alias(CountAll()))` 与 `WithStartTime`。manifest 更新为 609 cases、222 个 differential-verified case、790 个 differential runtime IDs、3413 条 associations（referenced 3172）；capability 119 个（36 DV）。

> 最新补充：Draft 4.316（2026-09-05），`case.output-after-events` 完成 after-gate 家族最后两个 execution：新增 ord 2 `ResultSetMonthScoped`（`java-runtime-04a6303bb13045ec2e3d`；static `java-98f4c2cff7997399ba90`）与 ord 5 `ResultSetSnapshotVariable`（`java-runtime-4621c599c2655d0464c4`；static `java-8038a4481e2996fd8a7f`）两个 case。Java/Go 各 12 条 records、0 differences：`select * from SupportBean output after 1 month` 以 2002-02-01T09:00:00.000 锚定（WithStartTime），2002-03-01T08:59:59.999（due − 1ms）抑制、2002-03-01T09:00:00.000 边界含端点交付 select-* 行（E3, intPrimitive 3）；`select theString from SupportBean#keepall output after 20 seconds snapshot when myvar_local=1`——被抑制事件在窗口快照中重现，20000 交付 [E1..E4]、21000 交付 [E1..E5]。Go 引擎：snapshot-when 的 pending 批次仅从非空更新批次重建，裸时间推进既不刷新也不交付快照（对齐 Java 仅在更新/变量变更时求值）。typed Go 使用 `OutputAfterCalendar(0,1,0)`、`OutputWhenWith(OutputSnapshot(),...)`、`WithStartTime` 与 `RegisterVariable`。manifest 更新为 606 cases、219 个 differential-verified case、786 个 differential runtime IDs、3409 条 associations（referenced 3168）；capability 119 个（36 DV）。

> 最新补充：Draft 4.316（2026-09-05），`case.output-after-events` 扩展 after-gate 时间切片：新增 ord 4 `ResultSetDirectTimePeriod`（`java-runtime-77a5ebb703ccb5fdbf63`；static `java-a71efa831c6880fb5722`）与 ord 1 `ResultSetEveryPolicy`（`java-runtime-88c88732359bcd33f9cd`；static `java-8e3b0bae1be282e4a000`）两个 case。Java/Go 各 9 条 records、0 differences：`output after 20 seconds` 在 t=1/6000/19999 抑制、20000 边界含端点交付 {E4}、21000 交付 {E5}；`output after 20 seconds every 5 seconds` 中 every 调度在 gate 期间锚定，25000 定时器一次交付 [E4, E5]、30000 交付 [E6]（E1..E3 被门控丢弃、E4 缓冲至首个 post-gate 批次）。advance-time 绝对跳变步覆盖虚拟时钟序列。typed Go 使用 `OutputAfterTime` 与 `OutputEveryTime` 组合。manifest 更新为 606 cases、219 个 differential-verified case、784 个 differential runtime IDs、3407 条 associations（referenced 3167）；capability 119 个（36 DV）。

> 最新补充：Draft 4.316（2026-09-05），新增 `output.core` 的
> `case.output-after-events` differential-verified 场景，对照固定 Java
> `ResultSetOutputLimitAfter.java` ordinals 3 与 6 的 `ResultSetDirectNumberOfEvents`
>（`java-runtime-34bfe3c4c56f505d50cd`；static `java-aacc84310d1fab722e9f`）与
> `ResultSetOutputWhenThen`（`java-runtime-499038c2c0fca09551c1`；static
> `java-310737339ed16858a1fc`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；
> 无 flags）：Java/Go 各 5 条 records、0 differences。事件计数 after-gate：keepall
> `output after 3 events` 按输入事件计数且第 3 个事件仍被抑制，E4/E5 以单行回调逐个交付；
> `select a.* from SupportBean#time(10) a output after 3 events when myvar0=true then set
> myvar1=true, myvar2=true`——E1..E3 抑制、运行时置 myvar0、E4 交付一条完整事件行
>（intPrimitive 0, theString E4）且两个 then 赋值经 variable-service 读取验证为 true
>（以 variable 记录入 trace）。typed Go 使用 `OutputAfterEvents`、`OutputWhen`、
> `SetOutputVariable`、`VariableRef`、`Equal`、`TimeWindow`、`KeepAll` 与
> `ReplayWithStatementsAndHandlers` 的 set-variable/read-variable 处理器。manifest 更新为
> 606 cases、219 个 differential-verified case、782 个 differential runtime IDs、3405 条
> associations（referenced 3167）；capability 119 个（36 DV）。

> 最新补充：Draft 4.315（2026-09-05），新增 `resultset.orderby-simple` 的
> `case.resultset-orderby-self-join` differential-verified 场景，对照固定 Java
> `ResultSetOrderBySelfJoin.java` ordinal 0 的 `ResultSetOrderBySelfJoinSimple`
>（`java-runtime-74b0c1ce48febfe83007`；static `java-7cbb50aac0764e25b3bc`；Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）：Java/Go 各 4 条 records、0 differences。
> 三路 SupportHierarchyEvent 自连接（c1 `#lastevent`，c2/p `#groupwin(event_criteria_id)#lastevent`，
> 相关 `in` 谓词含可空 parent 候选）连续交付三条 row-per-event 批次（cnt 经净增量 -1+2、-2+2 保持
> 为 2，`cast(count(*), int)`），加一条保留 join 集的 statement-iterator 快照（均按 c2.priority
> 升序）。Go 引擎：`snapshotJoinAggregateBatch` 对读取非键标量的无分组 join 聚合改为按保留 join
> 元组逐行输出（Java AGGREGATED_UNGROUPED RowPerEvent 迭代器形状），与既有交付形状一致。typed Go
> 使用扁平 `JoinMany`/`JoinSource`、`AggregateStream.Where`（`In[int]`/`InOf` SQL 三值 null 语义）、
> `Cast[int64,int]`/`CountAll`、`GroupWindow`/`LastEvent` 与 iterator replay。manifest 更新为 605
> cases、218 个 differential-verified case、780 个 differential runtime IDs、3403 条 associations
>（referenced 3167）；capability 119 个（36 DV）。

> 最新补充：Draft 4.314（2026-09-05），新增 `resultset.orderby-simple` 的
> `case.resultset-orderby-multi-delivery` differential-verified 场景，对照固定 Java
> `ResultSetOrderBySimple.java` ordinal 0 的 `ResultSetOrderByMultiDelivery`
>（`java-runtime-1c3d57ae9e8d4bca739f`；static `java-5151c7da40772952b49a`；Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；无 flags）：Java/Go 各 3 条 records、0 differences。
> 三个顺序语句 part 固定 ESPER-409 投递批次契约：pattern part 一个 B 事件完成两个 every 分支并
> 一次投递两行按 `a.theString` desc 排序的批次 [A2, A1]；`output every 3 events` part 缓冲三个
> A 事件不输出、在 B 交付三行时立即触发（限值按 match 行计数）[A3, A2, A1]；groupwin+time(10)
> rstream part 在 11 秒边界一次输出两行删除流回调 [A2, A1]。引擎修复：`patternBatch` 在 match
> 收集后按投递行重算 outputInserted/Removed（此前按输入事件计数导致限值提前触发），并开放 pattern
> 查询的 order-by（ResultField 校验 + 交付批次 RSP 级排序），含两个引擎回归测试。typed Go 使用
> `PatternFrom`/`Every`/`FollowedBy`/`LikeOf`/`TagField`、`ResultField`、`OutputAllEveryEvents`、
> `GroupWindow`/`TimeWindow`/`WithRemoveStreamOnly`。manifest 更新为 604 cases、217 个
> differential-verified case、779 个 differential runtime IDs、3402 条 associations（referenced
> 3166）；capability 119 个（36 DV）。

> 最新补充：Draft 4.313（2026-09-05），新增 `resultset.orderby-simple` 的
> `case.resultset-orderby-join` differential-verified 场景，对照固定 Java
> `ResultSetOrderBySimple.java` ordinals 1-2 的 `ResultSetIterator`
>（`java-runtime-53aea47cdd71b80fdbb9`；static `java-3d5138ecf75168eb575d`）与
> `ResultSetAcrossJoin`（`java-runtime-4a0adaff4741914b9ad1`；static
> `java-61243b98a3005c75950a`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；
> 无 flags）：Java/Go 各 4 条 records、0 differences。`ResultSetIterator` 经两条有序
> statement-iterator 快照观察连续 length(10) join（按 price 排序 CAT/15、IBM/49、CAT/50、
> IBM/100，随后 KGB/75 插入第四位；附加 listener 不记录）；`ResultSetAcrossJoin` 重放两个
> output-every-6-events 变体：order by price 在五条预置字符串补满六条 join 增量行后一次性输出
> 六行 new-only（KGB/1、IBM/2、CMU/3、CAT/5、CAT/6、IBM/6），order by theString, price 输出
> symbol-only 六行（CAT 5/6、CMU 3、IBM 2/6、KGB 1），覆盖 join 上未选择列的 order-by 与
> 按 join 增量计数的输出限值语义。typed Go 使用 `Join`/`OnEqual`/`JoinField`、
> `SelectFrom`、`OrderBy`/`Ascending`、`OutputAllEveryEvents` 与不订阅语句的 iterator
> replay；`resultset.orderby-simple` remaining 移除 "ORDER BY on join result sets"。
> manifest 更新为 603 cases、216 个 differential-verified case、778 个 differential runtime
> IDs、3401 条 associations（referenced 3166）；capability 119 个（36 DV）。

> 最新补充：Draft 4.312（2026-09-05），`case.resultset-orderby-aggregate-grouped` 扩展为
> `ResultSetOrderByAggregateGrouped.java` 全部 8 个 execution（Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；新增 ordinals 5-7：`ResultSetLastJoin`
> `java-runtime-12d689fc781af934b76d`（static `java-663c2d183bbd4dc98fa2`）、`ResultSetIterator`
> `java-runtime-680e7b6016501fed42cb`（static `java-0de948d5133afc56e9fe`）、`ResultSetLast`
> `java-runtime-18eacf385e5ac00e8a80`（static `java-726632af45ffc7989e27`）；无 flags）：
> Java/Go 各 11 条 records、0 differences。场景新增 output-last every-6-events 的 row-per-group
> 双批次（CMU/104/3、IBM/102/7、CAT/106/11，随后 DOG/206/1、CMU/204/13、IBM/202/14，区间内无事件的
> 组被抑制）、以及 grouped length(10) join 的两条 statement-iterator 快照（每组两条 join 行携带组级
> sum；4 行后 5 行，无 mode any、按先例固定确定性顺序）。typed Go 使用 `OutputLastEveryEvents`、
> `JoinField`、`LengthWindow`、`GroupBy`、`Sum`、`Alias`、`OrderBy`、`Ascending`，iterator case
> 使用不订阅语句的自定义 replay 路径。同时将 ordinal 1 的 static ID 修正为完整
> `java-015649f9c0449597e55a`（static-manifest 为准），oracle runner script 增加 Windows
> classpath 分号/原生路径自适应。manifest 更新为 602 cases、215 个 differential-verified case、
> 776 个 differential runtime IDs、3399 条 associations（referenced 3166）；capability 119 个
> （36 DV）。

> 最新补充：Draft 4.310（2026-09-04），新增 `output.when-basic` 的
> `resultset-output-limit-row-limit-variable` differential-verified 场景，对照固定 Java
> `ResultSetOutputLimitRowLimit.java` ordinal 9 的 `ResultSetLengthOffsetVariable` execution
>（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime
> `java-runtime-c591e5e05eb22cddfd00`；static ID `java-299a17bb6319c7f2e6de`；无 flags）：Java/Go
> 各 69 条 records、0 differences，覆盖 comma、keyword 和 SODA 三个部署形式，每个形式 21 条 iterator
> snapshot 与 2 条 output-every-5 listener flush。场景固定 `myrows`/`myoffset` 的 nullable Integer setter
> 序列、length(5) 窗口、E1-E10 事件以及负数、零值、null 和 oversized limit/offset；setter 事件不计入输出批次。
> typed Go 使用 `RegisterVariable`、`SetVariables`、`VariableRef`、`LimitExpression`、`OffsetExpression`、
> `LengthWindow` 与 `OutputEvery`；严格 scenario/oracle validator 固定 Java 元数据、payload 顺序、
> listener/iterator shape 和 value/order/null/time/record-count mutation。manifest 更新为 601 cases、214 个
> differential-verified case、768 个 differential runtime IDs、3391 条 associations（referenced 3158）；
> capability 119 个（35 DV）。

> 最新补充：Draft 4.309（2026-09-04），新增 `output.when-basic` 的
> `resultset-output-limit-row-limit-invalid` differential-verified 场景，对照固定 Java
> `ResultSetOutputLimitRowLimit.java` ordinal 8 的 `ResultSetInvalid` execution（Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime `java-runtime-1d2703c5e4976fa0dd90`；
> static ID `java-b2d63277ca12f283120d`；无 flags）：Java/Go 各 4 条 compile-rejected records、0 differences。
> 场景按固定 `RegressionPath` 先注册 string `myrows = 'abc'`，再依次验证 numeric-type 与 unknown-variable
> 的 limit/offset 诊断；无 statement deploy 或事件输出。typed Go 新增 `LimitExpression`/`OffsetExpression`
> 类型安全链式入口，并修复动态 modifier 在 deferred output、grouped/trigger iterator snapshot 的当前变量
> 快照、排序和 result-window 边界语义；严格 scenario/oracle validator 固定 metadata、exact diagnostics、
> build-error phase、probe 顺序及 value/order/record-count mutation。manifest 更新为 600 cases、213 个
> differential-verified case、767 个 differential runtime IDs、3388 条 associations（referenced 3157）；
> capability 119 个（35 DV）。

> 最新补充：Draft 4.308（2026-09-03），新增 `output.when-basic` 的
> `resultset-output-limit-row-limit-negative-rowcount` differential-verified 场景，对照固定 Java
> `ResultSetOutputLimitRowLimit.java` ordinal 7 的 `ResultSetGroupedSnapshotNegativeRowcount` execution
>（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime
> `java-runtime-6bf4cf3ded03c9ff57fa`；static ID `java-0109d52ee4e8b36575b9`；无 flags）：Java/Go
> 各 2 条 records、0 differences。场景固定 1 秒部署前 timer、空 iterator snapshot、E1/10、E2/5、E3/20、E1/30
> 四个事件，以及 11 秒 grouped snapshot listener；descending `sum(intPrimitive)`、`limit -1` unlimited 与 `offset 1`
> 的结果为 E3/20、E2/5。typed Go 使用 `LengthWindow`、`GroupBy`、`Sum`、`OutputSnapshotEvery`、descending
> `OrderBy`、`Limit(-1)` 和 `Offset(1)`；strict scenario/oracle validator 固定 Java 元数据、exact EPL、payload
> 顺序、timer boundary、listener/iterator shape，以及 value/order/time/record-count mutation；Go validation
> 同时保持 negative offset rejection。manifest 更新为 599 cases、212 个 differential-verified case、766 个
> differential runtime IDs、3387 条 associations（referenced 3156）；capability 119 个（35 DV）。

> 最新补充：Draft 4.307（2026-09-03），新增 `output.when-basic` 的
> `resultset-output-limit-row-limit-context-grouped` differential-verified 场景，对照固定 Java
> `ResultSetOutputLimitRowLimit.java` ordinals 0 和 4 的
> `ResultSetLimitOneWithOrderOptimization`、`ResultSetFullyGroupedOrdered` executions
>（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes
> `java-runtime-0a4f187046b734b5dc9a`、`java-runtime-19d52dc587ea246a3e07`；static IDs
> `java-3f5845fdac39997e9b3d`、`java-bf49a03f52cc732a2cf6`；无 flags）：Java/Go
> 各 30 条 records、0 differences。场景固定 order-optimized `length_batch(10)`/`length_batch(5)`
> limit-one 的 ordered listener + iterator 边界，以及 `StartS0EndS1` keepall
> `output snapshot when terminated` 的 3 次单键/多键状态重置；ordinal 4 固定
> `length(5)` grouped `sum(intPrimitive)` 的 order-by-aggregate limit 2 迭代器替换、淘汰与排序。
> typed Go 使用 `LengthBatch`、`KeepAll`、`CreateInitiatedTerminatedContext`、
> `OutputSnapshotWhenTerminated`、`GroupBy`、`Sum`、`OrderBy` 和 `Limit`；严格
> scenario/oracle validator 固定 Java 元数据、生命周期边界、payload 顺序、listener/iterator
> records、aggregate replacement/eviction 以及 value/order/state/time/record-count mutation。
> manifest 更新为 598 cases、211 个 differential-verified case、765 个 differential runtime IDs、
> 3386 条 associations（referenced 3155）；capability 119 个（35 DV）。

> 最新补充：Draft 4.306（2026-09-03），新增 `output.when-basic` 的
> `resultset-output-limit-row-limit` differential-verified 场景，对照固定 Java
> `ResultSetOutputLimitRowLimit.java` ordinals 1 和 3 的
> `ResultSetBatchNoOffsetNoOrder`、`ResultSetBatchOffsetNoOrderOM` executions
>（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes
> `java-runtime-840e283d8ac639591083`、`java-runtime-20d0d1451ce549bf53b5`；static IDs
> `java-2d2f2a8e404c07c322c6`、`java-789d7de75e91392eb5e1`；无 flags）：Java/Go
> 各 16 条 records、0 differences。场景固定 wildcard `irstream`
> `length_batch(3) limit 1` 的 listener + iterator 边界，第二 execution 额外固定
> SODA `toEPL()` 与 EPL-to-model 编译；E1/E2/E3 冲刷 new E1，E4/E5/E6 冲刷
> new E4 + old E1。typed Go 使用 `LengthBatch`、`WithOldStream`、`Limit` 和
> iterator replay；严格 scenario/oracle validator 固定 Java 元数据、payload、旧新顺序、
> batch flush、时间和 value/old-new/time/record-count mutation。manifest 更新为 597
> cases、210 个 differential-verified case、763 个 differential runtime IDs、3384 条
> associations（referenced 3153）；capability 119 个（35 DV）。

> 最新补充：Draft 4.305（2026-09-03），新增 `resultset.aggregate-local-group` 的
> `resultset-querytype-local-group-by` differential-verified 场景，对照固定 Java
> `ResultSetQueryTypeLocalGroupBy.java` ordinal 13 的 `ResultSetLocalGroupedMultiLevelAccess`
> execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime
> `java-runtime-87b3bcfdf164bc939fe5`；static ID `java-b0b3444c9ac5a1297cd0`；无 flags）：
> Java/Go 各 1 条 listener snapshot record、3 条有序新行、0 differences。场景固定
> `SupportBean#keepall`、外层 `group by theString,intPrimitive`、五个 `window(*)`
> local `group_by` 维度以及 10 秒 snapshot；typed Go 使用 `KeepAll`、`GroupBy`、
> `LocalGroupBy`、`WindowEvents`、`OutputSnapshotEvery` 与 `OrderBy`，完整保留
> SupportBean 事件身份及各维度到达顺序。严格 scenario/oracle validator 固定 Java
> 元数据、EPL、payload、timer boundary、listener shape、完整事件字段和 value/order/time/
> record-count mutation；ordinal 14、row-remove、multikey/plugin、join/context/table/FAF
> 与 planning/invalid executions 仍列为 remaining。manifest 更新为 596 cases、209 个
> differential-verified case、761 个 differential runtime IDs、3382 条 associations
> （referenced 3151）；capability 119 个（35 DV）。

> 最新补充：Draft 4.304（2026-09-02），新增 `resultset.aggregate-filtered` 的
> `resultset-aggregate-filtered-w-math-context` differential-verified 场景，对照固定 Java
> `ResultSetAggregateFilteredWMathContext.java` ordinal 0 的 `ResultSetAggregateFilteredWMathContext`
> execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime
> `java-runtime-fa8b6d5d6fb58a905f23`；static ID `java-aba2cfbf41a3be9809f1`；无 flags）：
> Java/Go 各 3 条 listener records、0 differences。场景固定 compiler MathContext precision 2
> `HALF_UP`、source helper 丢弃 `setScale` 后的 BigDecimal `0`、`0`、`1`，观察 unbounded
> `avg(bigdec)` 的 `0`、`0`、`0.33` 新行；typed Go 使用环境级 `WithDecimalMathContext`、
> `AvgExact` 与 `big.Rat`，并以 plan identity/invalid-option 测试固定配置边界。严格
> scenario/oracle validator 固定 Java 元数据、EPL、payload、listener shape 与 mutation。
> manifest 更新为 595 cases、208 个 differential-verified case、760 个 differential runtime
> IDs、3381 条 associations（referenced 3151）；capability 119 个（34 DV）。

 [CHANGELOG.md#6EAD]
> 最新补充：Draft 4.304（2026-09-02），新增 `resultset.aggregate-filtered` 的
> `resultset-aggregate-filtered-w-math-context` differential-verified 场景，对照固定 Java
> `ResultSetAggregateFilteredWMathContext.java` ordinal 0 的 `ResultSetAggregateFilteredWMathContext`
> execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime
> `java-runtime-fa8b6d5d6fb58a905f23`；static ID `java-aba2cfbf41a3be9809f1`；无 flags）：
> Java/Go 各 3 条 listener records、0 differences。场景固定 compiler MathContext precision 2
> `HALF_UP`、source helper 丢弃 `setScale` 后的 BigDecimal `0`、`0`、`1`，观察 unbounded
> `avg(bigdec)` 的 `0`、`0`、`0.33` 新行；typed Go 使用环境级 `WithDecimalMathContext`、
> `AvgExact` 与 `big.Rat`，并以 plan identity/invalid-option 测试固定配置边界。严格
> scenario/oracle validator 固定 Java 元数据、EPL、payload、listener shape 与 mutation。
> manifest 更新为 595 cases、208 个 differential-verified case、760 个 differential runtime
> IDs、3381 条 associations（referenced 3151）；capability 119 个（34 DV）。
> 最新补充：Draft 4.303（2026-09-02），新增 `resultset.aggregate-filtered` 的
> `resultset-aggregate-filter-named-parameter` differential-verified 场景，对照固定 Java
> `ResultSetAggregateFilterNamedParameter.java` ordinals 4-7 的四个 execution（Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-7bc068fcf2ea07b9c1f7`、
> `java-runtime-dd319218418ee0418b21`、`java-runtime-4b24ef27ade0eae24258`、
> `java-runtime-3d732054eac8d5b8ba14`；shared static ID `java-0c29efb6d43971aba5c4`；无 flags）：
> Java/Go 各 24 条 listener records、0 differences。场景覆盖 length(2) `leaving(filter:...)`
> 的淘汰事件判定、keep-all `nth(..., filter:theString like 'A%')` 的零基逆序资格历史、
> 虚拟时钟边界的过滤 `rate(1s)`，以及 length(3) 时间戳/数量 rate 的首次可报告窗口淘汰值。
> typed Go 使用 `LengthWindow`、`Leaving`、`FilterAggregate`、`Nth`、`Rate`、
> `RateByTimestamp` 与 `RateQuantityByTimestamp`；`internal/esper/expr.go` 修正无界过滤 rate
> 在精确时间边界的 Null/非零状态。严格 scenario/oracle validator 固定 Java 元数据、EPL、
> Null/time/count mutation。manifest 更新为 594 cases、207 个 differential-verified case、759 个 differential runtime IDs、3380 条 associations（referenced 3151）；capability 119 个（34 DV）。

> 最新补充：Draft 4.302（2026-09-02），新增 `resultset.aggregate-having` 的
> `resultset-querytype-aggregate-grouped-having` differential-verified 场景，对照固定 Java
> `ResultSetQueryTypeAggregateGroupedHaving.java` 全部 4 个 execution（Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-1474d172cf4f2a19b5d7`、
> `java-runtime-88a7913c4758d8de0bc9`、`java-runtime-dbe24180b80fd3c64d66`、
> `java-runtime-aafa294bfd2a1104befd`；shared inventory/static ID
> `java-29aa3ed4e339f5786c1f`，specific static candidates `java-cc01f532a81e94885155`、
> `java-a24c796fd8d8a7f7d632`；无 flags）：Java/Go 各 6 条 listener records、0 differences。
> 场景覆盖 `length_batch(3)` 分组 `count(*) > 1` 的 wildcard 与 join 逐事件冲刷、
> `irstream sum(price) >= 50` 的单视图/join 孪生、预扣除 remove-stream 旧行，以及
> `where` 在 `#length(3)` 之后的 unmatched-symbol 淘汰边界；typed Go 使用
> `LengthBatch`、`LengthWindow`、`Filter`、`GroupBy`、`CountAll`、`Sum`、`Having`、
> `JoinMany`、`JoinField`、`WithOldStream` 与显式 SupportBean 属性投影。运行时修复将
> grouped aggregate 的非 key 事件绑定新行按 Java per-event 形状发送，并保持原有旧行分类。
> manifest 更新为 593 cases、206 个 differential-verified case、755 个 differential runtime
> IDs、3376 条 associations（referenced 3150）；capability 119 个（34 DV）。
> 最新补充：Draft 4.301（2026-09-02），新增 `resultset.aggregate-group-by` 的
> `resultset-querytype-row-per-event` differential-verified 场景，对照固定 Java
> `ResultSetQueryTypeRowPerEvent.java` 全部 7 个 execution（Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；shared static/inventory ID
> `java-1a361248f817f0b81296`；无 flags）：Java/Go 各 28 条 records、0 differences。场景覆盖
> irstream sum 的视图/join 孪生与淘汰后旧行、`window(s0.*)`+`sb` keepall join 的逐元组行、
> ESPER-571 无分组 having 绑定当前事件、where 预视图过滤、distinct 聚合窗口/无界矩阵。
> 引擎修复：`expressionTreeReadsCurrentEvent` 识别 join-event 列使无分组 join 聚合路由到
> 最新补充：Draft 4.300（2026-09-02），新增 `resultset.aggregate-having` 的
> `resultset-querytype-row-per-group-having` differential-verified 场景，对照固定 Java
> `ResultSetQueryTypeRowPerGroupHaving.java` 全部 5 个 execution（Java commit
> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-13f0da7834ee6587f0b9`、
> `java-runtime-0c30e4c1d3e8f0d1f3f7`、`java-runtime-3eb8a8a63a4f3f0b7c1d`、
> `java-runtime-4d8a8f7d6f3e2c1b0a9e`、`java-runtime-8b7c6d5e4f3a29181716`；shared static/inventory ID
> `java-5c6f5f9c8c0f3a1b395c`；无 flags）：Java/Go 各 7 条 execution records、0 differences。
> 场景覆盖 `ResultSetProcessorRowPerEvent` 形状、聚合/非聚合 grouped having、join 与
> where/length 边界；typed Go 使用 `GroupBy`、`Having`、`JoinMany`、`LengthWindow` 与
> explicit projections。manifest 更新为 592 cases、205 个 differential-verified case、
> 751 个 differential runtime IDs、3372 条 associations（referenced 3150）；capability 118 个（34 DV）。


> `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-fabf6dfeea92bd82d953`、
> `java-runtime-b77f112e44eb71ef5269`、`java-runtime-8e64b633898a3cf68ed8`、
> `java-runtime-3673c61f7b1a281d9970`、`java-runtime-cd60cf2c28460a91c7d1`；shared static/inventory
> ID `java-002a2b5ee61a47346e61`；无 flags）：Java/Go 各 11 条 records、0 differences。场景覆盖
> 通配行计数门控、irstream sum(price) having 的单视图/join 孪生与预扣除 remove-stream 旧行、
> time_batch count 冲刷与 declared-expression having 编译边界；typed Go 使用 `CountAll`、`Sum`、
> `GreaterOrEqual`、`GroupBy`、`Having`、`WithOldStream`、`TimeBatch` 与 `DefineExpression`/
> `ExpressionRef`，`internal/esper` 新增 grouped remove-stream PRE-removal 求值与
> `validateAggregateHavingContainment`（Java 原句）。manifest 更新为 591 cases、204 DV、744 DV
> runtime IDs、3365 associations；capability 118 个（34 DV）。
> 最新补充：Draft 4.299（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-aggregate-sorted-minmax-by-no-alias` deploy-only statement-metadata differential-verified 场景，对照固定 Java `ResultSetAggregateSortedMinMaxBy.java` ordinal 3 的 `ResultSetAggregateNoAlias`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime `java-runtime-bb6969a66cad8464ae18`；shared static/inventory ID `java-553516b9d01c12a13172`；无 flags）：Java/Go 各 2 条 records（deployed 确认 + 有序 types 数组）、0 differences、无事件。场景覆盖五个无别名投影的 Java 自动命名与三个 String 型差分列的类型 token；表示登记：Java 将 whole-event `minby`/`minbyever` 输出与 `sorted()` 行渲染为 `java.util.Map`，Go typed API 保持 SupportBean 事件身份，三个非差分列登记为表示差异；typed Go 使用 `TimeWindow`、`Aggregate`、精确 Java 自动命名的 `Alias`、`Property`、`MaxBy`/`MinBy`、`MaxByEver`/`MinByEver` 与 `SortedEvents`，`internal/compat` 将 `deployed` 步骤路由至协议 handler；manifest 更新为 590 cases、588 implemented、203 个 differential-verified case、739 个 differential runtime IDs、3360 条 associations（referenced 3134）。
> 最新补充：Draft 4.298（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-aggregate-sorted-first-last` differential-verified 场景，对照固定 Java `ResultSetAggregationMethodSorted.java` ordinals 5-6 的 `ResultSetAggregateSortedFirstLast` 与 `ResultSetAggregateSortedFirstLastEnumerationAndDot`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-2ec322fbd681b83590f1`、`java-runtime-b7e36a11fb9b9c982249`；static IDs `java-0ded2b32c81677a9d36f`、`java-5eb997e432653280b46b`；shared inventory ID `java-0ded2b32c81677a9d36f`；无 flags）：Java/Go 各 2 条 listener records、0 differences。场景覆盖 keyed table 的 sorted 首尾事件/桶/键访问与重复键桶插入序（E1a/E1b 同键 1、E6a/E6b 同键 6）、`minBy`/`maxBy` 取最低/最高键桶首事件、点式 `firstEvent().theString` 与 `firstOf`/`lastOf` 枚举投影；typed Go 使用 `CreateTable`、`IntoTable`、`SortedAccessBy`、`TableField`、`Method`、`EnumFirstOf` 与 `EnumLastOf`，严格 scenario/oracle validator 固定 metadata、payload、values/order/time/record-shape 并拒绝 malformed JSON、duplicate keys 与 trace mutation；manifest 更新为 589 cases、202 个 differential-verified case、738 个 differential runtime IDs、3359 条 associations（referenced 3133）；capability 119 个（34 DV）。
> 最新补充：Draft 4.297（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-aggregate-sorted-grouped` differential-verified 场景，对照固定 Java `ResultSetAggregationMethodSorted.java` ordinal 11 的 `ResultSetAggregateSortedGrouped`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime `java-runtime-3ffe177eb6a90acb699a`；static candidate `java-5a1abe9fb7420df7353d`；shared inventory ID `java-0ded2b32c81677a9d36f`；无 flags）：Java/Go 各 5 条 listener records、0 differences。场景覆盖 keyed table 的 grouped sorted collection、缺失 key、重复 sort key、`firstKey`/`lastKey` 和 Java scalar `sortcol` Null 投影；typed Go 使用 `CreateTable`、`IntoTable`、`SortedAccessBy`、`TableField`、`Method`、`EventValue` 与 `OnEvent.SelectFromTableWhere`，严格 scenario/oracle validator 固定 metadata、payload、values/order/time/record-shape，并拒绝 malformed JSON、duplicate keys、trace mutation；Go-only normalizer 保留 Java sortcol mutation 检测；manifest 更新为 588 cases、586 implemented cases、201 differential-verified cases、736 differential-verified runtime IDs、3357 associations（referenced 3133）；capability 119 个（34 个 differential-verified）。
> 最新补充：Draft 4.296（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-aggregate-sorted-table-access` differential-verified 场景，对照固定 Java `ResultSetAggregationMethodSorted.java` ordinals 7-9 的 `ResultSetAggregateSortedGetContainsCounts`、`ResultSetAggregateSortedSubmapEventsBetween` 与 `ResultSetAggregateSortedNavigableMapReference`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-d7d60fec056d6eb16659`、`java-runtime-0c0e45c751e3cafeb454`、`java-runtime-2117ba8232651a61a058`；static IDs `java-ad219f9a27aebc7885fc`、`java-4b989f8ba297c8e8d426`、`java-db7b9e752ef9e4000cc5`；shared inventory ID `java-0ded2b32c81677a9d36f`；无 flags）：Java/Go 各 325 条 listener records、0 differences。场景覆盖 table-backed sorted `get`/`contains`/counts、inclusive `eventsBetween`/`subMap` ranges、duplicate-key buckets、missing-key Nulls 与 detached navigable-map snapshot；typed Go 使用 `CreateTable`、`IntoTable`、`SortedAccessBy`、`TableField`、`Method`、`EventValue` 与 `OnEvent.SelectFromTableWhere`，严格 validator 拒绝 malformed JSON、重复字段、非布尔边界和 trace mutation；manifest 更新为 587 cases、200 个 differential-verified case、735 个 differential runtime IDs、3356 条 associations（referenced 3133）；capability 119 个（34 DV）。
> 最新补充：Draft 4.295（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-aggregate-window` differential-verified 场景，对照固定 Java `ResultSetAggregationMethodWindow.java` ordinals 1-3 的 `ResultSetAggregateWindowTableAccess`、`ResultSetAggregateWindowTableIdentWCount` 与 `ResultSetAggregateWindowListReference`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-785f2999e48fbaa6eb77`、`java-runtime-9ef8f9a367e788b5afec`、`java-runtime-6652f083e2b0f3dffc04`；static IDs `java-f881d0116e155d3e9063`、`java-804f7ea3bb20de0d35e8`、`java-f1014305fc8b701e8ea0`；shared inventory ID `java-313287657d14b5c686f8`；无 flags）：Java/Go 各 6 条 listener records、0 differences。场景覆盖 length(2) table first/last event access、keep-all table first/last property access 与 count、以及 list-reference values；typed Go 使用 `EventValue`、`WindowAccessBy`、`IntoTable`、`TableField`、`Method`、`Property`、`LengthWindow`、`KeepAll` 与 `OnEvent` table trigger，严格 scenario/oracle validator 固定 metadata、payload、values/order/time/Null/record-shape 并拒绝 malformed、duplicate-key 与 trace mutation；manifest 更新为 586 cases、584 implemented cases、199 个 differential-verified case、732 个 differential runtime IDs、3353 条 associations（referenced 3133）。

# Esper 9.0.0 Go 移植：执行路线图与遗漏检查



> 文档定位：本文件只维护当前阶段、优先级、remaining 和风险。完整范围与架构见 [实施规划](esper-go-port-implementation-plan.md)，日常步骤见 [执行手册](esper-go-port-runbook.md)，差分、合成数据和验收口径见 [质量策略](esper-go-port-quality-strategy.md)，历史见 [CHANGELOG](../CHANGELOG.md)。统计数字以 `testdata/compat/capability-manifest.json` 的已校验 `summary` 为唯一来源。

## 0. 实时状态入口

> 最新补充：Draft 4.435（2026-09-17），ResultSetOutputLimitRowPerGroup none/default 簇 + row-per-group 纯过期 new 行修复：新登记 `case.resultset-output-limit-row-per-group-none` 与 `case.resultset-output-limit-row-per-group-default`（均出生即 DV，umbrella 再拆 8 个 execution 至 12）——ord1-4 NoneNoHaving/NoneHaving[Join]（无 output 子句，逐事件即时输出）与 ord5-8 DefaultNoHaving/DefaultHaving[Join]（`output every 1 seconds` 缓冲逐事件增量行，过期组发 null 值 new 行，调度锚定首个事件）。ResultAssertExecution 双跑（istream + irstream）+ 共享 ResultAssertInput 调度（各 320 步 scenario）。Java/Go 各 62 + 38 条 records、0 differences。**共享核心修复（`internal/esper/runtime.go`）**：`outputLimitExpiryNew` 扩展到 row-per-group 结果形态（`rowPerGroupShape`：grouped + 非聚合裸列 + 非表/命名窗口 + 非 row-per-event）——Java `ResultSetProcessorRowPerGroupImpl.processViewResult` 的 keysAndEvents 同时含 newData 与 oldData 键，纯过期批次对每个键生成 new 行（ord1-4 在 t=5700/6300/7000 的过期 new 行）；全聚合 grouped 形态保持 suppress-pure-expiry 契约。11 条相邻 diff 回归全绿。

> 最新补充：Draft 4.434（2026-09-16），ResultSetOutputLimitRowPerGroup output-all 簇 + OutputConditionTime force-dispatch 修复：新登记 `case.resultset-output-limit-row-per-group-all`（出生即 DV，umbrella 再拆 4 个 execution 至 20）——ord9/10 `ResultSet9/10AllNoHaving[Join]`（`java-runtime-3cb45ffbbce3a1039f72` / `java-runtime-2cfe8200a666f421582d`，`order by symbol`）、ord11/12 `ResultSet11/12AllHaving[Join]`（`java-runtime-2a425da1594958f77a88` / `java-runtime-073783d29500f840e025`，`having sum(price) > 50` × 3 个 outputlimit-opt hint 变体，无 order-by——ENABLE hint 与 order-by 编译冲突故 9/10 不循环 hint）。ResultAssertExecution 双跑 + 共享调度（636 步 scenario）。Java/Go 各 94 条 records、0 differences。**共享核心修复（`internal/esper/runtime.go` + `internal/compat/scenario.go`）**：(1) `applyAllEveryTime` 分组分支按 Java `groupReps` 语义跟踪 last-generated 行与 having 可见性（`allEveryOutputRows`/`allEveryOutputVisible`/`allEveryUpdatedKeys`）——interval-start 行仅在其自身通过 having 时作 old 投递，updated-but-failed 组不发 new 且标记代表行不可见，空组在 empty-having 通过时重发 null 聚合行（t=7200 MSFT null）。(2) Java `OutputConditionTime`/`Crontab` 在静默边界 force-dispatch 空对回调（`OutputStrategyUtil` forceUpdate）：`applyAllEveryTime`、`applyLastEveryTime(+Grouped)` 与 `OutputEveryTimePolicy` 路径在边界触发但无输出时返回 forced batch，`finishOutput` 放行 forced 空 batch 至 `applyOutputAssignments`；parity trace 维持既定约定只记录携带负载的回调——Java oracle 跳过相同空对，compat 记录器跳过空 listener batch。(3) `applyLastEveryTimeGrouped` old 行按 `lastEveryOutputVisible` 门控（interval-start 行须通过 having），全抑制批次（updatedGroupKeys）仍合并以失效陈旧 pending 行并武装调度，`mergeLastOutputPending` 在 empty-incoming 路径同样丢弃陈旧 pending 行并将未分组 `<all>` 键归一。

> 最新补充：Draft 4.433（2026-09-16），ResultSetOutputLimitRowPerGroup having/first/snapshot 簇：新登记 `case.resultset-output-limit-row-per-group-having-first-snap`（出生即 DV，umbrella 再拆 6 个 execution 至 24）——ord17/18 `ResultSet15/16LastHaving[Join]`（`java-runtime-0537627a9e2ced9a101d` / `java-runtime-5662e97901c7b1960530`，`having sum(price) > 50` + output last every 1s × 3 个 outputlimit-opt hint 变体）、ord19/20 `ResultSet17FirstNoHaving[Join]`（`java-runtime-7c3c8427c5d48c24774e` / `java-runtime-54b18a72ff7fa42b3afe`，output first every 1s 含过期驱动行）、ord21/22 `ResultSet18SnapshotNoHaving[Join]`（`java-runtime-ebfb2b66f2dd8778a08b` / `java-runtime-5d74da396c739c122464`，output snapshot + `order by symbol`，空组剔除）。ResultAssertExecution 双跑 + 共享 ResultAssertInput 调度（796 步 scenario）。Java/Go 各 114 条 records、0 differences。**共享核心修复（`internal/esper/runtime.go`）**：(1) `ResultBatch` 新增 `updatedGroupKeys`（批次触达的全部组键，含过期），经 `mergeLastOutputBatch` 合并；更新但无新行的键（having 拒绝最新状态）使 pending 旧行失效；(2) `applyLastEveryTimeGrouped` 对每个 updated 键发 old（有前次输出用前次输出，否则 null-prior 按 having 空组可见性门控）——Java 在组最新状态失败 having 时仍发 old（t=7200 IBM new 抑制、old{IBM,72} 照发）；(3) `aggregateBatch` 过期补发新行扩展到全部显式 output-limit 策略（原仅 output-last 族）——output first 的过期行对齐 Java `updateOutputCondition(0,1)`。manifest 更新为 676 cases / 674 implemented / 302 DV cases / 1122 DV runtime IDs / 3682 associations / referenced 3331 / unreferenced 805。

> 最新补充：Draft 4.432（2026-09-16），ResultSetOutputLimitRowPerGroup last×join×order-by 簇：新登记 `case.resultset-output-limit-row-per-group-last`（出生即 DV，umbrella 再拆 4 个 execution 至 30）——ord13 `ResultSet13LastNoHavingNoJoin`（`java-runtime-930299a6192880bda3bc`）、ord14 `ResultSet14LastNoHavingJoin`（`java-runtime-79057f1ac92150b68e68`，SupportBean#keepall join）、ord15/16 WOrderBy 孪生（`java-runtime-c30ea1262639b9008249` / `java-runtime-101fbc773a180b386246`，`order by symbol` 精确序）。ResultAssertExecution 双跑契约：plain select（istream，old 恒 null）+ `select irstream`，共享 ResultAssertInput 虚拟时间调度（#time(5.5 sec) + output last every 1s，320 步 scenario）。Java/Go 各 48 条 records、0 differences。**共享核心修复（`internal/esper/runtime.go`）**：(1) `applyLastEveryTimeGrouped` 普通 row-per-group 改走 previous-output-old 路径（old = 每组上次输出行，含已空组保留输出），不再用 delta 合并 old，并按 selector 门控合成 old——同时关闭 rollup output-last istream 潜在缺陷；(2) `aggregateBatch` 在 output-last 策略族下为纯过期批次补发受影响组的移除后新行（Java output-last helper 跟踪 insert+expiry 全部更新键），其余策略的纯过期抑制不变；(3) `applyFirstEveryTime` 分组 having 分支 New→Old 复制按 remove-stream selector 门控。manifest 更新为 675 cases / 673 implemented / 301 DV cases / 1116 DV runtime IDs / 3682 associations / referenced 3331 / unreferenced 805。

> 最新补充：Draft 4.431（2026-09-16），ResultSetOutputLimitRowPerGroup multikey 尾簇：新登记 `case.resultset-output-limit-row-per-group-multikey`（出生即 DV，umbrella 再拆 4 个 execution 至 34）——ord39 `ResultSetOutputFirstMultikeyWArray`（`java-runtime-4b69198e8a2cc665a379`，int[] 数组内容相等分组键 + output first every 10s 首行即发后抑制）、ord40 `ResultSetOutputAllMultikeyWArray`（`java-runtime-49a1acbfbe84da2b0479`，theString+longPrimitive 键 #keepall output all every 1s 全组重发）、ord41 `ResultSetOutputLastMultikeyWArray`（`java-runtime-80584d4ff67f59c3a260`，output last 仅更新组）与 ord42 `ResultSetOutputSnapshotMultikeyWArray`（`java-runtime-dbe1c30970ff2232fc35`，无窗口分组 snapshot 每 10s 创建序行）。Java/Go 各 6 条 records、0 differences。**共享核心修复（`internal/esper/runtime.go`）**：`applyAllEveryTime` 分组分支无条件合成 old 行——Java 仅 irstream/rstream 发 remove stream；old 合成现按 selector 门控，修正非 irstream output-all 的多余 old 行。manifest 更新为 674 cases / 672 implemented / 300 DV cases / 1112 DV runtime IDs / 3682 associations / referenced 3331 / unreferenced 805。

> 最新补充：Draft 4.430（2026-09-16），ResultSetOutputLimitRowPerGroup 事件计数簇：新登记 `case.resultset-output-limit-row-per-group-events`（出生即 DV，从 `case.output-row-per-group` umbrella 拆出 5 个 execution）——ord27 `ResultSetGroupByDefault`（`java-runtime-fb1d7cc0c950463969d1`，默认策略缓冲逐事件行）、ord29 `ResultSetNoJoinLast`（`java-runtime-c55c536922e604536dd8`，output last every 2 events 全局计数器）、ord32 `ResultSetNoJoinAll`（`java-runtime-33f3e496a6431e2b7d10`，output all 重发未变组）、ord33 `ResultSetJoinLast`（`java-runtime-896a57e1bb14df31d330`）与 ord34 `ResultSetJoinAll`（`java-runtime-897df7824f16ef7db1c9`，SupportBeanString join 孪生）；hinted executions 各跑 3 个 ENABLE/DISABLE_OUTPUTLIMIT_OPT hint 变体（deploy/undeploy-all 轮次）。Java/Go 各 25 条 records、0 differences。**共享核心修复（`internal/esper/runtime.go`）**：`applyAllEveryEvents` 按处理器行形状分流——row-per-group（仅键+聚合）的 `output all` 每组一行 + irstream 的区间起点 old 行（新增 `allEveryOld` 跟踪），此前有界分组 `output all` 错走逐事件 pending 路径（Java `ResultSetProcessorRowPerGroupOutputAllHelper` 语义）。manifest 更新为 673 cases / 671 implemented / 299 DV cases / 1108 DV runtime IDs / 3682 associations / referenced 3331 / unreferenced 805；`output.core` capability 获首批 5 个 DV runtime IDs。

> 最新补充：Draft 4.429（2026-09-16），ResultSetAggregateFirstLastWindow 文件收官：新增 `case.resultset-aggregate-firstlastwindow-star`（出生即 DV）覆盖最后三个未关联 execution——ord0 `ResultSetAggregateStar`（`java-runtime-00be68da7736fcafb968`，first(*)/first(sb.*)/last(*)/last(sb.*)/window(*)/window(sb.*)/firstever(*)/lastever(*) 于 #length(2)，EventBean 渲染为 row 字段对象）、ord2 `ResultSetAggregateUnboundedStream`（`java-runtime-da477a833deb82cf0225`，无窗口 first/last 即 ever 语义，Go 侧用 FirstEver/LastEver）、ord20 `ResultSetAggregateLastMaxMixedOnSelect`（`java-runtime-edb70b7eb3a9cd4217e8`，keepall 命名窗口 + like 'A%' 过滤 insert-into + like 'B%' on-select 的 last(mw.intPrimitive)/max(mw.intPrimitive)——常量键 Literal(1) 分组 on-select 每触发一行聚合，last() 可降 max() 单调）。Java/Go 各 18 条 records、0 differences，零引擎改动（FirstEver[Event](EventValue[Event]()) 与常量键 SelectFromNamedWindowGroupBy 均直接可用）。manifest 更新为 672 cases / 670 implemented / 298 DV cases / 1103 DV runtime IDs / 3682 associations / referenced 3331 / unreferenced 805；`ResultSetAggregateFirstLastWindow.java` 全部 25 个 execution 完成关联。

> 最新补充：Draft 4.428（2026-09-16），ResultSetAggregateCountSum 文件收官（`case.resultset-aggregate-count-sum` 扩展至 12/13 executions + ord12 登记 intentionally-different）：`resultset-aggregate-count-sum` 链新增三 case，覆盖固定 Java `ResultSetAggregateCountSum.java` ord6 `ResultSetAggregateCountOneViewCompile`（`java-runtime-a9af0eeb0ed82e3ac363`，eplToModel 编译路径的 count-one-view 孪生——分组 count(*)/count(distinct volume)/count(volume) 于 #length(3)，含多行组过期投递）、ord9 `ResultSetAggregateCountDistinctGrouped`（`java-runtime-d52c75b9bcbbb9806320`，无窗口 group by symbol 的 count(distinct price)，新组 old 行带零值聚合）与 ord11 `ResultSetAggregateCountDistinctMultikeyWArray`（`java-runtime-8d52ec09b7ee23463eed`，count(distinct int[]) + count(distinct {intOne,intTwo}) 于 SupportEventWithManyArray#length(3)——数组内容相等与元组 distinct，过期后 c0 降 c1 升）。Java/Go 各 70 条 records、0 differences，零引擎改动（CountDistinct[any] 的 fallback 深打印键覆盖非 comparable 数组/元组输入，null 跳过语义与 Java 一致）。ord12 `ResultSetAggregateCountSumInvalid`（`java-runtime-43aefbafe9af60b5168b`）登记 `case.resultset-aggregate-count-sum-invalid` intentionally-different：五条 tryInvalidCompile 断言 null 字面量聚合的编译拒绝前缀，类型化 API 结构性无法表达 null 字面量聚合参数（沿用 local-group-invalid 先例）。manifest 更新为 671 cases / 669 implemented / 297 DV cases / 1100 DV runtime IDs / 3679 associations / referenced 3328 / unreferenced 808；`ResultSetAggregateCountSum.java` 全部 13 个 execution 完成关联。

> 最新补充：Draft 4.427（2026-09-16），ViewIntersect 文件收官差分链（新登记 `case.view-intersect-closure` 出生即 differential-verified，含一项共享核心修复）：新增链 `view-intersect`（6 cases / 57 步），覆盖固定 Java `ViewIntersect.java` 剩余六个未关联 execution：ord6 `ViewIntersectPattern`（`java-runtime-edb2cfcf01622829e41e`，pattern 生产+消费 + length(2) intersect）、ord14 `ViewIntersectGroupTimeUnique`（`java-runtime-bb2c317ddb0be1894e9b`，groupwin+time+unique+sort 复合）、ord15 `ViewIntersectSubselect`（`java-runtime-2c83921c540e86f9eee2`，subselect 成员判定 intersect）、ord16 `ViewIntersectFirstUniqueAndLengthOnDelete`（`java-runtime-f8abfbe10b8b71648894`，firstunique+firstlength 命名窗口 + on-delete）、ord17 `ViewIntersectTimeWinNamedWindow`（`java-runtime-dde215412ea6cd2bb327`）与 ord18 `ViewIntersectTimeWinNamedWindowDelete`（`java-runtime-57b98063fdef9954a959`，time+unique 命名窗口两变体）。Java/Go 各 57 条 records、0 differences。**共享核心修复**：命名窗口 composite insert 在 intersect 模式下把未被接纳的 incoming event 作为 removal 转发给每个 child（Java `IntersectAsymetricView` 向全部 view 推送 removalEvents）——此前 firstunique 丢弃的重复键事件仍占用 firstlength 槽位，导致后续插入被静默吞掉（E3@3 丢失）；修复后两侧逐记录一致。manifest 更新为 670 cases / 668 implemented / 297 DV cases / 1097 DV runtime IDs / 3675 associations / referenced 3324 / unreferenced 812；`ViewIntersect.java` 全部 19 个 execution 完成关联（13 个既有 + 6 个本单元），`view.window-core` remaining 收窄至 reclaim hints/iterator-fragment metadata/共享 trace 面。

> 最新补充：Draft 4.418（2026-09-15），resultset 本地分组收官差分链（文件全覆盖 + 两个登记为 intentionally-different 的编译面 execution，含共享核心 plain grouped on-select 面）：新增 `case.resultset-querytype-local-group-closure`（链 id `resultset-querytype-local-group-closure`），覆盖固定 Java `ResultSetQueryTypeLocalGroupBy.java` ordinals 22 `ResultSetLocalGroupedOnSelect`（`java-runtime-efa4ac181b956105b4bb`）与 25 `ResultSetLocalUngroupedAggAdditionalAndPlugin`（`java-runtime-27dff810bb25f959acbc`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）；ords 15/16（Planning/Invalid）登记为 intentionally-different（见下）。Java/Go 各 6 条 records、0 differences（2 cases / 13 个 scenario 步骤，无虚拟时间）：(1) ord 22 keep-all 命名窗口 + insert-into + plain grouped on-select（`on SupportBean_S0 select theString, sum(intPrimitive) as c0, sum(intPrimitive, group_by:()) as c1 from MyWindow group by theString`）——两次触发各交付 3 行（任意序，双侧规范 theString 序）：{E1,40,150}{E2,70,150}{E3,40,150} 与 {E1,100,210}{E2,70,210}{E3,40,210}，`group_by:()` 的 sum 为语句级（全部 taken 行求和，每组行同值 150/210）；(2) ord 25 无分组 row-per-event 聚合 14 列：countever(*, filter)/countever(*)、concatstring、sc（插入序标量集合，c6 按键 [10]/[20]/[10,-1]/[20,30]、c7 语句级 [10]/[10,20]/[10,20,-1]/[10,20,-1,30]）、leaving、rate(3)（引擎时间零两侧恒 null）、nth(…,1)，逐值钉定。**共享核心（4.418）**：(a) 新公共 API `TriggerStream.SelectFromNamedWindowGroupBy`（plain grouped on-select，仅明细层一行/组；rollup 行为不变）；(b) grouped on-select 求值新增 `AllGroup: matched...` 绑定，使零键 LocalGroupBy（group_by:()）按语句级全部 taken 行求值而普通聚合保持按组；(c) rollup + LocalGroupBy 组合在 Build 拒绝，文案对齐 Java 原句 "Roll-up and group-by parameters cannot be combined"（trigger.go on-select 形态 + plan.go:5036/5040 措辞统一）；引擎回归 `TestOnSelectGroupedNamedWindow`/`TestRollupLocalGroupRejected`。运行侧表示选择：concatstring 与 sc() 以有状态类型化插件聚合镜像（Java 侧为 configuration 插件；插件状态回放为 delta 协议——Leave 旧域/Enter 新域——故每列独立实例 + 对称 Leave，pinned 无界流不退役事件，可观测语义即 Java 的 ever 集合）。ords 15/16 登记：Planning 为编译期 plan-forge 内省（@Hook INTERNAL_AGGLOCALLEVEL），无可观测事件行为、无 Go plan-hook API（沿用 expr-filter-optimizable plan-hook 先例）；Invalid 十条编译拒绝中 Go 仅 rollup+local-group 一条有面（Java 精确文案已钉定），其余九条因类型化 API 结构性不存在（表列为类型化声明、命名参数为包装器参数等）。严格 loader 固定 Java 元数据、13 步 case/send 顺序与载荷取值（载荷对象严格字段校验）、15 种 trace 变异与 15 种 raw 畸变全部拒绝。manifest 更新为 666 cases、664 implemented、290 个 differential-verified case、27 个 intentionally-different case、1057 个 differential runtime IDs、3655 条 associations（referenced 3309、unreferenced 827——四个 runtime id 中三个已由旧 umbrella case 关联）；capability `resultset.aggregate-local-group` remaining 清空，ResultSetQueryTypeLocalGroupBy.java 全部 28 个 executions 覆盖完毕。

> 最新补充：Draft 4.459（2026-09-18），`context.partition` 扩展 `case.context-hash-segmented`（出生即 DV），对照固定 Java `ContextHashSegmented.java` ordinals 1/5/6（Filter `java-runtime-514d6623af4af2d18516`、BySingleRowFunc `java-runtime-7f880063fe0f23046c59`、ScoringUseCase `java-runtime-74e1f67a0acf62775846`；共享 static `java-0564864de64ece6e7772`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。ord 1 用语句级 Filter 模拟 context 级 `from SupportBean(intPrimitive > 10)`（preallocate 下可观测等价：被过滤事件不产出、不触碰 lastevent）；ord 5 用 `Func1+EventValue` 复现 plug-in SRF `myHash(*)`，`HashAlgorithmJavaHashCode` 对 int 为恒等使 bucket=value%4；ord 6 pin DEFAULT/MAP 表示：两类型共享 crc32(userId) hash context + context-bound unique(productId,keyword) named window + grouped aggregate insert-into 重分区 + 永不触发的 on-delete。ord 8 ContextHashInvalid 暂缓：六个编译错误检查需要 Go 不具备的命名 hash 函数注册表、context 级过滤表达式和 named-window-in-partition-criteria 面。Java/Go 各 31 条 records、0 differences。覆盖 7/9 executions（ord 0/4 已由既有 case 覆盖）。

> 最新补充：Draft 4.458（2026-09-18），`context.partition` 扩展 `case.context-admin-listen`（出生即 DV），对照固定 Java `ContextAdminListen.java` ordinals 2/3/4/6（Category `java-runtime-2021f021c6e12684fb81`、Nested `java-runtime-4b1466f2815a381815b2`、AddRemoveListener `java-runtime-410d2c5d3daf011b6c7b` OBSERVEROPS+RUNTIMEOPS、MultipleStatements `java-runtime-6dd25578221d74ff6660`；共享 static `java-3a026095a61c4060c91b`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。ord 5 ContextAdminPartitionAddRemoveListener 暂缓：scenario B 需要 nested initiated-parent context（NewNestedContext 拒绝 ContextInitiatedTerminated parent）。共享核心修复：`ContextStateEvent.RuntimeURI` 填充；`activated` 在 eager 分区物化后发出；teardown 时 flat 非-hash 分区不再发 `deallocated`（Esper 单层非-hash controller 的 terminateChildContexts=false 跳过分区终止），lifecycle 终止仍经 `releaseContextPartitionKindLocked` 通知。Java/Go 各 37 条 records、0 differences。覆盖 4/7 executions（ords 0/1 已由 case.context-partitioning 覆盖）。

> 最新补充：Draft 4.457（2026-09-18），`context.partition` 扩展 `case.context-category`（出生即 DV），对照固定 Java `ContextCategory.java` 全部 9 个 execution（SceneOne `java-runtime-edb899dd318dc4e8711e`、SceneTwo `java-runtime-440efde2e13b0063a968`、WContextProps `java-runtime-cbe8b6c2ba86887f6807`、BooleanExprFilter `java-runtime-ad038edc0893e86eba46`、ContextPartitionSelection `java-runtime-fe484daf28031390497b`、SingleCategorySODAPrior `java-runtime-96843edb4ce366e1d74e`、Invalid `java-runtime-51b59dd76ca097972003`、DeclaredExpr{isAlias=true} `java-runtime-629da2acd413b0688d68`、DeclaredExpr{isAlias=false} `java-runtime-76b9f0c6ca0a53be2ab0`；共享 static `java-2275d4c280d0acadad30`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。共享核心修复：category context 从首个匹配改为向所有匹配 category 分区扇出（`partitionsForEvent` + `processContextCategoryFanOut`，嵌套 category 分区惰性创建）；已知残留：context-bound named-window insert 仍只路由到首个匹配分区（未覆盖）。Java/Go 各 75 条 records、0 differences。覆盖 9/9 executions。

> 最新补充：Draft 4.456（2026-09-18），`context.partition` 扩展 `case.context-variables`（出生即 DV），对照固定 Java `ContextVariables.java` 全部 5 个 execution（SegmentedByKey `java-runtime-700973f399356686a11e`、Overlapping `java-runtime-821c37bbc3256af33468`、IterateAndListen `java-runtime-ea85235554e99c41e863`、GetSetAPI `java-runtime-4d98fe6487ed39e7d1de` RUNTIMEOPS、Invalid `java-runtime-c7b860b3ff30ed78f3f5`；共享 static `java-2443c804eb31da7902e7`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。两处共享核心修复：overlapping context 分区键改为按发起事件 identity + 每语句 ordinal 派生（原全局计数器使同一 start 事件在每个语句落到不同分区，on-set 写入永远到不了关联 select 读取的分区）；`visitQueryExpressions` 覆盖 output CountExpr/IntervalExpr，context 变量在 output-rate 子句中触发作用域校验。Java/Go 各 53 条 records、0 differences；`context X create variable` 为 env 级注册 fixture + VariableChangeListener IR pair（approved difference），ord-4 钉 Go 错误措辞、Java 精确前缀在 oracle 内断言。

> 最新补充：Draft 4.455（2026-09-18），`epl.variable-onset` 扩展 `case.epl-variables-event-typed`（出生即 DV），对照固定 Java `EPLVariablesEventTyped.java` 全部 6 个 execution（SceneOne `java-runtime-0ccc8cc9831c8681b7b1`、SceneTwo `java-runtime-683d12f7c34c448319da`、Config `java-runtime-cc00e385af1428a74014`、SetProp `java-runtime-067180db58b5432f57dc`、Invalid `java-runtime-400881e0dc41c6d3e4e4`、CreateSchema `java-runtime-0794ac05da19ce34dbaf`；共享 static `java-0d1b65aff5dcb7751e5b`；flags SERDEREQUIRED/RUNTIMEOPS/INVALIDITY；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。新增共享核心 `SetVariablePropExpr`（copy-on-write 属性写入 + `name.prop` 输出列携带已求值赋值，空接收者 no-op 仍发列值；snapshotQuery 按列对齐）。Java/Go 各 61 条 records、0 differences；ord 4 钉 type-mismatch 类别、ord 5 EVENTTYPE 依赖边 oracle 内断言（approved difference）。

> 最新补充：Draft 4.454（2026-09-18），`epl.variable-onset` 扩展 `case.epl-variables-create` differential-verified 场景，对照固定 Java `EPLVariablesCreate.java` ordinals 0/1/2/3/5/6（`java-runtime-67e6441b95a4ca30917b`/`74285cbcf0d7aeabf02f`/`06466d91981a6c410b39`/`75cb3e01ac92790b5194`/`71775e7db7d1ffd875ba` RUNTIMEOPS/`4ac4b7b111d10a20f74d`；ord 4 `java-runtime-b2c51a16c93eaf0cb126` 仅编译失败登记 implemented；共享 inventory `java-6a445f399d99f5d4785d`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）：Java/Go 各 77 条 records、0 differences。module-scoped 变量建模 create-variable 生命周期；VariableChangeListener 承载 IR pair；顺序 on-set 赋值与 redeploy-reset 语义对齐。manifest 更新为 696 cases、694 implemented、322 DV、1217 DV runtime IDs、3767 associations（referenced 3403、unreferenced 733）；capability 121 个（39 DV）。

> 最新补充：Draft 4.453（2026-09-18），`expr.filter-expressions` 扩展 `case.expr-filter-in-and-between` differential-verified 场景，对照固定 Java `ExprFilterInAndBetween.java` ordinals 0/5/6/7/8（`java-runtime-0d06d25e978384a0b008`/`d16fe1fd7693643a5001`/`9245458815076f0baee3`/`a3336b696b1ae9b2c831`/`ed506bfbf3734b4fff03`；ord 4 `java-runtime-9472ab3c76f8e25de952` 仅 JVM index-planning 编译拒绝，登记 implemented；共享 static `java-17cece2bf9c2df0b27f1`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）：Java/Go 各 101 条 records、0 differences。引擎修复 in-list 常量逐 slot 投递（multiMatch 覆盖重复字面量/动态候选，保留无状态 fast path）。manifest 更新为 695 cases、693 implemented、321 DV、1211 DV runtime IDs、3760 associations（referenced 3396、unreferenced 740）；capability 121 个（39 DV）。

> 最新补充：Draft 4.452（2026-09-18），`infra.namedwindow.views` 扩展 `case.infra-namedwindow-consumer` differential-verified 场景，对照固定 Java `InfraNamedWindowConsumer.java` ordinals 0-2 全部三个 execution（`java-runtime-c2d6b88fc4d77c643aab`/`a707367e42b2736c2fcb`/`229ba7f65962eb4594a1` EXCLUDEWHENINSTRUMENTED；共享 static `java-04f7e86e470bf275affa`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）：Java/Go 各 17 条 records、0 differences。场景覆盖 keepall/length 聚合 consumer 与 expr_batch 大批次释放；引擎修复 expression-batch 触发器求值与 history/queue 复制的 O(n²) 分配（10k flush 437ms）。manifest 更新为 694 cases、692 implemented、320 DV、1206 DV runtime IDs、3754 associations（referenced 3390、unreferenced 746）；capability 121 个（39 DV）。

> 最新补充：Draft 4.451（2026-09-18），`infra.namedwindow.views` 扩展 `case.infra-namedwindow-processing-order` differential-verified 场景，对照固定 Java `InfraNamedWindowProcessingOrder.java` ordinals 0-6 全部七个 execution（`InfraDispatchBackQueue` 六变体 `java-runtime-d103aeca629813a82acf`/`0c77245232ebd6281a47`/`cef00c40d92d73111b6e`/`21dd5ef783d585d85dfd`/`59a4da55e98ca67b95c2`/`99f94152e6a94e3e60bc` EXCLUDEWHENINSTRUMENTED、`InfraOrderedDeleteAndSelect` `java-runtime-c56598034a18ee372891`；共享 static `java-78aff9650bba15e78056`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）：Java/Go 各 68 条 records、0 differences、零引擎改动。场景覆盖 back-queue insert-into/on-update 链的原子 IR 对与声明序 on-trigger 的 live-lookup 语义；契约修正：trigger event 缩减为模块引用属性、注解按语句携带各自 provided 类。manifest 更新为 693 cases、691 implemented、319 DV、1203 DV runtime IDs、3751 associations（referenced 3387、unreferenced 749）；capability 121 个（39 DV）。

> 最新补充：Draft 4.450（2026-09-18），`infra.namedwindow.views` 扩展 `case.infra-namedwindow-on-delete-indexes` differential-verified 场景，对照固定 Java `InfraNamedWindowOnDelete.java` ordinals 1-4（`InfraStaggeredNamedWindow` `java-runtime-0dcb2b72f505c7931a45` STATICHOOK、`InfraCoercionKeyMultiPropIndexes` `java-runtime-c4c336036fdd92d803f7`、`InfraCoercionRangeMultiPropIndexes` `java-runtime-a58ae70ca908579af50a`、`InfraCoercionKeyAndRangeMultiPropIndexes` `java-runtime-2d6d018e91b5664f3734`；共享 static `java-06bf0eb71230b3119293`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）：Java/Go 各 118 条 records、0 differences。场景覆盖跨窗口 staggered on-delete 直接子语句次序与三类多属性隐式索引计数（含 between/not-between 不建索引、undeploy 归零）。共享核心修复：trigger 目标窗口惰性物化（`ensureNamedWindowLockedInModule`）+ consumer wave 内直接派发先于 mutation trigger 自身批次。manifest 更新为 692 cases、690 implemented、318 DV、1196 DV runtime IDs、3744 associations（referenced 3380、unreferenced 756）；capability 121 个（39 DV）。

> 最新补充：Draft 4.449（2026-09-18），`infra.namedwindow.views` 的 `infra-namedwindow-on-delete-silent` differential-verified 场景，对照固定 Java `InfraNamedWindowOnDelete.java` ordinals 5/6 `InfraNamedWindowSilentDeleteOnDelete`（`java-runtime-38dd6f716920e46075c7`）与 `InfraNamedWindowSilentDeleteOnDeleteMany`（`java-runtime-6bce45980de6b3f7aa9f`；共享 static `java-06bf0eb71230b3119293`；无 flags；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）：Java/Go 各 32 条 records、0 differences。场景覆盖 `@hint('silent_delete')` 语义：length(2) 窗口按 `p00 = theString` 删除与 groupwin(theString)#length(2) 窗口 delete-all——create 直接子语句只见 insert 与 length-2 过期 IR 对、从不见被静默删除的行，on-delete 输出仍以 new data 投递被删行，count 尾视图观察全部增量。**共享核心修复**：`HintSilentDelete` 此前在 statement_metadata.go 已定义但从未消费；`queueNamedWindowDeltaLocked` 现在在 delta owner 携带该 hint 时对直接 named-window 派发剥离 `delta.Old`（对应 Java `OnExprViewNamedWindowDelete.clearDeliveriesRemoveStream`）。Go 侧 Java 'create' 语句自身 listener 映射为 `FromNamedWindow(...).CreateNamedWindowQuery(WithOldStream())`；每 case 独立 env+engine 对应 undeployAll 边界。同文件 ord 1（STATICHOOK）与 ords 2–4（assertIndexCount 需隐式索引推断）仍 deferred。

> 最新补充：Draft 4.448（2026-09-17），`trigger.table-named-window` 的 `infra-nwtable-on-update` differential-verified 场景，对照固定 Java `InfraNWTableOnUpdate.java` ordinals 0/1/4/5/6/7 六个未关联 execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime IDs `java-runtime-f8090e148364d7b15116`/`922adbe6628b17d5ec61`/`046926326cad5f0172cd`/`702d919aaadcbb188467`/`5cc56f78a52d6a452948`/`b23b2fbc3cc233237ac9`；static IDs `java-bc54a78188b2249a0a9f`/`java-c67318a7541b42eb7940`/`java-9ab08590ab6533e21139`；无 flags）：Java/Go 各 42 条 records、0 differences。场景覆盖 where 子句更新（trigger id vs 行 intPrimitive 作用域、new=更新后行/old=更新前行 IR 对）、自关联子查询赋值读取同一 infra 的更新前值（ESPER-507）、group-by-int-array 多键聚合子查询多行结果赋 null。**共享核心修复**：(1) named-window updateWhere 在窗口锁外求值 predicate+updater，自引用子查询不再死锁；(2) keepall on-update 为 remove+reinsert，更新行移至迭代序尾部；(3) FAF mutation 延迟 named-window consumer 投递至下一工作边界（waveKey 按边界隔离 wave，send/insert/advance-time 入口保留 pending delta）。新登记出生即 DV `case.infra-nwtable-on-update` 映射 `trigger.table-named-window`（goRefs +3、DV IDs +6）。manifest：690 cases、688 implemented、316 DV cases、1190 DV runtime IDs、associations 3738（referenced 3374、unreferenced 762）。

> 最新补充：Draft 4.447（2026-09-17），`trigger.table-named-window` 的 `infra-nwtable-on-delete` differential-verified 场景，对照固定 Java `InfraNWTableOnDelete.java` ordinals 0-5 全部六个 execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime IDs `java-runtime-6d190be4d9e8b76b11f3`/`992fcd3db7f15b8254cd`/`2ab09353c939b15ee52e`/`e795679c4403d309f805`/`11809b3f1d90b37d52b8`/`b7a9b1b53c0009e7d347`；static IDs `java-eb28fd3f17513a399ac9`/`java-132a1f9e7f2c434a19e9`/`java-e3aeca986a0cfee60dc7`；无 flags）：Java/Go 各 106 条 records、0 differences。场景覆盖 where 子句删除（trigger 事件 id vs 行 a/b 作用域、concat 与范围谓词）、pattern 触发无条件 delete-all（Go 经 insert-into + OnRecord trigger 物化）、事件触发 delete-all 以 new data 投递被删行。**共享核心修复**：(1) mutation-trigger 延迟派发覆盖 `triggerDeleteAllTable`，并在 send 循环与 routed-event 循环按 statement 部署序合并延迟 trigger 批次与 consumer-wave 派发（Java 按部署序交错投递）；(2) FAF 聚合查询标记 delta forced，空源上 ungrouped 聚合返回行（count(*)=0）。新登记出生即 DV `case.infra-nwtable-on-delete` 映射 `trigger.table-named-window`（goRefs +1、DV IDs +6）。manifest：689 cases、687 implemented、315 DV cases、1184 DV runtime IDs、associations 3732（referenced 3368、unreferenced 768）。

> 最新补充：Draft 4.446（2026-09-17），`join.basic` 的 `epl-other-plan-in-keyword` differential-verified 场景，对照固定 Java `EPLOtherPlanInKeywordQuery.java` ordinals 0-8 全部九个 execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime IDs `java-runtime-cf29d6b68db89aa0b8a8`/`c982eaac78b6fd1b45ae`/`c5daa1f63378ab985abd`/`dd7da2447d43068347ff`/`d1bd81d5833b3bb70b20`/`b537100e9c81481cfe31`/`8607ff3a12954cbb6373`/`fb1e28fea28321f53bd1`/`36887b719198c76fca9d`；flags `INVALIDITY`）：Java/Go 各 114 条 records、0 differences、零引擎改动。场景覆盖 not-in 仅部署、multi-idx/single-idx unidirectional join、keepall 命名窗口与 primary-key table 的 on-trigger 查询面、multirow 子查询 selectFrom 集合（含 coercion-absence 仅部署探针）、常量表达式 in-join、以及 plan-3stream（4 轮）/plan-2stream（13 轮）仅部署编译形态；每条语句携带 byte-exact INTERNAL_QUERY_PLAN @Hook 注解（编译期 no-op 镜像）。新登记出生即 DV `case.epl-other-plan-in-keyword` 映射 `join.basic`（goRefs +1、DV IDs +9）。manifest：688 cases、686 implemented、314 DV cases、1178 DV runtime IDs、associations 3726（referenced 3362、unreferenced 774）。

> 最新补充：Draft 4.445（2026-09-17），新增 `epl.other.select-expr` capability 与 `case.epl-other-select-expr` differential-verified 场景，对照固定 Java `EPLOtherSelectExpr.java` ordinals 0-5 全部六个 execution（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；shared static/inventory ID `java-02b24f8edc7d5c4f5cba`；runtime IDs `java-runtime-7222e4dfd73a239bf53c`/`a1605a2ba0d017fa91f1`/`bc47c8b86afd9b19f5c6`/`446476d182df93abd788`/`d01d8e953908acc44955`/`5371694959860be64ff6`；无 flags）：Java/Go 各 23 条 records、0 differences。场景覆盖表达式优先级自动列名（`3*2+1`/`3*(2+1)`）、insert-into 公开流的 graph select 嵌套属性路径、31 个关键字字段名与 `count(*)` 关键字别名、12 种转义字符串变体（引号/unicode 转义/语句名与描述）、`deployed`+`types` 记录的 event-type schema 内省、以及 `#length(3)` 窗口后过滤的 window stats。Go 侧要点：EPL 自动列名用显式 `Alias()` 对齐；plain `Select` 中的 `count(*)` 为非聚合（恒 0），keywords 用例改走 `Aggregate`；graph select 用 `Property()` 链 + `RegisterMap`/`WithNestedPropertySchema` 的 MyStream schema；监听器仅挂 `s0` 部署（producer insert 语句不挂，匹配 Java 按语句挂监听）。manifest 更新：687 cases / 685 implemented / 313 DV cases / 1169 DV runtime IDs / 3717 associations；capability 121 个（39 DV）。
> 最新补充：Draft 4.426（2026-09-16），MoreWindows 差分闭环——`view.basic-windows` capability remaining 全部清空：`view-parameterized-by-context` 链扩展 `more-windows` case，覆盖固定 Java `ViewParameterizedByContext.java` ordinal 2 `ViewParameterizedByContextMoreWindows`（`java-runtime-6d60bed2a335972423d2`，此前未与任何 case 关联）。该 execution 在十二种窗口种类上逐个回放 context 参数化尺寸（length_batch/time/ext_timed/time_batch/ext_timed_batch/time_length_batch/time_accum/firstlength/firsttime/sort/rank/time_order，全部由 context.miewl.intSize 定尺寸），Java 侧无监听断言——钉定可观测面为每 kind 一条 deployed 标记（12 条），两侧编译、部署、分区初始化逐一成功。Java/Go 各 80 条 records、0 differences，引擎侧零新增数据路径改动（4.425 的表达式尺寸解析机制直接覆盖全部种类；本单元补齐十种窗口种类的表达式尺寸 API 面：LengthBatchExpr/TimeBatchExpr/ExternallyTimedExpr/ExternallyTimedBatchExpr/TimeLengthBatchExpr/TimeAccumExpr/FirstLengthExpr/FirstTimeExpr/SortWindowExpr/RankWindowExpr/TimeOrderExpr，validate() 接受表达式参数，尺寸/周期消费点改读状态解析值——静态规格解析回退到声明值，全部下游读取单一来源化）。两条 trace 变异全部拒绝（标记形状漂移、标记丢失）。manifest：case.view-parameterized-by-context +1 runtime ID（3/3 executions 全覆盖）、669 cases、296 DV cases、1091 DV runtime IDs、associations 3669（referenced 3318、unreferenced 818）；capability `view.basic-windows` remaining 清空、DV runtime IDs 45→46。

> 最新补充：Draft 4.425（2026-09-16），ViewParameterizedByContext 上下文参数化视图尺寸差分（新登记 `case.view-parameterized-by-context` 出生即 differential-verified，含共享核心 LengthWindowExpr）：新增链 `view-parameterized-by-context`（2 cases / 76 步），覆盖固定 Java `ViewParameterizedByContext.java` ordinal 0 `ViewParameterizedByContextLengthWindow`（`java-runtime-71761cb17e7d22393efc`）与 ordinal 1 `ViewParameterizedByContextDocSample`（`java-runtime-de2ad7eb74b76b16867a`）。**共享核心（LengthWindowExpr）**：`LengthWindowSpec` 新增 `SizeExpr`，新公共 API `LengthWindowExpr(expr)`——窗口尺寸表达式在每个上下文分区的首个事件时以分区变量（含 ContextInitiatingEvent() 等上下文属性）一次性求值（resolveLengthWindowSize，TimeWindowExpr/windowExprDurations 先例）；`terminated after 1 year` 以 CreateOverlappingPatternTerminatedContext + TimerIntervalCalendar(1 年) 表达（每个启动事件一个分区）。Java/Go 各 68 条 records、0 differences：三分区 P1=2/P2=4/P3=3 的 count(*) 逐事件行、容量封顶、{P1:0,P2:1,P3:0}→{P1:2,P2:4,P3:3} 迭代器向量（mode any 双侧规范排序）逐值一致。**伴随的 ungrouped 聚合旧行修正**：上下文分区语句的 ungrouped irstream 聚合不再配对 null-prior 旧行（plan.query.contextName 门控），ungrouped 混合裸列选择不再投递 previous-as-old 旧行（ungroupedMixedRowPerEvent 门控，分组形状不受影响——TestGroupedAggregateAndHaving 回归钉定）。MoreWindows execution（十二种上下文参数化窗口）暂缺：表达式尺寸变体目前仅 TimeWindowExpr/LengthWindowExpr，登记为 capability remaining。四条 trace 变异全部拒绝。manifest：新 case +1（668→669 cases、667 implemented、296 DV cases、1090 DV runtime IDs、associations 3668、referenced 3317、unreferenced 819）；capability `view.basic-windows` DV runtime IDs 43→45，remaining 更名 MoreWindows 缺口。

> 最新补充：Draft 4.424（2026-09-16），ViewTimeBatch suite 差分切片（新登记 `case.view-timebatch-suite` 出生即 differential-verified，`case.view-timebatch-basic` 部分差分升级 +5，零引擎改动）：新增链 `view-time-batch`（8 cases / 120 步），覆盖固定 Java `ViewTimeBatch.java` 八个确定性 execution：SceneOne（5a110975a6ab7750e433，time_batch(1 sec) 2500/3500 新旧行对/4500 old-only/E6/E7 节奏，2499 静默）、10Sec（77fc0819a391d361a0c8，部分批迭代器检查点 + 11000/21000/31000/41000 锚对）、StartEagerForceUpdateSceneTwo（cb1e1193f3ace6152a25，START_EAGER, FORCE_UPDATE 提示的空批投递对记录协议不可见）、MonthScoped（1a1b45f9c756465a0191，time_batch(1 month) 历法边界减一毫秒静默）、StartEagerForceUpdate（736d2f58158461c2c777，无逗号空格变体）、Multirow（9ae5cbb72669d84a7f7f）、MultiBatch（a3c81710a21ac12db682，连续满批 new+old 对）、NoRefPoint（5e01b6f92f8d8bc5851e，部署锚定 600000 deadline 与 null-theString 裸 bean）。Java/Go 各 35 条 records、0 differences，Go 的 TimeBatch/TimeBatchForce/TimeBatchCalendar 已与 Java 一致。两条排除项：ViewTimeBatchLonger（28a898700ff4cba95f14，未播种 Random 调度不可确定性重放，留在 basic case 为 implemented-not-DV）与 ViewTimeBatchRefPoint（7d3c38fa4d2477a0be87，time_batch 参考点参数暂无 Go API——开放覆盖缺口）。六条 trace 变异全部拒绝。manifest：新 case 登记 +1，basic case 升级 differential-verified（部分：6 个 ViewTimeBatch IDs 中 5 个 DV，其余 3 个 ViewFirstTime DV IDs 由 4.422 保留）；compat 校验器重算 summary——668 cases、666 implemented、295 DV cases、1088 DV runtime IDs、associations 3666（referenced 3315、unreferenced 821）；capability `view.basic-windows` remaining 移除 ViewTimeBatch suite、DV runtime IDs 35→43。

> 最新补充：Draft 4.423（2026-09-16），ViewLengthWinWPropertyDetail 差分闭环（新登记 `case.view-lengthwin-property-detail`，出生即 differential-verified，零引擎改动）：新增链 `view-length-win-property-detail`（单 case `w-property-detail`），覆盖固定 Java `ViewLengthWin.java` ordinal 2 `ViewLengthWinWPropertyDetail`（`java-runtime-9b050d42ae8cdfd3fa0d`，此前未与任何 case 关联）。该 execution 在 `#length(3)` 窗口上投影 mapped('keyOne')/indexed[1]/nested.nestedNested.nestedNestedValue/mapProperty/arrayProperty[0] 五列并以三属性 where 门控：默认 bean（indexed [1,2]）被接纳、setIndexed(1,Integer.MIN_VALUE) 重发被过滤、恢复后重发被接纳，Java/Go 各 2 条 records、0 differences。Go 侧以 `Property[T](EventValue[Event](), path)` 根级路径表达全部五列与过滤条件（event-map-core 先例），mapped/nested/mapProperty 以 map 字段承载；oracle 侧镜像 pinned bean 的键控访问器形态——`java.util.Map` 型 nested 成员会被 Esper 解析为 MAPPED 属性而拒绝点号导航，故以本地 POJO（getMapped(String)/getIndexed(int) 键控重载 + LocalNested/LocalNestedNested 片段）建模，map 列以裸排序对象归一化（时钟钉定 epoch）。四条 trace 变异全部拒绝（过滤重发行出现、indexed/mapped 列漂移、map 形状漂移）。manifest：新 case 登记 +1（667 cases、665 implemented、294 DV cases、1080 DV runtime IDs、associations 3658、referenced 3312、unreferenced 824）；capability `view.basic-windows` remaining 移除 ViewLengthWinWPropertyDetail、DV runtime IDs 34→35。

> 最新补充：Draft 4.422（2026-09-16），ViewFirstTime suite 差分切片（`case.view-timebatch-basic` 部分差分升级，零引擎改动）：新增链 `view-first-time`（3 cases / 38 步），覆盖固定 Java `ViewFirstTime.java` 全部三个 execution：Simple（`java-runtime-9733dfdbee02d899bb23`，`firsttime(1 month)` 于 2002-02-01T09:00:00Z 锚定，E2 在 deadline 前一毫秒仍被接纳、deadline tick 发送的 E3 永不被接纳、迭代器保持 [E1,E2]）、SceneOne（`java-runtime-b1061971825b14664ff2`，c0/c1 投影 + deadline 保留迭代器 + E3/E4 永不被接纳）、SceneTwo（`java-runtime-ac89ffce844dc00114f7`，irstream 通配 + **静默 deadline 过期**——1500/1600/2000 推进零投递，Java assertListenerNotInvoked 钉定）。Java/Go 各 13 条 records、0 differences；Go 的 FirstTime 机制（接纳窗口、静默过期、迭代器保留）已与 Java 一致，零引擎改动。注意：既有 Go 单元测试的 Simple 时间线是改写版（E3 在 deadline 前发送），链上钉定的是 Java 逐字时间线。四条 trace 变异全部拒绝（deadline 接纳泄漏、保留行漂移、预接纳快照泄漏、过期行出现）。manifest：`case.view-timebatch-basic` 状态升级 differential-verified（部分差分：3/38 runtime IDs DV，其余 35 个留在各自 suite 的 implemented-not-DV 状态，先例 4.242-4.418 的 view-length-batch）；capability `view.basic-windows` remaining 移除 ViewFirstTime suite、DV runtime IDs 31→34；summary 666 cases、293 DV cases、1079 DV runtime IDs。

> 最新补充：Draft 4.421（2026-09-16），view/time-win 场景对差分闭环（`case.view-timewindow-scenes` 从 implemented 升级 differential-verified，零引擎改动）：`view-time-win` 链在其头部扩展 `scene-one`（`java-runtime-25dfb49811c974a34e5d`）与 `scene-two`（`java-runtime-a8007c80ae5756bb4ddb`）两个 case（Java 执行序 0/1），链增至 23 cases / 103 records、Java/Go 0 differences：scene-one 钉定 `#time(10 sec)` 十秒生命周期——逐事件 new 行、old [E1]/[E2]/[E3] 于 11000/12000/13000、11000 前静默（10999 迭代器不变）、22000 一次投递双行 old [E4,E5]、十二处迭代器检查点；scene-two 钉定 11000 的组过期（E1..E4 同批 old、E5 留至 12000 未观察）与六处迭代器检查点。两 execution 共用同一 Go 构建（`TimeWindow(10*time.Second)` irstream * + WithOldStream）；oracle 侧两臂逐字转写 `@Name('s0') select irstream * from SupportBean#time(10 sec)`。变异家族随记录布局整体 +33 平移并新增四条 scene 变异（过期 old 行漂移、过期后快照漂移、组过期行漂移、记录丢失），全部拒绝。manifest 更新为 666 cases、292 个 differential-verified case、1076 个 differential runtime IDs；capability `view.basic-windows` DV runtime IDs 29→31（31/31 javaRefs 全部 DV 覆盖），ViewTimeWin 范围 remaining 清空——ViewTimeWin.java 全部 17 个 executions 至此获得差分验证（capability 其余 suite remaining 不变）。

> 最新补充：Draft 4.420（2026-09-16），view/time-win 十五个 execution 差分闭环（`case.inventory.view-time-win` 从 implemented 升级 differential-verified，含两项共享核心修复）：新增链 `view-time-win`（场景 `testdata/parity/view-time-win.json`，21 个 per-spec case / 194 步，全部虚拟时间 advance-time 协议），覆盖固定 Java `ViewTimeWin.java` ordinals 2-16 全部 15 个 execution：just-select-star（63f096fecc7fa8382cd1）、sum（9b1ddabf0088cb326211）、sum-group-by（6edb156e4a10bcba9784）、sum-w-filter（9239909fee9398909be1）、month-scoped（71fe6ccae381fc1a0843）、w-prev（aa228ddbee38aa48a796）、prepared-stmt（fc4ed4ace0c09a9154da）、variable-stmt（323ea34bd1e1c38e3895）、time-period（87e95838609b5cc88869）、variable-time-period（469f129b7fa2da1d0b14）、time-period-params ×7（共享 java-runtime-19a1a7c856e9567f7aac；七种拼写各自单独成 case，因为 Java tryTimeWindow 每轮 advanceTime(0) 复位而 Go 时钟不可倒退）、flip-timer ×4（2deb887a/d563c454/cc6e1ac3/c9c0f3b2，含历法月边界与大起点时间）。Java/Go 各 70 条 records、0 differences。**共享核心修复一（无裸列 ungrouped 聚合过期行）**：纯移除批（时间到期、无新事件）此前对 ungrouped 聚合一律补发当前状态新行——Java 只在 select 完全聚合时才如此（EPLInsertInto 的 minD/maxD 在 61 s 到期断言更新），混合裸列 select（ViewTimeSum 的 symbol,volume,sum(price)）的 35 s 到期无任何投递；`aggregateBatch` 的 emitNew 分支现以新谓词 `aggregateDefinitionHasBareSelections` 门控。**共享核心修复二（同刻定时回调逆部署序）**：三个 JDK 17 探针（存档于 tools/java-oracle/probes/，覆盖视图过期与 pattern timer:at 两种形态）钉定 Java 对同刻 schedule 回调按语句槽位使后部署语句先投递（prepared-stmt/variable-stmt/time-period/variable-time-period 四 case 在 5000 ms 同时到期时 Java 恒为 s1(E2) 先于 s0(E1)，与语句名无关）；Go `Engine.advanceTime` 的语句过期循环现按等优先级组反转（reverseStatementGroups），事件驱动派发保持注册序。运行侧：变量以 TIME_WIN_ONE int 4→3 / TIME_WIN_TWO double 4000→0.05 的 set-variable 步驱动部署期快照；prepared-stmt 以 DeployWithParameters 双部署；七拼写统一解析为 30000 s。六种 trace 变异全部拒绝。manifest 更新为 666 cases、291 个 differential-verified case、1074 个 differential runtime IDs；capability `view.basic-windows` DV runtime IDs 14→29，remaining 收窄为 SceneOne/SceneTwo 的差分面（已由 case.view-timewindow-scenes 覆盖 implemented）。

> 最新补充：Draft 4.419（2026-09-16），view/length-batch 文件收官差分链（4.242 遗留两个 prev-on-batch execution 闭合，含共享核心 prev 族批窗口修复）：既有 `case.view-length-batch`（链 id `view-length-batch`）的差分覆盖从 7 个 runtime 扩展到 9 个（场景 javaRuntimes 列表 8→10），覆盖固定 Java `ViewLengthBatch.java` ordinal 5 `ViewLengthBatchNormal{runType=VIEW}`（`java-runtime-d22e3122427d1dd8ceb0`）与 ordinal 6 `ViewLengthBatchPrev`（`java-runtime-03b48f31fe26fedf4d4b`；两 execution static 共用 `java-15a175599615ad531fa0`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）——两者自 Draft 4.242 起因「prev-on-batch 评估分歧」保持 deferred。Java/Go 各 58 条 records、0 differences（新增 normal-view 9 条 = 空迭代器 + E1/E2/E5 部分批快照 + E3/E6/E9 三次交付，prev 1 条）。**共享核心修复（由 Java IStreamRelativeAccess 源码逐行钉定）**：Go 此前把 length/time 批窗口 flush 的整批 history 按行前缀（batch prefix through the row）供 prev 族使用——Java 的批视图经单一 IStreamRelativeAccess 交付整批，RelativeAccessByEventNIndexGetterImpl 仅给被 flush 的行挂 accessor：`prev(n)` 按行锚定到批前缀（`getRelativeToEvent` 的 indexPerEvent），而 `prevtail(n)` 绝对取 `lastNewData[n]`、`prevcount` 取整批大小、`prevwindow` 取整批 newest-first，对每一条被 flush 的行同值；旧行（下次 flush 时 accessor 被移除）与部分批 iterator 行（从未被交付，无 accessor）prev 族全部为 null（四个 ExprPreviousEvalStrategy* 均先查 accessor）。Go 侧：批 flush 块新增 `prevBatchByEvent`（identity→整批，投递序）并经 `projectionEvalContext` 以新 `EvalContext.PreviousWindowBatch` 模式注入（`PreviousWindowAccess` 保持 false，plain prev/prior 仍走行前缀 History）；PrevTail 批模式绝对索引、PrevCount/PrevWindow 读整批；快照路径对批窗口流把各行 historyByEvent 置空（部分批行 prev 族/prior 为 null，对齐 Java 无 accessor）；单事件 flush（len==0 守卫改为 len>0）同样发布批 buffer。**既有测试修偏**：`TestViewExternallyTimedBatchRefWithPrevParity` 此前把 Go 旧前缀行为钉为断言（A 行 prevWindow=[1.0]），与 Java `ViewExternallyTimedBatchRefWithPrev` 源钉定值（A/B/C 三行 prevWindow 全为 [3,2,1]、prevTail0/1=1/2、prevcount=3；E 轮旧行 prev 族全 null）相反，已按 Java 向量改写并加强（新增 D 行单事件批与旧行 null 断言）。运行侧：`prev` case 投影补齐 `irstream *` 暴露的 price/volume/feed 列（此前休眠分支仅六个别名）；新增 6 种 trace 变异全部拒绝（prevwindow 前缀化、prevtail/prevcount 行内漂移、快照行 prev 非 null、旧行/新行 prevString 漂移）。manifest 更新为 666 cases、664 implemented、290 个 differential-verified case、1059 个 differential runtime IDs、3657 条 associations（referenced 3311、unreferenced 825）；capability `view.basic-windows` DV runtime IDs 12→14，`ViewLengthBatch.java` 10 个 executions 全部获得 Go disposition（9 个差分验证 + invalid 编译面 twin 单列 intentionally-different）。
> 最新补充：Draft 4.417（2026-09-15），resultset/querytype having 链 join 三 execution 激活（4.241 遗留缺口闭合，含共享核心无聚合 join 修复）：既有 `resultset-query-type-having` 链从 7 个 runtime 扩展到 10 个，覆盖固定 Java `ResultSetQueryTypeHaving.java` ordinal 3 `ResultSetQueryTypeStatementJoin`（`java-runtime-c5e8204a0ec635e5ded3`）、ordinal 5 `ResultSetQueryTypeNoAggregationJoinHaving`（`java-runtime-0be8c1b9dd6f114b3919`）与 ordinal 6 `ResultSetQueryTypeNoAggregationJoinWhere`（`java-runtime-3566968f29b3c05f40ee`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）——此前因「滑动窗口 join old/new 分类分歧」保持未登记（4.241 remaining note）。Java/Go 各 28 条 records、0 differences（10 个 case；ord 3 发送 SBS DELL 种子 + 7 个 DELL 市场事件，交付 new {5,7.5}/{8,9.5}/{6,8.8} 与 old {5,10.2}；ords 5/6 回放 11 个 SYM1/SYM2 spread 发送（volume=-1 钉定），交付 new(20,10,10)、old(20,10,10)、new(18.5,20,1.5)、old+new(18.5,20,1.5/18.5,16,2.5)、old+new(18.5,16,2.5/12,16,4)，having 与 where 两孪生逐值相同）。**引擎修复（共享核心，`internal/esper/runtime.go`）**：首轮差分回放证明 4.241 缺口真实存在——Go 把「select/HAVING 均无聚合函数、无 group-by」的 join 语句送进未分组聚合状态机，产生滑窗到期时把 old 元组重新投递为 new、下一次发送投递过期 old 行、末次 a 侧滑动投递缺失、以及 where 变体每次首发都伴随 null-prior 镜像 old 行等分歧；Java 对该形状路由到 HANDTHROUGH/UNAGGREGATED_UNGROUPED（ResultSetProcessorFactoryFactory 分支 1）。修复：`aggregateBatch` 检测 `definition.join != nil && len(groupBy) == 0 && aggregateDefinitionHasNoAggregates(definition)` 时委托新函数 `unaggregatedJoinBatch`——每个新 join 元组投影一行 new、每个离开元组一行 old，均逐元组经 having 门控（where 已在入口过滤新旧元组流），零聚合状态参与。修复后两孪生与 Java 逐值一致。场景侧 `cases[]` 重排为 Go runner 发射序（trace 逐索引比较，Java oracle 按 cases[] 顺序发射，两侧 case 步序必须一致）。新增 6 种 trace 变异全部拒绝（`r := trace.Records[i]` 复制结构体，slice 变异须直接赋值 `trace.Records[i].Old`）；引擎回归 `TestUnaggregatedJoinRowPerTuple` 双孪生钉定 5 次投递向量。全量 esper/parity/compat/app 套件绿，全部既有链 evidence 回放绿（引擎改动零回归）。manifest 更新为 663 cases、661 implemented、289 个 differential-verified case、1055 个 differential runtime IDs、3651 条 associations（referenced 3308、unreferenced 828）；case `case.resultset-query-type-having` 与 capability `resultset.aggregate-having` 扩展至 10 个 DV runtime IDs，capability remaining 移除 join 孪生缺口。
> 最新补充：Draft 4.416（2026-09-15），resultset orderby 第六个差分链（iterator 面，文件收官）：新增 `case.resultset-orderby-rowperevent-iterator`（链 id `orderby-rowperevent-iterator`），覆盖固定 Java `ResultSetOrderByRowPerEvent.java` ordinal 0 `ResultSetIteratorAggregateRowPerEvent`（`java-runtime-7bef2fa8f755e74b24ac`；static `java-d88b4c5e379242ff177b`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）——该文件全部 11 个 executions 至此全覆盖。Java/Go 各 2 条 snapshot records、0 differences，1 case / 11 个 scenario 步骤（无虚拟时间、无输出策略、无 listener）：join `SupportMarketDataBean#length(10) x SupportBeanString#length(100)`（symbol=theString）通过 statement ITERATOR 读取两次，每行携带当前窗口和（第一次 214 = 50+49+15+100，第二次 289 = 214+75），`order by symbol` 精确排序 CAT,CAT,IBM,IBM / CAT,CAT,IBM,IBM,KGB；Java 的 `assertPropsPerRowIterator` 为逐索引精确顺序，故 snapshot op 为严格模式（无 mode:any），两侧 trace 均不归一化行序；别名列保留 Java 引擎生成的 `sumPrice`。**引擎修复（共享核心，snapshot 排序）**：`snapshotJoinAggregateBatch` 构建的条目此前不含 group/source 事件，`orderAggregateResults` 对裸上下文求值导致 JoinField 排序键全部 Missing、行保持 join 创建序；snapshot row-per-event 分支的分组条目现携带 `sourceEvent`（行自身的 join 元组），且 `orderAggregateResults` 用各行自身事件求值排序键；join snapshot 路径（`snapshotJoinAggregateBatch`，rowrecog 语句使用）同法加固。全量 esper + parity 套件绿，既有链无行为变化。manifest 更新为 663 cases、661 implemented、289 个 differential-verified case、1052 个 differential runtime IDs、3648 条 associations（referenced 3305、unreferenced 831）；capability `resultset.orderby-simple` goRefs 增加新链文件；ResultSetOrderByRowPerEvent.java 至此全部 11 个 executions 覆盖完毕（6 个差分验证 + 5 个既有引擎级 parity 测试）。
> 最新补充：Draft 4.415（2026-09-15），resultset orderby 第五个差分链（join row-per-event 聚合三连，零引擎改动）：新增 `case.resultset-orderby-rowperevent-agg-join`（链 id `orderby-rowperevent-agg-join`），覆盖固定 Java `ResultSetOrderByRowPerEvent.java` ordinals 2 `ResultSetRowPerEventJoinOrderFunction`（`java-runtime-3864ed6701fd9371d2d6`；static `java-bb2fc1b5fb51cb943021`）、9 `ResultSetRowPerEventJoinMax`（`java-runtime-eb2d5eb23ce35ef9d935`；static `java-7c35d3dcefb677a20097`）与 10 `ResultSetAggHaving`（`java-runtime-73a76f426926d18792f2`；static `java-9c135512047209da35dd`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go 各 3 条 records、0 differences，3 cases / 32 个 scenario 步骤（无虚拟时间），全部基于 join `SupportMarketDataBean#length(10) x SupportBeanString#length(100)`（symbol=theString），`output every 6 events` 按「结果行」计数、在第 6 个结果行创建时一次性交付：ord 2 六行 CAT,CAT,CMU,IBM,IBM,KGB 携带 join 输出累计和 11,11,22,19,19,23（Java 源只钉 symbol 序列，和值由 Java oracle 提供）；ord 9 嵌套 `max(sum(price))` 六行 CAT,CAT,CMU,CMU,IBM,IBM 值 11,11,21,21,18,18（各自行创建时刻的 join 输出前缀最大值，由 Java 断言钉定）；ord 10 `having sum(price) > 0`（此处恒真）交付同序六行 11,11,21,21,18,18（由 Java 断言钉定）。**本单元文档化了既有「未分组 join 聚合行折叠」限制对该形状不再适用**：join row-per-event 聚合按 join 对交付一行，嵌套 max 与普通运行和按各行创建时刻的 join 输出前缀求值，与 Java 完全一致；引擎级由 `TestNestedMaxOfSumJoinHistoricalPrefix` 钉定（交付 [CAT 11,11]、[IBM 18,18]、[CMU 21,21]），嵌套 `aggregateExtreme` 修复来自 Draft 4.414，本单元零引擎改动。manifest 更新为 662 cases、660 implemented、288 个 differential-verified case、1051 个 differential runtime IDs、3647 条 associations（referenced 3304、unreferenced 832——三个 runtime id 此前未被任何 case 关联）；capability `resultset.orderby-simple` goRefs 增加新链文件；该文件仅剩 ord 0（iterator 面）未覆盖。
> 最新补充：Draft 4.414（2026-09-15），resultset orderby 第四个差分链（order-function 与嵌套 max(sum)，含共享核心嵌套聚合修复）：新增 `case.resultset-orderby-rowperevent-agg`（链 id `orderby-rowperevent-agg`），覆盖固定 Java `ResultSetOrderByRowPerEvent.java` ordinals 3 `ResultSetRowPerEventOrderFunction`（`java-runtime-e5b38082f9b30ce8a13b`；static `java-f583dbb6e793f0fae373`）与 5 `ResultSetRowPerEventMaxSum`（`java-runtime-1be3cb0efcdb94f20d5f`；static `java-55bd91f3bd2eecde7acf`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go 各 2 条 records、0 differences，2 cases / 14 个 scenario 步骤（无虚拟时间）：ord 3 `select symbol, sum(price) from SupportMarketDataBean#length(10) output every 6 events order by volume*sum(price), symbol` 六事件一次性交付六行，按 (volume*sum=0.0, symbol) 排序为 CAT,CAT,CMU,IBM,IBM,KGB，各携带自身事件时刻的运行窗口和 18,23,6,2,12,3；ord 5 `select symbol, max(sum(price)) ... order by symbol` 交付 CAT,CAT,CMU,CMU,IBM,IBM，值为 15,21,8,10,3,7（嵌套 max 的历史前缀最大值，运行和单调时等于该行自身事件处的运行和）。两条断言均为 `assertPropsPerRowNewOnly`（精确行序），两侧 trace 不做任何顺序归一化；未别名聚合列保留 Java 引擎生成的列名 `sum(price)`/`max(sum(price))`。**引擎修复（共享核心）**：Go 此前没有嵌套 `max(agg)`——只有 Avg 实现了 Esper 的历史前缀嵌套聚合求值，Max/Min 走 `aggregateExtreme`（其逐事件求值会清空 `Group`，内层聚合看不到事件而返回 null）；`aggregateExtreme` 现在镜像嵌套 Avg 的分支（内层表达式为聚合时遍历 EverGroup 并对每个累积前缀求值取极值），由 `TestNestedMaxOfSumHistoricalPrefix` 钉定（overlay 去掉修复即以 `<nil>` 失败）。manifest 更新为 661 cases、659 implemented、287 个 differential-verified case、1048 个 differential runtime IDs、3644 条 associations（referenced 3301、unreferenced 835）；capability `resultset.orderby-simple` goRefs 增加新链文件。
> 最新补充：Draft 4.413（2026-09-15），resultset 本地分组第十个差分链（solution-pattern 除法 + 虚拟时间边界，含共享核心到期顺序修复）：新增 `case.resultset-querytype-local-group-by-solution-pattern`（链 id `resultset-querytype-local-group-solution-pattern`），覆盖固定 Java `ResultSetQueryTypeLocalGroupBy.java` ordinal 12 `ResultSetLocalGroupedSolutionPattern`（`java-runtime-ae96db5ed464e562e6d7`；static `java-13f0da7834ee65870fb0`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go 各 3 条 records、0 differences，1 case / 23 个 scenario 步骤（纯虚拟时间 0/10/20/30s）：`count(*) / count(*, group_by:())` 在 `#time(30 sec)` 上按 theString 分组、`output snapshot every 10 seconds`，三个边界分别断言 A/B/C = 1/6、3/6、2/6 → 3/12、7/12、2/12 → 6/12、5/12、1/12；第三个分母为 12（而非 18）正是 t=0 那一批在同一 tick 快照前已到期的可观测证据。`pct` 必须按浮点除法计算（Esper 默认除法为浮点，整数除会得 0/1）。**引擎修复（共享核心）**：同一 tick 的「时间窗口到期 vs 时间快照」顺序取决于该次时钟推进覆盖了多少个输出周期——`Engine.advanceSpan` 记录本次推进跨度，仅当 `advanceSpan > policy.Interval` 时保留 deadline==tick 的事件可见（本次推进恰好覆盖一个周期时由到期优先，正是 ordinal 12 所需）。该规则由对固定 Esper 的 JDK 17 探针确定（窗口 10/20/30/40/60s × 各整除周期，含单次跳跃变体），两个参考执行方向分别被钉定，并由 `TestTimeWindowSnapshotTickSpanRule` 在进程内复现；`case.resultset-aggregate-limit-snapshot` 仍为 0 differences。已记录的已知缺口：当整组在同一 span>interval 边界到期时 Java 仍交付该组行而 Go 丢弃空组（暂无 scenario 覆盖）。manifest 更新为 660 cases、658 implemented、286 个 differential-verified case、1046 个 differential runtime IDs、3642 条 associations（referenced 3299、unreferenced 837 不变）；capability `resultset.aggregate-local-group` 的 remaining 收窄为 plan 钩子/非法诊断与 grouped on-select/plugin 聚合。
> 最新补充：Draft 4.412（2026-09-15），resultset 本地分组键表示与回读差分链：新增 `case.resultset-querytype-local-group-by-keys`（链 id `resultset-querytype-local-group-keys`），覆盖固定 Java `ResultSetQueryTypeLocalGroupBy.java` ordinals 18/19/24/26/27 —— ord 18 `ResultSetLocalUngroupedSameKey`（`java-runtime-890001d4334d5de6c50a`；static `java-2c2e1d80b0046f88b67e`）、ord 19 `ResultSetLocalGroupedSameKey`（`java-runtime-a2b77511196632040e51`；static `java-b0aa55f10cfa580ccd4f`）、ord 24 `ResultSetLocalEnumMethods`（`java-runtime-b6a938fde543383eb73c`；static `java-98ac70ee0f434579c8c8`）、ord 26 `ResultSetLocalMultikeyWArray`（`java-runtime-81855e4095ee0ca7cadd`；static `java-6f7f7c3ba440787d3117`）、ord 27 `ResultSetLocalUngroupedOnlyWGroupBy`（`java-runtime-e1253cd2c17a180c243a`；static `java-e3b7ff9f0f3bf5872d7b`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；flags 全空）。Java/Go 各 20 条 records、0 differences，5 cases / 25 个 scenario 步骤、无虚拟时间：object-array 事件上的两个单表达式局部键（{10,10}{21,11}{12,22}{13,35}{27,14}）；外层 `group by g1` 与跨外层组共享状态的局部键（`X` 行 {13,34,35}、收尾行 {47,26,14}）；`window(*)/window(intPrimitive)` 经 `firstOf()` 与 `first(*)` 取属性回读 statement-wide 与按当前事件键选中的局部层（c0/c1 为发出事件本体）；int[]/long[]/double[] 及三键元组按**内容深度相等**分组（E4 的不同 int[]{1} 实例并入 E1/E5：{23,24,24,13,46}，E5 {37,36,24,24,60}，E6 {27,39,27,15,75}，E7 {27,55,40,27,91}）；`first(*, group_by:())` 恒取首个事件（外层键变化后 c0 仍为 1）。**引擎修复（共享核心，由 parity review 的 P2 发现驱动）**：Go 的本地分组键比较沿用 `compareValues`/`reflect.DeepEqual`，浮点标量与 `[]float64`/`[]float32` 键走 Go 的 `==` 语义（`-0.0 == 0.0`、`NaN != NaN`），与 Java 的键语义正好相反（Java 用 `Double.equals` 与 `Arrays.equals(double[])`，即 `doubleToLongBits`，`-0.0 != 0.0` 且 `NaN == NaN`）；新增 `localGroupKeyFloatEqual`（value.go）与 `javaDoubleToLongBits`/`javaFloatToIntBits`（NaN payload 规范化为单一 quiet-NaN，与 Java 一致）并接入 `localGroupValueEqual`（expr.go），对浮点标量、浮点数组与 object-array 组件按位比较，其余键路径不变。新语义类别由三个 Go 回归钉定：`TestLocalGroupByArrayKeyDeepContentEquality`（数组键内容相等、类型分层、三键元组、全量非局部和）、`TestLocalGroupKeyFloatBitSemantics`（五条键路径；去掉修复以 `scalarSum=3` 失败、去掉 NaN 规范化以 `scalarSum=8, want 12` 失败）与 `TestEnumMethodsOverLocalGroupAggregate`（enum 方法经 `EnumFirstOf`/`NestedField` 读取 `LocalGroupBy`，语句级与按组层取值不同）。typed Go 使用 `RegisterObjectArray` + `FromAny`+`Field[any,T]`（ord 18/19）、`RegisterStruct[SupportThreeArrayEvent]`（ord 26）、`LocalGroupBy`/`Sum`/`WindowEvents`/`WindowValues`/`EnumFirstOf`/`FirstEventValue`/`NestedField`。manifest 更新为 659 cases、657 implemented、285 个 differential-verified case、1045 个 differential runtime IDs、3641 条 associations（referenced 3299、unreferenced 837 不变）；capability `resultset.aggregate-local-group` 的 remaining 收窄为 solution-pattern 除法/计划钩子/非法诊断、grouped on-select 与 plugin 聚合。
> 最新补充：Draft 4.411（2026-09-15），resultset 本地分组 context-terminated 差分链：新增 `case.resultset-querytype-local-group-by-context-terminated`（ResultSetQueryTypeLocalGroupBy ords 17/23：四种子形态的 terminated 快照 + 聚合 order-by 并列序），Java/Go 各 6 条 records、0 differences；引擎修复：局部 group_by 键未被外层 group by 覆盖时必须走 row-per-event（此前误判为 fully-aggregated）。summary：658 cases、284 DV cases、1040 DV runtime IDs。
> 最新补充：Draft 4.410（2026-09-15），resultset 本地分组 row-remove 差分链：新增 `case.resultset-querytype-local-group-by-row-remove`（ResultSetQueryTypeLocalGroupBy ords 20/21：命名窗口逐键删除与全删、ungrouped 零回调 vs grouped null 聚合行），Java/Go 各 15 条 records、0 differences，20 个 scenario 步骤；**引擎修复**：命名窗口删除的 row-for-event 旧行分支补齐流选择器门控（此前默认 istream-only 查询也会投递仅 old 回调，Java 不会）。summary：657 cases、283 DV cases、1038 DV runtime IDs。
> 最新补充：Draft 4.409（2026-09-15），resultset 本地分组 ungrouped-agg 差分链：新增 `case.resultset-querytype-local-group-by-ungrouped-agg`（ResultSetQueryTypeLocalGroupBy ords 0/1/2/7：四档 ungrouped 局部和、SQL 统计列、事件值局部聚合、局部组 HAVING），Java/Go 各 14 条 records、0 differences，21 个 scenario 步骤，零引擎改动。summary：656 cases、282 DV cases、1036 DV runtime IDs。
> 最新补充：Draft 4.408（2026-09-15），resultset 本地分组 grouped 差分链：新增 `case.resultset-querytype-local-group-by-grouped`（ResultSetQueryTypeLocalGroupBy ords 10/11/14：length(4) 逐事件十列、snapshot-every 双边界多层级和/计数/window 列），Java/Go 各 9 条 records、0 differences，26 个 scenario 步骤，零引擎改动。summary：655 cases、281 DV cases、1032 DV runtime IDs。
> 最新补充：Draft 4.407（2026-09-15），resultset 本地分组 extended 差分链：新增 `case.resultset-querytype-local-group-by-extended`（ResultSetQueryTypeLocalGroupBy ords 8-9：unidirectional join 局部和、length(4) 三层 window/count/sum 列），Java/Go 各 7 条 records、0 differences，13 个 scenario 步骤，零引擎改动。summary：654 cases、280 DV cases、1029 DV runtime IDs。
> 最新补充：Draft 4.406（2026-09-15），resultset 本地分组 ungrouped 差分链：新增 `case.resultset-querytype-local-group-by-ungrouped`（ResultSetQueryTypeLocalGroupBy ords 3-6：iter 迭代快照、SODA 文本/模型孪生、列名渲染），Java/Go 各 13 条 records、0 differences，20 个 scenario 步骤，零引擎改动，零核心改动。summary：653 cases、279 DV cases、1027 DV runtime IDs、25 intentionally-different。
> 最新补充：Draft 4.405（2026-09-14），event/json 子域第二个差分链：`event-json-adapter` 链覆盖 EventJsonAdapter 三个可观测 executions（insert-into、create-schema 串转换、doc-sample），Java/Go 各 8 条 records、0 differences；适配器两类诊断升级为 ErrorInvalidRule 分类并由进程内测试断言；manifest 拆分出 `case.event-json-adapter-invalid`（intentionally-different）。summary：652 cases、278 DV cases、1023 DV runtime IDs、25 intentionally-different。
> 最新补充：Draft 4.404（2026-09-14），context/nested 缺口闭环 + 引擎校验：`TestContextNestedInvalidParity` 钉定 ContextNestedInvalid 两个编译期诊断；引擎新增分段上下文事件类型要求校验（`validateSegmentedContextEventType`，类型承载分段层级下列表外语句类型以 ErrorInvalidRule + Java 原文模板在 key 解析前拒绝）。manifest goTests 补记并关闭 4.402 缺口 notes；状态与 summary 计数不变。
> 最新补充：Draft 4.403（2026-09-14），view/time-accum 覆盖缺口闭环 + 引擎修复：四个 ViewTimeAccum 缺口执行（PreviousAndPrior 两场景、Sum、GroupedWindow）+ MonthScoped（TimeAccumCalendar 首个端到端覆盖）获得真实 Java-behavior parity pin。引擎修复：prev 族访问历史对 time_accum 窗口此前恒空（提供者只处理 time-order/sorted），现按插入序窗口提供 newest-first prev 访问历史；修复前 SceneOne pin 失败、修复后通过。manifest goTests 补记并关闭 4.402 缺口 notes；状态与 summary 计数不变。
> 最新补充：Draft 4.402（2026-09-14），go-unit 完整性修复：manifest 全量对账 `goTests` 声明与仓库实际定义的 Go 测试。补齐 4.401 event-json 链缺失的三个 run-family 测试（真实编写并通过）；`case.view-union-basic` 重指向真实 union parity 测试；删除两处从未编写的测试名并将 ViewTimeAccum previous/prior、sum、grouped-window 与 ContextNestedInvalid 登记为开放覆盖缺口（case notes）。状态与 summary 计数不变。
> 最新补充：Draft 4.401（2026-09-12），event/json 子域首个差分链：新增 `event-json-sender-getter` 链覆盖 EventJsonEventSenderParseAndSend（`java-runtime-8d0258718d883f9c5497`）与 EventJsonGetterMapType（`java-runtime-b36d999f7edc275dacd2`）（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 2 条 listener records、0 differences：json-sender-parse-and-send 经 `@JsonSchema create json schema MyEvent(p1 string)` 的 JSONSender Parse 精确字节 `{"p1": "abc"}` 后 SendEvent，listener 投递 {p1:"abc"}；json-getter-map-type 的嵌套 Map prop 经 sendEventJson 投递 {x:"y"}（minimaljson 紧凑输出），投影为 kind/row 包装的 {x:"y"}，getter 表面分歧（prop.somefield? 不广告）在 manifest capability remaining 中声明。嵌套 Map 以 kind/row 包装规范化（双侧一致）。Go 侧零引擎改动（RegisterJSON + JSONSender Parse/SendEvent + FromAny + NormalizeResults）。manifest 更新为 651 cases、277 个 differential-verified case、1020 个 differential runtime IDs、3619 条 associations（referenced 3293、unreferenced 843 不变——两个 runtime ID 已由各自案例预关联，per-case dv 列表 = 各自单 ID）。
> 最新补充：Draft 4.395（2026-09-13），infra named-window 第十四个差分切片（final：pattern/TTL 族）+ invalid 处置：新增 `case.infra-namedwindow-views-final-views`（链 id `infra-named-window-final-views`），覆盖 ord 52 `InfraPattern`（`java-runtime-476271957d6ffdb3a878`）、ord 57 `InfraNamedWindowTimeToLiveDelete`（`java-runtime-3c2f3a2696127c04b2a6`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；inventory flags 全空）。Java/Go trace 各 8 条 records、0 differences（3 listener + 5 snapshot，30 个 scenario 步骤）：pattern 消费者 `every a=PAT(key='S1') or a=PAT(key='S2')`（S2 完成即退出整个 or，后续 S1 静默），TTL 窗口按插入时刻 `current_timestamp()+longPrimitive` 逐行定 deadline、绝对虚拟时间 0/500/1000/2000 到期，5 个 any 快照；无引擎改动。同时新增 `case.infra-namedwindow-views-invalid`（`intentionally-different`，ord 46/47/48 零输出编译/部署诊断）。manifest 更新为 651 cases、273 DV cases、1018 DV runtime IDs；`InfraNamedWindowViews` 58/58 全覆盖。
> 最新补充：Draft 4.398（2026-09-12），perf §4.9.3 剩余半部：dispatchSync 派发路径的 ResultBatch 借用——订阅者不 clone（newSubscriberUpdate 同步脱离行至 SubscriberRow，batch 切片从不触及订阅者）、listener i 除末位借用者外私有浅 clone、sink 恒借用（单 listener 派发零 clone）。持有契约：消费者可无限期保留交付的 batch（数组永不复用）；非末位消费者获私有浅 clone 使 slice 级变更不泄漏；深层变更经共享 Result 指针各处可见（既有语义，记录于 Listener）。保持不变：async outbound-pool clone（task 独占闭包）、replayListener clone-on-retain、faf 快照仅在预派发失败时恢复（已记录派发至多一次）。非空变量快照复制经调查排除（快照在派发期间被写入：subqueryEngineVariable 注入 + output-policy THEN 赋值；只读视图需跨切面 map 重构——留档后续单元）。基准：StatelessFilterSend/accepted 9→8 allocs · 1584→1536 B；GenericFilterSend/getter-accepted 19→18 allocs；DispatchBorrowListeners 1/2/4 listeners 7/8/10 allocs（每额外 listener +1 而非 +2）；-race 绿（Threading/Outbound/Replay/Subscriber/Dispatch）。Go 侧零引擎语义变更（dispatchSync 借用 + 持有契约注释；库内无消费者变更 batch 切片——corpus grep 证实）。manifest 计数不变（无 capability/案例变更）。
> 最新补充：Draft 4.393（2026-09-12），database 子域第三个实现切片：新增 `epl-database-restart` 链覆盖 `EPLDatabaseJoin.java` 的 EPLDatabaseRestartStatement execution（`java-runtime-0d41625df368897ca97b`，ordinal 18；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Go runner 已固定并验证 100 次 undeploy/send/redeploy/send 生命周期，Java/Go listener trace 各 100 条、0 differences；该证据仅证明覆盖场景的输出等价与进程内未部署发送守卫，不证明 MySQL/JDBC 连接获取、释放或连接池耗尽语义，因此 runtime/case 仍保持 implemented，未登记 differential-verified。scenario 现显式钉定 lifecycle label 与 count；同单元顺带修正两处陈旧 docstring。
> 最新补充：Draft 4.394（2026-09-13），infra named-window 第十三个差分切片（insert 形状族）：新增 `case.infra-namedwindow-views-insert-shape`（链 id `infra-named-window-insert-shape`），覆盖 ord 28 `InfraDoubleInsertSameWindow`（`java-runtime-1b9f5b6ccf6b49cdba09`）、ord 36 `InfraIntersection`（`java-runtime-f739aa91028d27aa572f`）、ord 54 `InfraSelectStreamDotStarInsert`（`java-runtime-9740441818d84a3ee94d`）、ord 56 `InfraOnInsertPremptiveTwoWindow`（`java-runtime-71402af94b27b4255ab8`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；inventory flags 全空）。Java/Go trace 各 8 条 records、0 differences（4/3/0/1，29 个 scenario 步骤，固定 epoch 时间）：双 insert 按 insert 边界逐个投递（create/s0/create/s0，引擎修复：`send` 对每个直接命名窗口 insert 触发器捕获一个 `namedWindowInsertBoundary`，先投递入站语句输出再逐个投递边界，保留 front 优先/back 延后与重入回调隔离）、length(2)/unique 交集过期、object-array stream-star 未声明列 c0 的已批准类型化构建器差异（回放 p0-only 形状）、TypeTrigger 双触发抢占级联。`InfraNamedWindowViews` 53/58 executions differential；manifest 更新为 649 cases、272 个 differential-verified case、1016 个 differential runtime ID。

> 最新补充：Draft 4.392（2026-09-11），infra named-window 第十二个差分切片（bean/schema 表示族）：新增 `case.infra-namedwindow-views-beans`，覆盖 ord 2 `InfraBeanBacked`（`java-runtime-f0a1da1fe931e132c21f`）、ord 35 `InfraBeanContained`（`java-runtime-fe6adc5803da60bf18f5`）、ord 37 `InfraBeanSchemaBacked`（`java-runtime-f195548d023dfbde1aed`）、ord 38 `InfraDeepSupertypeInsert`（`java-runtime-baa0bd4ca41b9b2c5f94`）。Java/Go trace 各 25 条 records、0 differences：bean/schema/嵌套片段/子类型插入四种表示与 FAF、on-trigger 观测均钉定；ord 2 更新波中消费者与触发语句记录的相邻顺序差异经 Java oracle 钉定并由既有 Go-normalizer 精确适配该一对（原始 Go trace 保留）。`InfraNamedWindowViews` 49/58 executions differential。

> 最新补充：Draft 4.391（2026-09-11），infra named-window 第十一个差分切片（消费者视图族）：新增 `case.infra-namedwindow-views-consumers`，覆盖 ord 43 `InfraFilteringConsumer`（`java-runtime-a7827f22ee0c135e84d2`）、ord 45 `InfraFilteringConsumerLateStart`（`java-runtime-502dd5b0e84f28fb2c68`）、ord 49 `InfraPriorStats`（`java-runtime-5ccc9535c9241efda4cc`）、ord 50 `InfraLateConsumer`（`java-runtime-5118f72a4d8684d593d0`）、ord 51 `InfraLateConsumerJoin`（`java-runtime-c49a6a43a1efd3fa0729`）。Java/Go trace 各 64 条 records、0 differences：消费者过滤器双流过滤、unique 替换对、prior null 先验、单变量统计派生视图状态与派生到派生的 irstream 对、迟到过滤聚合消费者的 preload、左外连接空填充行与 any-order 迭代态均钉定。`InfraNamedWindowViews` 45/58 executions differential。

> 最新补充：Draft 4.390（2026-09-11），infra named-window 第十个差分切片（#groupwin 按组保留 + 迟到分组视图）：新增 `case.infra-namedwindow-views-groupwin`，覆盖 ord 26 `InfraLengthWindowPerGroup`（`java-runtime-083a289ee5f82dd87ad3`）、ord 27 `InfraTimeBatchPerGroup`（`java-runtime-accaf82c3c846493832b`）、ord 44 `InfraSelectGroupedViewLateStart`（`java-runtime-3326973240f20b92dced`）、ord 55 `InfraSelectGroupedViewLateStartVariableIterate`（`java-runtime-0ec49d4098c9796540b0`）。Java/Go trace 各 29 条 records、0 differences：组迭代序（组建序+组内插入序）、length 仅插入时过期、每组 cap 过期的 new+old 单次投递、time_batch 以组首到达为锚点并以组建序拼接同刻 flush、迟到分组消费者的 preload 与 `having` 迭代期变量求值均钉定；计数型迭代器断言以空字段投影编码。`InfraNamedWindowViews` 40/58 executions differential。

> 最新补充：Draft 4.389（2026-09-11），infra named-window 第九个差分切片（虚拟时间批窗口）：新增 `case.infra-namedwindow-views-time-batch`，覆盖 ord 16 `InfraTimeBatch`（`java-runtime-1ad42a8ed8025c4a0730`）、ord 17 `InfraTimeBatchSceneTwo`（`java-runtime-1eb11f5cf275069ef6b5`）、ord 18 `InfraTimeBatchLateConsumer`（`java-runtime-bb926d092e7110db203f`）、ord 23 `InfraTimeLengthBatch`（`java-runtime-22bf6b3644a24862df7d`）、ord 24 `InfraTimeLengthBatchSceneTwo`（`java-runtime-dd924e1b7e500df135f8`）。Java/Go trace 各 43 条 records、0 differences：批量 rollover 的 new+old 单次投递、全空 flush 无回调、时间锚点跨空批不重置、time_length_batch 尺寸触发与时间触发并存的重新武装、中途部署聚合消费者的 preload 跳过（整批求和）均钉定；批窗口上的无分组聚合消费须以 `Aggregate(...)` 建模。`InfraNamedWindowViews` 36/58 executions differential。

> 最新补充：Draft 4.388（2026-09-11），infra named-window 第八个差分切片（虚拟时间 time_order/time_accum）：新增 `case.infra-namedwindow-views-time-order-accum`，覆盖 ord 9 `InfraTimeOrderWindow`（`java-runtime-320f1b03eddb24520308`）、ord 10 `InfraTimeOrderSceneTwo`（`java-runtime-ed5bfc247d088a603224`）、ord 14 `InfraTimeAccum`（`java-runtime-d74b2c9c614b26973637`）、ord 15 `InfraTimeAccumSceneTwo`（`java-runtime-8b4e2e7d81bb990c7d72`）。Java/Go trace 各 101 条 records、0 differences：边界 old-only 过期、过期行单次双流透传、time_accum 多行突发按插入序释放、最新/末行删除的定时器重排与取消均钉定；internal/esper 零改动。`InfraNamedWindowViews` 累计 31/58 execution differential；manifest 更新为 644 cases、267 个 differential-verified case、994 个 differential runtime IDs、3592 条 associations。

> 最新补充：Draft 4.387（2026-09-11），infra named-window 第七个差分切片（事件时间）：新增 `case.infra-namedwindow-views-ext-time`，覆盖 ord 6 `InfraExtTimeWindow`（`java-runtime-c960628819cddf06d0bd`）、ord 7 `InfraExtTimeWindowSceneTwo`（`java-runtime-aa7fe5003096f5bf1cf8`）、ord 53 `InfraExternallyTimedBatch`（`java-runtime-3af75eefff14ce16ae81`）。Java/Go trace 各 49 条 records、0 differences：滑动 ext_timed 单次 new+old 投递、墓碑静默、epoch 参照 ext_timed_batch 的静默累积与首刷无 old 均钉定；无定时器/无 advance-time，internal/esper 零改动。`InfraNamedWindowViews` 累计 27/58 execution differential；manifest 更新为 643 cases、266 个 differential-verified case、990 个 differential runtime IDs、3588 条 associations。

> 最新补充：Draft 4.386（2026-09-11），infra named-window 第六个差分切片（虚拟时间）：新增 `case.infra-namedwindow-views-time`，覆盖 ord 4 `InfraTimeWindowSceneTwo`（`java-runtime-879d6ad8aee378657a02`）、ord 5 `InfraTimeFirstWindow`（`java-runtime-5f5a65dd5f72c8e24619`）、ord 8 `InfraExtTimeWindowSceneThree`（`java-runtime-a6bfdd3389c077127861`）。Java/Go trace 各 57 条 records、0 differences：滑动窗口边界过期（含独立 consume 消费者）、firsttime 部署锚定静默关闭、等时推进与 12999 静默均钉定；链新增 `advance-time` 步骤协议与按投递时刻的虚拟时钟标记。`InfraNamedWindowViews` 累计 24/58 execution differential；manifest 更新为 642 cases、265 个 differential-verified case、987 个 differential runtime IDs、3585 条 associations。

> 最新补充：Draft 4.385（2026-09-11），infra named-window 第五个差分切片（length_batch/sort）：新增 `case.infra-namedwindow-views-lengthbatch-sort`，覆盖 ord 19 `InfraLengthBatch`（`java-runtime-81f4d92e62dbafdd7f2f`）、ord 20 `InfraLengthBatchSceneTwo`（`java-runtime-ce4a95ec7941743a7bfe`）、ord 21 `InfraSortWindow`（`java-runtime-7edfd8382e4eee6892dc`）、ord 22 `InfraSortWindowSceneTwo`（`java-runtime-5fb3e9a4d20bc3964f5f`）。Java/Go trace 各 80 条 records、0 differences：批量释放边界语义（首刷无 old、后续刷携带被替换批）、未刷数据删除静默、sort 最小 N 保留/等值最新在前/自我驱逐均钉定。`InfraNamedWindowViews` 累计 21/58 execution differential；manifest 更新为 641 cases、264 个 differential-verified case、984 个 differential runtime IDs、3582 条 associations（referenced 3293、unreferenced 843）。

> 最新补充：Draft 4.384（2026-09-11），infra named-window 第四个差分切片（length/firstlength）：新增 `case.infra-namedwindow-views-length`（链 `infra-named-window-length-views`），覆盖 ord 11 `InfraLengthWindow`（`java-runtime-d931bfbbbaa3ea632c2b`）、ord 12 `InfraLengthWindowSceneTwo`（`java-runtime-793f6ec7608e25556803`）、ord 13 `InfraLengthFirstWindow`（`java-runtime-b53dbf8f541e79537f32`）、ord 25 `InfraLengthWindowSceneThree`（`java-runtime-c656dd0eeae6618a4162`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 65 条 records、0 differences：length 的 FIFO 驱逐单次 IR pair、firstlength 满窗零回调丢弃与删除后尾部补位、SupportBean_A 触发与通配消费者投影均钉定。`InfraNamedWindowViews` 累计 17/58 execution differential；manifest 更新为 640 cases、263 个 differential-verified case、980 个 differential runtime IDs、3578 条 associations（referenced 3293、unreferenced 843）。

> 最新补充：Draft 4.383（2026-09-11），infra named-window 第三个差分切片（unique/firstunique）：新增 `case.infra-namedwindow-views-unique`（链 `infra-named-window-unique-views`），覆盖 ord 32 `InfraUnique`（`java-runtime-63daa6cfa27a786c0480`）、ord 33 `InfraUniqueSceneTwo`（`java-runtime-1ee31df7428f7c8e85eb`）、ord 34 `InfraFirstUnique`（`java-runtime-bb670bc21e9d4333cefd`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 46 条 records、0 differences：unique 的重复键替换为单次 IR pair 投递、删除释放键后重新接纳；firstunique 的重复键零回调吞掉；多行快照以 any 模式 + 规范排序协议钉定（HashMap 迭代序不可契约）。`InfraNamedWindowViews` 累计 13/58 execution differential；manifest 更新为 639 cases、262 个 differential-verified case、976 个 differential runtime IDs、3574 条 associations（referenced 3293、unreferenced 843）。

> 最新补充：Draft 4.382（2026-09-11），infra named-window 第二个差分切片（retention 视图基础）：新增 `case.infra-namedwindow-views-retention`（链 `infra-named-window-retention-views`），覆盖 ord 0 `InfraKeepAllSimple`（`java-runtime-26c44410c8018a34d696`）、ord 29 `InfraLastEvent`（`java-runtime-b85cc831b5c23a570cf7`）、ord 30 `InfraLastEventSceneTwo`（`java-runtime-59100403b3c6affd0729`）、ord 31 `InfraFirstEvent`（`java-runtime-c2f54d9eb104d061950c`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 43 条 records、0 differences。last-event 替换为单次 IR pair 投递、firstevent 丢弃零回调、无匹配删除静默、以及 iteration 快照在删除后归空全部钉定；窗口语句先于消费者的扫描顺序再次实测确认，internal/esper 零改动。`InfraNamedWindowViews` 累计 10/58 execution differential（ord 3、1、39-42、0、29-31），manifest 更新为 638 cases、261 个 differential-verified case、973 个 differential runtime IDs、3571 条 associations（referenced 3293、unreferenced 843）。

> 最新补充：Draft 4.381（2026-09-11），infra named-window 域新差分链，同时修复一处共享核心派发顺序缺陷：`InfraNamedWindowViews.java`（58 execution，此前 1 个 differential：ord 3 `InfraTimeWindow` 由 `case.time-window-long-running` 覆盖）新增首个切片 `case.infra-namedwindow-views-keepall-delete` 覆盖 ord 1 `InfraKeepAllSceneTwo`（`java-runtime-14f3c7199cf587fea29a`）与 ord 39-42 四个 on-delete 别名变体 （`InfraWithDeleteUseAs` `java-runtime-6d4a6e1168f100add522`、`InfraWithDeleteFirstAs` `java-runtime-8fe8d92928db5eba5fc3`、`InfraWithDeleteSecondAs` `java-runtime-b47bdbdf2ac26de6cb0d`、`InfraWithDeleteNoAs` `java-runtime-594ea469769e1990e60f`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 144 条 records、0 differences。引擎修复：named window 窗口语句（direct create-window 子节点）输出在 delete/mutation 路径此前晚于消费者波投递，现于 `flushNamedWindowConsumerWaveLocked` 起始处（且无待发语句输出时）先排空 direct 派发，对齐 Java 的窗口子节点先于 tail-view 消费者语义（插入波原本已正确）；独立复审指出无条件排空会违反「mutation 预处理语句输出先于 direct 子节点」不变量，已按其最小修复加守卫。新增引擎回归测试钉定两波顺序（overlay 删除该排空则测试失败），全量 parity 套件与内部测试全绿。后续项：混合 mutation 波（update-istream + on-delete 触发监听器 + 窗口语句 + 消费者）中触发语句自身输出的位置与 Java 主派发序不同，需独立 oracle 场景钉定后再处理。本切片同时确认 `InfraNamedWindowViews` 其余 53 个 execution 均已有 Go 实现（go-unit 证据），并按 oracle harness/retention 族规划为 14 个后续差分切片（keep-alive 族→unique 族→length 族→batch/sort 族→time 族→事件时间族→time_order/time_accum 族→time_batch 族→groupwin 族→consumer-preload 族→bean/schema 族→insert-shape 族→pattern/TTL 族→invalidity 族）。manifest 更新为 637 cases、260 个 differential-verified case、969 个 differential runtime IDs、3567 条 associations（referenced 3293、unreferenced 843）；capability `infra.namedwindow.views` DV 列表 +5。

> 最新补充：Draft 4.380（2026-09-11），TimeBatch 历史引擎缺口修复 + `epl-database-timebatch` 差分链覆盖 `EPLDatabaseJoin.java` 的 EPLDatabaseTimeBatch execution（`java-runtime-fb0cea6fe1e469ee8237`，ordinal 5；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。引擎修复（internal/esper/runtime.go，+170/-2，两项协同缺陷）：(1) updateJoin 到达路径——触发事件被批窗口缓冲时（triggerNewRows 为空）跳过无驱动的历史轮询（对齐 Java TimeBatchView 不更新子视图、join 组合器不在边界前运行）；(2) expireJoin 释放路径——按释放行重驱动触发性历史源的轮询（每释放行一次、lineage 绑定使轮询行仅与本释放元组配对、按释放序输出、上一批行在下一边界级联移除）。新增引擎测试 TestDatabaseTimeBatchMatchesJava（填入原 comment 占位：静默缓冲/10s 边界一次 3 行批投递 [100,50,20]/轮询键 [10 5 2]/20s [90 80] 新 + [100 50 20] 旧）与 TestTimeBatchHistoricalJoinSnapshotReadOnly（快照只读幂等、无投递副作用）。差分链：`epl-database-timebatch` 单 case 7 条 records（iterator 计数 1/2/3 累积 + 边界刷行 listener 3 行 [100,50,20] + 刷后清空 + 重缓冲 [90]/[80]），listener/count 记录协议，epoch 断言不计入记录。值规范化同 4.377（scale-0 字符串、null 标记）。`case.inventory.epl-database-join` dv 列表 10→11（EPLDatabaseTimeBatch 达 DV；TimeBatchOM/Compile 因 SODA/eplToModel 编译面缺失保持延后）。Go 引擎改动即本单元修复本身（runtime.go 快照重驱动 + 释放重驱动），其余零改动。manifest 更新为 636 cases、259 个 differential-verified case、964 个 differential runtime IDs、3562 条 associations（referenced 3293、unreferenced 843 均不变——fb0c runtime ID 已由本案例预关联，per-case dv 列表 10→11）。
> 最新补充：Draft 4.379（2026-09-11），database 子域处置收尾单元：`case.inventory.epl-database-join` 的十一个 implemented-not-differential execution 逐一定性（manifest difference 全量枚举，无 trace 变更）。TimeBatch/TimeBatchOM/TimeBatchCompile 三重奏延后并记录探针发现：Go 侧 time_batch 窗口在历史驱动流上缓冲后不于时间边界释放已连接行（窗口释放/轮询交互需引擎级调查），OM/Compile 变体另需 Go 不具备的 SODA/eplToModel 编译面；MySQLDatabaseConnection 为纯 JDBC 冒烟（无 Esper 观测量）；InvalidSQL 内嵌 MySQL 版本相关错误 1064 文本（intentionally-different 诊断面）；InvalidBothHistorical/InvalidPropertyEvent/InvalidPropertyHistorical/Invalid1Stream/InvalidSubviews 为编译路径诊断断言——类型化构建器结构性阻止循环历史替换、自引用解析为 Missing 产生零行（引擎测试钉定），而历史流上的视图被接受而非拒绝（已记录分歧）；RestartStatement 确定性但延后待真实 MySQL 连接生命周期核算（100 循环防 'Too many connections'）。探针代码已按纪律移除；Go 侧零引擎改动。manifest 案例状态与计数不变（636 cases、259 个 differential-verified case、963 个 differential runtime IDs、3562 条 associations、referenced 3293、unreferenced 843）。
> 最新补充：Draft 4.378（2026-09-11），database 子域第二个差分切片：新增 `epl-database-join-2` 链覆盖 `EPLDatabaseJoin.java` 的五个 execution——2HistoricalStarInner `java-runtime-8961178ee540998a99f3`、JoinIndexNullType `java-runtime-a8180e298e71e8135b38`、WithPattern `java-runtime-3ddf3346fbe67a639057`、Variables `java-runtime-0a048d0e0005df5f6279`、3Stream `java-runtime-fc8c20664fc4787fc987`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 11 条 records、0 differences，listener/count 记录协议：2historical-star-inner 双排除键历史内连接（三条负发送 E1/A1/A10 不触发 + {a:B,b:3,c:B,d:B} 一次投递 + D4 负匹配）；join-index-null-type 的 null 类型键在 #unique(id) 下匹配不到任何行（capture-empty 0）；with-pattern 的 timer:interval(5 sec) 模式经 AdvanceTime 驱动（5000 行 {mychar:Y}、9999 静默、10000 行 {mychar:Y}，静默推进计数入 trace）；variables 的 queryvar 变量参数化历史查询（5→50、6→60；Java 的 v1/v2 流序反转为构建器文本不可观测——已记录）；3stream 的双 #lastevent 窗口 + 无限制历史逐周期重询（{T2,T2,30}、{T3,T3,40}，stateless-flag-false 进程内断言）。`case.inventory.epl-database-join` dv 列表 5→10。Go 侧零引擎改动（FromHistoricalOn + JoinMany/On + JoinPatternSource/TimerInterval + SetVariable + NormalizeResults）。manifest 更新为 636 cases、259 个 differential-verified case、963 个 differential runtime IDs、3562 条 associations（referenced 3293、unreferenced 843 均不变——五个 runtime ID 已由本案例预关联）。
> 最新补充：Draft 4.377（2026-09-11），database 子域首个 differential-verified 链：新增 `epl-database-join` 差分链覆盖 `EPLDatabaseJoin.java`（21 个 execution）的首切片五个 execution——SimpleJoinLeft `java-runtime-67745eb75f864ea17fed`、SimpleJoinRight `java-runtime-45e68d7cf756824c19eb`、StreamNamesAndRename `java-runtime-4675598d7044c43fd088`、PropertyResolution `java-runtime-33cc6c0610b2c801c1bb`、2HistoricalStar `java-runtime-a6682a71c8c463babcff`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 9 条 records、0 differences，listener/count 记录协议：四个 9 列全行投影（SimpleJoinLeft 的 ${id} 键控查询、SimpleJoinRight 的流序反转及九个事件类型属性类型进程内钉定、StreamNamesAndRename 的 a..i 别名映射、PropertyResolution 的 ${s1.arrayProperty[0]} 嵌套索引参数选中 mybigint-10 行且 mynumeric 为 NULL）加 2HistoricalStar 的双历史同键 keepall 谱系（两行 listener + iterator 计数 1/2 + SB(20) 负匹配不触发）。值规范化：BigDecimal scale-0 列记录为字符串（"5000"/"100"/…），NULL 列以 null 标记编码，MySQL CHAR 尾随空格剔除镜像于 fixture 值；Java oracle 跑在已建立的 esper-mysql mysql:8.0 Docker fixture（create_testdb.sql）上，Go 侧经函数式 HistoricalProvider 喂同一份 10 行规范 fixture（无需驱动）。对账：manifest 案例关联补入四个缺失 execution（SimpleJoinLeft/Variables/InvalidSQL/StreamNamesAndRename，inventory 21 之 4）；剩余 execution（TimeBatch/OM/Compile、WithPattern、Variables、3Stream、RestartStatement、JoinIndexNullType、invalid 族、MySQLDatabaseConnection）保持 implemented-not-differential 待后续切片。Go 侧零引擎改动（FromHistoricalOn + HistoricalProvider 函数喂行 + JoinMany/SelectFrom 投影 + KeepAll 窗口）。manifest 更新为 636 cases、259 个 differential-verified case、958 个 differential runtime IDs、3562 条 associations（referenced 3293、unreferenced 843 均不变——四个对账 ID 已被其他案例引用）。
> 最新补充：Draft 4.376（2026-09-11），dataflow 域第十五个 differential-verified 链：新增 `dataflow-lifecycle-blocking` 链覆盖 `EPLDataflowAPIRunStartCancelJoin.java` 的三个 blocking-mode execution（BlockingCancel `java-runtime-91a5f5421ce63806e2c4`、FastCompleteBlocking `java-runtime-42082e1062bebbe49a0a`、RunBlocking `java-runtime-bcc34451f5417d84e6fa`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 10 条 records、0 differences，state/count/lifecycle 记录协议（有界确定性等待替换 sleep/spin，延迟值不入 trace）：blocking-cancel 在 RUNNING 可观测后取消，取消经 join 以错误类 token `cancellation-exception` 冒泡（Java 的 EPDataFlowCancellationException 消息文本经由同部署第二实例的字节精确阻塞 run() 路径在进程内断言——该 run() 为唯一抛出点）、CANCELLED + 空 capture；fast-complete-blocking 钉定 not-done-before-run 负属性 token、COMPLETE 与单行 capture，tryAssertionAfterExec 进程内断言；run-blocking 在 RUNNING 观察后释放门控源，COMPLETE + 单行（源提交计数 2 = bean + final marker，进程内）。`case.dataflow-lifecycle` dv 列表 13→16——BlockingRunJoin（`java-runtime-e13449306814341577ad`）因 deltaJoin>=500ms 墙钟断言永久延后、BlockingMultipleRunnable（`java-runtime-955a5d67e2668f50f0f5`）因多源 run() 守卫差异保持 implemented-not-differential。Go 侧零引擎改动（CustomTypedSource 门控源 + State()/Join/Cancel 内省 + OnSignal 冲刷 capture）。manifest 更新为 636 cases、258 个 differential-verified case、953 个 differential runtime IDs、3558 条 associations（referenced 3293、unreferenced 843 均不变——三个 runtime ID 已由本案例预关联，per-case dv 列表 13→16）。
> 最新补充：Draft 4.375（2026-09-11），dataflow 域第十四个 differential-verified 链：新增 `dataflow-lifecycle-cancel-join` 链覆盖 `EPLDataflowAPIRunStartCancelJoin.java` 的七个 start-mode cancel/join execution（NonBlockingJoinCancel `java-runtime-893c8283ee90019f37fa`、NonBlockingJoinException `java-runtime-e1bdb19779e28fa49bc2`、NonBlockingException `java-runtime-69dbb4e0d879421b52f7`、NonBlockingCancel `java-runtime-f053fbdc01fad03ba5a6`、NonBlockingJoinMultipleRunnable `java-runtime-827b0af4ea14c59da2bc`、NonBlockingJoinSingleRunnable `java-runtime-a9a21d69ccb78f9fd152`、FastCompleteNonBlocking `java-runtime-de4fc2179a2cb72a97e6`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 18 条 records、0 differences，state/count 记录协议（门控 channel 源镜像 Java latch 指令；有界等待替换全部 sleep/spin 且不记录延迟值）：join 仅经 cancel 返回（RUNNING + 空 capture；Go Join 冒泡 ErrorCanceled 而 Java join 为 void——记录状态值不记错误文本）、start 模式吞掉源异常（Go 经 Join 冒泡——语义差异已记录，共同观测量为 COMPLETE + 空 capture）、立即抛出源同样 COMPLETE、start 后立即可观测 RUNNING 且 cancel 同步、双源门控下 RUNNING 保持至全部释放后 join（两行）、单源形态（getCurrentCount==2 与 COMPLETE 后 cancel 的静默语义为进程内断言）、有限 beacon 自行完成（有界轮询替换忙等；tryAssertionAfterExec——join 空操作、run/start-after-complete 拒绝——进程内断言）。BlockingMultipleRunnable（`java-runtime-955a5d67e2668f50f0f5`）保持 implemented-not-differential：Java run() 对非单源图抛 UnsupportedOperationException 而 Go Run 无源计数守卫，重放该形态需引擎语义变更（已记录）。Go 侧零引擎改动（CustomTypedSource 门控源 + OnSignal 冲刷 capture + Join/Cancel 状态机内省）。manifest 更新为 636 cases、258 个 differential-verified case、950 个 differential runtime IDs、3558 条 associations（referenced 3293、unreferenced 843 均不变——七个 runtime ID 已由本案例预关联，per-case dv 列表 6→13）；该 case 剩 5 个 execution（BlockingCancel、FastCompleteBlocking、RunBlocking、BlockingRunJoin-RED 墙钟断言永久延后、BlockingMultipleRunnable-守卫差异）。
> 最新补充：Draft 4.374-a（2026-09-11），dataflow 域第十三个 differential-verified 链：`case.dataflow-lifecycle`（18 rts，最大剩余 dataflow 块）开始差分覆盖，新增 `dataflow-lifecycle-core` 链一次覆盖六个 GREEN execution（ConfigAndInstance `java-runtime-67b54e408ca6f86b4ca5`、APIStatistics `java-runtime-c8cbd0bc1aeb3eba6006`、ParameterInjectionCallback `java-runtime-1dd9223830c5409fc252`、OperatorInjectionCallback `java-runtime-1092d7e9660b84bc6e6d`、InvalidJoinRun `java-runtime-7e40b511fb6b6db584fd`、BlockingException `java-runtime-3bdb22d6cdef39f0336c`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 49 条 records、0 differences，count/state/value 记录协议：saved-config/saved-instance CRUD 全周期（空注册表探测、四条错误路径 token：instantiation-not-found/save-not-found/already-exists/instance-already-exists、saved-config run() 经 EventBusSink 的 listener 触发 + COMPLETE）；两算子统计形态（source submitted 2/port-0 2、capture 0；时间量与语句属性断言按政策留在进程内——Go 无语句内省面）；参数注入回调（provider 恰好 3 次上下文、排序参数名、首上下文值、factory 一致性、解析值 abc/def/xyz——provider 覆盖图内 propTwo）；算子注入回调（provider 替换运行时，1 上下文）；InvalidJoinRun 同步状态机（join-before-exec/run-after-cancel/start-after-cancel token + INSTANTIATED/CANCELLED + 幂等 cancel）；BlockingException（阻塞 run 冒泡源错误 + COMPLETE + 空 capture）。null 探测以规范化 token "absent" 记录（Go value 面无法输出 JSON null，沿 token 约定）；异常消息文本不入 trace。其余 12 个 execution 保持 implemented-not-differential（cancel/join 族待后续单元；BlockingRunJoin 的 deltaJoin>=500ms 墙钟断言永久延后——join-blocking 语义已由引擎测试钉定）。Go 侧零引擎改动（Stats/OperatorStats 内省 + SavedDataflow* CRUD + Parameter/OperatorProvider 上下文 + 状态机错误面）。manifest 更新为 636 cases、258 个 differential-verified case、943 个 differential runtime IDs、3558 条 associations（referenced 3293、unreferenced 843 均不变——六个 runtime ID 已由本案例预关联，per-case dv 列表=6 个 GREEN ID）。
> 最新补充：Draft 4.373（2026-09-11），dataflow 域第十二个 differential-verified 链：`case.dataflow-exceptions` 由 implemented 升级为 differential-verified，对照固定 Java `EPLDataflowAPIExceptions.java`（`java-runtime-c1d106a7e1cc70655006`，整类单 direct execution；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 12 条 records、0 differences，count/state/value 记录协议（无数据行）：source-throw 流（处理程序恰好一次上下文归因 DefaultSupportSourceOp#0、实例 COMPLETE——异常由 Java 包装为 'Support-graph-source generated exception: ...' 而消息文本按 invalidity 政策仅进程内断言不入 trace）与 operator-throw 流（处理程序收到未包装原始异常并吞掉——恰好一次上下文归因 MyExceptionOp#1、流正常完成）。算子 pretty-print 以规范化裸端口形式记录（Java 进程内断言其含 '<SupportBean>' 全串；Go 剥离包限定类型标签与隐式默认输出后缀——规范化决策钉定于 scenario 描述）。语义注记：Go 的 Fail 策略经 Run/Join 冒泡错误而 Java 的 start-and-swallow 静默完成——COMPLETE 状态与单次处理程序调用是共同观测量。`case.dataflow-invalid-graph`（19 个编译前缀探针）与 `case.dataflow-custom-properties`（Java forge 反射内部，无可重放观测量）确认保持 implemented-not-differential（既有 difference 文本已覆盖）。Go 侧零引擎改动（CustomTypedSource + CustomPorts 错误算子 + DataflowExceptionHandler 上下文记录 + DataflowError 归因/pretty-print 内省）。manifest 更新为 636 cases、257 个 differential-verified case、937 个 differential runtime IDs、3558 条 associations（referenced 3293、unreferenced 843 均不变——c1d1 runtime ID 已由本案例预关联，dv 列表=[c1d1]）。
> 最新补充：Draft 4.372（2026-09-11），dataflow 域第十一个 differential-verified 链：新增 `dataflow-ports-feedback` 差分链一次覆盖 `EPLDataflowInputOutputVariations.java` 全部三个 execution——`case.dataflow-feedback` 由 implemented 升级为 differential-verified（Factorial `java-runtime-98c5bdc6afa84705b9db` + LargeNumOps `java-runtime-bddc8c72bd9d2a2c8869`），`case.dataflow-captive-emitter-ports` 的 dv 列表补入 FanInOut `java-runtime-6309a6b7e0f0ba981a97`（两案例达全 DV；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 6 条 records、0 differences：fan-in-out 四 beacon 源入 2 进 2 出 MyCustomOp，数值 submitPort 交叉声明端口类型（S0 字符串→SchemaTwo 型 OutTwo、S1 整数→SchemaOne 型 OutOne，Java 不做运行时类型检查；Go 以非类型化 CustomPorts 按声明序映射 OutOne=0/OutTwo=1），四个 capture 各读满两行（S1-10/S1-20、S0-A1/S0-A2）；factorial 以 TempResult 输出回环自边递归 5·4·3·2 终止于 FinalResult 的 120，实例自行 COMPLETE；large-num-ops 17 级恒等链单行 A1（Java 的 select: 子查询参数无类型化 API 对应，Go 以恒等自定义算子建模——已记录差异）。assertEqualsAnyOrder 以双侧发射时按 canonical fields JSON 排序冻结；BeaconSource 终止标记因自定义算子无 onSignal 而在源通道消亡，capture 读为 current 批次语义（加载性事实）。Go 侧零引擎改动（CustomPorts + ConnectFeedbackPorts 自环 + DataflowEmission 端口路由 + OnSignal 吞没）。manifest 更新为 636 cases、256 个 differential-verified case、936 个 differential runtime IDs、3558 条 associations（referenced 3293、unreferenced 843 均不变——三个 runtime ID 已由两案例预关联）。
> 最新补充：Draft 4.371（2026-09-11），dataflow 域第十个 differential-verified 链：`case.dataflow-captive-emitter-ports` 的 `EPLDataflowAPIStartCaptive` execution（`java-runtime-407e42478546e709d774`，整个类为单 direct execution；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）由 implemented 升级为 differential-verified。Java/Go trace 各 11 条 records、0 differences，count/state/capture 混合记录协议（沿 create-start-stop-destroy 约定，固定 epoch 时间）：captive 启动契约（runnables=0、emitters=1 且键为 Emitter NAME 参数 'src1'——命名路由差异已记录）、两次累积提交 {E1,10}/{E2,20}、FinalMarker 信号清空 current 而不产生数据行（信号≠行）、getAndReset 批次双行、E3 落入新 current、信号后实例仍 RUNNING（captive 无完成路径，取消是唯一出口）、cancel 后 CANCELLED，加 HelloWorld 文档示例流仅实例化（INSTANTIATED，真实 LogSink 算子）。图 A 字节精确 `@name('flow') create dataflow MyDataFlow Emitter -> outstream<MyOAEventType> {name:'src1'}DefaultSupportCaptureOp(outstream) {}`（`}` 与 DefaultSupportCaptureOp 间无空格）。Java 透传原始 Object[] 而 Go 经注册结构体物化——行规范化一致（已记录适配）。FanInOut（`java-runtime-6309a6b7e0f0ba981a97`，数值索引 submitPort）保持 implemented-not-differential 且不入 dv 列表（Go 命名端口面由引擎单元测试钉定）。Go 侧零引擎改动（StartCaptive + CaptiveEmitter Submit/SubmitSignal + Process/OnSignal current-received 捕获算子 + DataflowRunning/Cancelled 状态内省）。manifest 更新为 636 cases、255 个 differential-verified case、933 个 differential runtime IDs、3558 条 associations（referenced 3293（+1）、unreferenced 843（−1）；407e runtime ID 补入案例关联，per-case dv 列表=[407e]）。
> 最新补充：Draft 4.370（2026-09-11），dataflow 域第九个 differential-verified 链：`case.dataflow-eventbus-collector` 由 implemented 升级为 differential-verified，对照固定 Java `EPLDataflowOpEventBusSink.java` 全部三个 execution（AllTypes `java-runtime-e6b4bb618f70b384cf03`、Beacon `java-runtime-1ecb769e10b4a818772c`、SendEventDynamicType `java-runtime-11a7b321e6901dbad740`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 13 条 records、0 differences，listener 记录协议（每次回调一条记录、固定 epoch 时间、语句标签 s0/s1）：all-types 四表示子运行（套件序 XML/OA/Map/POJO）经 `MyGraph DefaultSupportSourceOp -> instream<T>{}EventBusSink(instream) {}` 各钉定两次同步 listener 投递 {1.1,1,one}/{2.2,2,two}（`EventBusSink -> s1` 输出流编译拒绝与双 sink 文档示例流仅实例化为内部断言）；beacon 以 iterations:3 常量 p0='abc'/p1=1 经 EventBusSink 交付三条（有界等待适配；Java 的 0<p1<10 松界由常量 1 满足——非随机性）；dynamic-type 由 sink collector 按原始 object-array 首字段路由到两个 @buseventtype schema（s0 {type:type1,p0:100,p1:abc} 先于 s1 {type:type2,f0:GE,f1:-1}，全投影含路由 type 字段——强于 Java 子集断言）。Java 类名字符串 collector 配置与 emitter 上下文身份保持已记录差异。Go 侧零引擎改动（EventBusSink/EventBusSinkWithCollector + DeployPlans 消费语句 + statement.Subscribe + NormalizeResults，消费语句先于图实例化部署订阅）。manifest 更新为 636 cases、254 个 differential-verified case、932 个 differential runtime IDs、3557 条 associations（referenced 3292、unreferenced 844 均不变）；per-case dv 列表 5 ID：两个 EventBusSource ID 由 `case.dataflow-eventbus` 链（draft 4.369）、Beacon ID 由 `case.dataflow-connector-output` 链差分验证，跨链证据拆分记录于案例 difference。
> 最新补充：Draft 4.369（2026-09-11），dataflow 域第八个 differential-verified 链：`case.dataflow-eventbus` 由 implemented 升级为 differential-verified，对照固定 Java `EPLDataflowOpEventBusSource.java` 与 `EPLDataflowOpFilter.java` 全部四个 execution（EventBusSource AllTypes `java-runtime-dfb59d3bd4798d57cc0c`、EventBusSource SchemaObjectArray `java-runtime-37aed9aedcbbb25c4826`、Filter AllTypes `java-runtime-73c4f6808b38087c5a59`、Filter Invalid `java-runtime-0bc68f1db4dd07d89a66`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 22 条 records、0 differences：eventbus-all-types 四表示子运行（POJO/Map/XML/OA）各钉定实例化后启动前发送丢弃（空读）、启动后双事件按发送序填充 {1.1,1,one}/{2.2,2,two}、取消后再发送丢弃（空读），另有两个 EventBusSource 无效编译（含引擎原生 `declated` 拼写）与文档示例流仅实例化的内部断言；eventbus-schema-objectarray 三个子运行钉定 `@public @buseventtype create objectarray schema` 下的信封行 {p0,p1}、底层行的位置规范化 {'0':'abc','1':100}（新规范化决策，scenario 描述钉定）与 filter+collector 子运行（'B' 被过滤的空读 + collector 重提交的 {p0:A,p1:101}；Java emitter 上下文身份断言无 Go 面，随开放 collector 模式记录）；filter-all-types 四表示经罐装源同步 run 钉定 'two' 行加双流出 captive 形态（{x,10} 过、{y,11} 拒；`flow:DefaultSupportCaptureOpStatic` 标签）；filter-invalid 五个编译期拒绝按 invalidity 政策零 trace 行（缺 filter 映射为 Go Build nil/非 bool 谓词拒绝；3/0 输出流在流式 API 不可表示、隐式 Integer→String 转换为 Go 编译错误、prev() 无 dataflow filter 上下文限制——全部记录为差异）。Java assertSame 捕获身份以规范化字段替代（已记录）。Go 侧零引擎改动（EventBusSource/WithUnderlying/WithFilterAndCollector + Filter/FilterWithPorts + StartCaptive CaptiveEmitter + DataflowOptions 实例身份）。manifest 更新为 636 cases、253 个 differential-verified case、930 个 differential runtime IDs、3557 条 associations（referenced 3292（+1）、unreferenced 844（−1）；Invalid runtime ID 按 epstatement-source 先例补入案例关联与 per-case dv 列表，4/4 execution DV）。
> 最新补充：Draft 4.368（2026-09-10），**运行时性能修复第三轮：无状态谓词编译（§4.7）与事件类型级候选裁剪（§4.1 第一阶段），非新 capability，不新增 DV/NFR 登记**：stateless 计划在解析时把已证明纯的谓词链编译为直线闭包（字段读取预解析候选路径、全部操作复用与泛型闭包相同的函数，编译集外整链回落；矩阵差分测试守护等价），StatelessFilterSend rejected -32%、accepted 约 -50%；语句 prepare 期解析事件类型接受描述符，派发环对可证不可接受语句在求值前跳过（事件侧接受名集合按类型缓存，context/update-istream/子查询/输出策略/variant/contained/historical/method 语句不可裁剪，指标审计全以 accepted/changed 为门故无可观测差异），64 语句/2 类型单引擎基准对 HEAD worktree 84→20 µs、68→4 allocs（约 4x/17x）。新增应用侧接入指引（docs §7：合并引擎为单引擎多语句、保持快速路径资格、发送/监听器建议）。三条既有重放链 0 differences；全量测试、定向 race、vet、gofmt 绿。过滤索引第二阶段（等值/IN/范围属性索引）保持未实施。

> 最新补充：Draft 4.367（2026-09-10），**运行时性能修复第二轮：派发路径固定分配削减（非新 capability，不新增 DV/NFR 登记）**：在 4.366 的 stateless 快速路径之上，消除其剩余每事件固定分配（accepted 16 → 8、rejected 10 → 4 allocs，同机交替 A/B 中位 -20% 时延）：纯谓词语句跳过 `matchesEventFilter` 的变量/engine-ref 装配、`send` 空变量快照、两个派发切片栈背衬、`finishExternalRoutes` 惰性排空上下文、`SendEvent` 免复制注册名读取、监听器快照缓存（全部 6 个变更点重建，订阅顺序契约不变）、`Literal` 构造期预装箱。零可观测语义变化；修复随之暴露的 nil 变量快照写入（`bindParameterValues`、触发器写回；5000 替换参数语句原稳定 panic 并因持锁 panic 的 deferred 自死锁表现为挂起）。三条既有重放链 0 differences；全量测试、定向 race、vet/gofmt 绿。详见 [运行时性能与执行模型记录](esper-go-performance.md) §2.4/§3.3。

> 最新补充：Draft 4.366（2026-09-10），**运行时性能修复（非新 capability，不新增 DV/NFR 登记）**：修复应用侧报告的两个 Go 移植性能缺口——(1) P0 属性访问元数据缓存：`structFieldValue` 原按字段名逐字段解析 `esper`/`json` tag 且无缓存（实测 512 字段读末字段 258.44 µs / 1557 allocs），现按 `reflect.Type` 构建不可变字段表（保持 tag 优先级、匿名嵌入候选回退、递归终止与 EqualFold 折叠语义，Plan hash 不变）；(2) P1 无状态过滤内部执行特化：单一 struct 源 + 纯谓词链 + 通配投影 + 默认输出策略的语句直接复用派发循环的过滤结果并产出单行批次，不再走 delta/history/投影/输出策略/变量装配；纯表达式白名单、纯内建函数标记、普通属性名 + 已声明字段、无注册 getter/JavaBean 访问器与 schema 身份校验之外的形状（用户 UDF、多槽位 IN、变量、窗口、输出策略、子查询、嵌套路径名、getter 属性等）继续走通用管线；另缓存派发顺序（部署/卸载/rollback 失效）。验证：`event-bean-property-fragment`（15 runtime IDs）、`expr-filter-optimizable`（9）、`expr-core-logical`（3）三条既有重放链改动前后均 0 differences；应用形态受控基准（213 字段事件 × 278 条双条件规则、单核）由 69.55 µs/候选降至 6.27 µs/候选，分配 445→10/事件批；新增常驻基准与 8 个回归测试；`go test ./...`、`-race`、stress、vet、facade/apidump 漂移与 diff 检查全部通过。后续性能工作项（过滤服务索引、单次谓词求值、WHERE 下推、结果集处理器特化等）记录于 [运行时性能与执行模型记录](esper-go-performance.md)。

> 最新补充：Draft 4.365（2026-09-10），dataflow 域第七个 differential-verified 链：`case.dataflow-beacon` 由 implemented 升级为 differential-verified，对照固定 Java `EPLDataflowOpBeaconSource.java` 四个 execution 中的三个（WithBeans `java-runtime-5438f7be9b56ca119b0c`、Variable `java-runtime-f54de0ae037b90b24551`、NoType `java-runtime-f1fc7e290467bab36366`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 4 条 records（capture 读取钉定，行负载在 new 数组内批量）、0 differences：with-beans 两个独立部署子运行（legacy bean 与无默认构造器 bean 经构造器选择+setter 物化，Go 侧零值分配为已记录差异）各释放一条 {'myfield':'abc'}；variable 以 `@public create Schema SomeEvent()` + `@public create variable int var_iterations=3` 路径部署后在实例化期解析 iterations=3，锁存读取一次交付三行空 Map underlying；no-type iterations:5 无类型图伪造瞬态 object-array 类型并逐迭代提交空 Object[0]，锁存读取交付五行（空 Map 与空 Object[0] 均规范化为 {'kind':'row','fields':{}}）；no-type instantiate-only（interval-0.5 p1-'abc' 图实例化后从不启动）以实例化成功为可观测、0 records。Fields execution（Math.random 值与 EventBusSink 轮询不确定）与 initialDelay/unbounded-cancel/cancel 子运行保持 real-time-only 不入确定性链（Fields 由既有 go-unit 覆盖钉定）。Go 侧零引擎改动（BeaconSourceWithOptions 字面量/IterationsExpression(RegisterVariable) + BeaconEventSourceWithUnderlying + 未类型化 event factory + Instantiate 后不启动）。manifest 更新为 636 cases、252 个 differential-verified case、926 个 differential runtime IDs、3556 条 associations（referenced 3291、unreferenced 845 均不变——四个 runtime ID 已由本案例预关联，新增 0 条关联对）；并移除 summary 中无 Go 结构体映射的死键 differentialVerifiedRuntimeIDs（round-trip 不再漂移）。
> 最新补充：Draft 4.364（2026-09-10），dataflow 域第六个 differential-verified 链 `case.dataflow-doc-samples`，对照固定 Java `EPLDataflowDocSamples.java` 的两个 execution（`EPLDataflowDocSamplesRun` `java-runtime-b41da4e41f347dcd49a8` static `java-613a9d944f2099f57eb4`、`EPLDataflowDocSamples`（EPLDataflowOpSelect 的同名内嵌执行）`java-runtime-aaf36f532374aca5a232` static `java-9e9fa5d76678f9113923`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 8 条 records、0 differences：hello-world BeaconSource -> LogSink 图阻塞排空（INSTANTIATED -> COMPLETE 状态迁移钉定）加上五 Select 仅实例化图（从未启动，INSTANTIATED 后 undeploy 计数 0）；14 个 parse-only EPL 语法片段在 oracle 内部断言（Go 无解析器面——已记录）；SODA round-trip 变体为 compile-path-only 差异（无运行时可观测）。Go 侧零引擎改动（DefineDataflow + BeaconSourceWithOptions + LogSink + 阻塞 Run）。manifest 更新为 636 cases、251 个 differential-verified case、923 个 differential runtime IDs、3556 条 associations（referenced 3291（+2）、unreferenced 845（−2）；路线图 epl 253→251（2 个新关联 dataflow runtime ID）。

> 最新补充：Draft 4.363（2026-09-10），subselect 域 invalidity 政策收尾单元：登记六个 compile-only/invalid 执行的处置（无 trace 行，Java 侧字节精确消息前缀由断言钉定）。(a) `case.subselect-in` 登记 `EPLSubselectInvalid`（`java-runtime-b19d939e1d67b9257320`）：数组型左操作数的 IN 子查询比较被 Java 编译期拒绝（Go 类型化 API 结构性阻止该形状，any-typed 洞已记录为已知限制）；(b) `case.subselect-aggregated-single-value` 登记 `EPLSubselectAggregatedInvalid`（`java-runtime-4a313ed4ff15caa50489`）并新增引擎校验：非分组单值子查询的聚合参数不得引用关联外流属性、聚合出现时标量投影不得读取聚合边界外的内流字段、having 同规则——Go 在 Build 时拒绝全部六个 Java 无效形状（TestSubselectAggregatedInvalidParity）；(c) `case.subselect-exists` 登记四个 OM/Compile 变体为 compile-path-only intentionally-different（SODA round-trip 无 Go 对应面）。DV-list 合并问题确认解决：全部 250 个 DV 案例均携带 per-case differentialVerifiedRuntimeIds（并集 922 == summary）。manifest 更新为 635 cases、250 个 differential-verified case、922 个 differential runtime IDs（不变）、3554 条 associations（referenced 3289、unreferenced 847）；路线图 epl 253→247（6 个新关联 runtime ID）。

> 最新补充：Draft 4.362（2026-09-10），dataflow 域第五个 differential-verified 链 `case.dataflow-epstatement-source`，对照固定 Java `EPLDataflowOpEPStatementSource.java` 的四个 execution（`EPLDataflowAllTypes` `java-runtime-2d7b6229c7a3b2bee40b`、`EPLDataflowStmtNameDynamic` `java-runtime-950696d7aa6356acb9d4`、`EPLDataflowStatementFilter` `java-runtime-8c8f6f03b11605a16d11`、`EPLDataflowInvalid` `java-runtime-ef085a37ed45eb35a13f`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 17 条 records、0 differences：all-types 以四种事件表示（POJO/Map/ObjectArray/XML，配对语句部署 id 插入图文本）各捕获 {1.1,1,one}/{2.2,2,two} 双行（double 无损渲染）；stmt-name-dynamic 钉定注册表生命周期——先于语句启动（空读）、部署即挂接 {id:E2}、卸载即分离（空）、重部署 {id:E4}、重定义 {id:XE6X}，固定部署 id MyDeploymentId；statement-filter 钉定过滤器选择器挂接既有与动态部署语句（B1/E1/A1）、卸载分离空读、重部署 A3、B2、取消后空。EPLDataflowInvalid 的三个拒绝按 invalidity 政策无 trace 行（Java 侧前缀断言；可表示的无绑定拒绝为 Go Build 错误类，其余两个无 Go 对应面登记于案例注释）。Go 引擎面改动：EPStatementSource 过滤器形态的订阅按语句键控（每次 undeploy 仅分离该语句的监听器，其余匹配语句持续投递，对齐 Java per-statement 监听器语义；命名/直接形态保持替换语义）（EPStatementSourceByDeployment/WithStatementFilter + 部署/卸载钩子）。manifest 更新为 635 cases、250 个 differential-verified case、922 个 differential runtime IDs、3548 条 associations（referenced 3283、unreferenced 853 均不变——四个 runtime ID 已由 umbrella 案例预关联，路线图计数维持 253）。

> 最新补充：Draft 4.361（2026-09-10），dataflow Select 流 `case.dataflow-select-representation` 由 implemented 升级为 differential-verified（新重放链），对照固定 Java `EPLDataflowOpSelect.java` 的 wrapper 表示双变体（`EPLDataflowOpSelectWrapper{wrapperWithAdditionalProps=false}` `java-runtime-9550df89e223e839e414`、`{wrapperWithAdditionalProps=true}` `java-runtime-13cbd82acb5791b30d07`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 2 条 records、0 differences：EventBusSource -> select-star passthrough -> DefaultSupportCaptureOp，无装饰变体捕获 {value:10}，装饰变体（insert-into 增加 hello 列使 B 成 wrapper 类型）捕获联合面 {value:10, hello:a}；Java Pair 拆分（underlying vs additional 命名空间）为表示性元数据，平坦捕获协议不可观测，登记于案例 difference 处置（oracle 内部断言 Pair 拆分）。Go 侧零引擎改动（DefineDataflow + Emitter + SelectPassThrough + Custom capture + captive emitter）。manifest 更新为 634 cases、249 个 differential-verified case、918 个 differential runtime IDs、3544 条 associations（referenced 3283、unreferenced 853 均不变）；EPLDataflowAllTypes 保持 umbrella 实现面不申索 DV。

> 最新补充：Draft 4.360（2026-09-10），dataflow Select 流 `case.dataflow-select-state` 由 implemented 升级为 differential-verified（新重放链，capability `dataflow.graph` DV 列表 +2），对照固定 Java `EPLDataflowOpSelect.java` 的两个基于时间的 Select execution（`EPLDataflowOutputRateLimit` `java-runtime-64f78048eb114d0e9cf9`、`EPLDataflowTimeWindowTriggered` `java-runtime-553841d9498fc8d6d1c7`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 8 条 records、0 differences，记录 time 钉定读取时的活动虚拟时钟（阶段边界可观测）：`output snapshot every 1 minute` 仅在虚拟时钟 tick 释放当前累积投影（t=65000 快照 {14}；t=125000 快照 {23}=14+3+6 无界累积；仅提交不发；cancel 抑制后续 tick）；`#time(1 minute)` 插入时输出 {2}/{7} 累积窗口、E1 在 5000+60000=65000 边界含过期后仅剩 {5}。Go 侧零引擎改动（SelectSnapshotEvery / SelectTimeWindow 由 Engine.AdvanceTime 驱动虚拟时钟）。manifest 更新为 634 cases、248 个 differential-verified case、916 个 differential runtime IDs、3544 条 associations（referenced 3283、unreferenced 853 均不变——两个 runtime ID 已由本案例预关联）。

> 最新补充：Draft 4.359（2026-09-10），dataflow Select 流升级：新链 `case.dataflow-select-flows` 将 `case.dataflow-select-iterate`（`EPLDataflowIterateFinalMarker` `java-runtime-59ce9d5f4b9ca475c272`）与 `case.dataflow-select-join`（`EPLDataflowFromClauseJoinOrder` `java-runtime-323dd1ec14f5ed5c0586`、`EPLDataflowOuterJoinMultirow` `java-runtime-21dcd981de8124bf387c`）的覆盖升级为差分验证（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 12 条 records、0 differences：iterate Select 的组内累积仅在 final marker 时释放（前测捕获为空并以独立 iterate 记录钉定 {E1,6}/{E2,5}/{E3,4} 的分组序与排序）；三个 from-clause 排列的内连接等待全部输入流（等待期空读被钉定）、连接行 {s0id:1,s1id:10,s2id:100} 与 from-clause 顺序无关、取消后提交为空；全外连接 keep-all 的未匹配行 {p00:S0_1, p10:null}。Go 侧零引擎改动（SelectIterate 分组形态、SelectJoin + ConnectInput 端口排列、FullOuter/KeepAll、captive emitters；compat 新增 NormalizeRows 行渲染助手）。manifest 更新为 634 cases、247 个 differential-verified case、914 个 differential runtime IDs、3544 条 associations（referenced 3283、unreferenced 853 均不变——三个 runtime ID 已由 umbrella 案例预关联）。

> 最新补充：Draft 4.358（2026-09-10），dataflow 域第四个 differential-verified 链 `case.dataflow-create-start-stop-destroy`，对照固定 Java `EPLDataflowAPICreateStartStopDestroy.java` 的两个 execution（`EPLDataflowCreateStartStop` `java-runtime-1a8426a71e3b5252a181` static `java-d21cf72eab5cb8f58a0c`、`EPLDataflowDeploymentAdmin` `java-runtime-2c3c90d3cdb64596c225` static `java-32675e64a2d679c673e4`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。两个 execution 均不启动实例——钉定的是定义注册表生命周期：注册目录、instantiate 状态、移除后的 not-defined 错误类（移除名与未知名双探针；Java 字节精确消息由 oracle 内部断言，记录仅携带错误类——Go 侧无文本匹配）、缺失查找、空注册表计数、重实例化；deployment-admin 以仅拓扑四算子图 instantiate（无行）。重放形状丢弃（侦察冻结）：Java deployment-id 维度与 statementName 无 Go 对应面；重部署随机 UUID、module item 数、HA 跳过不予表示。Go 侧零引擎改动（DefineDataflow/Emitter/CustomSource/Select/Custom + saved-configuration 注册表 API）；differ 本单元补齐 Count 字段比较（此前 count 记录不参与判别）。manifest 更新为 633 cases、246 个 differential-verified case、911 个 differential runtime IDs、3541 条 associations（referenced 3283、unreferenced 853）。

> 最新补充：Draft 4.357（2026-09-10），dataflow 域第三个 differential-verified 链 `case.dataflow-op-lifecycle`，对照固定 Java `EPLDataflowAPIOpLifecycle.java` 的三个 execution（`EPLDataflowTypeEvent` `java-runtime-ff077635ccc5082a06a1`、`EPLDataflowFlowGraphSource` `java-runtime-12a29443119ff32fd057`、`EPLDataflowFlowGraphOperator` `java-runtime-4bf013f93c9464b8e890`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 21 条 records、0 differences：编译-only 图的声明输出端口类型名（EventBean<MySchema> 以 schema 名钉定）、配置化自定义源的完整算子生命周期（instantiated/属性 abc/含 instance id1 与 user object 的算子上下文/open/next(0..2)/close，终态 complete）、以及 名称键控工厂闭包注入的 line-feed 源向生命周期算子投递两个载荷（onInput abc/def）。重放形状适配（双侧一致、侦察冻结）：Java 编译期 forge 阶段与部署期 factory-initialize 可观测在 Go 流式 builder 无对应面，双侧丢弃；仅有源图加装 DefaultSupportCaptureOp 使提交有连接边；post-deploy 空排断言为静态列表机制产物不予表示。Go 侧零引擎改动（CustomSource/Custom + 名称键控工厂闭包注入 + DataflowOptions InstanceID/UserObject + Open/Process/Close 钩子）。manifest 更新为 632 cases、245 个 differential-verified case、909 个 differential runtime IDs、3539 条 associations（referenced 3281、unreferenced 855）。

> 最新补充：Draft 4.356（2026-09-10），dataflow 域第二个 differential-verified 链 `case.dataflow-types`（capability `dataflow.graph` 的 differential runtime IDs 扩展至 3（connector 链已有 DV 条目）），对照固定 Java `EPLDataflowTypes.java` 的两个 execution（`EPLDataflowBeanType` `java-runtime-a277455fea9aac5b640b` static `java-66adf2795b74a041e84e`、`EPLDataflowMapType` `java-runtime-8006afc99e90b53a008c` static `java-7da6a895a5d5336ecb27`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 各 4 条 records、0 differences：EPL 文本 create-dataflow 图（DefaultSupportSourceOp 单个罐装事件扇出到两个捕获 sink，阻塞 run() 同步排空）——bean 类 型（outstream<SupportBean>）与 @public map schema 型（outstream<MyMap>，path 多部署）；通用 WPort sink 的 port-0 投递以 capture-port-0 操作钉定。Go 侧零引擎改动：DefineDataflow + 字面量 BeaconSource + Custom 捕获算子（记录事件与输入端口）+ Connect 扇出 + 同步 Run；Java oracle 直接安装 regression-lib 使用真实算子类。manifest 更新为 631 cases、244 个 differential-verified case、906 个 differential runtime IDs、3536 条 associations（referenced 3278、unreferenced 858）。

> 最新补充：Draft 4.355（2026-09-10），`epl.fromclausemethod` 能力首个 differential-verified 链 `case.epl-fromclausemethod-variable`，对照固定 Java `EPLFromClauseMethodVariable.java` 的五个行为 execution（`EPLFromClauseMethodConstantVariable` `java-runtime-b6134319ddad78ee7888`、`EPLFromClauseMethodNonConstantVariable{soda=true}` `java-runtime-7fa38fe9688391d3f9da`、`{soda=false}` `java-runtime-1c125ff69ef065d76608`、`EPLFromClauseMethodContextVariable` `java-runtime-26c863d3cc9d5aca7cc7`、`EPLFromClauseMethodVariableMapAndOA` `java-runtime-2989d0cd737e30ae5633`）。Java/Go trace 各 14 条 records、0 differences：常量形方法源（`_10_`/`_20_`）、非常量服务的 postfix 变量按触发行重采样且中流 on-set 立即可见（两个 soda 编译变体回放同一 Go builder）、重叠 initiated 上下文的每分区变量与分区内 on-set 隔离（`_1_context_postfix`/`_2_b`）、无触发 Map/OA handler 的迭代器快照行。Go 侧零引擎改动：FromMethod/FromMethodOn + MethodProviderFunc 每触发行携带可见变量快照轮询，常量/非常量对比由 provider 闭包表达。第 6 个 execution `EPLFromClauseMethodVariableInvalid` 及 context execution 的两个尾部 invalid 编译按 invalidity 政策处置（无 trace 行；可移植的上下文变量作用域 Build 拒绝已由引擎测试钉定，反射解析类 invalid 无 Go 等价，登记于 umbrella 案例注释）。manifest 更新为 630 cases、243 个 differential-verified case、904 个 differential runtime IDs、3534 条 associations（referenced 3276、unreferenced 860）；capability `epl.fromclausemethod` 获得差分验证（38 个 DV capability）。

> 最新补充：Draft 4.354（2026-09-10），`epl.subselect.*` 引擎单元扩展 `case.epl-subselect-order-of-eval-index` differential-verified 场景至 5 runtime IDs，新增 `EPLSubselectOrderOfEvalNoPreeval`（`java-runtime-ac2d59129befe9a3b088` static `java-04d47cea26b120f5f805`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 47 条 records（既有 45 条字节不变）、0 differences：与 preeval-on 案例字节相同的两条 not-in 自子查询语句在 `selfSubselectPreeval=false` 下重放——not-in 过滤器观测事件到达前的子查询窗口（空集 → SQL 空集规则为真），首个 E1/5 输出完整 20 属性 SupportBean select-* 行（charPrimitive `\u0000`、原始数值零列），事件在处理结束前仍被子查询窗口接受（第二次发送静默）。Go 引擎新增语句级选项 `WithSelfSubselectPreeval(false)`（querySpec/Query 字段 selfSubselectPosteval；Statement.process 延迟自子查询窗口接受至语句自身 filter/where 求值之后，非 context 语句；默认路径字节不变，preeval-on 钉定测试全绿）；runner 的 no-preeval 案例注册全 20 列 SupportBean 面（含 char NUL 默认的解码镜像）。Java oracle 以第二 Configuration/独立 URI 构建 preeval-off 运行时。该 suite 双 suite（OrderOfEval+Index）+ NoPreeval 三源全闭环；manifest 更新为 629 cases、242 个 differential-verified case、899 个 differential runtime IDs、3529 条 associations（referenced 3276、unreferenced 860）。

> 最新补充：Draft 4.353（2026-09-10），`epl.subselect.*` 扩展 `case.subselect-in` differential-verified 场景至 15 runtime IDs，新增 `EPLSubselectInWildcard`（`java-runtime-d52b12c5372d3922a9b2`，ord 6；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 54 条 records（既有 52 条字节不变）、0 differences：`s0.anyObject in (select * from SupportBean_S1#length(1000))` 的全事件通配子查询——anyObject 持有同实例 S1 时 `value=true`，换成类外 S2(同 id) 时 `value=false`（S2 从不进入 S1 窗口，类检查 equals 恒假）。Go 侧 `SubqueryIn[any]` + `EventValue[any]()` 全事件行，anyObject 为 `any` 接口字段，S1/S2 为不同结构体使结构化 DeepEqual 复现 Java 类检查 equals（等值异实例场景本执行未触及，已记录差异边界）；scenario 的嵌套 anyObject 以 `{"type","id"}` 编码、解码按类型还原具体结构体。Go 零引擎改动。`EPLSubselectOrderOfEvalNoPreeval` 经侦察裁定为引擎缺口单元延后（Java 为 `selfSubselectPreeval=false` 运行时配置测试，Go 引擎硬编码子查询先行接受；待专设 preeval-off 开关单元）；该 suite 累计 15/16 execution DV（剩 `EPLSubselectInvalid` 编译-invalid）；manifest 更新为 629 cases、242 个 differential-verified case、898 个 differential runtime IDs（不变）、3528 条 associations（referenced 3275、unreferenced 861）。

> 最新补充：Draft 4.352（2026-09-09），`epl.subselect.*` 扩展 `case.subselect-aggregated-single-value` differential-verified 场景至 15 runtime IDs，新增 EPLSubselectAggregatedSingleValue 的多外stream范围强制转换对（`EPLSubselectUngroupedJoin3StreamKeyRangeCoercion` `java-runtime-73e90c44540e5ba6b1fd`、`EPLSubselectUngroupedJoin2StreamRangeCoercion` `java-runtime-7d4d91983868b27f14b1`，ords 12/13；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 112 条 records（既有 57 条不变）、0 differences，7 个新 case（4+3，按语句生命周期拆分）：3-stream 的 `between` 反转（10..-1 反转为 [-1,10] 得 8）、`>=/<=` 不反转（终值 null）、单边 `>` 得 13 / `<` 得 21、2-stream 的两个 between 方向（20..13 反转得 27）与 `>=/<=` 不反转（null）、null 端点全部输出 null 行。Go 侧 `JoinField` 按 FROM 位置读取每个外stream（JoinEvents 经子查询求值传播）、`BetweenOf` 混合类型比较内建范围反转与 null-不匹配、`Of` 形式比较不做反转、`SubquerySum` 空集输出 null。Go 零引擎改动。该 suite 累计 20/21 execution DV（剩 `EPLSubselectAggregatedInvalid` 编译-invalid 待 invalidity 政策处置）；manifest 更新为 629 cases、242 个 differential-verified case、898 个 differential runtime IDs（不变）、3527 条 associations（referenced 3274、unreferenced 862）。

> 最新补充：Draft 4.351（2026-09-09），`epl.other.distinct` 的 `case-epl-as-keyword-backtick-behavioral` 扩展至 6/7 execution DV，新增 EPLOtherAsKeywordBacktick 的 FAF/on-trigger/merge 三重奏 ords 0/2/4（`EPLOtherFAFUpdateDelete` `java-runtime-472d2c12a99c291f275c` static `java-038bfd4f4e8affcc6db9`、`EPLOtherOnTrigger` `java-runtime-c0ea9e858846b717b2e4` static `java-a77b21b8c405a856cc18`、`EPLOthernMergeAndUpdateAndSelect` `java-runtime-4da78c382449b37e5599` static `java-6709ceb5d59d1cb4fcda`）。Java/Go trace 11 条 records、0 differences：FAF update/delete 的 `order` 反引号别名（OnDemand UpdateWhere/DeleteWhere + NamedWindowField 候选行访问）、on-select 触发流×主键表 join（OnEvent SelectFromTableWhere + TableField）、on-merge/on-update/on-select 链（MergeIntoNamedWindowWhen + WhenMatchedAny、UpdateNamedWindow、SelectFromNamedWindow）。Go 侧 on-demand/trigger update 需要显式谓词，故 Java 的无 where 全行形态以 Literal(true) 表达（行为等价）。Go 零引擎改动。该 suite 仅剩 ord 6（split-stream contained，编译-only，两级 unnest 待裁决）延后。
>
> 最新补充：Draft 4.350（2026-09-09），`epl.other.distinct` 新增 `case-epl-as-keyword-backtick-behavioral` differential-verified 场景，对照固定 Java `EPLOtherAsKeywordBacktick.java` 的行为三重奏 ords 3/5/1（`EPLOtherUpdateIStream` `java-runtime-5c48441abdc543566dd1` static `java-6221f5f24bafea3b2dde`、`EPLOtherSubselect` `java-runtime-7ca72ccdfaf2f99f4ca6` static `java-351607a2b5d02174024a`、`EPLOtherFromClause` `java-runtime-a9b9ecfe0dc6693d0e31` static `java-59c7bc096b154685a83b`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 3 条 records、0 differences：update-istream 别名改写、lastevent 子查询别名、双流 lastevent join 的保留字别名。Go 零引擎改动。该 suite 累计 3/7 execution DV；剩余 ords 0/2/4 和 6 延后至后续单元。

> 最新补充：Draft 4.349（2026-09-09），`view.window-core` 扩展 `case-viewgroup-merge-view` differential-verified 场景，对照固定 Java `ViewGroup.java` 追加 ord 17 表达式 groupwin（`ViewGroupExpressionGrouped` `java-runtime-563f2c37fb66e3d067ca` static `java-383fcd83c5bd59dead03`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 119 条 records（既有 116 条字节不变 + 3 条新记录）、0 differences：`select irstream * from SupportBeanTimestamp#groupwin(timestamp.getDayOfWeek())#length(2)`——groupwin 键为任意表达式（对 epoch-milli 时间戳求 Calendar 星期几），键按表达式值分组（三个周二事件共享一个 length(2) 组），E3 驱逐 E1（套件钉定扁平 old 列表长度为 1）。Go 零引擎改动：GroupWindow 接受任意 Expr 键（Func1 从时间戳计算星期几——键值从不 surface 于 select * 行）。场景对 null-groupId 发送省略 payload 键（Java 两参构造器状态；runner 映射 absent → nil 指针）。该类累计 15/20 execution DV；剩余 7（编译消息）、8（性能门）、19（SERDEREQUIRED）；manifest 更新为 628 cases、241 个 differential-verified case、892 个 differential runtime IDs、3519 条 associations（referenced 3266、unreferenced 870）；capability 120 个（37 DV）。

> 最新补充：Draft 4.348（2026-09-09），`view.window-core` 引擎单元扩展 `case-viewgroup-merge-view` differential-verified 场景，对照固定 Java `ViewGroup.java` 追加视图级 reclaim ords 2/3/9（`ViewGroupReclaimTimeWindow` `java-runtime-88d7b731431c59d99f3a` static `java-62dfe7eb67b16a22d54f`、`ViewGroupReclaimAgedHint` `java-runtime-afc05b1a18402bb17f56` static `java-dd075b3c1e961a8ae056`、`ViewGroupReclaimWithFlipTime` `java-runtime-33b5cb01913d5d23292b` static `java-582906d19fd45149cfeb`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 116 条 records（既有 108 条字节不变 + 8 条 count 记录）、0 differences：`@Hint('reclaim_group_aged=30,reclaim_group_freq=5')` 的 `#groupwin(theString)#time(3000000)` 钉定调度计数 10→1→0（10 组各一个计时回调；回收+新组后余 1；undeploy-all 后 overall 归 0）；`reclaim_group_aged=5,freq=1` 的 keepall 10 槽×100 发送钉定迭代器计数 600→601（E0..E3 逐步回收，E4..E9 存活）；flipTime 语义（aged=1,freq=5）钉定 1→2→2 跨 aged 边界（4999 不扫、5000 扫、age 1 的 E2 存活）。Go 引擎：groupwin 链新增视图级回收清扫——@Hint aged/freq 触发的按组子状态清扫（不活跃严格大于 aged 即删；至多每频率窗口一次、挂靠于进入事件；AdvanceTime 单独不触发；被回收组静默 detach 无旧行——Go 时间窗惰性过期无需取消调度）。新增内省 API：Statement.ScheduleCount 与 Engine.ScheduleCountOverall 从窗口/模式/输出状态合成待决回调计数（每个仍有未来 deadline 工作的时间驱动窗口状态记 1——镜像 Java TimeWindowView 非空持把柄生命周期）。协议扩展：TraceRecord/Step 增 Count 字段与 schedule-count/iterator-count/schedule-count-overall 操作。该类累计 14/20 execution DV；剩余 7（编译消息）、8（性能门）、15/18（case.view-group-matrix 已 implemented）、17（表达式 groupwin）、19（SERDEREQUIRED）；manifest 更新为 628 cases、241 个 differential-verified case、891 个 differential runtime IDs、3518 条 associations（referenced 3266、unreferenced 870）；capability 120 个（37 DV）。

> 最新补充：Draft 4.347（2026-09-09），`view.window-core` 扩展 `case-viewgroup-merge-view` differential-verified 场景，对照固定 Java `ViewGroup.java` 追加分组时间窗 ords 10-13/16 五个 execution（`ViewGroupTimeBatch` `java-runtime-7bd36b6fe5567b066794` static `java-407b99bc298a0479afc2`、`ViewGroupTimeAccum` `java-runtime-842bde62118b9b8cae2d` static `java-1eaa11c89e08defabf22`、`ViewGroupTimeOrder` `java-runtime-806120fdd2130ab1f275` static `java-9857545a1f0866b93be2`、`ViewGroupTimeLengthBatch` `java-runtime-737a5f1ffd4c6a6c8924` static `java-7928a7ae8b34bf7979fa`、`ViewGroupTimeWin` `java-runtime-68ef8076bc96595cbfd0` static `java-dc624811d027aaa2ccdb`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 108 条 records（既有 85 条字节不变 + 23 条新记录）、0 differences：`select irstream *` over `#groupwin(symbol)#time_batch(10 sec)`（按组锚定调度，11000 批 [10,20]、21000 空重振仍发一次更新 new=null/old=[10,20]）、`#time_accum(10 sec)`（即发插入 + 整窗过期 old 对）、`#groupwin(groupId)#time_order(timestamp, 10 sec)`（timestamp 序释放 E2@12000/E4@12500）、`#time_length_batch(10 sec, 100)`（纯时间驱动）、`#time(10 sec)`（逐事件过期）。Go 零引擎改动：GroupWindow/GroupWindowKeys 内嵌 TimeBatch/TimeAccum/TimeOrder/TimeLengthBatch/TimeWindow 规格均已有，Engine.AdvanceTime 驱动（scenario `advance-time` op，RFC3339Nano），runner 时间格式化镜像 Java 可选节秒模式（整秒无小数、非零毫秒三位）。全量 internal/esper 套件无回归。该类累计 11/20 execution DV；剩余：2/3/9（视图级 reclaim + 调度计数内省）、7（编译消息）、8（性能门）、15/18（case.view-group-matrix 已 implemented）、17（表达式 groupwin）、19（SERDEREQUIRED）；manifest 更新为 628 cases、241 个 differential-verified case、888 个 differential runtime IDs、3515 条 associations（referenced 3263、unreferenced 873）；capability 120 个（37 DV）。

> 最新补充：Draft 4.346（2026-09-09），`view.window-core` 引擎适配单元扩展 `case-viewgroup-merge-view` differential-verified 场景，对照固定 Java `ViewGroup.java` 追加派生值统计视图 ords 1/4/5/6（`ViewGroupStats` `java-runtime-03ed11fd1e5c3a3d11c2` static `java-e4b54babfa2a09c22a65`、`ViewGroupCorrel` `java-runtime-74e476ad16bf62d4a9e0` static `java-de5c0d5eec6a2fd1f36f`、`ViewGroupLinest` `java-runtime-942d359f7bb1302684bb` static `java-a523ae3aedcca205e7c5`、`ViewGroupMultiProperty` `java-runtime-47877af7a1d114850723` static `java-608b22a211fb780bb332`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 85 条 records（既有 12 条字节不变 + 73 条新记录）、0 differences：groupwin 上的 #uni/#correl/#linest 派生值视图（四语句 price/volume × last3/all 平均矩阵含逐语句 ordered iterator 快照、跨组合并的相关系数 NaN→1.0 演进、linest slope/YIntercept NaN→100.0/49000.0 演进、多键 groupwin irstream datapoints 对与跨组有序快照）。validation-first spike 证明监听器路径无需行为改动；引擎面改动为：(1) 将 groupwin 隐式聚合分组注入从 aggregateBatch 的批内局部副本迁至部署时持久应用到语句聚合定义（否则分组聚合语句的 Snapshot/输出上限路径因 plan.groupBy 为空而返回空结果/错误分类——ord-6 快照由此修复）；(2) 加法式访问器：LinearRegression 补 DataPoints/N/XSum/YSum/SumXSq/SumYSq/SumXY/XAverage/YAverage/XVariance/YVariance/X-Y-StandardDeviationPop/Sample（Java BaseStatisticsBean 一次求和公式按相同操作顺序复刻，消除两次求和公式的 ULP 漂移差异），UnivariateStatistics 计算切换为同一一次求和公式（此前为两次求和均值偏差公式，ULP 漂移不同）；compat 差分协议新增非整数 JSON 数字数值化规范（Java minimaljson 对 ≥1e7 双精度写 5.0E7 科学计数、Go 写 50000000，解析值相等）。批准适配：Java select-* 行携带完整派生字段集，Go 以访问器投影同一统计面；ord-5 的 slope/YIntercept 之外的行内属性断言现已全部可表达并被 trace 覆盖。剩余 ords：2/3/9（视图级 reclaim + 调度计数内省）、7（编译消息）、8（性能门）、10-13/16/18（虚拟时间批次机制）、17（表达式 groupwin）、19（SERDEREQUIRED）。该类累计 6/20 execution DV；manifest 更新为 628 cases、241 个 differential-verified case、883 个 differential runtime IDs、3510 条 associations（referenced 3263、unreferenced 873；4 个 runtime 中 3 个已被 case.view-group-matrix 引用）；capability 120 个（37 DV）。

> 最新补充：Draft 4.345（2026-09-09），`view.window-core` 引擎单元新增 `case-viewgroup-merge-view` differential-verified 场景，对照固定 Java `ViewGroup.java` 的 ords 0/14（`ViewGroupObjectArrayEvent` `java-runtime-a3b6bef89e22a122cc7a` static `java-4c1562348946dab6dc09`、`ViewGroupLengthWin` `java-runtime-639bc9b69621f3b6a417` static `java-48fb0b51c8d6abc3e858`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 12 条 records、0 differences，闭环 4.334 期间 spike 发现的关键分歧：Go 的隐式聚合分组注入不再对 groupwin 上的普通 select 聚合触发（runtime.go 注入门去掉 aggregateDefinitionReadsNonKeyEvent 触发臂）——Java 的 groupwin 父视图是合并/联合点，`select p1,sum(p2) from ...#groupwin(p1)#length(2)` 保持单一非分组聚合于跨组联合之上（sp2 演进 10/21/33/36，组内驱逐 13−10），标量列读当前事件；同时 univariate 注入触发器扩展匹配 #correl/#linest 表达式（Java 将其绑定为 groupwin 的按组子视图，与 #uni 相同）。ViewGroupLengthWin 钉定分组 length 窗口保留与组内驱逐的 irstream 对。oracle 以 object-array 事件类型注册 OAEventStringInt（逐字节镜像套件注册）并使用 common 模块的 SupportBean。批准适配：Java 的 #uni/#correl/#linest 派生值子视图（ords 1/4/5/6，逐插入发布 irstream 旧行）在 Go 以分组聚合表达、旧行无可表达对应物，延后至派生值视图单元；reclaim ords 2/3/9 需视图级回收加调度计数内省面；时间窗 ords 10-13/16/18 需虚拟时间批次机制；ord 7 编译消息、ord 8 性能门、ord 19 SERDEREQUIRED 各自延后。全量 internal/esper 套件无回归（含既有 view-group 矩阵与 avg-per-sym DV）。该类累计 2/20 execution DV；manifest 更新为 627 cases、240 个 differential-verified case、879 个 differential runtime IDs、3504 条 associations（referenced 3262、unreferenced 874）；capability 120 个（37 DV）。

> 最新补充：Draft 4.344（2026-09-09），`query.insert-into-route` 以 implemented-not-DV 登记 `EPLInsertIntoEventPrecInvalid`（`java-runtime-eb5b556ae1a0a4b15822` static `java-a98a462920f6d64d6172`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）收尾该类至 11/11 全处置（10 DV + 1 implemented-not-DV）。按 invalidity 政策（无 trace 行）新增 Build 时 `validateEventPrecedence` 校验通路，横跨 route、split-branch 与 named-window-merge 三个路径：event-precedence 表达式必须返回 int（拒绝 string、null 指针、int16/short 等非整型，对齐 Java 的 Integer-only 规则），且当输出 schema 静态已知时只能引用输出事件的属性（对齐 Java 的 "considering only the result event itself and not incoming streams"）；子查询内部表达式按其自身源解析、不做遍历。批准差异：消息文本为引擎内部诊断；FAF 子 case 以 Go 消息拒绝（Java 文本不同）；表目标 precedence 因「路由目标须预注册」的既有理由拒绝；保留字解析错误在类型化 API 中不可表示。Go 引擎改动全量套件无回归。manifest 更新为 626 cases、239 个 differential-verified case、877 个 differential runtime IDs（不变）、3502 条 associations（referenced 3261、unreferenced 875）；capability 120 个（37 DV）。

> 最新补充：Draft 4.343（2026-09-09），`query.insert-into-route` 引擎单元扩展 `case.insertinto-event-precedence` differential-verified 场景，对照固定 Java `EPLInsertIntoEventPrecedence.java` 追加 ords 5-8 四个子查询驱动 precedence execution（`EPLInsertIntoEventPrecSubqueryOnSplitSODA` `java-runtime-9e3c8c45438c687465e7` static `java-f4c03617f60ee0545ce7`、`EPLInsertIntoEventPrecSubqueryInsertIntoSODA` `java-runtime-03a6d8b8db3ead5f09f1` static `java-fb9b48e8cd69b99e0c59`、`EPLInsertIntoEventPrecSubqueryMergeSODA` `java-runtime-e153c3dff68d79fbb048` static `java-650346dcf5e73c9412fd`、`EPLInsertIntoEventPrecSubqueryOnInsertSODA` `java-runtime-d021d80d0590fb79c637` static `java-41715255e7a576306856`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 79 条 records（既有 57 条字节不变 + 22 条新记录）、0 differences：四个路由上下文（on-split、insert-into、on-merge not-matched、on-select-from-window）重放同一契约——子查询为 null 时 FIFO a,b，`SupportBeanNumeric#lastevent` 预置后按 intOne/intTwo 逆序翻转（b,a），反向预置后还原（a,b）；Java 的 SODA 标志仅编译路径（harness 内文本往返），行为可观测相同。Go 引擎三缺口修复：(1) visitQueryExpressions 现访问 query.eventPrecedence、merge-action 与 split-branch precedence 表达式，使 precedence 子查询进入语句子查询运行时注册表；(2) evaluatePrecedenceExpr 携带语句附加变量（含注册表），路由与 split 调用点传入——此前子查询优先级静默降 0/FIFO；(3) TriggerQuery.Query 复制 spec.eventPrecedence（此前 on-select 路由丢弃）；完整性巡检另修复 OnDemand 与 Pattern 查询字面量缺复制（使 FAF "fire-and-forget routes do not allow event-precedence" 拒绝由死代码变为活路径；JoinQuery/Aggregate/From 路径原本已复制）。oracle 新增多模块部署约定与 @FAF 模块（ord-8 窗口预填充）、SupportBeanNumeric 本地镜像类（run script classpath 不含 regression-lib）。Go 引擎改动全量套件无回归。剩余：ord 10 invalid 子 case 1-4（precedence 表达式 Build 校验）与 6/7 无 Go 拒绝面处置；该类累计 10/11 execution DV；manifest 更新为 626 cases、239 个 differential-verified case、877 个 differential runtime IDs、3501 条 associations（referenced 3260、unreferenced 876）；capability 120 个（37 DV）。

> 最新补充：Draft 4.342（2026-09-07），`query.insert-into-route` 扩展 `case.insertinto-event-precedence` differential-verified 场景，对照固定 Java `EPLInsertIntoEventPrecedence.java` 追加 ords 4/9 两个 execution（`EPLInsertIntoEventPrecNonConstInsertIntoContainedEvent` `java-runtime-e99c72ba6838bd3b23f2` static `java-54f031b12e38a8768102`、`EPLInsertIntoEventPrecConstantInsertIntoOutputRate` `java-runtime-aa3e3052e879188342c6` static `java-facdb3c80027dcca2998`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 57 条 records（既有 21 条字节不变 + 36 条新记录）、0 differences：两级包含事件路由（`LvlA[b]` 进 LvlB、`LvlB[c]` 进 LvlC）各携带针对被路由目标事件求值的非常量 event-precedence，链式路由经共享优先级队列交错，五次 LvlA 发送的 LvlC id 展开序逐条钉定（C,B,D,A / C,A,B,D / A,B,C,D / H,D,A,G,F,B,I,E,C / B,G,H,D,F,A,C,I,E）；三条 `output every 2 events` 路由（precedence 1/2/3、各语句 id 偏移 1/2/3）两次发送后单批次按语句间优先级、语句内 FIFO 输出 13,23,12,22,11,21（Java 的 `computeEventPrecedence(3, *)` 静态调用对路由输出事件求值恒为常量 3，Go 直接钉常量——批准适配）。Go 零引擎工作；oracle 以镜像类与本地 computeEventPrecedence 注册（run script classpath 不含 regression-lib，形状与语义逐字节复制套件）。延后：ords 5-8（子查询驱动 precedence）需引擎单元（visitQueryExpressions 不注册 eventPrecedence/split/merge precedence 子查询、evaluatePrecedenceExpr 缺语句变量致子查询优先级静默降 0、TriggerQuery.Query 丢弃 spec.eventPrecedence 三个具体缺口）；ord 10 的 invalid 子 case 拆分为该引擎单元（precedence 表达式校验 1-4）与无 Go 拒绝面处置（表目标 precedence、保留字解析错；FAF 子 case 5 已由 faf 拒绝测试覆盖）。该类累计 6/11 execution DV；manifest 更新为 626 cases、239 个 differential-verified case、873 个 differential runtime IDs、3497 条 associations（referenced 3256、unreferenced 880）；capability 120 个（37 DV）。

> 最新补充：Draft 4.341（2026-09-07），`epl.insertinto.populate` 以 implemented-not-DV 登记 `EPLInsertIntoInvalid`（`java-runtime-6ff1f053ec9bfb43434b` static `java-0f8e12979444ffc17f49`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）收尾该类至 13/13 全处置（12 DV + 1 implemented-not-DV）。按既定 invalidity 政策（无 trace 行），Go Build/send 拒绝测试钉定可复现子 case：未知列（具名 dummyField 与裸未名列）、接口层级不匹配、setter 抛错经 Send 浮出（制造错误逐字断言）、错误类型转换 either-behavior 在运行中观测为 Go 侧错误（Java 明确包络内）、同构 schema 投影正例部署成功；依赖未建模概念的子 case 登记为批准差异（long→int 与 null→int 类型不匹配消息——Go 做转换、构造器未找到与构造器抛错、ABCStream/xmltype 自动声明的 insert-into 类型、null 类型列、xmltype XMLDOM 前置；MyMap(dummy) 属性未找到——Go map 目标开放，该 Java 拒绝在 map 目标上无 Go 对应物）。Go 零引擎工作；manifest 更新为 626 cases、239 个 differential-verified case、871 个 differential runtime IDs（不变）、3495 条 associations（referenced 3254、unreferenced 882）；capability 120 个（37 DV）。

> 最新补充：Draft 4.340（2026-09-07），`epl.insertinto.populate` 扩展 `case.epl-insert-into-populate-underlying` differential-verified 场景，对照固定 Java `EPLInsertIntoPopulateUnderlying.java` 追加 ords 9/10 的 valid 半（`EPLInsertIntoArrayPOJOInsert` `java-runtime-56175b64c415bf33ddf1` static `java-bfd55528c9aca96a5ffe`、`EPLInsertIntoArrayMapInsert` `java-runtime-facd4a0c64ae48571ee6` static `java-3980f002801d7b6b6d3b`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 30 条 records（既有 26 条字节不变 + 4 条新记录）、0 differences：`until timer:interval(10 sec)` 终止的 pattern（`every s=S0 -> e=SB(theString=s.p00) until ...`）在虚拟时间推进后单行路由起始捕获与重复捕获数组（startEvent id=1/p00=G1；endEvent len 2 intPrimitive 2,3）——oracle 新增无记录 advanceTime op（与套件 env.advanceTime 同一调用），Go 以 Engine.AdvanceTime 表达；ord 9 的 POJO 目标按套件 public static 本地类 FQN 注册；ord 10 循环 objectarray/map/default 三表示（object-array 事件按位置 underlying 发送，map 族按字段 map；Avro/JSON leg 未建模，规范化路由行跨表示相同，与套件逐表示断言同值一致）。Java 的两个 invalid 编译半为 implemented-only Go 测试：数组列入非数组属性在 Build 拒绝；单事件入数组属性在 Build/部署被接受但路由执行时拒绝（timer 推进返回 Java 在编译期报告的 TypeMismatch）。Go 零引擎工作。该类累计 12/13 execution DV，仅剩 ord 12（invalidity implemented-only）；manifest 更新为 626 cases、239 个 differential-verified case、871 个 differential runtime IDs、3494 条 associations（referenced 3253、unreferenced 883）；capability 120 个（37 DV）。

> 最新补充：Draft 4.339（2026-09-07），`epl.insertinto.populate` 扩展 `case.epl-insert-into-populate-underlying` differential-verified 场景，对照固定 Java `EPLInsertIntoPopulateUnderlying.java` 追加 ords 7/8 两个 execution（`EPLInsertIntoCharSequenceCompat` `java-runtime-f04a53b7cbe0320f681f` static `java-2c966eacbdd140219337`、`EPLInsertIntoBeanFactoryMethod` `java-runtime-e79b48b60120e27bafbc` static `java-7444c102966e032f2884`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 26 条 records（既有 24 条字节不变 + 2 条新记录）、0 differences：charsequence-compat 为零事件部署冒烟——`create schema ConcreteType as (value java.lang.CharSequence)` + `insert into ConcreteType select "Test" as value` 在 objectarray/map/default 三表示下必须编译且部署成功、流保持静默（Java 另有 Avro leg，map 基 oracle 未建模；Go 的 JSON 注册为超集不另钉）；factory-method 以普通结构体注册表达 Java 工厂方法配置的目标（批准适配，填充可观测相同）——`insert into SupportBeanString select 'abc' as theString`（Java 以 listener+subscriber 双面断言，Go 单订阅观测同一路由行）与 5 列 sensor 投影（int 字面量加宽进 double 列）。Java 的两个无默认构造器反射检查属支持类内省而非引擎行为，延后为非引擎项。Go 零引擎工作。该类累计 10/13 execution DV，仅剩 ord 9/10（until-pattern 按表示数组填充，invalid 半 implemented-only）与 ord 12（invalidity implemented-only）；manifest 更新为 626 cases、239 个 differential-verified case、869 个 differential runtime IDs、3492 条 associations（referenced 3251、unreferenced 885）；capability 120 个（37 DV）。

> 最新补充：Draft 4.338（2026-09-07），`epl.insertinto.populate` 扩展 `case.epl-insert-into-populate-underlying` differential-verified 场景，对照固定 Java `EPLInsertIntoPopulateUnderlying.java` 追加 ords 0/1/2/11 四个 execution（`EPLInsertIntoCtor` `java-runtime-706d3ed19b9a9b680430` static `java-a12a755abb439b882d37`、`EPLInsertIntoCtorWithPattern` `java-runtime-42b687e10ab403b319bd` static `java-6842f27f1803589c55ed`、`EPLInsertIntoBeanJoin` `java-runtime-43b50e297c1593da8ec3` static `java-f4cea06a7d5a2310de97`、`EPLInsertIntoWindowAggregationAtEventBean` `java-runtime-5c663b3a17e6e4aac880` static `java-92ef442cc9f141361514`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 24 条 records（既有 11 条字节不变 + 13 条新记录）、0 differences：构造器位置填充矩阵——4 列投影进 (String,Integer,int,boolean) 形（含 null String 与 null boxed）、装箱值位置性落入原生槽（theString=E1、intBoxed=null、intPrimitive=100）、lastevent join 通配与流名投影（assertSame 身份经路由保持）、列清单被忽略的构造器默认（intPrimitive=99，Go 以 WithJSONDefaults 表达）、同型构造器经双过滤 lastevent 流、`[2]` 重复 pattern 捕获（MatchUntil(2,2)+TagEvents 进 []Event 目标，st1 ids [E1,E2]）、`window(*) @eventbean` 经 Aggregate(WindowEvents()) over KeepAll 按插入序路由全保留窗口。批准适配：Java 构造器/工厂目标以 Go 结构体目标按名投影表达；列清单忽略语法与 stage-3 全限定名目标无可观测差异。Go 零引擎工作。多 stage 执行按 Java undeploy 边界拆分为新 runtime 子 case。延后：ord 7（CharSequenceCompat 部署冒烟）+ord 8（工厂方法可观测半）小单元、ord 9/10（until-pattern 按表示数组填充，invalid 半 implemented-only）、ord 12（invalidity，implemented-only Build-error 单元限可复现子 case）。该类累计 8/13 execution DV；manifest 更新为 626 cases、239 个 differential-verified case、867 个 differential runtime IDs、3490 条 associations（referenced 3249、unreferenced 887）；capability 120 个（37 DV）。

> 最新补充：Draft 4.337（2026-09-07），`expr.filter.optimizable` 扩展 `case.expr-filter-optimizable-boolean-rebool` differential-verified 场景，对照固定 Java `ExprFilterOptimizableBooleanLimitedExpr.java` 追加 ords 1/4 两个 execution（`ExprFilterOptReboolMixedValueRegexpRHS` `java-runtime-f6199a434eee24910555` static `java-04dfad86bf629b059d7c`、`ExprFilterOptReboolNoValueExprRegexpSelf` `java-runtime-c21cb50c065d46513b04` static `java-a8b7e007999a8eff5613`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 34 条 records（既有 29 条字节不变）、0 differences：mixed-value-regexp-rhs 为 6 语句块——常量变量（`create constant variable string MYVAR = '.*abc.*'` + `theString regexp MYVAR`）、context 值（`context MyContext ... theString regexp context.s0.p00`）、every leg pattern tag（`pattern[s0=SupportBean_S0 -> every SupportBean(theString regexp s0.p00)]`）与常量拼接（`theString regexp '.*' || 'abc' || '.*'`）四个监控语句在 S0(1,".*abc.*") 启动后对 SB("xabsx") 全假、SB("xabcx") 全真；no-value-expr-regexp-self 为自比较 `p00 regexp p01`（S0(1,"abc",".*c") 双真、S0(2,"abc",".*d") 双假）。Go 零引擎工作：`RegisterVariable(name, value, ConstantVariable())` + `VariableRef[string]`、`ContextPatternField` + `WithContext`（4.336 既有 pattern-start 适配）、`PatternFrom(...).Then(...).Every()` + `TagField`、`Concat`；Java 的 6 语句单部署以 Go 分离部署等价表达（filter service 为 runtime 级），Java 共享 REBOOL 索引/值的 plan 断言为引擎内部范畴照例出域。该类累计 12/14 execution DV，仅剩 ord 2（墙钟主断言，EXCLUDEWHENINSTRUMENTED）与 ord 11（intentionally-different，compile-only plan forge）；manifest 更新为 626 cases、239 个 differential-verified case、863 个 differential runtime IDs、3486 条 associations（referenced 3245、unreferenced 891）；capability 120 个（37 DV）。

> 最新补充：Draft 4.336（2026-09-07），`expr.filter.optimizable` 扩展 `case.expr-filter-optimizable-boolean-rebool` differential-verified 场景，对照固定 Java `ExprFilterOptimizableBooleanLimitedExpr.java` 追加 5 个 execution（ordinals 6/7/8/12/13：`ExprFilterOptReboolContextValueDeep` `java-runtime-273669bcae26e6ec054d` static `java-cfeb9cb387bbcf07e580`、`ExprFilterOptReboolContextValueWithConst` `java-runtime-1da34c09fa675957b063` static `java-6f1b75c3a4b2d081eef2`、`ExprFilterOptReboolPatternValueWithConst` `java-runtime-11f60c98a851b822d97f` static `java-c68e1b5ed06f5dbaa49a`、`ExprFilterOptReboolDuplicateLike` `java-runtime-0f83ba904d5457bbc6d2` static `java-6cafa38b41cc5bb01851`、`ExprFilterOptReboolDuplicateRegexp` `java-runtime-5253f42525fa48e8784c` static `java-636dd440a66dbb237561`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`）。Java/Go trace 29 条 records（14 条既有记录字节不变 + 15 条新记录）、0 differences：context 值读取进入 filter regexp 操作数（`p10 regexp p11 || context.s0.p00` 与 `p10 || 'abc' regexp context.s0.p00`，S0(1,".*X")/S0(1,"x.*abc") 启动分区后 S1 三条/两条钉定匹配矩阵）；pattern tag 值进入 followed-by filter 操作数（`pattern[s0=SupportBean_S0 -> SupportBean_S1(p10 || 'abc' regexp s0.p00)]`）；重复 like 与 not-regexp 合取守卫（`theString like '%' and theString like '%'`、`regexp "test.*" and not regexp ".*\\.gov" and not regexp ".*\\.org"`——双引号 EPL 字面量与 `\\.` 转义逐字保留）。批准适配：Java filter 启动的 context（start 流与消费语句流不同）在 Go 以 pattern-initiated context 表达（Go filter-start 仅在消费语句自身流事件上求值，pattern-start 状态接收全事件；分区生命周期与 context 值读取可观测等价），引擎级 filter-start context 列入 backlog；scenario 的 create-context 语句加 `@name('ctx')`（Java 自动命名 stmt-0 会落入 oracle 的 s* 监听规则）。Go 零引擎工作：`RegexpMatch` + `Concat`/`Literal`、`ContextPatternField[string]("s0","p00")` + `WithContext`、`TagField`、`Like`、`Not` 全部既有 API 组合。延后：ord 1+4（MixedValueRegexpRHS 6 语句块与 NoValueExprRegexpSelf，N+2 单元）；ord 2（墙钟 delta<1000 主断言，EXCLUDEWHENINSTRUMENTED，行为核心与已验证 ord 0 重复）；ord 11 intentionally-different（compile-only SupportFilterPlanHook plan forge，先例 case.expr-filter-optimizable-value-limited-disqualify）。该类累计 10/14 execution DV；manifest 更新为 626 cases、239 个 differential-verified case、861 个 differential runtime IDs、3484 条 associations（referenced 3243、unreferenced 893）；capability 120 个（37 DV）。

> 最新补充：Draft 4.335（2026-09-07），`epl.other.distinct` 新增 `case-epl-other-select-expr-stream-selector` differential-verified 场景，对照固定 Java `EPLOtherSelectExprStreamSelector.java` 的 alias-with-properties 对（ordinals 8/9：`EPLOtherNoJoinWithAliasWithProperties` `java-runtime-89123cf55af0a7f5987e` static `java-b5352434faea014585a8`、`EPLOtherJoinWithAliasWithProperties` `java-runtime-b53494cb36a6b54c2c6f` static `java-9682b92ad00f7295162d`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []）。Java/Go 各 2 条 records、0 differences：无 join 的 stream-dot 别名选择（`select theString.* as s0, intPrimitive as a, theString.* as s1, intPrimitive as b from SupportBean#length(3) as theString`——`s0`/`s1` 经事件信封渲染为嵌套事件行，`a`/`b` 为普通属性别名）与 length-keepall 内连接上的混合选择（`select intPrimitive, s1.* as s1stream, theString, symbol as sym, s0.* as s0stream`——`SelectSourceEvent` 渲染两侧源事件行；首条 bean 发送因内连接未完成不产出记录）。typed Go 零引擎工作：`Alias(name, EventValue[esper.Event]())` 做 stream-dot 整事件别名，`JoinMany(JoinSource ×2).Select(SelectFrom(...), SelectSourceEvent(...))` 做混合投影。Java oracle 与 run script 已在前一单元预提交（`b110f3f5d`，端到端 3 次字节一致）；本单元重建 scenario 后重跑 oracle，输出与 committed Java trace 逐字节一致。场景 9 步（4/5）、listener-only，严格 validator 固定逐字 EPL/发送序列/payload 值（MD feed 以指针字段严格区分 `""` 与 null），9 种 raw 畸变 + 6 种 trace 变异全部拒绝；manifest 更新为 626 cases、239 个 differential-verified case、856 个 differential runtime IDs、3479 条 associations（referenced 3238、unreferenced 898）；capability 120 个（37 DV）。该文件累计 12/17 execution DV（ords 4-15）；其余：ord 0/16（invalid 编译，无 Go 拒绝面）、ord 1/2（insert-into transpose 路由与 pattern 起源 insert-from，延后）、ord 3（SODA object model join alias，编译器域）。

> 最新补充：Draft 4.334（2026-09-07），`epl.other.distinct` 新增 `case-epl-other-stream-expr` differential-verified 场景，对照固定 Java `EPLOtherStreamExpr.java` 的 5 个 stream method expression execution（ordinals 1/4/5/6/7：`EPLOtherStreamFunction` `java-runtime-67e9ea0d239585623711` static `java-e571ee83c24576b8aba7`、`EPLOtherStreamInstanceMethodAliased` `java-runtime-cc45d135a75bb01736f0` static `java-b448cd11a74aaf56dbab`、`EPLOtherStreamInstanceMethodNoAlias` `java-runtime-469a37a746e59d25a115` static `java-572869619d74ba3c95be`、`EPLOtherJoinStreamSelectNoWildcard` `java-runtime-a59b12bbe5788257c37b` static `java-534aeb16b8d33707670a`、`EPLOtherPatternStreamSelectNoWildcard` `java-runtime-827ea8daeea9baec40cf` static `java-dfb3493bb3eb08a778db`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []）。Java/Go 各 10 条 records、0 differences：静态方法 where 过滤（`volumeGreaterZero(s0)`/`(*)`/`EventBean(s0)`/`EventBean(*)` 四种参数形式，零体积事件被过滤、体积 100 事件触发全四条）；keepall 内连接上流对象列（`select s0 as s0stream, s1 as s1stream` 与无别名 `select s0, s1`，经 `SelectSourceEvent` 渲染为事件行）；别名（`s0.getVolume() as volume, s0.getPriceTimesVolume(2) as pvf`）与逐字无别名（`s0.getVolume(), s0.getPriceTimesVolume(3)`）实例方法投影；followed-by pattern（`every e1=MD -> e2=Bean(compareEvents(e1, e2))`）静态 UDF 过滤引用先前 tag。Go Method 表达式经 `Method[T](EventValue[T](), ...)` 事件自身调用，`Func1[esper.Event, bool]` + `EventValue[esper.Event]()` 做 where 过滤，`Property[string](PatternEvent(tag), name)` 做 pattern tag 导航，`SelectSourceEvent` 做 stream-as-object 列。延后：ord 0（chained parameterized — chained Method resolution 返回 missing，需引擎工作）、ord 2/3（absent join side 返回 missing 而非 null）、ord 8（invalid 编译，无 Go 拒绝面）。场景 30 步（8/4/7/6/5）、listener-only，严格 validator 固定逐字 EPL（含尾随空格）/发送序列/payload 值，8 种 raw 畸变 + 7 种 trace 变异全部拒绝；manifest 更新为 625 cases、238 个 differential-verified case、854 个 differential runtime IDs、3477 条 associations（referenced 3236、unreferenced 900）；capability 120 个（37 DV）。

> 最新补充：Draft 4.333（2026-09-07），`infra.namedwindow.views` 新增 `case.infra-nwtable-subq-correl-coerce` differential-verified 场景，对照固定 Java `InfraNWTableSubqCorrelCoerce.java` 全部 8 个 execution（`InfraNWTableSubqCorrelCoerceSimple` 8 种 namedWindow/enableIndexShareCreate/disableIndexShareConsumer/createExplicitIndex flag 组合；runtime `java-runtime-0f3833a87dcd9b209935`、`f1f621deec2314404f2a`、`4f850a69b870e08ebfd5`、`1616d20fc73fe4be679c`、`0422895f8568f52e0956`、`9161da818506b8194079`、`3a94283dcac4e9ae4dea`、`ce6e53f117adccdf9f59`；shared static `java-5a8fbdfa647a5bef3660`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []）。Java/Go 各 56 条 records、0 differences：双 Map schema（EventSchema e0/e1/e2、WindowSchema col0/col1/col2）上 `col2 = es.e2` 字符串相等与 `col1 = es.e1` int→long 强转相等的相关标量子查询，跨隐式子查询索引、`enable_window_subquery_indexshare` 共享索引、`disable_window_subquery_indexshare` 消费 hint 与双键 `create index MyIndex (col2, col1)` 显式索引全部 8 种组合——hint 对该形状 plan-inert，8 个 execution trace 逐字节一致，索引选择路径经 scenario 元数据钉定。mid-case `undeploy(s0)` + redeploy 后的第 7 条记录（E6 再次命中 W4）证明子查询索引在语句启动时从既有 infra 内容重建。typed Go 以 `SubqueryValueWithOptions(col0, SubqueryWhere(And(Equal(col2, OuterField(e2)), EqualOf(Field[any,int64](col1), OuterField[int](e1)))))` 构建——EqualOf 的 big.Rat 比较覆盖 int→long 强转，引擎子查询索引探针收集器识别 equal-of 并对探针值做列型强转；多键 `CreateIndex("MyIndex", []string{"col2","col1"}, IndexHash, false)` 与 selective undeploy/redeploy harness op 均有先例。场景 176 步（非索引 21/索引 23）、严格 validator 固定逐字 EPL/发送序列/payload 值/undeploy 目标，10 种 raw 畸变 + 7 种 trace 变异全部拒绝；manifest 更新为 624 cases、237 个 differential-verified case、849 个 differential runtime IDs、3472 条 associations（referenced 3231、unreferenced 905）；capability 120 个（37 DV）。

> 最新补充：Draft 4.332（2026-09-07），`resultset.aggregate-filtered` 新增 `case.resultset-aggregate-filter-named-parameter-sorted-join` differential-verified 场景，对照固定 Java `ResultSetAggregateFilterNamedParameter.java` 的 3 个 sorted 缺口 execution（ordinals 15/17/18：`ResultSetAggregateAccessAggSortedBound{join=true}` `java-runtime-6b6e0d2290261cb8cd93` static `java-ea6830fd215ba36a098b`、`ResultSetAggregateAccessAggSortedUnbound{join=true}` `java-runtime-236d99a9d77ed3932510` static `java-d726134be3446919675e`、`ResultSetAggregateAccessAggSortedMulticriteria` `java-runtime-398a780be4d755650285` static `java-a26ba8e7bb8f4e6c9f2d`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []）。Java/Go 各 18 条 records、0 differences：last-event join（`SupportBean_S1#lastevent` + `SupportBean#length(4)`/`#keepall`）上的 `maxby/minby(intPrimitive, filter:theString like 'A%'|'B%').theString` 与 `maxbyever/minbyever(...)` 字符串投影（9 条 bound 记录逐条钉定窗口驱逐，含 B1 离开 length(4) 使 bSorted 收缩到 [b2] 与空过滤集渲染 null）；`sorted(intPrimitive, doublePrimitive, filter:...)` 双键 multicriteria 升序排序（intPrimitive 1==1 平局由 doublePrimitive 4<10 决出 [b2,b1]）。join 的 sorted 事件数组列必须经 `SortedEventsBy[esper.Event](JoinEventValue[esper.Event](1), key, false)` 读取 bean 成员（裸 SortedEvents 会返回空 schema 的 joinTuple 行）；maxby/minby 组合为 `FilterAggregate[string](MaxBy/MinBy(JoinField, key), filter)`，MaxByEver/MinByEver 为变长键。SupportBean schema 三案例统一携带 doublePrimitive（join case 发送钉 Java 2 参 sendEvent 默认 -1；时间线内 double 均为整数值，两侧均渲染为整型 JSON 数字）。场景 29 步（13/9/7）、listener-only，严格 validator 固定逐字 EPL/发送序列/payload 三字段值，10 种 raw 畸变 + 8 种 trace 变异全部拒绝；manifest 更新为 623 cases、236 个 differential-verified case、841 个 differential runtime IDs、3464 条 associations（referenced 3223、unreferenced 913）；capability 120 个（37 DV）。延后：ord 19（audit/reuse）、ord 20（invalid 编译，无 Go 拒绝面，批准差异）。

> 最新补充：Draft 4.331（2026-09-07），`resultset.aggregate-filtered` 新增 `case.resultset-aggregate-filter-named-parameter-linear-join` differential-verified 场景，对照固定 Java `ResultSetAggregateFilterNamedParameter.java` 的 3 个 join/mixed-filter 缺口 execution（ordinals 9/11/13：`ResultSetAggregateAccessAggLinearBound{join=true}` `java-runtime-d652fb38c70d8bcc778f` static `java-85b8ba64c1be948a5676`、`ResultSetAggregateAccessAggLinearUnbound{join=true}` `java-runtime-4add988d1015cdae8bb2` static `java-97884ae57325e3acd3f2`、`ResultSetAggregateAccessAggLinearBoundMixedFilter` `java-runtime-97101f0ae69f6fad8c29` static `java-9d0ccec4c0bb19e6a14c`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []）。Java/Go 各 19 条 records、0 differences：join（`SupportBean_S1#lastevent` + `SupportBean#length(5)`/`#keepall`）上的过滤 access 聚合 `first/last/window(intPrimitive, filter:theString like 'A%'|'B%')` 与 `firstever/lastever/countever(...)`（窗口驱逐经 11 条 listener 记录逐条钉定，kickoff `SupportBean_S1(0)` 对空 bean 侧 join 不产出记录）；keepall 上 `window(sb, filter:A%)/window(sb)/window(filter:B%, sb)` 事件数组投影渲染为事件行。关键机制：join 聚合的组行是 joinTuple，FilterAggregate 以每个组事件为 ctx.Event 评估谓词，JoinField 的 joinTuple 回退路径即完成 bean 侧绑定——实现前 spike 零引擎工作解决。typed Go 构建 `JoinMany(...).Aggregate(Alias(FilterAggregate(First/Last/WindowValues/FirstEver/LastEver/CountEver(JoinField))))` 与 `From(...).Window(KeepAll).AsRecord().Aggregate(Alias(FilterAggregate(WindowEvents())))`。场景 30 步（15/9/6）、listener-only，严格 validator 固定逐字 EPL/发送序列/payload 值，11 种 raw 畸变 + 9 种 trace 变异全部拒绝；manifest 更新为 622 cases、235 个 differential-verified case、838 个 differential runtime IDs、3461 条 associations（referenced 3220、unreferenced 916）；capability 120 个（37 DV）。延后：ord 15/17/18（sorted join + 双键 multicriteria）、19（audit/reuse）、20（invalid 编译，无 Go 拒绝面，批准差异）。

> 最新补充：Draft 4.330（2026-09-07），`infra.namedwindow.views` 新增 `case.infra-named-window-insert-from` differential-verified 场景，对照固定 Java `InfraNamedWindowInsertFrom.java` 7 个 execution 中的 4 个（ordinals 0/1/5/6：`InfraCreateNamedAfterNamed` `java-runtime-b3f6cb7b36c5211c8822` static `java-b0917219c462cba9d770`、`InfraInsertWhereTypeAndFilter` `java-runtime-e601b3cc7f827d578185` static `java-cff4a3193da2b063a351`、`InfraNamedWindowInsertLenientPropCount{rep=MAP}` `java-runtime-46011542d6e9d34a87f5` 与 `{rep=OBJECTARRAY}` `java-runtime-8138dd777290d00417d1` 共用 static `java-282d7b64878866ea4709`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []）。Java/Go 各 18 条 records、0 differences：type-by-window 创建（`create window MyWindowTwo as MyWindow` 无 insert 关键字、无种子）与共享 insert 交付（windowOne 全行记录 + selectOne 仅 theString 投影）；insert-from 核心——create-window-insert 种子（纯 keepall、`insert where theString like 'A%'` 过滤 keepall、`#unique(intPrimitive)` 部署时按 id 去重保留末条 C3/C5）的部署时行复制绝不抵达 attach-after-deploy 的 create 语句 listener，经 ordered/mode-any 快照验证（A1..C5、A1/A4、C3/C5），四个逐窗口过滤路由 insert（B9→Two、A8→IWT、C7→Three、D6→Four）各交付恰一条 listener 记录，证实种子是一次性复制、持续流转必须显式 insert-into；lenient 部分列 insert（两条 insert 各供一列、另一列 null 填充）跨 MAP 与 OBJECTARRAY schema 快照为有序 c0/c1 对。typed Go 的种子是 create 部署步的 harness 映射——源语句 Snapshot + Underlying + engine.InsertNamedWindow（retention 去重自然生效；where 种子过滤为 theString 首字符检查，对本场景 payload 集精确）——被种子的 create 语句以普通 `FromNamedWindow(...).Query(...)` 消费者部署从而种子行绝不触及 attach-after-deploy listener，持续流转用 `OnEvent(...).Filter(Like).InsertIntoNamedWindow(CopyMatchingFields)` 路由，lenient schema 以 `NewMapSchema`/`NewObjectArraySchema` 指针 c1 字段 null 精确渲染；场景 50 步（7/23/10/10）、严格 validator 固定每 case 步骤交错/逐字 EPL/payload 值/snapshot 目标与模式（ordered vs any），12 种 raw 畸变 + 10 种 trace 变异全部拒绝；manifest 更新为 621 cases、234 个 differential-verified case、835 个 differential runtime IDs、3458 条 associations（referenced 3217、unreferenced 919）；capability 120 个（37 DV）。延后：ord 2（OM toEPL 往返 + 4 表示矩阵，其本体不可移植）、ord 3（invalid 编译，Go 无拒绝面）、ord 4（variant stream + id? 投影未证实）为批准差异/延后项。

> 最新补充：Draft 4.329（2026-09-07），`infra.namedwindow.views` 新增 `case.infra-named-window-on-update-misc` differential-verified 场景，收尾固定 Java `InfraNamedWindowOnUpdate.java` 全部 8 个 execution（本 case 覆盖剩余 4 个：`InfraUpdateNonPropertySet` `java-runtime-5d041a3958410a90fa9a` static `java-26306de901985e70904f`、`InfraSubclass` `java-runtime-9ea51cb84d163be79693` static `java-d920e63f2081f1d72d2f`、`InfraUpdateCopyMethodBean` `java-runtime-43feff597147c7867ca7` static `java-6eb1ea7dc80c3078f47f`、`InfraUpdateWrapper` `java-runtime-09f56f49bab53388bb2d` static `java-6b566143dbbb00ea202b`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；javaFlags 钉 []（runtime inventory 为准，static-manifest 对全文件过近似地标 SERDEREQUIRED，仅 InfraUpdateCopyMethodBean 在 Java 中真正声明且对 Go harness 无影响））。Java/Go 各 5 条 records、0 differences：非属性 set 子句更新（`set mywin.setIntPrimitive(10), setBeanLongPrimitive999(mywin)`——Java oracle 以 suite 一致的 plugin single-row function 注册逐字编译，Go 以批准差异落地为字面量赋值）在 update 语句自身交付 old/new 对（投影 intPrimitive,longPrimitive 后 new 10/999、old 1/0）；子类更新将 SupportBean 触发路由进 SupportBeanAbstractSub 类型窗口（Go 以折叠结构体 + 指针字段 null 精确渲染 v1），create 消费者携带 update 的 old 前像（Java 默认 irstream 交付）；copy-method bean 更新在字段级拷贝批准差异下运行（无需拷贝钩子）；wrapper-select 窗口（`select *, 1 as p0`）在 schema 声明 p0，插入用 CopyMatchingFields + SetColumn p0，快照投影 theString,p0。wrapper 的 create+insert 为单一合并 deploy 步（探针证实：固定树上 wildcard+额外属性的 create-window-as-select 不在公共类型注册，分离 insert 模块无法解析；与 Java 套件单模块编译一致）。快照行两侧对称投影到 Java 断言字段，mode-any 快照以差分规范行序输出。场景 29 步、严格 validator 固定每 case 步骤交错/逐字 EPL（含 5 空格缩进）/payload 值/snapshot 目标，11 种 raw 畸变 + 8 种 trace 变异全部拒绝；manifest 更新为 620 cases、233 个 differential-verified case、831 个 differential runtime IDs、3454 条 associations（referenced 3213、unreferenced 923）；capability 120 个（37 DV）。InfraNamedWindowOnUpdate.java 至此 8/8 execution 全部 differential-verified。

> 最新补充：Draft 4.328（2026-09-07），`infra.namedwindow.views` 新增 `case.infra-named-window-on-update` differential-verified 场景，对照固定 Java `InfraNamedWindowOnUpdate.java` 8 个 execution 中的 4 个（ordinals 1/2/6/7：`InfraMultipleDataWindowIntersect` `java-runtime-f921cf2543cbb10b4150` static `java-b0afef60c0bc90fc51d8`、`InfraMultipleDataWindowUnion` `java-runtime-0947ec873ea298b60209` static `java-a5f1d32f26cb39f39266`、`InfraUpdateMultikeyWArrayPrimitiveArray` `java-runtime-9870ee394d3099ef85e8` static `java-0b7af7284d24833e1753`、`InfraUpdateMultikeyWArrayTwoFields` `java-runtime-a25a18f2754aeae08015` static `java-cdea340e2ff2cabfd0f3`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；全部 flags 空，runtime inventory 为准，static-manifest 对全部 8 个 execution 过近似地标了 SERDEREQUIRED）。Java/Go 各 10 条 records、0 differences：intersect（`#unique(theString)#length(2)`）与 retain-union 组合窗口上的 on-update（`set intPrimitive=intPrimitive*100 where theString=id`）在 create 语句 listener 上交付 update old/new——new 为更新后行（E2/300）、old 为更新前行（E2/3），其前是两次 insert 交付（逐次 sequence 1-3）；multikey-with-array 更新经单字段与双字段 `int[]` 深相等 where 匹配（`mw.intOne = sewia.array`，`mw.id = sewia.id and mw.intOne = sewia.array`），覆盖空数组相等（E4 匹配 U3/12）与不匹配行原样不动（IDX/加长数组），以窗口快照断言（E1=13/E2=10/E3=11/E4=12；ID1=12/ID2=10/ID3=11）。快照行两侧对称投影到 Java 断言字段（theString,intPrimitive / id,value——27 属性全行渲染不是钉定契约），mode-any 快照以差分协议规范行序输出（升序 marshal-fields 串），union 快照保持 Java 测试的 theString 有序 pin；Java oracle 本地镜像两个 SupportEvent*Array bean（regression-lib 不在 classpath，且 bean 型 on-update copy-on-write 要求 Serializable），create EPL 带 `@public`（oracle 分模块部署的跨模块可见性惯例）。typed Go 使用 `CreateNamedWindow` + `NamedWindowRetention(IntersectWindows/UnionWindows(Unique,LengthWindow))`/`KeepAll`、`OnEvent(...).InsertIntoNamedWindow(CopyMatchingFields)`、`OnEvent(...).UpdateNamedWindow`（`NamedWindowField` 对 `Field` 谓词：标量 `Equal`、数组 `EqualOf`）与 `SetColumn` 赋值、`statement.Snapshot` 迭代读取；Go union-update 组合保留分支（updateCompositeWhereState union path）首次获得差分覆盖。场景 46 步、严格 validator 固定每 case 步骤交错/EPL/payload/snapshot 模式（union 有序、其余 mode any），11 种 raw 畸变 + 8 种 trace 变异全部拒绝；manifest 更新为 619 cases、232 个 differential-verified case、827 个 differential runtime IDs、3450 条 associations（referenced 3209、unreferenced 927）；capability 120 个（37 DV）。剩余 executions 0/3/4/5（方法调用 set 子句、子类 typing、copy-method 配置、wrapper select）延后至后续单元并登记批准差异适配。

> 最新补充：Draft 4.327（2026-09-07），`infra.namedwindow.views` 扩展 `case.infra-nwtable-subq-filtered-correl` differential-verified 场景，对照固定 Java `InfraNWTableSubqFilteredCorrel.java` 全部 7 个 execution（单一内部类 `InfraNWTableSubqFilteredCorrelAssertion`，7 个 execution 共用 static `java-cef495b8aa35bb19a5ac`；ordinals 0-6 runtime 依次为 `java-runtime-6823e53d322aa502295b`、`java-runtime-9ff8e85038e7fa442f0b`、`java-runtime-11e662916b4e4bebf3ed`、`java-runtime-fdb6dcb460167c720624`、`java-runtime-d8db6be7fe12e953f8f2`、`java-runtime-df8a638d47fce3bbe9e7`、`java-runtime-1e359a7b2b7cee11d210`；Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；全部 flags 空）。Java/Go 各 28 条 records、0 differences：keepall 命名窗口（`@public create window MyInfra#keepall as select * from SupportBean`）与复合主键表（`@public create table MyInfra (theString string primary key, intPrimitive int primary key)`）双形态、`enable_window_subquery_indexshare` 创建 hint、`disable_window_subquery_indexshare` 消费 hint 与显式 `@name('index') create index MyIndex on MyInfra(theString)` 的 7 种 flag 组合下，过滤相关标量子查询（`select (select intPrimitive from MyInfra(intPrimitive<0) sw where s0.p00=sw.theString) as val from S0 s0`）的语义——每 case 4 条 consume 记录 val 为 null/-2/-3/null（S0 发送 (10,E1)、(20,E2)、(-3,E3)、(20,E4)）；两个 hint 在该形状下 plan-inert（共享索引分支要求空 filter，表分支不读取 disable hint），7 个 execution trace 逐字节一致，unit 通过 scenario 元数据固定 index-choice 路径。typed Go 将内层 filter 折入 `SubqueryWhere(And(Less[int](Field[any,int], Literal(0)), Equal[string](Field[any,string], OuterField[string])))`（NW/table 统一形状，结果等价）、`SubqueryCardinalityMode(SubqueryNullOnMultiple)`，显式索引用 live `NamedWindow/Table("MyInfra").CreateIndex("MyIndex", []string{"theString"}, IndexHash, false)` catalog op（先例 infra_nwtable_create_index_parity_test.go）；Java oracle harness 同时注册 `SupportBean_S0` 与 `S0` 类型名并按 `S0` 名发送（固定 EPL 消费 `from S0 s0`，Esper 按类型名路由；固定树 suite 未挂接该文件的 runner，行为以 oracle harness 重放约定为准）。严格 scenario/oracle validator 固定 Java 元数据、94 步精确交错顺序（create/insert/[index]/consume 与 8 次 send 的位置逐一固定）、payload 值与每 case 步数（13/14），10 种 raw 畸变 + 6 种 trace 变异全部拒绝。manifest 更新为 618 cases、231 个 differential-verified case、823 个 differential runtime IDs、3446 条 associations（referenced 3205、unreferenced 931）；capability 120 个（37 DV）。
> 最新补充：Draft 4.294（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-aggregate-sorted-no-data-window` differential-verified 场景，对照固定 Java `ResultSetAggregateSortedMinMaxBy.java` ordinal 6 的 `ResultSetAggregateNoDataWindow`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtime `java-runtime-af551963966a83d26468`；static candidate `java-f645b6fb41fd8c63d7f0`；collection execution inventory shared ID `java-553516b9d01c12a13172`；无 flags）：Java/Go 各 4 条 listener records、0 differences。场景覆盖无数据窗口上的 `maxbyever`/`minbyever` 历史极值与 `maxby`/`minby` 当前极值，E1/1、E2/2、E3/0、E4/3 依次推进 E1/E2/E3/E4 与 E1/E1/E3/E3 边界；typed Go 使用 `MaxBy`/`MinBy`、`MaxByEver`/`MinByEver`、`EventValue` 与 `Property`，严格 scenario/oracle validator 固定 static/runtime 元数据、payload、values/order/time/record-shape，并拒绝 malformed JSON、duplicate keys、quoted/non-integer payload 与 trace mutations；manifest 更新为 585 cases、198 个 differential-verified case、729 个 differential runtime IDs、3350 条 associations（referenced 3133）。
> 最新补充：Draft 4.293（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-querytype-rollup-orderby-unidirectional` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRollupHavingAndOrderBy.java` ordinals 4-7 的 `ResultSetQueryTypeOrderByTwoCriteriaAsc{join=false}`、`{join=true}`、`ResultSetQueryTypeUnidirectional` 与 `ResultSetQueryTypeOrderByOneCriteriaDesc`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；runtimes `java-runtime-b09e67f84d7426e94834`、`java-runtime-7859d904aebfc874ac11`、`java-runtime-7f3911bcedf70526d5df`、`java-runtime-2ff9039bfd91c9054507`；static IDs `java-715eae421ccf37a5062a`、`java-24e47ca92d533e352474`、`java-ba719758f9ea057c8d79`；无 flags）：Java/Go 各 8 条 listener records、0 differences。场景覆盖 time-batch rollup 的 Null subtotal、两键升序和单键降序、lastevent join、unidirectional cube 驱动更新及 irstream old/new 行；typed Go 使用 `TimeBatch`、`GroupByRollup`、`GroupByCube`、`Sum`、`WithOldStream`、`OrderBy` 与 `JoinMany`/`Unidirectional`，并修复 runner 侧多键 `OrderBy` 选项覆盖问题。scenario、Java/Go trace、differential evidence、Java oracle/launcher 与 replay/mutation tests 已登记；manifest 更新为 584 cases、582 implemented、197 个 differential-verified case、728 个 differential-verified runtime IDs、3349 条 associations（referenced 3132），capability 119 个（34 个 differential-verified）。
> 最新补充：Draft 4.292（2026-09-01），新增 `resultset.aggregate-access` 的 `resultset-querytype-rollup-having-iterator` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRollupHavingAndOrderBy.java` ordinals 2-3 的 `ResultSetQueryTypeIteratorWindow{join=false}` 与 `{join=true}`（Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`；source `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java`；runtimes `java-runtime-1ce0d3fc4b6fed76d38d`、`java-runtime-7730d794daee6dac92c7`；shared/static ID `java-efb06d5c38363ea3a5d0`；无 flags）：Java/Go 各 8 条 iterator snapshot records、0 differences。场景覆盖 `SupportBean#length(3)` 的 `rollup(theString)` 叶组与 Null subtotal、FIFO eviction 后的 any-order iterator snapshots，以及可选 `SupportBean_S0#keepall` join；typed Go 使用 `LengthWindow(3)`、`GroupByRollup`、`Sum` 与等价 keepall join，严格 validator 固定 runtime metadata、payload、snapshot order/membership、Null、时间和 record shape，并拒绝 malformed scenario、duplicate keys 与 trace mutation。可重放输入、Java trace、Go trace、differential evidence 分别为 `testdata/parity/resultset-querytype-rollup-having-iterator.json`、`testdata/parity/resultset-querytype-rollup-having-iterator.trace.json`、`testdata/parity/resultset-querytype-rollup-having-iterator.go.trace.json`、`testdata/parity/resultset-querytype-rollup-having-iterator.evidence.json`；manifest 更新为 583 cases、196 个 differential-verified cases、724 个 differential-verified runtime IDs、3345 条 associations（referenced 3128），capability 119 个（34 个 differential-verified）。
> 最新补充：Draft 4.291（2026-09-01），新增 `resultset.aggregate-dimensional` 的 `rollup-grouping-funcs-faf-dedicated` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRollupGroupingFuncs.java` ordinal 2 的 `ResultSetQueryTypeFAFCarEventAndGroupingFunc`（runtime `java-runtime-e8f49362431d4c33baa8`；shared/static ID `java-1ae66c9985dc53282af0`；execution flag `FIREANDFORGET`）：Java/Go 各 1 条 FAF record、12 条 rows、0 differences。场景覆盖 public `CarWindow#keepall` 的六个 `SupportCarEvent` 插入、`grouping sets((name, place), name, place, ())` 的 detail→name subtotal→place subtotal→overall 顺序、grouping bits、`grouping_id` 和 Null 投影；typed Go 使用 `GroupByGroupingSets`、`Sum`、`Grouping`、`GroupingID` 与 `ExecuteFireAndForget`，严格 scenario/oracle validator 固定 metadata、payload、values/order/time/record-shape，并拒绝 malformed scenario、duplicate keys、payload 与 trace mutation；manifest 更新为 582 cases、195 个 differential-verified case、722 个 differential runtime IDs、3343 条 associations（referenced 3126）。
> 最新补充：Draft 4.289（2026-08-31），新增 `resultset.aggregate-dimensional` 的 `resultset-querytype-rollup-dimensionality-dedicated` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRollupDimensionality.java` 的 `ResultSetQueryTypeUnboundGroupingSet2LevelUnenclosed` ordinal 10（runtime `java-runtime-20c08346e2644a7201e5`）、`ResultSetQueryTypeBoundCube3Dim` ordinal 11（runtime `java-runtime-994120aef6b9ff1c0e75`）和 `ResultSetQueryTypeContextPartitionAlsoRollup` ordinal 17（runtime `java-runtime-71fb38d67af471287089`），三 execution、五 cases、Java/Go 各 21 条 records、0 differences，无 flags。场景覆盖 unbound grouping-set、bounded cube/显式 grouping-sets、SegmentedByString context-partition rollup；typed Go 使用 `GroupByGroupingSets`、`GroupByCube`、`CreateKeyContext`/`WithContext`、`Sum`、`CountAll`、`Grouping`、`GroupingID`，严格 validator 固定 metadata、payload、行序、时间、subtotal Null、cube masks 与 record shape，并覆盖 malformed/duplicate/marker/payload/trace mutation rejection。已登记 scenario、Java/Go trace、differential evidence、Java oracle/launcher 和 replay/mutation tests；manifest 更新为 580 cases、193 个 differential-verified case、720 个 distinct differential runtime IDs、3341 条 associations（referenced 3125）。剩余为 invalid compile-error diagnostics、超出已验证形式的 combined/nested grouping specifications，以及完整 Context-partition rollup matrix。
> 最新补充：Draft 4.288（2026-08-31），复核固定 Java `ResultSetQueryTypeRowForAll.java` ordinal 11 的 `ResultSetQueryTypeRowForAllNamedWindowWindow`（runtime `java-runtime-a86df6b7b7185701f64b`；shared inventory ID `java-00cfc4061ba3a5f163ff`；static candidate `java-5fb1c0c6839daa19fded`；无 flags）后确认该 execution 已由既有 `case.row-for-all` 完整覆盖：`resultset-row-for-all` 场景和 Java trace/evidence 含六条 Java/Go 一致 records，`TestResultSetQueryTypeRowForAllNamedWindowWindowParity` 已覆盖 typed named-window create/insert/delete、FIFO `window(intPrimitive)`、删除后的 old-stream 行及生命周期顺序。为避免重复 trace 和 runtime association，本轮不新增 case 或 manifest 统计；下一优先级转向 `resultset.aggregate-dimensional` 的未差分 rollup execution。
> 最新补充：Draft 4.287（2026-08-31），新增 `resultset-querytype-row-for-all-static-method-double-nested` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRowForAll.java` ordinal 12 的 `ResultSetQueryTypeRowForAllStaticMethodDoubleNested`（runtime `java-runtime-f8487c524573a3a2f08c`；shared inventory ID `java-00cfc4061ba3a5f163ff`；static candidate `java-421838b50fc1c8eff9f6`；无 flags）：Java/Go 各 1 条 record、0 differences。场景覆盖 `SupportBean` 上 `MyHelper.doOuter(MyHelper.doInner(last(theString)))` 的嵌套静态方法组合：E1 映射为 `oiE1io`，仅新流 listener 输出；typed Go `Last[string]` 与嵌套 `Func1` 表达式按固定 scenario 重放；严格 validator 固定 helper EPL、metadata、payload、value/order/time/record-shape，并拒绝 malformed scenario、duplicate keys 与 trace mutation；manifest 更新为 579 cases、192 个 differential-verified case、717 个 differential runtime IDs、3338 条 associations（referenced 3125）；capability 119 个（34 DV）。
> 最新补充：Draft 4.286（2026-08-31），新增 `resultset-querytype-row-for-all-select-avg-std-group-by-uni` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRowForAll.java` ordinal 10 的 `ResultSetQueryTypeRowForAllSelectAvgStdGroupByUni`（runtime `java-runtime-de3a01d12a22bb33ee43`；shared inventory ID `java-00cfc4061ba3a5f163ff`；static candidate `java-7ba8b4893d30071e2fa5`；无 flags）：Java/Go 各 4 条 records、0 differences。场景覆盖 `SupportMarketDataBean#groupwin(symbol)#length(2)#uni(price)` 的 `istream average`：A/1 输出 1.0、B/3 输出 3.0、A/3 输出 2.0、A/10 callback 仅用于 listener reset、A/20 在每符号 length(2) 淘汰后输出 15.0；typed Go `GroupWindow`、`LengthWindow(2)`、`UnivariateStatistics(price).Average()` 与 new-only listener replay；递归 `univariate-statistics*` 依赖检测使 `#uni` 使用隐式 group-window key，同时保留普通 `Avg` 的 row-for-all 语义；严格 scenario/oracle validator 固定 metadata、payload、value/order/time/record-shape 并拒绝 mutation；manifest 更新为 578 cases、191 个 differential-verified case、716 个 differential runtime IDs、3337 条 associations（referenced 3124）；capability 119 个（34 DV）。
> 最新补充：Draft 4.285（2026-08-31），新增 `resultset-querytype-row-for-all-select-avg-expr-std-group-by` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRowForAll.java` ordinal 9 的 `ResultSetQueryTypeRowForAllSelectAvgExprStdGroupBy`（runtime `java-runtime-873d7f4fc2344cd9c631`；shared inventory ID `java-00cfc4061ba3a5f163ff`；static candidate `java-d8d6953f53a7a43d2324`；无 flags）：Java/Go 各 2 条 records、0 differences。场景覆盖 `SupportMarketDataBean#groupwin(symbol)#length(2)` 上 `istream avg(price)` 的 row-for-all 全局平均：A=1 输出 1.0、B=3 输出跨 group-window 分区的 2.0；typed Go `GroupWindow`、`Avg` 与严格 scenario/oracle validator 固定 payload、metadata、value/order/time/record-shape 及 mutation rejection；为保留纯聚合 row-for-all 语义，bounded aggregate runtime 仅在投影读取非 key 属性或 into-table 时 materialize 隐式分组；manifest 更新为 577 cases、190 个 differential-verified case、715 个 differential runtime IDs、3336 条 associations（referenced 3123）；capability 119 个（34…
> 最新补充：Draft 4.284（2026-08-31），新增 `resultset-querytype-row-for-all-having-avg-group-window` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRowForAllHaving.java` ordinal 2 的 `ResultSetQueryTypeAvgRowForAllWHavingGroupWindow`（runtime `java-runtime-d20a1ee344797d87678b`；shared inventory/static ID `java-45a56190252ed051c35f`；无 flags）：Java/Go 各 3 条 records、0 differences。场景覆盖 `SupportMarketDataBean#unique(symbol)` 的 `istream avg(price) <= 0` having：A=-1、A=5 替换后抑制、B=-6、C 正均值抑制、C=-2 恢复输出；typed Go `Unique`、`Avg`、`Having` 与严格 scenario/oracle validator 固定 payload、metadata、value/order/time/record-shape 及 mutation rejection；manifest 更新为 576 cases、189 个 differential-verified case、714 个 differential runtime IDs、3335 条 associations（referenced 3122）；capability 119 个（34 DV）。
> 最新补充：Draft 4.283（2026-08-29），新增 `resultset-querytype-row-for-all-having-sum-join` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRowForAllHaving.java` ordinal 1 的 `ResultSetQueryTypeRowForAllWHavingSumJoin`（runtime `java-runtime-dfb97881f562ac33e873`；shared inventory `java-45a56190252ed051c35f`；static candidate `java-bf6637bf00f9e0e78f89`，无 flags）：Java/Go 各 3 条 records、0 differences。场景覆盖两个 `SupportBeanString`/`SupportBean` 十秒 time-window 的 key join、`irstream sum(longBoxed) > 10`、t5/t8/t10 新旧 listener 行与双窗口首个到期（t5 new 25、t8 new 20/old 25、t10 old 20）；typed Go `Join`、nullable `LongBoxed`、两个 `TimeWindow`、`Sum`、`Having`、`WithOldStream` 与严格 scenario/oracle validator 固定 join metadata、payload、value/order/time/record-shape；manifest 更新为 575 cases、188 个 differential-verified case、713 个 differential runtime IDs、3334 条 associations（referenced 3121）；capability 119 个（34 DV）。
> 最新补充：Draft 4.281（2026-08-29），新增 `resultset-aggregate-minmax-named-window-wever` differential-verified 场景，对照固定 Java `ResultSetAggregateMinMax.java` ordinals 2-3 的 `ResultSetAggregateMinMaxNamedWindowWEver{soda=false}` 与 `{soda=true}`（runtime `java-runtime-7d0a94525b0038b397fd`、`java-runtime-840f1ca5610dd00d3ec0`；static candidate `java-09115f6e876ce88a42f9`，flags `EXCLUDEWHENINSTRUMENTED`）：Java/Go 各 8 条 records、0 differences。场景覆盖 public `NamedWindow5m#length(2)` 的 typed `Min`/`Max` 当前极值、`MinEver`/`MaxEver` 历史极值与 FIFO 淘汰，SODA/EPL 双表示路径共享 listener 语义；runner/oracle 严格拒绝缺失/额外/重复字段、flags/metadata 不匹配、非法 payload 与 value/order/null/time/count trace mutation。manifest 更新为 573 cases、186 个 differential-verified case、711 个 differential runtime IDs、3332 条 associations（referenced 3119）；capability 119 个（34 DV）。
> 最新补充：Draft 4.279（2026-08-29），新增 `resultset-aggregate-median-and-deviation` differential-verified 场景，对照固定 Java `ResultSetAggregateMedianAndDeviation.java` ordinals 0-2 的 `ResultSetAggregateStmt`、`ResultSetAggregateStmtJoinOM`、`ResultSetAggregateStmtJoin`（runtime `java-runtime-101db664ea346721e986`、`java-runtime-2f533a8a1bc0d93ae649`、`java-runtime-2667a7eadeb7a458d7ce`；static candidates `java-b11b233b0ea7da05217f`、`java-66a516ce6e14456627b2`、`java-a5c65b307ecdb2d0355b`）：Java/Go 各 21 条 records、0 differences。场景覆盖过滤 `length(5)` 分组 `irstream` `median(all)`/`median(distinct)`/样本 `stddev`/`avedev`，及 SupportBeanString DELL/IBM/AAA 种子驱动的两个 grouped join 变体；Go 增加持久化 StdDev enter/leave/Welford 状态、路由 clone 隔离和 NaN poisoning 边界测试，Java 独立 NaN redeployment 不进入有限 JSON trace。runner/oracle 严格拒绝缺失/额外字段、错误价格、case 顺序及 trace value/order/time/record-count mutation；manifest 更新为 571 cases、184 个 differential-verified case、708 个 differential runtime IDs、3329 条 associations（referenced 3116）；capability 119 个（34 DV）。
> 最新补充：Draft 4.280（2026-08-29），新增 `resultset-aggregate-minmax-no-data-window-subquery` differential-verified 场景，对照固定 Java `ResultSetAggregateMinMax.java` ordinal 0 的 `ResultSetAggregateMinMaxNoDataWindowSubquery`（runtime `java-runtime-f1fedff9a25cca57fefa`，static candidate `java-347fe4c3739663bf45c4`）：Java/Go 各 4 条 records、0 differences。场景覆盖无窗口 SupportBean `max`/`min` 与 SupportBean_S0 `#lastevent` 标量子查询 `max`/`min`，S0-only update 静默、空标量子查询为 Null、后续状态更新及严格事件顺序；typed Go `SubqueryValue`/`LastEvent` surface、固定 Java metadata、payload/duplicate/malformed/trace mutation rejection 均纳入验证。manifest 更新为 572 cases、185 个 differential-verified case、709 个 differential runtime IDs、3330 条 associations（referenced 3117）；capability 119 个（34 DV）。
> 最新补充：Draft 4.277（2026-08-28），新增 `resultset-aggregate-minmax-groupby-om-viewcompile` differential-verified 场景，对照固定 Java `ResultSetAggregateMaxMinGroupBy.java` ordinals 1-2 的 `ResultSetAggregateMinMaxOM`（runtime `java-runtime-2408009c4113ea2214c4`，static candidate `java-8ef85c99fae01e5e4431`）与 `ResultSetAggregateMinMaxViewCompile`（runtime `java-runtime-30dbed587648347e610a`，static candidate `java-a340ca15a11cf67e3170`）：Java/Go 各 24 条 records、0 differences。场景覆盖两条表示路径共享的过滤 `length(3)` 分组 `irstream` min/max all/distinct、重复极值、全局 FIFO 淘汰、null 跳过与全 null 输出；Java oracle 额外严格执行 SODA `SerializableObjectCopier`/`toEPL` 与 EPL-to-model compile 校验。Go 使用等价 typed immutable Query/Plan，表示入口差异已在 manifest/case notes 明确记录，差分范围限定为共享 listener 语义；runner/oracle 严格拒绝缺失/额外字段、非整数 payload、错误 case 顺序以及 trace value/order/time/record-count mutation。manifest 更新为 569 cases、182 个 differential-verified case、703 个 differential runtime IDs、3324 条 associations（referenced 3111）；capability 119 个（34 DV）。
> 最新补充：Draft 4.276（2026-08-28），新增 `resultset-aggregate-minmax-groupby` differential-verified 场景，对照固定 Java `ResultSetAggregateMaxMinGroupBy.java` 的 `ResultSetAggregateMinMax`（ordinal 0，runtime `java-runtime-6ee286d6f857ddbbd091`）与 `ResultSetAggregateMinNoGroupHaving`（ordinal 4，runtime `java-runtime-cd645170c5defa3996da`）：Java/Go 各 14 条 records、0 differences。场景覆盖过滤 `length(3)` 分组 `min(all)`/`max(all)` 与 distinct 聚合的全局滑窗、重复极值稳定性、跨组 IBM/DELL 双行 listener batch、null volume 跳过与全 null；并覆盖无分组 `time(5 sec)` `having volume > min(volume) * 1.3` 的静默/命中边界。Go runner/oracle 严格拒绝缺失/额外/重复字段、非整数与错误 payload、错误 case 顺序以及 trace value/order/null/time/record-count mutation；manifest 更新为 568 cases、181 个 differential-verified case、701 个 differential runtime IDs、3322 条 associations（referenced 3109）；capability 119 个（34 DV）。
> 最新补充：Draft 4.275（2026-08-28），新增 `resultset.querytype-row-for-all` differential-verified 场景，对照固定 Java `ResultSetQueryTypeRowForAll.java` ordinals 4-8 的 5 个 execution（`ResultSetQueryTypeRowForAllSumOneView`、`ResultSetQueryTypeRowForAllSumJoin`、`ResultSetQueryTypeRowForAllAvgPerSym`、`ResultSetQueryTypeRowForAllSelectStarStdGroupBy`、`ResultSetQueryTypeRowForAllSelectExprGroupWin`）：Java/Go 各 33 条 records、0 differences。场景覆盖 10 秒 time-window sum 的 listener 与 iterator expiry snapshots、keepall join matching sum、groupwin per-symbol avg、typed wildcard group-window row 与 scalar group-window selection；空 join aggregate iterator 初始 null 语义由 bounded runtime 修复对齐。manifest 更新为 568 cases、180 个 differential-verified case、699 个 differential runtime IDs、3322 条 associations（referenced 3109）；capability 119 个（34 DV）。
> 最新补充：Draft 4.273（2026-08-28），新增 `resultset-aggregate-sorted-multi-criteria-simple` differential-verified 场景，对照固定 Java `ResultSetAggregateSortedMinMaxBy.java` 的 `ResultSetAggregateMultipleCriteriaSimple`（ordinal 4，1 个 runtime）：Java/Go 各 4 条 records、0 differences。场景覆盖 keepall `sorted(theString desc, intPrimitive desc)` 的双键字典序、完整 SupportBean identity 与四步有序快照 `[C/10]`、`[D/20,C/10]`、`[D/20,C/15,C/10]`、`[D/20,D/19,C/15,C/10]`；runner/oracle 严格拒绝缺失/额外/重复字段、非整数与 int32 越界 payload 及 trace value/order/time/record-count mutation。manifest 更新为 566 cases、178 个 differential-verified case、693 个 differential runtime IDs、3316 条 associations（referenced 3104）；capability 118 个（33 DV）。
> 最新补充：Draft 4.270（2026-08-28），新增 `resultset-aggregate-firstlastwindow-indexed` differential-verified 场景，对照固定 Java `ResultSetAggregateFirstLastWindow.java` 的 `ResultSetAggregateFirstLastIndexed`（1 个 runtime）：Java/Go 各 4 条 records、0 differences。场景覆盖 typed `first`/`last` indexes 0..3 的 length(3) new-only 输出、未满窗口时的 Null、越界 index 3 的 Null 和窗口淘汰后的 FIFO 历史移位；join、dynamic index、prev/nth、wildcard、iterator、FAF、output-rate、invalid 与生命周期变体继续保留在 aggregate-access umbrella。manifest 更新为 563 cases、175 个 differential-verified case、688 个 differential runtime IDs、3311 条 associations（referenced 3102）；capability 118 个（33 DV）。
> 最新补充：Draft 4.271（2026-08-28），新增 `resultset-aggregate-nth` differential-verified 场景，对照固定 Java `ResultSetAggregateNTh.java` 的 `ResultSetAggregateNTh`（runtime `java-runtime-1a257602734874ff3fc4`）：EPL/SODA 两个生命周期阶段各 3 条 listener records，Java/Go 各 6 条 records、0 differences。场景覆盖 grouped keepall 的 typed `nth(0)`/`nth(1)`、output last every 3、`theString` 排序、null prior 值与九事件状态轨迹；Go runner 同步拒绝缺失/额外字段及非整数 payload。manifest 更新为 564 cases、176 个 differential-verified case、689 个 differential runtime IDs、3312 条 associations（referenced 3102）；capability 118 个（33 DV）。

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
> join 家族三 execution 因滑动窗口 old/new 分类分歧保持未登记（4.241 的
> manifest 登记缺口同轮闭合；该缺口已于 Draft 4.417 以
> UNAGGREGATED_UNGROUPED join 结果形态修复并登记）。manifest 更新为 552 cases、163 个
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
> 窗口滑动下 old/new 分类分歧暂登记 remaining（该缺口已于 Draft 4.417 以
> UNAGGREGATED_UNGROUPED join 结果形态修复并登记）。manifest 更新为 538 cases、152 个
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

截至 2026-09-05，manifest v2 的已校验摘要为：

| 维度 | 数值 |
| --- | --- |
| Capability | 119 |
| Case | 609 |
| Case differential-verified | 222 |
| Differential-verified runtime | 790 / 4,136 |
| Runtime 已关联 | 3,172 / 4,136（76.7%） |
| Runtime 未关联 | 964 |
| Representative scenario | 107 / 107 通过 |
| Intentionally-different case | 23 |
| NFR-verified case | 0 |
| 质量摘要 | Docker/stress/race 已通过；performance pending |

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

- epl 剩余 247 个未关联 runtime，优先 subselect、insertinto、database、dataflow 和方法源。
- infra 剩余 156 个未关联 runtime，优先表、Named Window、mutation 和 transaction。
- join 与 outer join 复杂链、unidirectional、Context Join。
- resultset 聚合和输出高级特性（filtered、math-context、访问聚合、rollup、row-limit 组合）。
- context 分区 selector、嵌套、生命周期、事务边界。
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
- epl 剩余 247 个未关联 runtime，优先 subselect、insertinto、database、dataflow 和方法源。
- infra 剩余 156 个未关联 runtime，优先表、Named Window、mutation 和 transaction。
- join 与 outer join 复杂链、unidirectional、Context Join。
- resultset 聚合和输出高级特性（filtered、math-context、访问聚合、rollup、row-limit 组合）。
- context 分区 selector、嵌套、生命周期、事务边界。
- expression 剩余类型、函数、脚本、枚举集合高级组合。
- event、expr、resultset、context、view 和 rowrecog 的未关联 runtime 按当前清单继续拆分。

### 5.1 P0 — 立即完成

1. 完成全量 `go test`、race、vet、布局和 diff 门禁，并将结果回写 Manifest v2。
2. 扩展 persisted differential evidence。当前有 206 个 differential-verified case（755 个 runtime）和 107/107 个通过的 representative scenario；下一步优先转换共享 runtime 和高风险状态能力，不在路线图手写完整场景名称列表。
3. 按未关联 runtime 和行为风险拆分下一批 epl/infra/expr/resultset/context 切片。
4. 外部服务 fixture 已本地验证（2026-08-14 MySQL/Kafka/RabbitMQ 全部门控 round-trip 通过）；暂不建设 CI，后续按执行手册定期本地 Docker 重放，并保持普通测试中的显式环境型 skip。
5. 已建立环境门控 stress 基线（`ESPER_STRESS=1 go test ./internal/esper -run '^TestStressSyntheticMediumLoad$'`）；已实现 `windowHistoryByEventRequired` 按需构建 `historyByEvent`，基线从 42.6s 降至 18.45s；继续优化剩余 filter/window/aggregate/join 热点后再宣称 NFR。
6. 运行时性能与执行模型：已完成的属性访问缓存与无状态过滤特化、以及尚未实施的过滤服务索引、单次谓词求值、WHERE 下推、结果集处理器特化等工作项，见 [运行时性能与执行模型记录](esper-go-performance.md)。这些工作项在对应 capability 语义证据完成后按工作单元推进，并保持差分、证据与 Plan 身份约束；在具备可重放阈值前不登记 `nfr-verified`。

### 5.2 P1 — 下一批高价值切片

按未覆盖 runtime 数量排序：

| 域 | 未覆盖 runtime | 关键子域/类 |
| --- | --- | --- |
| epl | 243 | subselect、insertinto、database、dataflow、方法源 |
| infra | 156 | 表、Named Window、mutation、transaction |
| event | 151 | 事件表示和 Serde 完整矩阵 |
| expr | 95 | 表达式函数、类型、脚本、枚举集合 |
| resultset | 35 | 聚合、输出、排序、分组 |
| multithread | 56 | 并发回归 |
| context | 45 | Context 分区、嵌套、生命周期 |
| rowrecog | 34 | Match Recognize |
| view | 22 | 视图高级组合 |

### 5.3 P2 — 清单与能力拆分

- 将 epl/expr/resultset 等粗粒度 capability 拆分为更细 case，便于追踪。
- 填充 static-manifest.json 与 source-test-manifest.json。

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
| epl | 243 | subselect、insertinto、database、dataflow、方法源 |
| infra | 156 | 表、Named Window、mutation、transaction |
| event | 151 | 事件表示和 Serde 完整矩阵 |
| expr | 95 | 表达式函数、类型、脚本、枚举集合 |
| resultset | 35 | 聚合、输出、排序、分组 |
| context | 45 | Context 分区、嵌套、生命周期 |
| multithread | 56 | 并发回归 |
| rowrecog | 34 | Match Recognize |
| view | 22 | 视图高级组合 |
### 6.2 清单与追踪遗漏

- static-manifest.json 目前几乎为空，需要把静态/编译期候选登记进去。
- source-test-manifest.json 目前几乎为空，需要把非 Regression 源资产（单元测试、集成测试）登记进去。
- epl/expr/resultset 等 capability 拆分过粗，需要继续细分为可验收的 case。
- 23 个 intentionally-different case 需要保持书面差异理由和测试证据。
- 当前未关联的 893 个 runtime 中，需要识别哪些属于平台无关核心语义，哪些属于 JVM 特有机制或性能阈值，并分别建立处置记录。

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
