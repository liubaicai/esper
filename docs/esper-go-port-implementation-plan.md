> 最新补充：Draft 4.110（2026-08-15），新增 `resultset-aggregate-max-time-window` 差分场景，对照 `ResultSetMaxTimeWindow`（`java-runtime-a137a3b5c9a0cbc904cf`）。场景用 SupportMarketDataBean#time(1 sec) + group by symbol + irstream `output every 1 seconds`：1000ms 同时触发首轮输出与双事件淘汰，new 为缓冲的 SYM1/1/1.0 + SYM1/2/2.0，old 为两个 null 聚合行，evidence 1 条 record、0 differences；复用 aggregate-grouped irstream old 行与事件锚定调度修复，无需新增运行时改动。Manifest v2 更新为 434 case、2,739 条关联、434 个 implemented（verification）、47 个 differential-verified、56 个 differential runtime、47 个 representative-verified、47/47 场景通过；`resultset.aggregate-group-by`、`output.core` 与 `view.window-core` 新增 `ResultSetMaxTimeWindow` 证据。

> 最新补充：Draft 4.109（2026-08-15），新增 `resultset-having-every-events` 差分场景，对照 `ResultSetHaving`（`java-runtime-f209dbfcc4be7dcfa536`）。场景用 SupportMarketDataBean#time(10 sec) + group by symbol + `having sum(price) >= 10` + irstream `output every 3 events` 双跑 3 条 records、0 differences。运行时修复事件计数 output 策略：`OutputEvery`/`OutputAllEveryEvents`/`OutputLastEveryEvents` 按 Java `OutputConditionCount` 分别累计输入 insert/remove 计数（new >= N 或 old >= N 即触发），having 过滤后的空批次也推进计数；纯三事件淘汰批次触发 old 输出。同时对齐 ungrouped irstream 聚合 null-prior old 行（oracle 探针验证 `select irstream sum(...) from ...#length(2) output every 3 events` 输出 old [null,10,30]），修正 `TestOutputRowForAllAggregateOldNewBatchMatchesEsper`。Manifest v2 更新为 433 case、2,738 条关联、433 个 implemented（verification）、46 个 differential-verified、55 个 differential runtime、46 个 representative-verified、46/46 场景通过；`resultset.aggregate-group-by`、`output.core` 与 `view.window-core` 新增 `ResultSetHaving` 证据。

> 最新补充：Draft 4.108（2026-08-15），新增 `resultset-aggregate-all-events` 差分场景，对照 `ResultSetNoJoinAll`（`java-runtime-d7e3f4e3457216b42c0b`）。场景用 SupportMarketDataBean#length(5) + DELL/IBM/GE filter + group by symbol + `output all every 2 events` 双跑两次批次输出，evidence 2 条 records、0 differences。新增公共 API `OutputAllEveryEvents(count)`：每个输入事件累积一行（带该事件当时的 group 聚合），输出边界对无新事件的 group 用代表行重发，对齐 Java `ResultSetProcessorAggregateGroupedOutputAllHelperImpl`，不合成快照 old 行。同时修正 `case.filter-window-aggregate-output` 的过度登记（该场景从未执行 output-all），`ResultSetNoJoinAll` 由本场景正式差分验证，`ResultSetSimpleNoJoinAll` 回到 implemented-only。Manifest v2 更新为 432 case、2,737 条关联、432 个 implemented（verification）、45 个 differential-verified、54 个 differential runtime、45 个 representative-verified、45/45 场景通过；`resultset.aggregate-group-by`、`output.core` 与 `view.window-core` 新增 `ResultSetNoJoinAll` 证据。

> 最新补充：Draft 4.107（2026-08-15），新增 `resultset-aggregate-snapshot-time-window` 差分场景，对照 `ResultSet18SnapshotNoHavingNoJoin`（`java-runtime-e5b7968f1c5225f9cce2`）。场景用 SupportMarketDataBean#time(5.5 sec) + group by symbol + `output snapshot every 1 seconds` 双跑 7 次快照，按保留事件逐行携带当前 group 聚合值；evidence 7 条 records、0 differences；复用 grouped snapshot row-per-event 路径，无需新增运行时改动。Manifest v2 更新为 431 case、2,738 条关联、431 个 implemented（verification）、44 个 differential-verified、55 个 differential runtime、44 个 representative-verified、44/44 场景通过；`resultset.aggregate-group-by`、`output.core` 与 `view.window-core` 新增 `ResultSet18SnapshotNoHavingNoJoin` 证据。

> 最新补充：Draft 4.106（2026-08-15），新增 `resultset-aggregate-first-time-window` 差分场景，对照 `ResultSet17FirstNoHavingNoJoin`（`java-runtime-02622af5ad124265131e`）。场景用 SupportMarketDataBean#time(5.5 sec) + group by symbol + irstream `output first every 1 seconds` 双跑 11 次输出，evidence 11 条 records、0 differences。运行时修复 aggregate-grouped `OutputFirstEveryTime`：纯时间淘汰批次把淘汰后当前聚合值作为 New（5700 IBM72、6300 MSFT null、7000 IBM48+YAH6），事件时刻仍按 group interval 首行节流；rollup/row-per-group/join/named-window 契约不变。Manifest v2 更新为 430 case、2,737 条关联、430 个 implemented（verification）、43 个 differential-verified、54 个 differential runtime、43 个 representative-verified、43/43 场景通过；`resultset.aggregate-group-by`、`output.core` 与 `view.window-core` 新增 `ResultSet17FirstNoHavingNoJoin` 证据。

> 最新补充：Draft 4.105（2026-08-15），`resultset-aggregate-last-time-window` 场景扩展 `ResultSet15LastHavingNoJoin`（`java-runtime-68d3d21c1d3e57d6ab3b`）：同一 200–7200ms 序列在 `having sum(price) > 50` + irstream `output last every 1 seconds` 下输出 IBM 75/97 new 与 6200 一条 expiry old，Java/Go 各 9 条 records、0 differences；复用 `OutputLastEveryTime` aggregate-grouped 修复，无需新增运行时改动。Manifest v2 更新为 429 case、2,736 条关联、42 个 differential-verified、53 个 differential runtime、42 个 representative-verified、42/42 场景通过；`resultset.aggregate-group-by` 与 `output.core` 新增 `ResultSet15LastHavingNoJoin` 证据。

> 最新补充：Draft 4.104（2026-08-15），新增 `resultset-aggregate-last-time-window` 差分场景，对照 `ResultSet13LastNoHavingNoJoin`（`java-runtime-cf84031b265129f653ee`）。场景用 SupportMarketDataBean#time(5.5 sec) + group by symbol + irstream `output last every 1 seconds order by symbol` 双跑 6 次输出，evidence 6 条 records、0 differences。运行时修复 `OutputLastEveryTime` aggregate-grouped 路径：普通更新不再把上一周期输出合成 old，old 只来自 leaving events（淘汰后聚合值、空组 null、order-by 排序）；rollup/row-per-group/join/named-window 契约不变。Manifest v2 更新为 429 case、2,735 条关联、429 个 implemented（verification）、42 个 differential-verified、52 个 differential runtime、42 个 representative-verified、42/42 场景通过；`resultset.aggregate-group-by`、`output.core` 与 `view.window-core` 新增 `ResultSet13LastNoHavingNoJoin` 证据。

> 最新补充：Draft 4.103（2026-08-15），`resultset-aggregate-time-window` 场景扩展 `ResultSet7DefaultHavingNoJoin`（`java-runtime-a217721770427537aa2c`）：同一 200–7200ms 序列在 `having sum(price) > 50` + irstream `output every 1 seconds` 下输出 IBM 75/97 new 与 6200 一条 expiry old，Java/Go 各 9 条 records、0 differences；复用 aggregate-grouped irstream old 行修复，无需新增运行时改动。Manifest v2 更新为 428 case、2,734 条关联、41 个 differential-verified、51 个 differential runtime、41 个 representative-verified、41/41 场景通过；`resultset.aggregate-group-by` 与 `output.core` 新增 `ResultSet7DefaultHavingNoJoin` 证据。

> 最新补充：Draft 4.102（2026-08-15），新增 `resultset-aggregate-time-window` 差分场景，对照 `ResultSet5DefaultNoHavingNoJoin`（`java-runtime-a7dd740df8a4fa0a88e6`）。场景用 SupportMarketDataBean#time(5.5 sec) + group by symbol + irstream `output every 1 seconds` 的完整 200–7200ms 序列双跑 6 次输出，含窗口淘汰 old 行（淘汰后聚合值 + 被淘汰事件非聚合列、空组 null 聚合）；evidence 6 条 records、0 differences。运行时修复 aggregate-grouped irstream old 行语义：普通更新不再输出 previous-state old 行，old 行仅由 leaving events 产生；row-per-group、join、named-window 与 rollup 的 previous/null-prior 契约保持不变。新增 `aggregateDefinitionReadsNonKeyEvent` 区分 Java `ResultSetProcessorAggregateGroupedImpl` 与 `ResultSetProcessorRowPerGroupImpl`。Manifest v2 更新为 428 case、2,733 条关联、428 个 implemented（verification）、41 个 differential-verified、50 个 differential runtime、41 个 representative-verified、41/41 场景通过；`output.core`、`resultset.aggregate-group-by` 与 `view.window-core` 新增 `ResultSet5DefaultNoHavingNoJoin` 证据。

> 最新补充：Draft 4.101（2026-08-14），新增 `resultset-aggregate-no-output` 差分场景，对照 `ResultSetNoOutputClauseView`（`java-runtime-dc2ac7e7293a204aa102`）。场景用 SupportMarketDataBean#length(5) + DELL/IBM/GE filter + group by symbol（无 output 策略）双跑两次逐事件输出。evidence 2 条 records、0 differences。Manifest v2 更新为 427 case、2,732 条关联、427 个 implemented（verification）、40 个 differential-verified、49 个 differential runtime、40 个 representative-verified、40/40 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetNoOutputClauseView` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.100（2026-08-14），新增 `resultset-aggregate-last` 差分场景，对照 `ResultSetNoJoinLast`（`java-runtime-1debb0e930f955641300`）。场景用 SupportMarketDataBean#length(5) + DELL/IBM/GE filter + group by symbol + `output last every 2 events` 双跑两次批次输出。evidence 2 条 records、0 differences。Manifest v2 更新为 426 case、2,731 条关联、426 个 implemented（verification）、39 个 differential-verified、49 个 differential runtime、39 个 representative-verified、39/39 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetNoJoinLast` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.99（2026-08-14），新增 `resultset-aggregate-default` 差分场景，对照 `ResultSetNoJoinDefault`（`java-runtime-fd011003ea3fe327b12c`）。场景用 SupportMarketDataBean#length(5) + DELL/IBM/GE filter + group by symbol + `output every 2 events` 双跑两次批次输出。evidence 2 条 records、0 differences。Manifest v2 更新为 425 case、2,730 条关联、425 个 implemented（verification）、38 个 differential-verified、48 个 differential runtime、38 个 representative-verified、38/38 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetNoJoinDefault` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.98（2026-08-14），新增 `rollup-output-first-having` 差分场景，对照 `ResultSetOutputFirstHaving{join=false}`（`java-runtime-327ce9d41c4e5489cc38`）。场景复用 3.5s SupportBean rollup 序列，用 `having sum > 100` + output-first 双跑 7 次输出。运行时修复 grouped output-first-with-having 的 old 行语义（new/old 都交付当前聚合值）。evidence 7 条 records、0 differences。Manifest v2 更新为 424 case、2,729 条关联、424 个 implemented（verification）、37 个 differential-verified、47 个 differential runtime、37 个 representative-verified、37/37 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputFirstHaving{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.97（2026-08-14），新增 `rollup-output-all-sorted` 差分场景，对照 `ResultSetOutputAllSorted{join=false}`（`java-runtime-569b2b4848133878bfdc`）。场景复用 3.5s SupportBean rollup 序列，用 `OutputAllEveryTime` + order-by 双跑 6 次输出。运行时修复 `OutputAllEveryTime` 的 order-by/limit 应用（纳入 defer output result window）。evidence 6 条 records、0 differences。Manifest v2 更新为 423 case、2,728 条关联、423 个 implemented（verification）、36 个 differential-verified、46 个 differential runtime、36 个 representative-verified、36/36 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputAllSorted{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.96（2026-08-14），新增 `rollup-output-all` 差分场景，对照 `ResultSetOutputAll{join=false}`（`java-runtime-8bc3878f4730fbf2fd21`）。新增公共 API `OutputAllEveryTime(interval)`：每个虚拟时钟周期输出完整当前状态（New=当前聚合、Old=上一周期输出，新 group 首次 old 为 null 占位），保留已变空 group 的 null 行，排序按 rollup 层级 + group 创建顺序。evidence 6 条 records、0 differences。Manifest v2 更新为 422 case、2,727 条关联、422 个 implemented（verification）、35 个 differential-verified、45 个 differential runtime、35 个 representative-verified、35/35 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputAll{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.95（2026-08-14），新增 `rollup-output-default-market` 差分场景，对照 `ResultSet2OutputLimitDefault`（`java-runtime-16db4dbf1209b932f029`）。场景复用 5.5s SupportMarketDataBean rollup(symbol) 序列，用 irstream output-every 双跑 6 次输出（含事件 delta 与窗口淘汰）；在上一轮 affected 顺序与空 tick 调度修复后直接通过。evidence 6 条 records、0 differences。Manifest v2 更新为 421 case、2,726 条关联、421 个 implemented（verification）、34 个 differential-verified、44 个 differential runtime、34 个 representative-verified、34/34 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSet2OutputLimitDefault` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.94（2026-08-14），新增 `rollup-output-no-limit-market` 差分场景，对照 `ResultSet1NoOutputLimit`（`java-runtime-1d3a41d9dab78e4757e0`）。场景复用 5.5s SupportMarketDataBean rollup(symbol) 序列，无 output 策略，双跑 12 次逐事件/淘汰输出。运行时修复 rollup remove 路径 affected 顺序（同一 delta 多事件时先 leaf 再 coarser）与 `OutputEveryTime` 空 tick 调度推进。evidence 12 条 records、0 differences。Manifest v2 更新为 420 case、2,725 条关联、420 个 implemented（verification）、33 个 differential-verified、43 个 differential runtime、33 个 representative-verified、33/33 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSet1NoOutputLimit` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.93（2026-08-14），新增 `rollup-output-first-market` 差分场景，对照 `ResultSet5OutputLimitFirst`（`java-runtime-1e0ad81982656cd82bcd`）。场景复用 5.5s SupportMarketDataBean rollup(symbol) 序列，用 irstream output-first 双跑 11 次输出（含事件时刻首行输出与窗口淘汰）。evidence 11 条 records、0 differences。Manifest v2 更新为 419 case、2,724 条关联、419 个 implemented（verification）、32 个 differential-verified、42 个 differential runtime、32 个 representative-verified、32/32 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSet5OutputLimitFirst` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.92（2026-08-14），新增 `rollup-output-last-market` 差分场景，对照 `ResultSet4OutputLimitLast`（`java-runtime-548e5460f3bc34ee8cbb`）。场景复用 5.5s SupportMarketDataBean rollup(symbol) 序列，用 irstream output-last 双跑 6 次输出。运行时修复 `OutputLastEveryTime` 空 tick 不推进调度的问题；Java oracle 跳过空回调并只在真实输出时递增 sequence。evidence 6 条 records、0 differences。Manifest v2 更新为 418 case、2,723 条关联、418 个 implemented（verification）、31 个 differential-verified、41 个 differential runtime、31 个 representative-verified、31/31 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSet4OutputLimitLast` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.91（2026-08-14），新增 `rollup-output-snapshot` 差分场景，对照 `ResultSet6OutputLimitSnapshot{join=false}`（`java-runtime-9d82a6fe2510f171a8f4`）。场景用 SupportMarketDataBean#time(5.5 sec) + rollup(symbol) + snapshot every 1s 的完整序列双跑。运行时修复时间型 output 的调度锚点（从首次事件开始，对齐 Java 1200/2200 等快照点）与 rollup 快照排序（层级 + 首条保留事件位置）；compat 层统一 Java `Instant.toString()` 的三位毫秒格式。evidence 7 条 records、0 differences。Manifest v2 更新为 417 case、2,722 条关联、417 个 implemented（verification）、30 个 differential-verified、40 个 differential runtime、30 个 representative-verified、30/30 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSet6OutputLimitSnapshot{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.90（2026-08-14），新增 `rollup-output-snapshot-order-limit` 差分场景，对照 `ResultSetOutputSnapshotOrderWLimit`（`java-runtime-3a98c0c727c06ec21cfd`）。场景用 unbound rollup + `output snapshot every 1 seconds order by sum(intPrimitive) limit 3` 双跑，1 秒快照只输出 top-3。Java oracle 增加 `snapshot-order-limit` case；Go 新增 runner、scenario/evidence 与 mutation 门禁。evidence 1 条 record、0 differences。Manifest v2 更新为 416 case、2,721 条关联、416 个 implemented（verification）、29 个 differential-verified、39 个 differential runtime、29 个 representative-verified、29/29 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputSnapshotOrderWLimit` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.89（2026-08-14），新增 `rollup-output-first-sorted` 差分场景，对照 `ResultSetOutputFirstSorted{join=false}`（`java-runtime-b46b070a7b151e7b4ecc`）。Java oracle 增加 `first-sorted` case；Go 新增 runner、10-record scenario/evidence 与 mutation 门禁。evidence 10 条 records、0 differences。Manifest v2 更新为 415 case、2,720 条关联、415 个 implemented（verification）、28 个 differential-verified、38 个 differential runtime、28 个 representative-verified、28/28 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputFirstSorted{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.88（2026-08-14），新增 `rollup-output-first` 差分场景，对照 `ResultSetOutputFirst{join=false}`（`java-runtime-d3ba0dd07bb4e54ef9af`）。Java oracle 增加 `first` case；Go 新增 runner、完整 10-record scenario/evidence 与 mutation 门禁。运行时修复 grouped irstream 的纯时间淘汰语义（当前聚合值作为 New、上一周期值作为 Old）并让 `OutputFirstEveryTime` 同时保留 New/Old、按 rollup 层级排序。evidence 10 条 records、0 differences。Manifest v2 更新为 414 case、2,719 条关联、414 个 implemented（verification）、27 个 differential-verified、37 个 differential runtime、27 个 representative-verified、27/27 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputFirst{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.87（2026-08-14），新增 `rollup-output-every-sorted` 差分场景，对照 `ResultSetOutputDefaultSorted{join=false}`（`java-runtime-c2e751ee09e3300d3dba`）。`RollupScenarioOracle` 按 case 区分无排序 `rollup` 与排序 `rollup-sorted`；Go 新增 `rollup_output_every_sorted.go` runner、`rollup_output_every_sorted_parity_test.go`、sorted scenario/evidence 与 mutation 门禁。evidence 3 条 records、0 differences。Manifest v2 更新为 413 case、2,718 条关联、413 个 implemented（verification）、26 个 differential-verified、36 个 differential runtime、26 个 representative-verified、26/26 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputDefaultSorted{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.86（2026-08-14），修正 `rollup-output-last` 差分证据并新增 `rollup-output-last-sorted` 场景。上一轮 Java oracle 在 `ResultSetOutputLast{join=false}` 的 EPL 中错误加入 `order by`，旧 evidence 实为 sorted 行为；现在 oracle 按 case 区分无排序 `last` 与排序 `last-sorted`。Go 运行时在 grouped `OutputLastEveryTime` 中同时实现 Java 的 rollup 自然输出顺序（leaf group → subtotal → grand total，同层稳定）与 old-row 语义（按输出 key 取上一周期已输出行，新 group 首次 old 为 null 聚合占位）。新增 `rollup-output-last-sorted` scenario/oracle/runner/parity test/mutation 门禁；两个 evidence 均 3 条 records、0 differences。Manifest v2 更新为 412 case、2,717 条关联、412 个 implemented（verification）、25 个 differential-verified、35 个 differential runtime、25 个 representative-verified、25/25 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputLastSorted{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.85（2026-08-14），修复 grouped `OutputLastEveryTime` 的 old-row 语义并把 `rollup-output-last` 登记为第 24 个差分场景。Java 对 grouped rollup 的 output-last 只输出发生变化的 group，old 行携带上一周期已输出值；新到达 group 的首次 old 行为 null 聚合占位行。Go 运行时新增 `applyLastEveryTimeGrouped` 与 `lastEveryOutputRows`（按输出 key 缓存上一周期行），`lastEveryNullResult` 保留投影 group 字段并把聚合字段置 null。新增 `internal/esper/rollup_output_last_parity_test.go`、`TestRunRollupOutputLastDiffWritesPassingEvidence`/`RejectsTraceMutations` 与 `testdata/parity/rollup-output-last.evidence.json`；差分 3 条 records、0 differences。Manifest v2 更新为 411 case、2,716 条关联、411 个 implemented（verification）、0 个 inventoried-only、24 个 differential-verified、34 个 differential runtime、24 个 representative-verified、24/24 场景通过；`output.core` 与 `resultset.aggregate-group-by` 新增 `ResultSetOutputLast{join=false}` 证据；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.84（2026-08-14），新增 `rollup-output-last` 差分场景基础设施（scenario JSON、Go runner、Java oracle、run script、parity runner mode）。Java oracle 已在本机生成 trace；首次差分暴露 Go `OutputLastEveryTime` 的 old-row 语义差异（Java 对新建 group 的首次 old 行输出 null，Go 输出上一周期聚合值），该差异记录为 remaining，本轮不登记 differential evidence，不改变 manifest 计数。

> 最新补充：Draft 4.83（2026-08-14），将 Java `ContextNested` 从 inventoried 提升为 implemented，`case.inventory.context-nested` 映射到 `context.partition`。Go 现有 `TestContextNestedSingleEventTriggerParity` 与 `TestContextNestedNestingFilterCorrectnessParity` 覆盖非 temporal key/category/hash 嵌套路由、三层 key 分区隔离、parent context 属性与 nested selector；新增 `TestContextNestedInvalidParity` 对照重复 child context 名拒绝。temporal/initiated/pattern/iterator/late-coming nested 生命周期仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、410 个 implemented（verification）、0 个 inventoried-only、23 个 differential-verified、23/23 场景通过；ContextNested 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.82（2026-08-14），实现 Java `ContextKeySegmentedWInitTermPrioritized` 的七个显式 initiated execution，`case.inventory.context-key-segmented-w-init-term-prioritized` 从 inventoried 提升为 implemented 并映射到 `context.partition`。新增 `internal/esper/context_key_segmented_w_init_term_prioritized_parity_test.go`，用 `CreateInitiatedTerminatedContext`/`By`、`CreateInitiatedContextBy` 与 grouped aggregate + `OutputWhenTerminated` 对照 `InitTermNoPartitionFilter`、`InitNoTerm`（soda 两变体）、`InitWCorrelatedTermFilter`、`FilterExprTermByFilter`、`FilterExprTermByFilterWExpr` 与 `Invalid`。显式 start 事件按 Java initiated-by 契约不进入聚合；implicit partition-by start 事件暂不进入聚合，相关 execution 仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、409 个 implemented（verification）、1 个 inventoried-only、23 个 differential-verified、23/23 场景通过；ContextKeySegmentedWInitTermPrioritized 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.81（2026-08-14），实现 Java `InfraNWTableCreateIndex` 的 late-create/drop/recreate/multiple-index/invalid execution，`case.inventory.infra-nwtable-create-index` 从 inventoried 提升为 implemented 并映射到 `query.fire-and-forget`。新增 `internal/esper/infra_nwtable_create_index_parity_test.go`，并为 NamedWindow/Table 增加 live `CreateIndex`/`DropIndex`：已有行立即入索引、后续写入原子维护、Context partition 继承 late index；对照 `InfraLateCreate`、`InfraDropCreate`、`InfraMultipleColumnMultipleIndex` 与 `InfraInvalid`。range/key widening、late-create scene two、on-select reuse 与 multikey FAF 变体仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、408 个 implemented（verification）、2 个 inventoried-only、23 个 differential-verified、23/23 场景通过；InfraNWTableCreateIndex 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.80（2026-08-14），实现 Java `ResultSetOutputLimitAggregateGrouped` 的八个 no-join execution（另有 ResultSet1/3 已有差分证据），`case.inventory.resultset-output-limit-aggregate-grouped` 从 inventoried 提升为 implemented 并映射到 `resultset.aggregate-group-by`。新增 `internal/esper/resultset_output_limit_aggregate_grouped_parity_test.go`，复用 grouped time-window 场景对照 Default/Last/First/Snapshot（含 having）、NoJoinDefault、NoJoinLast、MaxTimeWindow 与 NoOutputClauseView。join/all/multikey-array 变体仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、407 个 implemented（verification）、3 个 inventoried-only、23 个 differential-verified、23/23 场景通过；ResultSetOutputLimitAggregateGrouped 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.79（2026-08-14），实现 Java `ResultSetOutputLimitRowPerGroupRollup` 的十个 no-join execution，`case.inventory.resultset-output-limit-row-per-group-rollup` 从 inventoried 提升为 implemented 并映射到 `resultset.aggregate-group-by`。新增 `internal/esper/resultset_output_limit_row_per_group_rollup_parity_test.go`，用现有 `GroupByRollup` + `OutputEveryTime`/`OutputLastEveryTime`/`OutputFirstEveryTime`/`OutputSnapshotEvery`/`OutputAll` 与 order-by/limit 对照 NoOutputLimit、Default、Last、First、Snapshot、SnapshotOrderWLimit、Default/DefaultSorted、Last/LastSorted。`output all every time` 与全部 join=true 变体仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、406 个 implemented（verification）、4 个 inventoried-only、23 个 differential-verified、23/23 场景通过；ResultSetOutputLimitRowPerGroupRollup 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.78（2026-08-14），实现 Java `ExprFilterOptimizableConditionNegateConfirm` 的十个 listener 可观测 execution，`case.inventory.expr-filter-optimizable-condition-negate-confirm` 从 inventoried 提升为 implemented 并映射到 `expr.filter-expressions`。新增 `internal/esper/expr_filter_optimizable_condition_negate_confirm_parity_test.go`，用 typed `And`/`Or`/`Equal`/`Like` 与 `CreateInitiatedTerminatedContext`/`PatternFrom(...).Every().Then(...)` 对照 OnePath/TwoPath/FourPath 的 context 与 pattern filter 矩阵。Java `SupportFilterPlanHook` 的 plan-node 内部断言、dataflow/stage/context-category/pattern 变体仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、405 个 implemented（verification）、5 个 inventoried-only、23 个 differential-verified、23/23 场景通过；ExprFilterOptimizableConditionNegateConfirm 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.77（2026-08-14），实现 Java `InfraNWTableOnSelect` 的非聚合 execution，`case.inventory.infra-nwtable-on-select` 从 inventoried 提升为 implemented 并映射到 `trigger.table-named-window`。新增 `internal/esper/infra_nwtable_on_select_parity_test.go`，用 typed on-trigger 对照 named-window/table 两种形态：`InfraOnSelectIndexSimple`、`InfraSelectCorrelationDelete`、`InfraSelectCondition`、`InfraSelectJoinColumnsLimit` 与 `InfraInvalid`。运行时修复：on-select 结果应用 order-by 与 limit/offset；trigger WHERE/select 校验拒绝 aggregate 与 prev/prior。剩余 execution 需要 on-select aggregate/group/having、window()/enum 投影、pattern trigger 与 index-plan hook，仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、404 个 implemented（verification）、6 个 inventoried-only、23 个 differential-verified、23/23 场景通过；InfraNWTableOnSelect 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.76（2026-08-14），实现 Java `EPLOtherCreateSchema` 的主要 execution，`case.inventory.epl-other-create-schema` 从 inventoried 提升为 implemented 并映射到 `event.inheritance`。新增 `internal/esper/epl_other_create_schema_parity_test.go`，用 typed `RegisterMap`/`RegisterObjectArray`/`RegisterJSON`/`RegisterAvro`/`RegisterStruct` 对照 create-schema：path/public/configured、col-def、primitive array、model POJO、nestable map/array fragment、inherit、copyfrom（object-array/map/deep）、variant（predefined/any）、invalid 与 type-parameterized 集合/映射/数组。运行时新增 `WithSchemaCopiedFrom`：按 Java copyfrom 语义复制字段与 nested fragment，但不建立 parent/inheritance 路由；ObjectArray schema 现在拒绝多 supertype。`EPLOtherCreateSchemaWithEventType`、Java public CRC identity 与 JVM class-name loading 仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、403 个 implemented（verification）、7 个 inventoried-only、23 个 differential-verified、23/23 场景通过；EPLOtherCreateSchema 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.75（2026-08-14），实现 Java `EPLOtherStaticFunctions` 的主要 execution，`case.inventory.epl-other-static-functions` 从 inventoried 提升为 implemented 并映射到 `client.extend.single-row-function`。新增 `internal/esper/epl_other_static_functions_parity_test.go`，用 Go-native `Func0`-`Func4`/`Func*Ctx` UDF 对照 Java 静态方法调用：单/双参数、无参数、复杂参数、嵌套调用、多 invocation、null 传播、数组参数、primitive conversion、chained instance/static、escape/pattern filter、where/group-by/having/order-by、runtime exception 与 current_timestamp 格式化。运行时修复：投影 Row 结果保留源事件，使 `output every` 等延迟输出策略能重新求值引用事件字段/表达式的 order-by key（对照 `Math.pow(price,2)` 排序）。`EPLOtherReturnsMapIndexProperty`、两个 timing perf execution 与 Java time enum API 仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、402 个 implemented（verification）、8 个 inventoried-only、23 个 differential-verified、23/23 场景通过；EPLOtherStaticFunctions 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.74（2026-08-14），实现 Java `EPLDatabaseJoin` 的主要 join executions，`case.inventory.epl-database-join` 从 inventoried 提升为 implemented 并映射到 `query.historical-sql`。新增 `EPLDatabase2HistoricalStar`/`Inner`（参数化历史流按触发事件 lineage 持久化、只与对应 keepall 事件配对、iterator 保留每个触发行）与 `EPLDatabaseWithPattern`（pattern timer 完成时逐次求值无触发历史侧）的 fake database/sql provider 对照；无触发 SQL 侧保持单触发替换以对齐 `EPLDatabase3Stream` 不重复输出。运行时同时保留 dependency-free method source 跨触发 lineage，方法源 join 套件全绿。`EPLDatabaseTimeBatch`/`OM`/`Compile`、`EPLDatabaseMySQLDatabaseConnection` 与剩余 invalid compile variants 仍为 remaining，不宣称差分 parity。Manifest v2 更新为 410 case、2,715 条关联、401 个 implemented（verification）、9 个 inventoried-only、23 个 differential-verified、23/23 场景通过；EPLDatabaseJoin 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.73（2026-08-14），为 Java `EPLDatabaseJoin` 建立第一批 Go 对照证据（case.inventory.epl-database-join 仍为 inventoried，不宣称完成）。新增 `internal/esper/database_join_class_parity_test.go`，用 fake database/sql provider 对照 `EPLDatabaseSimpleJoinLeft`、`EPLDatabaseSimpleJoinRight`（含结果类型）、`EPLDatabaseStreamNamesAndRename`、`EPLDatabasePropertyResolution`、`EPLDatabaseJoinIndexNullType` 与 `EPLDatabaseInvalidPropertyHistorical`/`Invalid1Stream` 的 typed 边界（自引用历史参数解析为 Missing 不产出）；`EPLDatabase3Stream`、`EPLDatabaseVariables`、`EPLDatabaseRestartStatement` 由既有 `database_join_misc_parity_test.go` 覆盖。识别四个运行时 gap 并记录 remaining：`EPLDatabase2HistoricalStar`/`Inner` 需要历史流按当前触发事件配对（Java 不与 keepall 全量事件做笛卡尔积）、`EPLDatabaseTimeBatch` 需要 historical+time_batch join 的 iterator/快照结果、`EPLDatabaseWithPattern` 需要 pattern 驱动未触发历史流求值。本轮不改 manifest 计数，不提升 case；下一轮优先修复这三个 join 运行时语义后再提升。

> 最新补充：Draft 4.72（2026-08-14），实现 Java `ResultSetQueryTypeIterator` 的 17 个 execution，`case.inventory.resultset-query-type-iterator` 从 inventoried 提升为 implemented 并映射到 `resultset.aggregate-group-by`。新增 `internal/esper/resultset_query_type_iterator_parity_test.go`，用 `Statement.Snapshot` 覆盖 order-by wildcard/projected rows、filtered-window iterator、row-per-group/row-per-event/row-for-all aggregate iterator（含 having）、以及 pattern iterator（unbound @IterableUnbound 与 lastevent window）。运行时改动：grouped aggregate iterator 按“每组第一个保留事件在窗口中的位置”排序（Java iterator 契约），而 output-snapshot listener 批次保持 group 创建顺序；row-per-event snapshot 按到达顺序遍历保留事件并支持 `ResultField` order-by；ungrouped row-per-event snapshot 不再输出空组 null 行；`@IterableUnbound` 以 `WithIterableUnbound()` 暴露并在 pattern 完成时保留匹配行供 iterator 读取。Java `TestSuiteResultSetQueryType#testResultSetQueryTypeIterator` 在固定 commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` 下通过；manifest 更新为 410 case、2,715 条关联、400 个 implemented（verification）、10 个 inventoried-only、23 个 differential-verified、23/23 场景通过。ResultSetQueryTypeIterator 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.71（2026-08-14），实现 Java `ResultSetQueryTypeRowPerGroup` 的 17 个 execution，`case.inventory.resultset-query-type-row-per-group` 从 inventoried 提升为 implemented 并映射到 `resultset.aggregate-group-by`。新增 `internal/esper/resultset_row_per_group_parity_test.go`，覆盖 grouped sum/avg over length 窗口与 join（irstream old rows）、单/双 key grouped count 与 iterator 顺序、order-by grouped aggregate、avg-expr group-by、method 表达式 group key、output snapshot every N 的 unbound iterator、named-window delete 的 group 置空、per-group window access、int-array group key（unbound/keepall/join）、null-typed group key、time-batch+unique grouped 交付。运行时新能力：`reclaim_group_aged/freq` hint（在 aggregate enter 上 sweep，nextSweepTime=now+freq，age>aged 的 group 删除；支持数值或数值变量参数；非法参数在 Build 拒绝）；聚合 group 按创建顺序迭代（对齐 Java LinkedHashMap iterator）；unbound 源 aggregate iterator 回退到实时 group；空 group 删除同步清理顺序追踪。1000-key unlimited-key execution 以 40-key 简化变体覆盖相同 aged/freq 边界。Java `TestSuiteResultSetQueryType#testResultSetQueryTypeRowPerGroup` 在固定 commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` 下通过；manifest 更新为 410 case、2,715 条关联、399 个 implemented（verification）、11 个 inventoried-only、23 个 differential-verified、23/23 场景通过。ResultSetQueryTypeRowPerGroup 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.70（2026-08-14），实现 Java `InfraNamedWindowTypes` 的 18 个 execution，`case.inventory.infra-named-window-types` 从 inventoried 提升为 implemented 并映射到 `infra.namedwindow.views`。新增 `internal/esper/infra_named_window_types_parity_test.go`，覆盖显式重命名列（struct/map 多源）、wildcard 窗口形状（no-fields、no-specification、with-fields、inheritance）、常量列、create-window 列清单 + cast、数组列、Map/ObjectArray/JSON/JSON-provided/Avro 表示的嵌套 schema 列与 fragment 属性导航、composite unique/keepall 与 union retention 形状、嵌套 fragment 上的 unique key。运行时修复：schema 继承现在允许子声明字段覆盖继承属性（Go 内嵌 struct 会把字段提升到子 schema 自己的发现字段），不再报 duplicate。Java `TestSuiteInfraNamedWindow#testInfraNamedWindowTypes` 在固定 commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` 下通过；manifest 更新为 410 case、2,715 条关联、398 个 implemented（verification）、12 个 inventoried-only、23 个 differential-verified、23/23 场景通过。InfraNamedWindowTypes 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.69（2026-08-14），实现 Java `ViewTimeWin` 的 15 个 execution，`case.inventory.view-time-win` 从 inventoried 提升为 implemented 并映射到 `view.basic-windows`。新增 `internal/esper/view_time_win_parity_test.go`，覆盖 istream/rstream 时间窗口淘汰与 iterator、ungrouped/grouped/filtered sum 聚合、prev/prevtail/prevcount/prevwindow 访问、calendar-month 窗口（`time(1 month)` 从 2 月 1 日精确在 3 月 1 日淘汰）、prepared statement 与变量窗口时长（秒/毫秒/分钟）、复合时间单位拼写（等价 Go duration）以及四个 flip-timer 变体（含大起始时间下的 calendar-month 边界）。运行时改动：`TimeWindowSpec` 增加 `CalendarYears/Months/Days` 与 `Expr` 字段，新增 `TimeWindowCalendar`/`TimeWindowExpr`/`TimeWindowSeconds`/`TimeWindowMilliseconds`/`TimeWindowMinutes` 等 typed builder；日历周期先 `AddDate` 再加毫秒余量（对齐 Esper `time(1 months 10 milliseconds)`）；表达式时长在部署时以语句变量/参数快照（对齐 Esper view-creation 求值），并随空窗口状态清理后在新状态重建时恢复（`windowExprDurations` 保存在 runtime 上）；window expression 纳入参数收集与校验。Java `TestSuiteView` 27/27 在固定 commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` 下通过；manifest 更新为 410 case、2,715 条关联、397 个 implemented（verification）、13 个 inventoried-only、23 个 differential-verified、23/23 场景通过。ViewTimeWin 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.68（2026-08-14），实现 Java `ViewUnion` 的 15 个 execution，`case.inventory.view-union` 从 inventoried 提升为 implemented 并映射到 `view.window-core`。新增 `internal/esper/view_union_parity_test.go`，用 typed `UnionWindows(...)` 对照 retain-union：firstunique+firstlength 双向、length_batch+unique、两/三 unique、sorted+sorted、time+unique（双向）、groupwin#length+groupwin#unique、pattern 派生流、子查询、named-window union（time/unique/firstunique/firstlength/length/keepall 子视图 + on-delete + 时间淘汰）与 nested-groupwin invalid。运行时改动：named-window composite retention 支持 UnionWindowMode（子视图含 TimeWindow/FirstLength），insert/update/expire 按 any-child retains 计算窗口内容；union old 流在事件离开全部子视图或到达即被全部子视图淘汰时输出，batch 子视图同时贡献 last flushed batch 与 currently accumulating batch（Java ref-count 模型）；`compositeChildRetainedEvents` 使 union snapshot 与 Java union iterator 一致。Java oracle 探针确认 `#groupwin(k)#length(n)#unique(k2) retain-union` 编译为 groupwin+length 与 groupwin+unique 两个子视图；`#uni` 派生视图映射为 Sum aggregate，pattern window 映射为 insert-into 派生流，group/merge mismatch invalid 因 Go 无 merge view 而不可表达（记为 API 差异）。Java `TestSuiteView` 27/27 在固定 commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` 下通过；manifest 更新为 410 case、2,715 条关联、396 个 implemented（verification）、14 个 inventoried-only、23 个 differential-verified、23/23 场景通过。ViewUnion 仍为 implemented 而非 differential-verified；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.67（2026-08-14），实现 Java `EPLInsertInto` 的 20 个 execution，`case.inventory.epl-insert-into` 从 inventoried 提升为 implemented 并映射到 `query.insert-into-route`。新增 `internal/esper/epl_insert_into_parity_test.go`，用 typed fluent `InsertInto`/`WithRemoveStreamOnly` 链式 API 对照：wildcard 三语句链式路由（单模块与独立部署）、Null 类型属性、多事件数组列、部分列目标显式 null、rstream-only 路由、named/unnamed delta+product 投影 + time(60) min/max 聚合消费者、wildcard 事件身份保留、join wildcard 投影、共享目标类型冲突拒绝、pattern 显式投影要求、Map/ObjectArray/JSON/Avro/JSON-class-provided 表示路由与 lenient 部分属性计数（MAP/OBJECTARRAY/JSON/AVRO）。运行时配套改动：`Environment.typeToName` 支持同一 Go struct 注册多个事件类型名（源与 insert-into 目标），type-based `SendEvent` 对歧义返回显式错误并引导使用 name-based `Engine.Send`（`TestSendEventAmbiguousGoTypeParity` 锁定）；`projectMapToSchema` 按目标 schema 类型做数值转换；未分组聚合在纯时间淘汰批次也输出当前行（对齐 Java 未分组 time-window 契约，分组 istream 继续抑制纯淘汰）。Java `TestSuiteEPLInsertInto` 15/15、`TestSuiteEPLInsertIntoWConfig` 2/2 在固定 commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` 下通过；manifest 更新为 410 case、2,715 条关联、395 个 implemented（verification）、18 个 intentionally-different、23 个 differential-verified、23/23 场景通过。EPLInsertInto 仍为 implemented 而非 differential-verified，不宣称行为 parity；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.67（2026-08-14），完成 statement metrics CPU 采样差异审查并登记为 approved intentional difference。根因确认：Java 通过 `ThreadMXBean.getCurrentThreadCpuTime()` 采样当前线程 CPU 时间；Go 无 portable per-goroutine CPU clock，Linux 用进程 `getrusage(RUSAGE_SELF)`（goroutine 可能跨 OS 线程迁移），非 Linux 构建无 sampler 时显式返回 0。`case.client-instrument-statement-metrics` 与 capability `client.instrument.statement-metrics` 均加入 intentionally-different verification 并记录 Java 行为、Go 行为、原因、影响范围（仅 CPUTime 数值可比性；WallTime 与计数不变）、替代方案（cgo `RUSAGE_THREAD` 无法跟踪 goroutine 迁移且引入 cgo 成本）和自动化测试证据（`TestClientInstrumentMetricsReportingStmtMetricsParity` 在 sampler 可用平台断言 CPU>0 与 WallTime 工作量）。Manifest 的 intentionally-different case 增至 18 个，capability 增至 7 个；statement metrics 的 WallTime/input/output 契约继续由全量测试保持绿色。

> 最新补充：Draft 4.66（2026-08-14），新增第二十三个 Java/Go 双跑代表性场景 `resultset-row-per-group-simple`，对照 `ResultSetQueryTypeRowPerGroup.ResultSetQueryTypeRowPerGroupSimple`（`java-runtime-d7060e63eda2baaf87f5`），并把该 runtime 从 inventoried 提升为 differential-verified。场景用 unbound SupportBean `group by theString` + sum/min/max 投影双跑 9 条 insert 记录；差分 0 differences。Manifest 更新为 410 case、2,715 条关联、23 个 differential-verified、33 个 differential runtime、23 个 representative-verified、23/23 场景通过。

> 最新补充：Draft 4.65（2026-08-14），再登记五个高风险 Java 类的 108 个未关联 runtime 为 inventoried-only case：`ContextKeySegmentedWInitTermPrioritized`（23）、`EPLInsertInto`（20）、`ExprFilterOptimizableConditionNegateConfirm`（26）、`InfraNWTableCreateIndex`（22）、`EPLDatabaseJoin`（17）。只记录 inventory 证据，不宣称 Go 实现或差分。Manifest 更新为 409 case、2,715 条关联、2,630 个唯一 runtime（63.6%）、未关联 1,506 个。

> 最新补充：Draft 4.64（2026-08-14），再登记五个高风险 Java 类的 90 个未关联 runtime 为 inventoried-only case：`EPLOtherStaticFunctions`（24）、`InfraNamedWindowTypes`（18）、`ResultSetQueryTypeRowPerGroup`（18）、`ViewUnion`（15）、`ViewTimeWin`（15）。只记录 inventory 证据，不宣称 Go 实现或差分。Manifest 更新为 404 case、2,607 条关联、2,522 个唯一 runtime（61.0%）、未关联 1,614 个。

> 最新补充：Draft 4.63（2026-08-14），`resultset-grouped-time-window` 场景扩展 `ResultSet3NoneHavingNoJoin`（`java-runtime-11c5505c0b19dd86671c`）：同一 0-8s 事件序列在 `having sum(price) > 50` 下只输出 IBM 75/97 两条，Java/Go 共 11 records / 0 differences；该 runtime 从 inventoried 提升为 differential-verified，Manifest 更新为 32 个 differential runtime。

> 最新补充：Draft 4.62（2026-08-14），新增第二十二个 Java/Go 双跑代表性场景 `resultset-grouped-time-window`，对照 `ResultSetOutputLimitAggregateGrouped.ResultSet1NoneNoHavingNoJoin`（`java-runtime-9061246c4c67a2c25364`），并把该 runtime 从 inventoried case 提升为 differential-verified。场景用 `select symbol, volume, sum(price) from SupportMarketDataBean#time(5.5 sec) group by symbol`（istream 默认）双跑 9 条 insert 记录；差分 0 differences。期间修复 Go 聚合结果集语义：纯时间淘汰批次（无新事件）不再输出 new 行；实现 `eventDelta.hadInput`/`NamedWindowDelta.External` 并在 insert/delete/update/merge/FAF/trigger/joinDelta 路径传播，使外部 old-only 批与窗口淘汰仍按 Java 契约输出；`TestResultSetGroupedTimeWindowIStreamParity` 锁定该契约。irstream 纯 grouped time-window 更新的 old 行仍为 documented gap。Manifest 更新为 399 case、2,517 条关联、22 个 differential-verified、31 个 differential runtime、22 个 representative-verified、22/22 场景通过。

> 最新补充：Draft 4.61（2026-08-14），将六个高风险 Java 类的 149 个未关联 runtime 登记为 inventoried-only case：`ContextNested`（33）、`ResultSetOutputLimitAggregateGrouped`（29）、`InfraNWTableOnSelect`（28）、`ResultSetOutputLimitRowPerGroupRollup`（24）、`EPLOtherCreateSchema`（18）、`ResultSetQueryTypeIterator`（17）。这些 case 只记录 Java runtime inventory 证据，不包含 Go 实现或差分证据，remaining 明确列出待实现；Manifest 更新为 398 case、2,517 条关联、2,432 个唯一 runtime（58.8%）、未关联 1,704 个。

> 最新补充：Draft 4.60（2026-08-14），新增第二十一个 Java/Go 双跑代表性场景 `rowrecog-aggregation`（`testdata/parity/rowrecog-aggregation.json` 与 `.evidence.json`），对照 `RowRecogAggregation.RowRecogMeasureAggregation`（`java-runtime-254b2ace6ea437689d15`）与 `RowRecogMeasureAggregationPartitioned`（`java-runtime-0de4e4485dc851b8c4a5`），新增 `tools/java-oracle/RowRecogAggregationScenarioOracle.java` 与 `run-rowrecog-aggregation.sh`。场景覆盖 A B* C 的 max/min/2x/last/first/count measures 与 partition-by-cat 的 sum measures，三次 unpartitioned 快照 + 两次 partitioned 快照；差分 10 records / 0 differences。Manifest 更新为 392 case、2,368 条关联、21 个 differential-verified、30 个 differential runtime、21 个 representative-verified、21/21 场景通过。

> 最新补充：Draft 4.59（2026-08-14），实现 stress 热点优化：新增 `windowHistoryByEventRequired`，普通 length/time 窗口不再每次 insert/remove/expire 重建 `historyByEvent`（这些窗口的 `eventDelta.history` 就是同一完整历史）；group、time-order/sorted/accum 与 batch 窗口仍保留按需/批量前缀 map。`TestStressSyntheticMediumLoad` 基线从 42.6s 降至 18.45s（12,200 事件，约 660 events/s），filter-window-aggregate 子项从 28.5s 降至 6.3s；全量 `go test ./... -count=1` 与 `go test -race ./... -count=1` 通过。剩余性能/NFR 开放。

> 最新补充：Draft 4.58（2026-08-14），补充 stress 基线根因：`go tool pprof -alloc_space` 显示 5,000 个 length(200) 聚合事件累计约 83GB 分配，其中 82.6% 来自 `windowHistoryByEvent`——每次 `insert` 都重建 `historyByEvent` map，并为窗口内每个事件复制完整历史切片；`aggregateGroupContext` 再复制一次。优化方向是仅在实际需要 Prev/Prior/窗口访问/历史表达式的查询中按需构建 `historyByEvent`，并复用不可变窗口快照，避免 O(events×window) 的分配。该优化作为下一切片，性能/NFR 仍未完成。

> 最新补充：Draft 4.57（2026-08-14），新增环境门控的周期 stress 基线 `TestStressSyntheticMediumLoad`（`ESPER_STRESS=1 go test ./internal/esper -run '^TestStressSyntheticMediumLoad$' -count=1 -timeout 5m`）：固定种子 42 覆盖 filter+length(200)+sum 快照、5,000 个 keyed context 分区、join 部署/卸载状态清理；语义不变量全部通过，但 12,200 事件耗时 42.6s（约 280 events/s），该低吞吐根因未定位，性能/NFR 不宣称完成。manifest `quality.stress` 记录 baseline-captured，普通 `go test` 显式 skip。

> 最新补充：Draft 4.56（2026-08-14），完成 Docker 外部依赖门控验证：MySQL 8.0（`TestDBConnectorMySQLDocker`、`TestSQLHistoricalProviderMySQLDocker`、`TestSQLHistoricalFireAndForgetMySQLDocker`、`TestSQLSinkMySQLDocker`）、Kafka 3.8.1（`TestKafkaDockerRoundTrip`，重建了损坏的 fixture）、RabbitMQ 3.x（`TestAMQPDockerRoundTrip`、`TestAMQPDockerSinkRoundTrip`）全部通过；文档记录精确命令与验证日期，manifest `quality.docker` 更新为 passed。普通 `go test ./...` 仍按环境变量显式 skip。

> 最新补充：Draft 4.55（2026-08-14），新增第二十个 Java/Go 双跑代表性场景 `context-keyed-subquery`（`testdata/parity/context-keyed-subquery.json` 与 `.evidence.json`），对照 `ContextKeySegmented.ContextKeySegmentedSubqueryFiltered`（`java-runtime-9aa54450b89fbc94ab73`），新增 `tools/java-oracle/ContextKeyedSubqueryScenarioOracle.java` 与 `run-context-keyed-subquery.sh`。场景双跑 keyed context 内 correlated `#lastevent` 子查询：G1 先空、s2 可见；新建 G2 为空、s3 可见；新建 G3 为空；G1 再触发看到 s3，差分 6 records / 0 differences，并规范 null 为 `{"state":"null"}`。Manifest 更新为 391 case、2,366 条关联、20 个 differential-verified、28 个 differential runtime、20 个 representative-verified、20/20 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.54（2026-08-14），新增第十九个 Java/Go 双跑代表性场景 `output-first-having`（`testdata/parity/output-first-having.json` 与 `.evidence.json`），对照 `ResultSetOutputLimitFirstHaving.ResultSetHavingNoAvgOutputFirstEvents`（`java-runtime-fe5c5a8ab0d234ff63c7`）与 `ResultSetHavingNoAvgOutputFirstMinutes`（`java-runtime-52135d56e04dfb024bfe`），新增 `tools/java-oracle/OutputFirstHavingScenarioOracle.java` 与 `run-output-first-having.sh`。events case 双跑 `having doublePrimitive > 1 output first every 2 events` 的 4 次可见输出；time case 双跑 `length(5) sum > 100 output first every 2 seconds` 的 3 次虚拟时钟输出（101/114/102），差分 7 records / 0 differences。Manifest 更新为 390 case、2,365 条关联、19 个 differential-verified、27 个 differential runtime、19 个 representative-verified、19/19 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.53（2026-08-14），新增第十八个 Java/Go 双跑代表性场景 `unidirectional-aggregate-join`（`testdata/parity/unidirectional-aggregate-join.json` 与 `.evidence.json`），对照 `EPLJoinUnidirectionalStream.EPLJoin2TableJoinGrouped`（`java-runtime-b89b3e49bf5d655c3bb6`），新增 `tools/java-oracle/UnidirectionalJoinScenarioOracle.java` 与 `run-unidirectional-join.sh`。场景双跑 `SupportMarketDataBean unidirectional, SupportBean#keepall where theString = symbol group by theString, symbol` 的 irstream 分组聚合：三次 driver 触发分别输出 new/old {E1,1/0}、{E1,2/0}、{E2,1/0}，差分 3 records / 0 differences。Manifest 更新为 389 case、2,363 条关联、18 个 differential-verified、25 个 differential runtime、18 个 representative-verified、18/18 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.52（2026-08-13），新增第十七个 Java/Go 双跑代表性场景 `match-recognize-simple`（`testdata/parity/match-recognize-simple.json` 与 `.evidence.json`），对照 `RowRecogOps.RowRecogConcatenation`（`java-runtime-ce4e140777e93d5226b5`），新增 `tools/java-oracle/MatchRecognizeScenarioOracle.java` 与 `run-match-recognize.sh`。场景双跑 pattern (A B) + measures + order by，差分 2 records / 0 differences。Manifest 更新为 388 case、2,362 条关联、17 个 differential-verified、24 个 differential runtime、17 个 representative-verified、17/17 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.51（2026-08-13），新增第十六个 Java/Go 双跑代表性场景 `rollup-output-every`（`testdata/parity/rollup-output-every.json` 与 `.evidence.json`），对照 `ResultSetOutputLimitRowPerGroupRollup.ResultSetOutputDefault{join=false}`（`java-runtime-3e89e6297511b7dfe531`），新增 `tools/java-oracle/RollupScenarioOracle.java` 与 `run-rollup.sh`。场景双跑 time-window + rollup + irstream + output every 1s，差分 3 records / 0 differences。Manifest 更新为 387 case、2,361 条关联、2,283 个唯一 runtime（55.1%）、16 个 differential-verified、23 个 differential runtime、16 个 representative-verified、16/16 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.50（2026-08-13），新增第十五个 Java/Go 双跑代表性场景 `output-after-last`（`testdata/parity/output-after-last.json` 与 `.evidence.json`），对照 `ResultSetOutputLimitAfter.ResultSetAfterWithOutputLast`（`java-runtime-523f306d58fe926463bf`），新增 `tools/java-oracle/OutputAfterScenarioOracle.java` 与 `run-output-after.sh`。场景双跑 `output after 4 events last every 2 events`，差分 1 record / 0 differences。Manifest 更新为 386 case、2,360 条关联、15 个 differential-verified、22 个 differential runtime、15 个 representative-verified、15/15 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.49（2026-08-13），新增第十四个 Java/Go 双跑代表性场景 `dataflow-connector-output`（`testdata/parity/dataflow-connector-output.json` 与 `.evidence.json`），对照 `EPLDataflowOpEventBusSink.EPLDataflowBeacon`（`java-runtime-1ecb769e10b4a818772c`），新增 `tools/java-oracle/DataflowConnectorScenarioOracle.java` 与 `run-dataflow-connector.sh`。场景双跑 BeaconSource → EventBusSink 的 connector 输出路径，差分 3 records / 0 differences。Manifest 更新为 385 case、2,359 条关联、14 个 differential-verified、21 个 differential runtime、14 个 representative-verified、14/14 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.48（2026-08-13），新增第十三个 Java/Go 双跑代表性场景 `time-window-long-running`（`testdata/parity/time-window-long-running.json` 与 `.evidence.json`），对照 `InfraNamedWindowViews.InfraTimeWindow`（`java-runtime-d00485a2855b2128289f`），新增 `tools/java-oracle/TimeWindowScenarioOracle.java` 与 `run-time-window.sh`。场景双跑 time(10s) named window 的 insert/expire/delete/snapshot 长序列，差分 10 records / 0 differences；同时修复空 snapshot 的 `ResultBatch.Time`（保留当前虚拟时钟）。Manifest 更新为 384 case、2,358 条关联、13 个 differential-verified、20 个 differential runtime、13 个 representative-verified、13/13 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.47（2026-08-13），新增第十二个 Java/Go 双跑代表性场景 `high-cardinality-context`（`testdata/parity/high-cardinality-context.json` 与 `.evidence.json`），对照 `ContextKeySegmented.ContextKeySegmentedLargeNumberPartitions`（`java-runtime-bd4672ec90862ee08790`）的缩减版，双跑 40 个 keyed context 分区的独立聚合与更新，差分 52 records / 0 differences；parity 协议将 keyed partition key 规范化为 `key:E00`。Manifest 更新为 383 case、2,357 条关联、12 个 differential-verified、19 个 differential runtime、12 个 representative-verified、12/12 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.46（2026-08-13），新增第十一个 Java/Go 双跑代表性场景 `deployment-restart-window`（`testdata/parity/deployment-restart-window.json` 与 `.evidence.json`），对照 `EPLJoinStartStop.EPLJoinStartStopSceneOne`（`java-runtime-4ae08580480e15494fea`），新增 `tools/java-oracle/DeploymentRestartScenarioOracle.java` 与 `run-deployment-restart.sh`。场景以 case 边界模拟 undeploy/redeploy，三个全新 deployment 覆盖匹配、不匹配、再次匹配，证明 length(3) join 状态彻底重置；差分 6 records / 0 differences。Manifest 更新为 382 case、2,356 条关联、11 个 differential-verified、18 个 differential runtime、11 个 representative-verified、11/11 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.45（2026-08-13），新增第十个 Java/Go 双跑代表性场景 `context-output-termination`（`testdata/parity/context-output-termination.json` 与 `.evidence.json`），对照 `ContextInitTerm.ContextInitTermOutputSnapshotWhenTerminated`（`java-runtime-4b19317271be7df8b569`），新增 `tools/java-oracle/ContextOutputScenarioOracle.java` 与 `run-context-output.sh`。场景双跑 every-minute cron initiated/terminated context + `output snapshot when terminated`，差分 2 records / 0 differences。期间确认 cron 起始 context 的“挂起”来自 Go 引擎 epoch 0 追赶全部历史分钟分区，部署前用 `WithStartTime` 对齐 Java 起点即可；parity 协议将 listener 记录与 snapshot 记录分离（listener 不再携带 partitions），`context-hash-segmented.evidence.json` 已重新生成。Manifest 更新为 381 case、2,355 条关联、10 个 differential-verified、17 个 differential runtime、10 个 representative-verified、10/10 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.44（2026-08-13），新增第九个 Java/Go 双跑代表性场景 `variable-deploy`（`testdata/parity/variable-deploy.json` 与 `.evidence.json`），对照 `EPLVariablesOnSet.EPLVariableOnSetWDeploy`（`java-runtime-78f23a1b470f6bd625c4`），新增 `tools/java-oracle/VariableDeployScenarioOracle.java` 与 `run-variable-deploy.sh`。场景双跑变量默认值、select 先部署/on-set 后部署、E/S 过滤与变量可见性，差分 6 records / 0 differences；Go 链式 API 无需改动。Manifest 更新为 9 个 differential-verified、16 个 differential runtime、9 个 representative-verified、9/9 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.43（2026-08-13），新增第八个 Java/Go 双跑代表性场景 `table-mutation`（`testdata/parity/table-mutation.json` 与 `.evidence.json`），对照 `InfraTableOnUpdate.InfraTableOnUpdateTwoKey`（`java-runtime-3707b3fa08171bbd791c`），新增 `tools/java-oracle/TableMutationScenarioOracle.java` 与 `run-table-mutation.sh`。场景双跑 create table + merge insert + keyed read + update IR，差分 7 records / 0 differences；parity 协议将 Event 结果统一规范化为 `kind:"row"`。Manifest 更新为 380 case、2,354 条关联、2,282 个唯一 runtime（55.1%）、15 个 differential runtime、8 个 representative-verified、8/8 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.42（2026-08-13），新增第七个 Java/Go 双跑代表性场景 `named-window-mutation`（`testdata/parity/named-window-mutation.json` 与 `.evidence.json`），对照 `InfraNamedWindowOnDelete.InfraFirstUnique`（`java-runtime-62b4f6edf3c24229e8ac`），新增 `tools/java-oracle/NamedWindowMutationScenarioOracle.java` 与 `run-named-window-mutation.sh`。场景双跑 firstunique named window 的 insert/duplicate-ignore/delete/count，差分 10 records / 0 differences；`compat.ReplayWithStatements` 增加多语句 listener 订阅并按 statement 独立 sequence，Go 链式 API 无需运行时改动。Manifest 更新为 379 case、2,353 条关联、2,282 个唯一 runtime（55.1%）、14 个 differential runtime、7 个 representative-verified、7/7 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.41（2026-08-13），新增第六个 Java/Go 双跑代表性场景 `subquery-length-window`（`testdata/parity/subquery-length-window.json` 与 `.evidence.json`），对照 `EPLSubselectAggregatedSingleValue.EPLSubselectUngroupedUncorrelatedInSelect`（`java-runtime-60f4742e4d330725c1e5`），新增 `tools/java-oracle/SubqueryScenarioOracle.java` 与 `run-subquery.sh`。场景双跑 `select (select max(id) from SupportBean_S1#length(3)) as value from SupportBean_S0`，空窗口 null、窗口滑动 100/200/200/200/190；差分 6 records / 0 differences。Go 链式 API 无需改动；`case.subquery-basic` 提升为 differential-verified/representative-verified。Manifest 更新为 2,352 条关联、2,281 个唯一 runtime（55.1%）、13 个 differential runtime、6 个 representative-verified、6/6 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.40（2026-08-13），新增第五个 Java/Go 双跑代表性场景 `pattern-timer-interval`（`testdata/parity/pattern-timer-interval.json` 与 `.evidence.json`），对照 `PatternObserverTimerInterval.PatternIntervalSpecExpressionWithProperty`（`java-runtime-36d18111e663f599bc14`），新增 `tools/java-oracle/PatternTimerScenarioOracle.java` 与 `run-pattern-timer.sh`。场景双跑 `every a=SupportBean -> timer:interval(intPrimitive seconds)`：t=10s 发送 E1(3)/E2(2)，t=12s 输出 E2、t=13s 输出 E1，差分 2 records / 0 differences。Go 链式 API 无需改动；`case.pattern-observer-timer-interval-expression` 提升为 differential-verified/representative-verified。Manifest 更新为 12 个 differential runtime、5 个 representative-verified、5/5 场景通过。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.39（2026-08-13），新增第四个 Java/Go 双跑代表性场景 `output-policy-iterator`（`testdata/parity/output-policy-iterator.json` 与 `.evidence.json`），对照 `ResultSetWildcardRowPerGroup`（`java-runtime-34a8b14ea96443475b9c`）、`ResultSetFirstSimpleHavingAndNoHaving`（`java-runtime-132314df55366e67ec84`）与 `ResultSetUnaggregatedOutputFirst`（`java-runtime-decf2ff614722c1273d1`），新增 `tools/java-oracle/OutputPolicyScenarioOracle.java` 与 `run-output-policy.sh`。场景双跑 `output all/first/last every 3 events` 分组聚合，每个事件后快照；Java oracle 实证这三类输出策略的 iterator 返回实时聚合状态并按当前过滤后窗口事件顺序逐组一行，Go 新增 live 快照路径（`snapshotOutputLimitedAggregateBatch(plan, now, live)` + `outputPolicyIteratorUsesSourceOrder`），默认 count/time 仍保留 last-output 视图。另修复 `output first every N events`：分组查询为逐组 polled 条件（新组立即输出、组内每 N 个事件后再次输出，grouped having 未命中不计入），非分组查询为 witnessed-first 全局算法（首个相关结果输出后，按全部已接受输入事件计间隔，间隔结束后的下一个相关结果输出），Go 的 `applyFirstEveryEvents` 拆为 grouped/ungrouped 两条路径；parity listener sequence 改为 compat replay 回调序号。差分 24 records / 0 differences；mutation 门禁通过。Manifest 更新为 378 case、2,351 条关联、2,280 个唯一 runtime（55.1%）、4 个 differential-verified、4 个 representative-verified。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.38（2026-08-13），新增第三个 Java/Go 双跑代表性场景 `join-length-window`（`testdata/parity/join-length-window.json` 与 `.evidence.json`），对照 `EPLJoinNoWhereClause` 的 `EPLJoinJoinWInnerKeywordWOOnClause`（`java-runtime-4aa869a9f1905a65ab3a`）与 `EPLJoinJoinNoWhereClause`（`java-runtime-7885da47c76db1ebb5f7`），新增 `tools/java-oracle/JoinScenarioOracle.java` 与 `run-join.sh`。场景双跑 `OrderEvent#length(3)` 与 `PaymentEvent#length(3)` 的 equi-join，普通输出与 `output every 2 events` 两个 case，显式 `order by orderId asc, amount asc`；覆盖同键多匹配、length 窗口淘汰边界、ORDER BY 多键和输出计数缓冲。Java oracle 实证非聚合 join 的 iterator 在 `output every N` 下仍为实时 join 状态（pending 可见），Go 的 `snapshotJoinBatch` 无需改动即一致；差分 12 records / 0 differences，Go 回归测试逐点固定 5 个 every-2 snapshot。Manifest 更新为 377 case、2,348 条关联、2,280 个唯一 runtime（55.1%）、3 个 differential-verified、3 个 representative-verified。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.37（2026-08-13），修复 `output every N events` 分组聚合 statement 的 iterator/output-view 语义：Java `statement.iterator()` 返回最后一次输出时每组的行（pending 不可见、未输出组为 null 聚合），Go 此前返回实时聚合状态。every-3 case 新增 4 个 snapshot 步骤，Java oracle 实证 iterator 按当前过滤后窗口事件顺序逐组一行、聚合值取最近输出批次每组最后一行；Go 实现 `lastOutputGroupRows` + `snapshotOutputLimitedAggregateBatch`（默认 count/time 输出策略启用，snapshot 策略保持实时），并修复 pending 批次累积丢失分组键对齐的缺陷。差分 11 records / 0 differences；`output all every N`、first/last、rollup/cube、context+output 组合仍留 remaining。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.36（2026-08-13），新增第二个 Java/Go 双跑代表性场景 `filter-window-aggregate-output`（`testdata/parity/filter-window-aggregate-output.json` 与 `.evidence.json`），对照 `ResultSetOutputLimitAggregateGrouped.ResultSetNoOutputClauseView`（`java-runtime-dc2ac7e7293a204aa102`）、`ResultSetNoJoinAll`（`java-runtime-d7e3f4e3457216b42c0b`）与 `ResultSetOutputLimitSimple.ResultSetSimpleNoJoinAll`（`java-runtime-a02b2a7f648b50c0eeb3`），新增 `tools/java-oracle/FilterWindowAggregateScenarioOracle.java` 与 `run-filter-window-aggregate.sh`。场景覆盖 filter + length(2) window + group-by sum + 窗口淘汰边界 + `output every 3 events` 输出策略。差分发现并修复两个语义问题：Go 链式 `.Filter(...).Window(...)` 先过滤后进窗口，而 Java `from Trade#length(2) where price > 10` 是事件先进窗口、where 只过滤输出/聚合，被过滤事件仍会淘汰窗口事件并产生移除行（runner 与 Go 测试改为 `.Window(...).Filter(...)`）；istream 分组聚合同一批次含新事件行与淘汰重算行时，Go 按淘汰组在前输出而 Java 按新事件组在前，`internal/esper/runtime.go` 的 affected-group 顺序按 `SelectIStream` 先 new 后 old、IRStream 保持 old-first。另记录剩余差异：`output every N` 分组聚合 statement 的 Java iterator 返回上次已发射的每组最新行而 Go 返回实时状态，留在 remaining。Manifest 更新为 376 case、2,346 条关联、2,280 个唯一 runtime（55.1%）、2 个 differential-verified、2 个 representative-verified。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.35（2026-08-13），关闭 Java `ContextHashSegmented` 的 3 个 execution：`ContextHashNoPreallocate`（`java-runtime-64ed27d8c2b0325c1cbd`）、`ContextHashSegmentedManyArg`（`java-runtime-5d429404e8e55f30567a`）和 `ContextHashPartitionSelection`（`java-runtime-c5406f66fdd34533bcb2`），新增 `internal/esper/context_hash_segmented_parity_test.go`。Go 增加显式 `HashAlgorithmCRC32`/`HashAlgorithmJavaHashCode` 以及 lazy/preallocated hash context constructors，既有 constructors 保持 FNV-1a。对照 Java `consistent_hash_crc32`/`hash_code`，lazy CRC32、16-bucket preallocated selector 和 many-argument test 固定 scalar/primitive hash parity：E1/E2 lazy buckets 为 0/1，selector buckets 为 5/15/9，many-argument CRC32/hash-code 在 granularity 1,000,000 下分别为 550184/67928；同时覆盖 descriptor `hash`、partition selector、aggregate `Prev`/`Prior` 和 last-event subquery。aggregate evaluation context 现在保留 group history，使 aggregate `Prev`/`Prior` 与 Java 对照一致。Object/array serializer、更多 event representation/single-row-function/scoring 组合、完整 context administration payload 与更广 hash 矩阵继续保持 difference。Manifest v2 当前记录 4,136 个可执行 runtime、2,343 条关联和 2,278 个唯一 runtime（55.1%）；375 个 case 中 357 implemented、1 differential-verified、17 intentionally-different。ContextHash 场景的 Java/Go trace 已持久化且差异为 0；本轮不需要 MySQL Docker。

> 最新补充：Draft 4.34（2026-08-13），关闭 Java `ContextKeySegmented` 的 3 个 execution：`ContextKeySegmentedSelector`（`java-runtime-b571b059e04938cfeb02`）、`ContextKeySegmentedLargeNumberPartitions`（`java-runtime-bd4672ec90862ee08790`）和 `ContextKeySegmentedNullSingleKey`（`java-runtime-66e2c18a214c52bc1506`），新增 `internal/esper/context_key_segmented_parity_test.go`。Go typed fluent Context 覆盖全量/精确/filtered/empty/unknown selector snapshot、`length(5)` 分区聚合、`Prev`/`Prior`/last-event subquery 计划、10,000 个 lazy key partition 和 nullable single-key 复用。Context-local event-stream subquery registry 只在当前 source type 可消费事件时复制分区变量，避免无关事件在大分区场景形成不必要的 O(partitions) clone。Java exact iterator/safeIterator API、精确 `InvalidContextPartitionSelector` payload、source-specific multi-event context filters 和 multi-statement multi-type context routing 仍保持差异。Coverage 2,275/4,140（55.0%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.33（2026-08-13），关闭 Java `ContextNestedNestingFilterCorrectness` 与 `ContextNestedSingleEventTriggerNested` execution（`java-runtime-5e2121241c64b55f93d2`、`java-runtime-39608a57f59d293b2563`），新增 `internal/esper/context_nested_parity_test.go`。Go typed fluent Context 覆盖非 temporal 的 key/category/hash nested 路由、三层 key 分区聚合隔离、父级 `ContextField` namespace、nested selector，以及 hash-over-hash descriptor/hash 属性。Java execution 中 initiated→initiated parent/child 与 partition→pattern child 分支仍由 Go 明确拒绝，复杂 temporal nested、完整 nested namespace/runtime administration 继续按 difference 保持开放。Coverage 2,272/4,140（54.9%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.32（2026-08-13），关闭 Java `ContextCategory` 剩余 8 个 execution（`java-runtime-440efde2e13b0063a968`、`java-runtime-cbe8b6c2ba86887f6807`、`java-runtime-ad038edc0893e86eba46`、`java-runtime-fe484daf28031390497b`、`java-runtime-96843edb4ce366e1d74e`、`java-runtime-51b59dd76ca097972003`、`java-runtime-629da2acd413b0688d68`、`java-runtime-76b9f0c6ca0a53be2ab0`），新增 `internal/esper/context_category_parity_test.go`。Go 部署首个 flat category statement 时即按 `category:name` 顺序 pre-materialize 固定分区，使 labels/IDs、空聚合快照和 undeploy 清理在任何匹配事件前可见；typed `Field[T,V]` source marker 用于拒绝与 category 声明事件类型不匹配的 flat statement，`KeepAll`+`Prior(0)` 对照单 category prior，descriptor/ID/category selector 与 collecting-filtered selector 覆盖 Java partition selection，declared `ContextLabel`/`Concat` expression 的 call/alias 两形态共用同一 immutable Plan。Java 非 bool category predicate 由 Go generic `Expression[bool]` 在编译期拒绝，textual SODA/alias 与精确 `InvalidContextPartitionSelector` payload 继续按 difference 保持开放。Coverage 2,270/4,140（54.8%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.31（2026-08-13），关闭 Java `ContextAdminListen.ContextAdminListenHash` execution（`java-runtime-fef052809bd46c94e548`），扩展 `internal/esper/context_admin_listen_parity_test.go`。Go 新增显式 `NewPreallocatedHashContext`/`CreatePreallocatedHashContext` 定义：部署首个 flat hash statement 时按 `hash:0` 到 `hash:n-1` 顺序 materialize 子 statement runtime，并通过共享 context partition 引用发出 allocation；descriptor 固定 bucket hash、engine partition ID 和 name/id 属性。Undeploy 保持 statement-removed、两次 deallocation、deactivated，随后 DestroyContext 发出 destroyed 的顺序。既有 `NewHashContext` 仍保持 lazy，Java CRC32/hash identifier semantics 与完整 context administration payload 继续按 difference 保持开放。Coverage 2,262/4,140（54.6%），374 个 case 中 357 个标为 mapped、0 个 partial、17 个 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.30（2026-08-13），关闭 Java `ContextAdminListen.ContextAdminListenInitTerm` execution（`java-runtime-de8ebffec0dd5a719711`），新增 `internal/esper/context_admin_listen_parity_test.go`。Go 以 typed `SupportBean_S0`/`SupportBean_S1` 生命周期事件、`SupportBean` context statement 和 `ContextStateListener`/`ContextPartitionStateListener` 对照 created、statement-added、activated、partition allocated/deallocated、statement-removed、deactivated、destroyed 的完整顺序，并固定跨事件类型启动/终止和分区 ID/key 稳定性。Java `ContextDeploymentID`、`RuntimeURI` 与完整管理事件 payload 仍按既有 context difference 保持开放。Coverage 2,261/4,140（54.6%），374 个 case 中 357 个标为 mapped、0 个 partial、17 个 approved-difference。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.29（2026-08-13），关闭 Java `ExprDefineAliasFor.ExprDefineAliasAggregation` 与 `ExprDefineInvalid` execution（`java-runtime-2626fc42b586909c263e`、`java-runtime-1e59e97457b03ad30658`），新增 `internal/esper/expr_define_alias_for_remaining_parity_test.go`。Go 以零参数 typed `DefineExpression` 保存 `sum(intPrimitive)`，用隐式 ungrouped `Aggregate` 对照 `total`/`total+1` 的 `10/11` 结果和 int schema；invalid 矩阵覆盖声明体未知字段、零参数声明多传参数和一参数声明缺参数的 `ErrorInvalidRule`/arity diagnostics。Java 的 `xxx` keyword、lambda 参数和 script-body parser 失败在 fluent AST 中没有对应文本入口，继续记录为 approved parser differences。Coverage 2,260/4,140（54.6%），374 个 case 中 357 个标为 mapped、0 个 partial、17 个 approved-difference。本轮不需要 MySQL Docker。

> 最新补充：Draft 4.28（2026-08-13），关闭 Java `ExprDefineAliasFor.ExprDefineDocSamples` execution（`java-runtime-d5797eaa543684206b0c`），新增 `internal/esper/expr_define_doc_samples_parity_test.go`。Go 以空动态 schema 和 typed `DefineExpression` 复现 `twoPI = Math.PI * 2` 常量示例，以及 `countPeople = count(*)`、10 秒时间窗和 `having countPeople > 10` 聚合示例；固定零参数声明元数据、结果字段类型，并验证两条计划可 Build/Deploy/Undeploy。Java execution 本身不发送事件，因此 Go 也不扩展为运行时输出断言。Textual `alias for` 继续归入 parser approved difference。Coverage 2,258/4,140（54.5%），374 个 case 中 357 个标为 mapped、0 个 partial、17 个 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.27（2026-08-13），关闭 Java `ExprDefineAliasFor.ExprDefineContextPartition` execution（`java-runtime-75a73d1d58ebd22f79a1`），新增 `internal/esper/expr_define_alias_context_partition_parity_test.go`。Go 用零参数 typed declaration 表达 `theString='a' and intPrimitive=1`，并以一次性 `TimerAt(origin)`/`TimerInterval(10m)` pattern context 精确映射 `start @now end after 10 minutes`；a/1 命中、b/1 不命中，十分钟终止且二十分钟不重开，同时固定 schema 与零参数元数据。Java textual `alias for` 继续归入 parser approved difference。Coverage 2,257/4,140（54.5%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.26（2026-08-13），关闭 Java `ExprDefineSplitStream` execution（`java-runtime-cc5f1abdc7e5df626003`），新增 `internal/esper/expr_define_split_stream_parity_test.go`。Go 以 typed `SplitAll` 和两个互补 `SplitIntoWhen` 分支复现 ABC/DEF 路由；`myLittleExpression(event)` 用可分析的 false conjunction 保留 Java 显式但未读取的完整 event 参数签名，输入 E1/10 时 ABC 无输出、DEF 收到原事件。Go 显式注册目标 schema，不从 EPL 合成流类型；同时修正 manifest 中既有 `ExprDefineOneParameterLambdaReturn` Java name 漏项，至此 `ExprDefineBasic` 27/27 runtime/name 均已登记。Coverage 2,256/4,140（54.5%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.25（2026-08-13），关闭 Java `ExprDefineNestedExpressionMultiSubquery` execution（`java-runtime-2d1509f7b72624aee099`），新增 `internal/esper/expr_define_nested_expression_multi_subquery_parity_test.go`。Go 以三个 typed declaration 复现跨模块表达式图：零参数 F1 读取 SupportBean `LastEvent`，F2(param) 读取按 `theString` 唯一保留并关联完整 event 的 scalar subquery，F3(s) 组合 F1 与 F2 并转发参数；E1/10 与替换后的 E1/11 依次得到 20、22，同时固定 F1/F2/F3 参数元数据和 int schema。Coverage 2,255/4,140（54.5%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.24（2026-08-13），关闭 Java `ExprDefineSubqueryNamedWindowCorrelated` execution（`java-runtime-467580fe1ac80f8da9d1`），新增 `internal/esper/expr_define_subquery_named_window_correlated_parity_test.go`。Go 用 `ExpressionParam[SupportBean_ST0]` 和 live `SubqueryEvents(FromNamedWindow(...))` 先按 `val0 = x.key0` 关联，再用 `EnumWhere` 筛选 `val1 > 10`；x/x/E2/E3 四次触发得到空、空、E2、E3 集合，并固定 `[]Event` schema 与参数元数据。Java 四种 textual qualification/alias spelling 由同一结构化绑定表达；额外 same-name `id` 场景以显式 inner `Field`/outer `Property` 完成 compile parity。Coverage 2,254/4,140（54.4%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.23（2026-08-13），关闭 Java `ExprDefineSubqueryNamedWindowUncorrelated` execution（`java-runtime-e790a1f17990c989dbdf`），新增 `internal/esper/expr_define_subquery_named_window_uncorrelated_parity_test.go`。Go 用零参数声明组合 `SubqueryEvents(FromNamedWindow(...))`、`EnumWhere` 与 `EnumOrderBy`，对 live keep-all 窗口筛选 `val1 > 10` 并按 `val0` 排序，调用点再链式筛选 `val1 < 100`；三次触发得到空集合、E1/E1、以及 [E1,E2]/E1，同时固定 present-empty `[]Event`、结果 schema、零参数元数据及窗口插入不触发 outer 输出。Java textual `alias for` 复用同一可观察语义。Coverage 2,253/4,140（54.4%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.22（2026-08-13），关闭 Java `ExprDefineSubqueryUncorrelated` execution（`java-runtime-4b7ab6634da7b78be0f5`），新增 `internal/esper/expr_define_subquery_uncorrelated_parity_test.go`。Go 用零参数 `DefineExpression`/`ExpressionRef` 包装 live ST0 `LastEvent` scalar subquery，外层 SupportBean 依次得到 E0/null、E1/ST0、E2/ST1；同时固定 string schema、零参数元数据，并验证 inner 更新不触发 outer 输出。Java 的 textual `alias for` 属性式调用复用同一可观察语义，继续归入 parser approved difference。Coverage 2,252/4,140（54.4%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.21（2026-08-13），关闭 Java `ExprDefineSubqueryCorrelated` execution（`java-runtime-477cfc2759bb2ec10309`），新增 `internal/esper/expr_define_subquery_correlated_parity_test.go`。Go 用 `ExpressionParam[SupportBean]`/`ExpressionRef` 把完整 outer event 传入声明体，并由 live ST0 `KeepAll` `SubqueryValueWithOptions` 按 `p00 = x.intPrimitive` 关联；显式 `SubqueryNullOnMultiple` 保持 Esper scalar cardinality，依次得到 E0/null、E1/null、E2/ST0，以及第二个 p00=100 row 后 E3/null，同时固定 string schema 与参数元数据。Java 的无参数 textual alias spelling 复用同一可观察语义，继续归入 parser approved difference。Coverage 2,251/4,140（54.4%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.20（2026-08-13），关闭 Java `ExprDefineSubqueryJoinSameField` execution（`java-runtime-ffaf4fe96407b8a530b1`），新增 `internal/esper/expr_define_subquery_join_same_field_parity_test.go`。Go 以单个 `ExpressionParam[Event]` 声明复用 live `KeepAll` scalar subquery，分别用 `JoinEventValue[Event](0/1)` 传入两个都含 `pcommon` 的 `LastEvent` source；输出依次为 null/null、null/10、10/10，并固定 int schema 与 Event 参数元数据。Java `alias for` 的无限定 `pcommon` 因两个 outer stream 而编译歧义；Go join 属性必须显式 source index，该歧义语法不可构造，归入既有 textual parser approved difference。Coverage 2,250/4,140（54.3%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.19（2026-08-13），关闭 Java `ExprDefineSubqueryCross` execution（`java-runtime-fe4b7fd6f2db94e45f24`），新增 `internal/esper/expr_define_subquery_cross_parity_test.go`。Go 用两个 `LastEvent` source 的 typed Cartesian `Join`，通过 `JoinEventValue` 把完整 ST0/ST1 event 传给双参数 `ExpressionRef`；声明体中的 live `KeepAll` `SubqueryValue` 同时关联 `x.id` 与 `y.p10`。先送 ST0/ST1 得到 null，插入 SupportBean 不触发外层输出，再送 ST1 得到 ST0，并固定 string schema 与 x/y 参数元数据。Java 无参数的 textual `alias for` 写法仅重复同一关联语义，继续归入既有 parser approved difference。Coverage 2,249/4,140（54.3%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.18（2026-08-13），关闭 Java `ExprDefineSubqueryMultiresult` execution（`java-runtime-b6d1dd679fbc5d92b9ab`），新增 `internal/esper/expr_define_subquery_multiresult_parity_test.go`。Go 用 live `KeepAll` event-stream subquery 覆盖两种 typed 声明：独立 `maxi`/`mini` 标量聚合表达式，以及返回 `SubqueryRow{maxi,mini}` 后经 `Property` 读取的多列结果；`SupportBean_ST0` 通过 `DivideFloat` 保持 Java double 除法，依次得到 0.2/0.4 和 0.2/2.0，并固定 float64 schema。第三个 Java `alias for` 文本仅是同一 row-valued 语义的 parser spelling，继续归入既有 approved difference。Coverage 2,248/4,140（54.3%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.17（2026-08-13），关闭 Java `ExprDefineSequenceAndNested` execution（`java-runtime-d37dc4b973ae7065db56`），新增 `internal/esper/expr_define_sequence_nested_parity_test.go`。Go 用 typed `CreateNamedWindow`/`InsertIntoNamedWindow` 建立两个 keep-all 窗口，以参数化 `ExpressionParam`/`ExpressionRef` 包装 correlated `SubqueryEvents`，再链式执行 `EnumTakeLast`、`EnumSelect` 和 `EnumSequenceEqual`；A 组窗口序列均投影为 B1/B2，得到 `val=true`。Java 的 `@Audit('exprdef')` 由 typed `StatementAudit(AuditExpressionDefinition)` 保留，并验证 `s0` 的 EXPRDEF callback 与 bool 结果 schema。Coverage 2,247/4,140（54.3%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.16（2026-08-13），关闭 Java `ExprDefineCaseNewMultiReturnNoElse` execution（`java-runtime-4eb413bcc8f6bd676d07`），新增 `internal/esper/expr_define_case_new_multi_return_no_else_parity_test.go`。Go 以 typed `ExpressionParam`/`ExpressionRef`、无 else 的 `CaseWhen[map[string]any]` 和参数可分析的 `Construct` 表达匿名 map 返回值，并经 `InsertInto("OtherStream")` 交给下游嵌套属性读取；输入 E1 时 `val0`、`c1`、`c2` 均为 null，A/B 分别得到 X/10、Y/20，同时固定 `val0` 的 map 结果 schema。Coverage 2,246/4,140（54.3%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.15（2026-08-13），关闭 Java `ExprDefineAnnotationOrder` execution（`java-runtime-c3444853bf2d3242e4fd`），新增 `internal/esper/expr_define_annotation_order_parity_test.go`。Java 验证 `@Name` 位于 inline expression 前后均生效；Go 的 typed builder 不存在文本注解位置，改以“先定义表达式再构造 source”和“先构造 source 再定义表达式”两种装配顺序对照，均得到 statement/metadata 名称 `s0`、`scalar()` 的 int 结果 schema 和输出 1。Coverage 2,245/4,140（54.2%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.14（2026-08-13），关闭 Java `ExprDefineScalarReturn` execution（`java-runtime-624a42b2506706e92ccc`），新增 `internal/esper/expr_define_scalar_return_parity_test.go`。Go 以 typed `ExpressionParam`/`ExpressionRef`、`EnumWhere` 表达集合返回声明的嵌套过滤，验证 `SupportCollection(E1,E2,E3,E4)` 得到 E3/E4；同时用 `CreateNamedWindow`、`SelectFromNamedWindow`、`CaseWhen` 和 `Cast` 对照 on-select 声明表达式：输入 `2` 不命中，`X` 命中窗口 0，`1` 命中窗口 1，并固定 `[]string` 结果 schema 与 long 类型输出。Coverage 2,244/4,140（54.2%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.13（2026-08-13），关闭 Java `ExprDefineOneParameterLambdaReturn` execution（`java-runtime-268b6a11030b69b72a4f`），新增 `internal/esper/expr_define_one_parameter_lambda_parity_test.go`。Go 以 typed `ExpressionParam`/`ExpressionRef`、`EnumWhere` 和 `Property` 表达声明表达式返回集合后再次链式过滤的语义，验证 `p00 < 10` 得到 E1/E2、二次 `p00 > 1` 得到 E2，保持顺序、空集合 present 语义和 `[]nestedST0` 结果 schema。Coverage 2,243/4,140（54.2%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.12（2026-08-13），关闭 Java `InfraTableOnUpdate` 的全部 4 个 execution（TwoKey、单数组键、双数组键、字符串转换双数组键），新增 `internal/esper/infra_table_on_update_parity_test.go` 与 `case.infra-table-on-update`。Go `UpdateTable` 按复合键/slice 内容键定位，双键交付 old/new pair，数组矩阵得到 11/21/31；NonGetter 由 analyzable `Construct(parse-int-array)` 转换 key。Java compile-only setter 赋值由 typed `Construct` 复制复合列后整列替换。Coverage 2,242/4,140（54.2%），374 个 case 中 357 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.11（2026-08-13），关闭 Java `InfraTableOnSelect` execution，新增 `internal/esper/infra_table_on_select_parity_test.go` 与 `case.infra-table-on-select`。Go `SelectFromTableWhere` predicate scan 在缺失 G1/G2 时不触发 listener，建组后读取 100/200，G2 再贡献 300 后第二 reader 得到 500；同时与 primary-key `SelectFromTable` miss 的 all-Null row 语义区分。Coverage 2,238/4,140（54.1%），373 个 case 中 356 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.10（2026-08-13），关闭 Java `InfraTableOnDelete` 的全部 2 个 execution（InfraDeleteFlow、InfraDeleteSecondaryIndexUpd），新增 `internal/esper/infra_table_on_delete_parity_test.go` 与 `case.infra-table-on-delete`。Go fluent Plan 覆盖 grouped into-table sum、keyed read、过滤单行 delete、delete-all、secondary-index 多行 sum 子查询和 aggregate row 更新/删除后的 index 一致性；普通 table on-delete listener 按 Java 契约将被删行交付为 new data，merge-delete 仍为 old data。Coverage 2,237/4,140（54.0%），372 个 case 中 355 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.09（2026-08-13），关闭 Java `InfraTableOnMerge` 的全部 7 个 execution（InfraTableOnMergeSimple、InfraOnMergePlainPropsAnyKeyed、InfraMergeWhereWithMethodRead、InfraMergeSelectWithAggReadAndEnum、InfraMergeTwoTables、InfraTableEMACompute、InfraTableArrayAssignmentBoxed），新增 `internal/esper/infra_table_on_merge_parity_test.go` 与 `case.infra-table-on-merge`。Go fluent Plan 覆盖 keyed merge insert/update、单键/双键/无键 insert-update-delete、merge 普通列与 into-table 聚合列共同持有、LastEvent 换组后 count(*)=0 行删除、window eventset/takeLast side-stream、双表有序插入、EMA burn/递推及 boxed 数组元素赋值。运行时合并 aggregate-owned 与 trigger-owned 表列，保留 grouped LastEvent 空组，并将 map-backed EventValue 严格物化为 typed struct/struct pointer。Coverage 2,235/4,140（54.0%），371 个 case 中 354 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.08（2026-08-13），关闭 Java InfraTableAccessDotMethod 的三个 execution（InfraPlainPropDatetimeAndEnumerationAndMethod、InfraAggDatetimeAndEnumerationAndMethod、InfraNestedDotMethod），新增 internal/esper/infra_table_access_dot_method_parity_test.go 与 case.infra-table-access-dot-method。Go 以 object-array PopulateEvent merge 表、typed Event/Event[]/bean 列、DateTimePlugin(getMinuteOfHour/after)、Method(GetMyProperty)、EnumTakeLast/EnumSelect/EnumCount/EnumTake 与 NestedField 组合表达 Java dot-method；覆盖 grouped/soda 共 12 个子测试。DateTimePluginFactoryContext 增加 Arguments 声明类型，工厂可按参数类型选择实现。Coverage 2,228/4,140（53.8%），370 个 case 中 353 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.07（2026-08-13），关闭 Java InfraTableAccessAggregationState 的全部 7 个 execution（InfraAccessAggShare、InfraTableAccessGroupedMixed、InfraTableAccessGroupedThreeKey 与 4 个 InfraNestedMultivalueAccess grouped/soda 组合），新增 internal/esper/infra_table_access_aggregation_state_parity_test.go 与 case.infra-table-access-aggregation-state。Go 以 typed Table 列 + IntoTable 表达聚合状态：三键表按 string/int/long 分组并输出 sum(double)/count(*) 轨迹 1000/1 至 2001/2；分组混合表在 LengthWindow(3) 内同时维护 count/count-distinct/window-access/sum，得到 3/2/[E1,E1,E1]/303、缺失 E2 为 Null、E2 建组后 1/1/[E2]/200；无主键 TimeWindow(10s) 的 window access 由第二个 reader 共享；嵌套 multivalue 覆盖 grouped/ungrouped 与 full-event/scalar-int 双窗口，验证 E1/E2 及 E3 淘汰后的值。Java window(*)/last(*)/first(*) 与 SODA 形态由 WindowAccessValue 的 Values/First/Last 投影和 immutable fluent Plan 表达。Coverage 2,225/4,140（53.7%），369 个 case 中 352 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.06（2026-08-13），关闭 Java InfraTableAccessCore 三个 execution：InfraSubquery（java-runtime-0ce6b391952f2fb495c5）、InfraOnMergeExpressions（java-runtime-e750fb3883fd94b1edd3）与 InfraNamedWindowAndFireAndForget（java-runtime-072f7acbf2fd102a24c1），新增三个 parity 测试。Go 以声明表达式 SubqueryValue(FromTable(...)) 配合 SupportBean_S0#LastEvent 标量子查询实现 Java 相关表访问；MergeIntoTableWhen 的 TableField(total) > 0 匹配分支正确读取所选表行并更新可选 value；length(2) named window 喂养 into-table sum 后，FAF select/delete/update/insert 分别返回 10、删除一行、写入 doublePrimitive=100、物化 (A,20)。同时修复两个运行时缺陷：trigger 语句现在按输入事件类型门控，避免无关事件错误触发；空 aggregate delta 不再重新物化 into-table，避免覆盖 merge/FAF 更新。Coverage 2,218/4,140（53.6%），368 个 case 中 351 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.05（2026-08-12），关闭 Java `InfraTableAccessCore.InfraMultiStmtContributing` execution（`java-runtime-7da3d0697bcb19fbb28f`），新增 `TestInfraTableAccessMultiStmtContributingParity`。Go 以 `IntoTable` 聚合语句贡献不同表列（s0sum/s0cnt/s0win 与 s1sum/s1cnt/s1win），运行时 `aggregateTableRows` 合并同表同作用域所有活动贡献者，使一条语句不再覆盖另一条的列；`LengthWindow(2)` enter/leave 产生未贡献列为 null/0，`sum`/`count` 合并正确。第二组矩阵验证两条语句共享同一 `sum` 列时得到 Java 轨迹 `10 -> 5 -> 7`，contributor listener 同样看到合并后的表值。`validateIntoTable` 放宽为允许部分列贡献和非表投影（如 `c0`），`replaceInScope` 保留同主键行身份。Coverage 2,215/4,140（53.5%）＃65 个 case 中 348 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.04（2026-08-12），关闭 Java `InfraTableAccessCore.InfraOrderOfAggregationsAndPush` execution（`java-runtime-0ea11b70b83cd854744a`），新增 `TestInfraTableAccessOrderOfAggregationsAndPushParity`。Go 以 `LengthWindow(2)` + `IntoTable` 同时物化 `sum(int)`、`sum(long)`、`WindowAccessBy` 和 `SortedAccessBy`，并分别验证 ungrouped/grouped 表的 aggregate push 与 typed table lookup：10/100/[E1]/[E1]、15/150/[E1,E2]/[E2,E1]、17/170/[E2,E3]/[E2,E3]。Java 的 sorted/window 表达式和 EPL/SODA 在 Go 中由 typed `WindowAccessValue`/`SortedAccessValue` 与 immutable fluent Plan 表达；完整 table metadata/index/context/join/invalid 矩阵继续开放。Coverage 2,214/4,140（53.5%），364 个 case 中 347 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.03（2026-08-12），关闭 Java `InfraTableAccessCore.InfraUngroupedWContext` execution（`java-runtime-3974da9ea932ebaa8481`），新增 `TestInfraTableAccessUngroupedContextParity`。Go 使用 `CreateKeyContext`、可分析的 `TypeName(EventValue)` + `CaseWhen` 跨事件类型 key 映射、Context-scoped `IntoTable` 和 `SelectFromTableWhere`，对照 A/B/C/D 分区的无主键 sum：A=21、B=41、C=30、D=40，重复读取 A 仍为 21，并验证四个 Context partition。Java 的 context `partition by theString from SupportBean, p00 from SupportBean_S0` 在 Go 中由显式 typed key expression 表达；完整 context/table metadata/index/join/invalid 矩阵继续开放。Coverage 2,213/4,140（53.4%），363 个 case 中 346 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.02（2026-08-12），关闭 Java `InfraTableAccessCore.InfraExpressionAliasAndDecl` execution（`java-runtime-fc9d76e19bd726f24e20`），新增 `TestInfraTableAccessExpressionAliasAndDeclParity`，拆分覆盖声明表达式作为 `IntoTable` 聚合输入、声明表达式直接读取表、以及声明表达式内 `lastevent` 子查询门控后的表读取。Go 使用 `DefineExpression`/`ExpressionParam`/`ExpressionRef`、`Sum`、`SubqueryValue`、`SubqueryExists` 和 typed `IntoTable` 链式 API；标量声明表达式调用参数在调用点捕获外层求值上下文，聚合声明表达式保留聚合组内惰性求值。Coverage 2,212/4,140（53.4%），362 个 case 中 345 mapped / 0 partial / 17 approved-difference。Java 声明表达式文本 `alias for` 与 `SupportBean_S1#lastevent` 内嵌表访问在 Go 中分别由 immutable fluent declaration 和显式 `SubqueryExists` 门控表达；完整 table metadata/index/context/join/invalid 矩阵继续开放。本轮不需要 MySQL Docker。

本轮继续完成根包拆分：根目录从 486 个 Go 文件收敛到 `doc.go`、`facade_generated.go`、`facade_test.go` 三个文件；90 个实现文件和 395 个白盒/parity 测试迁入 `internal/esper` 并使用 `package esper`。AST 生成器维护类型别名、常量和顶层函数转发，Go 类型系统快照校验迁移前后 2,564 条公开对象/方法签名完全一致。结构门禁新增根文件数量、内部包名、facade 漂移、public/internal API 等价和 capability `goRefs` 存在性检查；416 条 Go 证据路径已同步。本轮不改变 Esper 行为或公共导入路径。

本轮完成 Go 项目结构收敛：公共根包 `github.com/liubaicai/esper` 与公开 `connectors` 保持稳定；仓库专用对账代码迁入 `internal/compat`，版本化兼容资产迁入 `testdata/compat`，命令实现迁入 `internal/app`，`cmd` 只保留薄入口。新增 ADR-015、`docs/project-layout.md`、`.gitignore`、`Makefile` 与 `scripts/check-layout.sh`，清理 coverage/log 生成物，并将既有 Go 源统一为 `gofmt` 格式。该结构选择遵循 project-layout 对公共库的适用规则，不创建会改变现有导入路径的 `pkg/esper`。本轮只改变仓库边界和开发工具组织，不改变 Esper 运行时行为。

本轮继续关闭 Java `ExprDefineBasic` 的 3 个 execution：`ExprDefineAggregationNoAccess`、`ExprDefineAggregatedResult`、`ExprDefineAggregationAccess`，新增 `expr_define_aggregation_parity_test.go`。Go 以 `Sum`/`CountAll`、声明表达式作为聚合输入和 `WindowAccessBy` + filtered access 表达 `sumA/sumB/countC`、`lambda1/lambda2` 与 `window(*).where(...)`，逐事件对照 Java 结果和空窗口集合语义。修复声明表达式参数在 aggregate group 内的求值边界：调用参数保留为惰性可分析表达式，避免整个声明绑定到代表事件；窗口访问的空 `Values()` 返回 present 的非 nil 空切片，匹配 Esper 空 collection。`case.expr-define-basic` 继续保持 mapped。Coverage 2,198/4,140（53.1%），346 case 中 329 mapped / 0 partial / 17 approved-difference。`OneParameterLambdaReturn`、ScalarReturn、AnnotationOrder、SequenceAndNested、CaseNew、Subquery 和 SplitStream 仍待后续切片。本轮不需要 MySQL Docker。

本轮关闭 Java `ExprDefineWildcardAndPattern` 的 pattern correlated-filter execution，新增 `TestExprDefineWildcardPatternParity`。Go 以 `PatternEvent("a")` 将捕获事件传入声明表达式 `abc`，对照 `b.intPrimitive = abc(a)` 回放 `a=E1(2)`、`b=E2(4)` 的单行匹配；同 case 的简单表达式、变量、参数校验、跨模块、wildcard、where、SODA 对照仍保持既有测试与登记差异。`case.expr-define-basic` 从 partial 提升为 mapped。Coverage 2,195/4,140（53.0%），346 case 中 329 mapped / 0 partial / 17 approved-difference。aggregation/subquery/case-new/split-stream 变体继续作为后续范围。本轮不需要 MySQL Docker。

本轮关闭 Java ExprEnumSelectFromEventsPlain 的 1 个 execution，新增 expr_enum_select_from_events_parity_test.go。测试以 typed struct 和 map event source 对照 selectFrom(x => id) 与 Null selector，覆盖非空/空/Null collection 及 Build/Deploy []string 结果元数据。Go struct 的 typed-nil slice 是 present empty；map-backed nil field 保留 Java Null collection。其余 selectFrom execution 的 index/size lambda、anonymous map row 和 scalar collection matrix 保持后续差异。

本轮关闭 Java `ExprEnumDataSources` suite（27 个 runtime），新增 `case.expr-enum-data-sources` 和 `expr_enum_data_sources_parity_test.go`。覆盖 ExprEnumProperty（事件属性集合上的 allOf）和数组属性 sumOf 的 Build/Deploy 路径。其余 25 个执行需子查询/命名窗口/访问聚合/prev函数/模式过滤/上下文属性/match-recognize/表行/变量等未移植特性，保持开放。此轮完成后 `expr.enum-collection-methods` 能力 remaining 清单全部清空，该能力所有 case 均已登记。Java `TestSuiteExprEnum` 28/28 oracle 沿用本轮实跑结果。

本轮关闭 Java `ExprEnumDocSamples` suite（10 个 runtime），新增 `case.expr-enum-doc-samples` 和 `expr_enum_doc_samples_parity_test.go`。覆盖 ExprEnumScalarArray（全面标量数组 enum 方法验证，element/index/size lambda 变体）和 ExprEnumHowToUse（链式 where + 复合谓词 + 嵌套属性路径）。其余 8 个执行需子查询/命名窗口/访问聚合/prev窗口/UDF集合/声明表达式数据源，保持开放。Java `TestSuiteExprEnum` 28/28 oracle 沿用本轮实跑结果。

本轮关闭 Java `ExprEnumNested` suite，新增 `case.expr-enum-nested` 和 `expr_enum_nested_parity_test.go`。覆盖三个执行：不相关嵌套（EnumMinOf 在 EnumWhere 谓词内调用同一集合）、min-by-where（EnumMinBy 返回最年轻者作为谓词比较值）、嵌套 anyOf（内层集合来自外层元素属性）。第四个执行 ExprEnumCorrelated 需从内层谓词引用外层元素（x.p00），Go 的 EnumElement 始终返回最内层上下文值，声明式外层元素引用保持开放。Java `TestSuiteExprEnum` 28/28 oracle 沿用本轮实跑结果。

本轮关闭 Java `ExprEnumChained` suite，新增 `case.expr-enum-chained` 和 `expr_enum_chained_parity_test.go`。Go 以表达式嵌套实现链式调用：`EnumWhere` 输出 `Expression[[]T]` 直接作为 `EnumMinOf` 输入，对应 Java `sales.where(x => x.cost > 1000).min(y => y.buyer.age)`；嵌套属性路径 `buyer.age` 通过 getPropertyPath 解析。覆盖直接求值与 Build/Deploy 部署态 typed 元数据、事件求值及空结果 Null 返回。Java `TestSuiteExprEnum` 28/28 oracle 沿用本轮实跑结果。

本轮关闭 Java `ExprEnumGroupBy` suite，并扩展已有 `case.expr-enum-groupby`。新增 `expr_enum_group_by_parity_test.go`，以 `EnumGroupBy[T,K]`（单 selector，原元素分桶）和 `EnumGroupBySelect[T,K,V]`（双 selector，typed value 分桶）覆盖标量/event 的 element-index-size 选择器、部署态 typed map 元数据、空/Null 输入、pointer/interface K/V 的 null key/value、序保留（LinkedHashMap key 首见序 + 桶内输入序）与 Build 缺失 collection/key/value 选择器 invalid。Java mismatched-lambda-arity EPL 诊断在 fluent API 下结构性不可能（两个 selector 共享 EnumElement/Index/Size 节点）；Java 的 parameterized lambda arity/type metadata 与精确 null-key generic descriptor 保持差异。enum_expr.go 注释补充 Null 契约。Java `TestSuiteExprEnum` 28/28 oracle 沿用本轮实跑结果。

# Esper 9.0.0 Go 全量移植规划实施文档

> 最新补充：Draft 4.01（2026-08-12），关闭 Java `InfraTableAccessCore.InfraExprSelectClauseRenderingUnnamedCol` execution（`java-runtime-4e6eba40e475800f57e1`），新增 `TestInfraTableAccessExprSelectClauseRenderingUnnamedColParity`。Go 以显式 alias 的链式 `SelectFromTable` 表达 Java 未命名表访问选择列，覆盖 typed key collection、窗口事件集合、完整 `StructOf` 表行、`Last` 事件和 `EnumTake(1)` 集合，并通过 `Plan.ResultSchema` 与运行时断言固定对应 Go 类型和值。Java 的表达式渲染列名改为 Go 显式 alias，Java Object[]/SupportBean[]/Collection 元数据改为 typed slice/struct。Coverage 2,211/4,140（53.4%），361 个 case 中 344 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 4.00（2026-08-12），关闭 Java `InfraTableAccessCore.InfraTableAccessMultikeyWArrayTwoArrayKey` execution（`java-runtime-4d9fa5fa8f6b26584af4`），新增 `TestInfraTableAccessMultikeyArrayTwoArrayKeyParity`。Go 以两个 `PrimaryKeyColumn[[]int]` 构造复合数组主键，使用 `OnEvent(...).InsertIntoTable` 写入三组 key pair，再由 `SelectFromTable` 验证 `([1,2],[1,2])`、`([1,2],[1,1])`、`([1,3],[1,1])` 命中 `10/30/20`，缺失 `([1,2],[1,2,2])` 为 Null，并用 typed `SubqueryValues` 投影三组无序复合 key。InfraTableAccessCore 的单数组/双数组 key execution 均已 mapped；其余 table method/access、context、join、split-stream 和完整 metadata/index/invalid 矩阵继续开放。Coverage 2,210/4,140（53.4%），360 个 case 中 343 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.99（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraTableAccessMultikeyWArrayOneArrayKey` execution（`java-runtime-ad82a78695c21c3397f0`），新增 `TestInfraTableAccessMultikeyArrayOneArrayKeyParity`。Go 以 `PrimaryKeyColumn[[]int]`、`OnEvent(...).InsertIntoTable`、`SelectFromTable` 和 typed `SubqueryValues(FromTable(...))` 表达单数组主键表，验证 `[1,2]`、`[2,1]`、`[1,2,1]` 按内容独立存储和覆盖、查询命中 `10/20/30`、缺失 `[1,2,2]` 为 Null，以及 `keys()` 的三条无序 key 快照。Coverage 2,209/4,140（53.4%），359 个 case 中 342 mapped / 0 partial / 17 approved-difference。双数组主键 execution（`java-runtime-4d9fa5fa8f6b26584af4`）继续作为下一切片。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.98（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraTableAccessCoreSplitStream` execution（`java-runtime-c2dd9e27bd44b4ce932b`），新增 `TestInfraTableAccessSplitStreamParity`。Go 以 `OnEvent(...).InsertIntoTable` 物化表行，再用 `SplitAll` + `SplitIntoWhen` 和 typed `SubqueryValue(FromTable(...))` 表达两个条件路由分支，验证 A/B 得到 `10/20`，未匹配 `id=3` 不输出。Coverage 2,208/4,140（53.3%），358 case 中 341 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.97（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraGroupedMixedMethodAndAccess` execution（`java-runtime-a9a1edf4356631165beb`），新增 `TestInfraTableAccessGroupedMixedMethodAndAccessParity`。Go 以 `GroupBy(theString)` + `LengthWindow(3)` + `IntoTable` 同时物化 `count(*)`、`count-distinct(intPrimitive)`、`WindowAccessBy` 和 `sum(longPrimitive)`，并以 `SelectFromTable` 验证 E1 三事件的 `3/2/[E1,E1,E1]/303`、缺失 E2 的 Null 以及 E2 建组后的 `1/1/[E2]/200`，同时固定结果 schema。Coverage 2,207/4,140（53.3%），357 case 中 340 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.96（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraGroupedTwoKeyNoContext`（`java-runtime-3ad6a5913e413b86733e`）与 `InfraGroupedThreeKeyNoContext`（`java-runtime-c8146c4631693746bd0f`）。新增 `TestInfraTableAccessGroupedTwoKeyNoContextParity` 与 `TestInfraTableAccessGroupedThreeKeyNoContextParity`，以 `GroupBy` + `IntoTable` + `SelectFromTable` 覆盖两列、三列主键，触发查询覆盖两个动态 key、固定第三 key `100L`、`sum`/`count` 和缺失组合的 present-null 语义。Coverage 2,206/4,140（53.3%），356 case 中 339 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.95（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraGroupedSingleKeyNoContext` execution（`java-runtime-0ff723605b504e101395`），新增 `TestInfraTableAccessGroupedSingleKeyNoContextParity`。Go 以 `GroupBy(theString)` + `IntoTable` + `SelectFromTable` 对照 Java A/B/C/D 单键聚合轨迹（21/41/30/40），并以显式 `OuterField`/`TableField` 保留触发事件与目标表行作用域，缺失 Z 返回 present-null。Coverage 2,204/4,140（53.2%），354 case 中 337 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.94（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraTopLevelReadGrouped2Keys` execution（`java-runtime-7e7f078ff379d781e502`），新增 `TestInfraTableAccessTopLevelReadGroupedTwoKeysParity`。Go 以 `RegisterObjectArray` + `GroupBy(c0,c1)` + `LengthWindow(2)` + `WindowAccessBy` + `Sum` 物化两列主键表行，按 Java 轨迹验证 G1/G2 初始窗口、G1 淘汰后的 present-null 以及 G2 的 `[20,20],500` 保留状态；typable fragment/SODA 由 `StructOf` map 和 immutable fluent Plan 表达。Coverage 2,203/4,140（53.2%），353 case 中 336 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.93（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraTopLevelReadUnGrouped` execution（`java-runtime-384086dfe490a2e3c1c0`），新增 `TestInfraTableAccessTopLevelReadUngroupedParity`。Go 以 `RegisterObjectArray` + `FromAny` + `LengthWindow(2)` + `WindowAccessBy` + `Sum` 物化无分组表行，并以 `SelectFromTableWhere` + `StructOf` 表达顶层 map 读取；10/20/30 轨迹得到 `[10],10`、`[10,20],30`、`[20,30],50`。Java typable `AggBean` 输出由 Go map projection 表达，grouped/context/method/access/join/split-stream/array-key table execution 仍开放。Coverage 2,202/4,140（53.2%），352 case 中 335 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.92（2026-08-12），继续关闭 Java `InfraTableAccessCore.InfraIntegerIndexedPropertyLookAlike` execution（`java-runtime-77d523200118fdbae252`），新增 `TestInfraTableAccessIntegerIndexedPropertyLookAlikeParity`。Go 以 `GroupBy(intPrimitive)` + `WindowAccessBy(EventValue)` + `IntoTable` 物化整数主键表行，再用 `SelectFromTable` 的显式 typed primary-key lookup 对照 `varaggIIP[1]` 的整行、窗口列、`last(*)` 和倒数第二项访问；SODA 由 immutable fluent Plan 表达。剩余 table method/access、context、join、split-stream 和 array-key execution 继续开放。Coverage 2,201/4,140（53.2%），351 case 中 334 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
> 最新补充：Draft 3.91（2026-08-12），完成根包实现拆分。根目录由 486 个 Go 文件降为三个公共 facade 文件；实现和白盒测试迁入 `internal/esper`。生成器与 Go 类型系统 API 门禁保证迁移前后 2,564 条公开对象/方法签名一致，capability 的 416 条 Go 引用同步到新路径。本轮无 Esper 行为变更。
>
> 最新补充：Draft 3.90（2026-08-12），完成 Go 项目结构收敛。公共根包与连接器导入路径保持不变；私有命令/对账实现迁入 `internal`，固定兼容资产迁入 `testdata/compat`，新增持续结构门禁、Makefile、忽略规则、目录说明和 ADR-015，并清除覆盖率/日志产物。本轮无 Esper 行为变更。
>
> 最新补充：Draft 3.89（2026-08-12），继续关闭 `ExprDefineBasic` 的 3 个聚合 execution（AggregationNoAccess、AggregatedResult、AggregationAccess），新增 `expr_define_aggregation_parity_test.go`。声明表达式调用参数改为 aggregate group 内惰性求值，窗口访问空 `Values()` 保留 present 空集合；`OneParameterLambdaReturn` 仍待后续切片。Coverage 2,198/4,140（53.1%），346 case 中 329 mapped / 0 partial / 17 approved-difference。本轮不需要 MySQL Docker。
>
> 最新补充：Draft 3.88（2026-08-12），新增 `expr_define_basic_parity_test.go`，对照 Java `ExprDefineBasic` 的 6 个 execution（SimpleLiteral、NoParameterArithmetic、NoParameterVariable 含运行时变量变更、StaticMethodSingleParam、Invalid 参数数量校验）。参数化表达式与聚合、filter 上下文参数化、子查询/模式/case-new/split-stream 变体保留为后续切片。Coverage 2,108/4,140（50.92%），337 case 中 318 mapped / 2 partial / 17 approved-difference。本轮不需要 MySQL Docker。
>
> 最新补充：Draft 3.87（2026-08-12），关闭 ViewIntersect 剩余 8 个 execution（FirstUniqueAndFirstLength/FirstLengthAndUnique、BatchWindow unique+batch 组合与 derived sum、GroupBy 分组、Sorted 排序替换、LengthOneUnique、GroupTimeLength），新增 `view_intersect_remaining_parity_test.go`。关键修复：`reconcileCompositeWindow` 的 `compositeIteratorEvents` 使用 per-child `windowIteratorEvents`（batch 窗口取 pendingNew）修正 post-flush snapshot 计数；`addToWindow` 的 CompositeWindow 路径在 intersect 模式下向所有 child 转发 removal，对齐 Java `IntersectDefaultView` 的 child 同步语义——事件离开交集（Unique 替换）或被 child 静默拒绝（FirstUnique 重复）时从每个 child 移除，释放 FirstLength slot。Coverage 2,102/4,140（50.77%），336 case 中 318 mapped / 1 partial / 17 approved-difference。下一步继续其他 capability 域。本轮不需要 MySQL Docker。
>
> 最新补充：Draft 3.86（2026-08-12），新增 `view_group_intersect_parity_test.go`，关闭 ViewGroup 12 个 execution 与 ViewIntersect 6 个 execution，覆盖 grouped length/time/batch/accum/order、grouped Correl/Linest/Avg、two/three unique、time+unique 正反声明、多键 unique。修复 `GroupWindow + Aggregate` 的隐式 groupwin aggregate state 隔离，GroupWindow keys 现在成为 runtime grouping dimension；Group reclaim/performance/serde、Intersect Named Window/pattern/subselect 仍保留后续。Coverage 2,094/4,140（50.58%），335 case 中 317 mapped / 1 partial / 17 approved-difference。下一步继续 ViewGroup/ViewIntersect 剩余矩阵及其他 capability 域。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.82（2026-08-12），ViewLengthBatch 子集（SceneOne/Size2/Size1）与 ViewUnionFirstUniqueAndFirstLength 关闭，新增 4 个 parity 测试。LengthBatch 对齐整批 new/old 和 iterator 当前批；Union 对齐 child retention 。Coverage 2,067/4,140（49.93%），323 case 中 305 mapped。下一步 ViewGroup/Intersect 和 ExprFilter 剩余。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.81（2026-08-12），ViewUnique suite 关闭（4 个 parity 测试）：单键/复合键/表达式参数/双窗口 unique 窗口。Coverage 2,067/4,140（49.93%），321 case 中 303 mapped。下一步 ViewGroup/Intersect/Union、ExprFilter 剩余。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.80（2026-08-12），ExprFilterExpressions 剩余可观测 execution关闭（OverInClause/StaticFunc/InstanceMethodWWildcard/NotEqualsConsolidate），新增 4 个 parity 测试。Coverage 2,067/4,140（49.93%），320 case 中 302 mapped / 1 partial / 17 approved-difference。下一步 ExprFilter 剩余、View Accum 系列。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.79（2026-08-12），ExprFilterOptimizable、ExprFilterInAndBetween 剩余、ExprFilterLargeThreading、ExprFilterWhereClauseNoDataWindowPerformance 关闭，覆盖 4 个 outerClass 级 runtime。新增 11 个 parity 测试（1 skip），表达 regexp OR、context OR、typeof、变量方法调用、IN/NOT IN 多值集合、OR 重写、部署时常量、多语句 IN 复用、pattern followed-by LIKE、100 语句 WHERE。Java filter service 内部索引与性能阈值为 Go-style 差异。新增 case.expr-filter-optimizable（mapped）。Coverage 2,067/4,140（49.93%），320 case 中 302 mapped / 1 partial / 17 approved-difference。下一步 ExprFilter 剩余、View Accum 系列。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.78（2026-08-11），ViewTimeAccum（10 execution）、ViewTimeLengthBatch（9 execution）、ViewFirstTime（3 execution）与 ViewExternallyTimedWin/ViewExternallyTimedBatched（10 execution）四 suite 关闭，共 32 个 runtime ID。链式 API 完整表达 #time_accum/#time_length_batch/#firsttime/#ext_timed/#ext_timed_batch：TimeAccum 到期整窗逐出并补齐 prev/prior 历史、日历月作用域、分组聚合与 RStream old 流；TimeLengthBatch 时间+长度双重触发、force_output 空批次强制回调、start_eager 部署即锚定、prev/prior 批次前缀投影；FirstTime 首个事件到达后经周期关闭闸门且保留事件永不逐出；ExternallyTimed 滑动窗以事件外部时间戳推进、非 calendar 锚点算术推进避免 1577 万次循环；ExternallyTimedBatch 事件到达先检查边界（越过边界 flush 当前窗口为 new/上一批为 old、新事件进下一批），ReferenceSet 时 scheduleAt=下一锚点不再双倍推进，WithReference 显式锚点 + 无参考点以首事件锚定。同步修复 TimeBatch 锚点语义（deltaAddWReference 相等边界停在当前周期，空批次后停止调度不再强制输出）与窗口 schedule 递归 coalesce（含 children/groups，支撑 AdvanceTimeSpan coalesce 路径到期触发）。新增 view_time_accum_parity_test.go 4 个、view_time_length_batch_parity_test.go 10 个、view_first_time_parity_test.go 3 个、view_ext_timed_parity_test.go 10 个（5 滑动 + 5 batch）parity 测试，case.view-timebatch-basic 的 javaNames/javaSourceFiles 同步扩展。Coverage 2,067/4,140（49.93%），319 case 中 301 mapped / 1 partial / 17 approved-difference。下一步 View Accum 系列、ExprFilter 剩余。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.77（2026-08-11），ViewExpressionWindow（13 execution）与 ViewExpressionBatch（13 execution）两 suite 关闭，共 26 个 runtime ID。链式 API 完整表达 #expr/#expr_batch：ExpressionWindow 从最旧逐出至 keep 谓词成立、expired_count 逐出计数、current_count/oldest_event/newest_event/oldest_timestamp/newest_timestamp/view_reference 窗口内置量经 EvalContext 注入；ExpressionBatch 默认 include=true，ExcludeTriggerEvent() 对齐 false 形态，触发谓词对 pending+新行求值，变量变化在时间推进边界重评估触发；Named Window retention 同步支持（expr 删除重算 keep、expr_batch 删除静默且不重算 trigger），expr_batch 迭代器返回当前批。GroupWindow 快照/遍历顺序改为组首次创建序（对齐 Java GroupByViewImpl/MergeView），表达式批次 Prev 按批次前缀投影。新增 view_expression_parity_test.go 26 个 parity 测试。新增 2 个 mapped case，Coverage 2,035/4,140（49.15%），319 case 中 301 mapped / 1 partial / 17 approved-difference。下一步 View Accum 系列、ExprFilter 剩余。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.76（2026-08-11），PatternInvalid（3 execution）与 PatternExpressionText（1 execution）关闭，共 4 个 runtime ID。引擎补三处校验对齐 Java：BetweenOf 跨域类型兼容（字符串字段配数值上下界 / 数值字段配字符串上界在 Build 期拒绝，对齐 String-to-numeric 不允许隐式转换）、CronSchedule 单值 day-of-month + 受限 weekday 组合拒绝（对齐 ScheduleSpecUtil dom/dow 冲突，多值 dom 仍允许）、pattern observer 参数内 subselect 拒绝（对齐 observer 参数不允许 subselect，覆盖 timer:interval/schedule/cron，filter 仍允许）；model API 补 while-guard Kind/children。新增 pattern_invalid_parity_test.go 与 pattern_expression_text_parity_test.go（约 150 fluent 形态的 model shape + 稳定 canonical description + 算子 marker + Build）。EPL 文本语法错误、timer:within 作 observer 根 / timer:interval 作 guard、空参数/within 字符串参数、timer:at 命名参数与单参数 arity、数值时区参数登记为 typed fluent 编译期不可表达差异。Coverage 2,009/4,140（48.53%），317 case 中 299 mapped / 1 partial / 17 approved-difference。Pattern 基础 suite（ExpressionText、Invalid）已完成；下一步 View Expression/Accum 系列、ExprFilter 剩余。本轮不需要 MySQL Docker。
>
> 最新补充：Draft 3.75（2026-08-11），PatternStartLoop 与 PatternSuperAndInterfaces 两 suite 关闭（各 1 个 runtime ID）：StartLoop 回放监听器回调内 10 次 deploy/undeploy/redeploy 重入循环（Java 部署时触发裸 not 根 vs Go Build 期拒绝裸负根，差异登记）；SuperAndInterfaces 以 WithSchemaParent 父 schema 链映射 Java 类层级、以最派生有效值映射 getter 覆盖链，28 个过滤用例精确匹配集全部对齐。当前 297 mapped / 1 partial / 17 approved-difference。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.74（2026-08-11），PatternOperatorFollowedByMax 系列全部关闭（5 个新增 runtime ID：FollowedByMax suite 5/5、2Noprevent/2Prevent/4Prevent 各 1/1）：新增运行时级 pattern 子表达式池（WithPatternSubexpressionMax/WithPatternSubexpressionPreventStart 引擎选项 + PatternRuntimeSubexpressionLimitEvent 监听器），对照 PatternSubexpressionPoolRuntimeSvcImpl 的 tryIncreaseCount/decreaseCount 以 per-event tracker 增量重放池计费（替换额度/净新增判定/无后继释放），per-statement 计数映射、trackWithMax-first 判定顺序与 undeploy 释放全部逐事件对齐 Java 断言。当前 295 mapped / 1 partial / 17 approved-difference。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.73（2026-08-11），PatternUseResult suite 6/6 关闭（6 个新增 runtime ID）：followed-by 过滤器标签属性相关（数值/字符串/boolean/区间/混合类型 Cast 比较）35 个子用例逐事件回放，every X1->every X2 同投影串双行触发对齐 Java harness 逐条 add；FollowedByFilter 固定种子 PRNG 断言三 userId 互异不变量；PatternTypeCacheForRepeat 单部署双语句验证 dissimilar object-array 类型标签数组不串型。当前 294 mapped / 1 partial / 17 approved-difference。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.72（2026-08-11），PatternRepeatRouteEvent 3/3 与 PatternStartStop 3/3 关闭（6 个新增 runtime ID）：RepeatRoute 验证 subscriber 回调内事件路由循环（1000/级联/timer 触发）；StartStop 验证 deploy/undeploy/redeploy 与 Statement.Start/Stop 生命周期。当前 293 mapped / 1 partial / 17 approved-difference。本轮不需要 MySQL Docker。
>
> 历史补充：Draft 3.71（2026-08-11），PatternOperatorOperatorMix 与 PatternDeadPattern 两 suite 关闭：OperatorMix 7 子用例覆盖 and/or/followed-by 算子优先级；DeadPattern 一千部署被 C 证伪后 A 遍历须廉价零触发。
>
> 历史补充：Draft 3.70（2026-08-11），PatternComplexPropertyAccess suite 5/5 关闭：Esper 属性路径（mapped/indexed/nested）在链式 filter 中映射为组合表达式 MapValue/ArrayAt/Property。
>
> 历史补充：Draft 3.69（2026-08-11），PatternGuardWhile suite 4/4 关闭（4 个新增 runtime ID）：AST 级表达式 guard（patternGuardWhileNode + PatternStream.WhileGuard），对齐 ExpressionGuard inspect 语义——true 放行、Boolean.FALSE 永久退出并向上 evaluateFalse、null 仅吞没；Simple/PatternOp（5 子用例含 SODA 等价形态）/PatternVariable/PatternInvalid 全部通过。
>
> 历史补充：Draft 3.68（2026-08-11），PatternObserverTimerSchedule suite 7/7 与 PatternObserverTimerScheduleTimeZoneEST 1/1 关闭（7 个新增 runtime ID，DateWithPeriod 此前已由 case.pattern-timer-cron 关联）：timer:schedule 全部 ISO-8601 形式与 named 参数形式逐场景回放，过去日期锚定跳过已到期次数、三种等价写法一致、followed-by 动态 ISO 逐臂求值、GMT-4 时区计算日期精确触发；typed spec 零 Period+StartAt 走一次性日期路径（对齐 Java date-only），零 Period 无 StartAt 构建期拒绝。
>
> 历史补充：Draft 3.67（2026-08-11），PatternObserverTimerAt suite 10/10 关闭（5 个新增 runtime ID）：timer:at cron 形式（weekday 列表 [1..5]=Monday..Friday、Sunday=0..Saturday=6 编号与 ScheduleSpec 一致）回放 2008-08-03 起整周扫描，精确命中周一至周五 08:00；替换参数（ParameterAt 1-based + DeployWithPositionalParameters）、变量字段（VariableRef）、cron 内联算术（Subtract/Add 常量表达式）与 */15 月步长部署全部通过。
>
> 历史补充：Draft 3.66（2026-08-10），PatternConsumingPattern suite 10/10 关闭（8 个新增 runtime ID）：@DiscardPartialsOnMatch/@SuppressOverlappingMatches 注解对应 DiscardPartialsOnMatch()/SuppressOverlappingMatches() 查询选项，or/and/not/guard/observer/match-until/every 全场景 28 个子用例原样回放一次通过；登记差异——Esper 按共享事件选择性丢弃部分匹配，Go 在完成事件上丢弃全部进行中实例（suite 场景可观测一致）。
>
> 历史补充：Draft 3.65（2026-08-10），PatternObserverTimerInterval suite 8/8 关闭（1 个新增 runtime ID）：every a -> timer:interval(intPrimitive seconds) 动态间隔逐事件武装回放（E2@12000/E1@13000 精确触发）；修复 sequence/or 定时器新鲜性缺陷——已完成定时器在后续时钟推进上重报满足不再重复触发父节点（patternSideFiredFresh 与 and 侧同规，timer 回调仅在侧于本触发前未满足时计为新鲜）。
>
> 历史补充：Draft 3.64（2026-08-10），PatternOperatorMatchUntil suite 9/9 关闭（7 个新增 runtime ID）：紧约束 [N:N]+until 达上限即触发不等终结符（对齐 EvalMatchUntilStateNode.isTightlyBound），宽松区间达上限仅停收集仍由 until 决定触发；已完成 match-until 释放子节点后改为保留满足持续上报（修复 and/or 父节点配对循环零转移丢触发）；MatchUntil 静态 bounds 放行 minimum=0（[:M]/[0:M] 形式）；52 个 W-harness 子用例按时间戳事件集回放，定时器触发归入下一事件桶。
>
> 历史补充：Draft 3.63（2026-08-10），PatternOperatorEveryDistinct suite 17/17 关闭（15 个新增 runtime ID）：EveryDistinct 复合根物化为 everyNode 携带独立 keyset，every 证伪重启以空 keyset spawn，match-until 仅在子状态永久完成时替换子节点以保留存活 every-distinct 连同 keyset，定义级 distinct key 求值补齐 tag 上下文；新增 EveryDistinctForCalendar 月作用域 calendar 到期与可变参数 EveryDistinct/(expiry, keys...) EveryDistinctFor，非常量 key 构建期拒绝。
>
> 历史补充：Draft 3.62（2026-08-10），PatternOperatorAnd/Or 两 suite 剩余 4 个 execution 关闭（And 4/4、Or 3/3）：and 改写为 eventsPerChild 双侧匹配列表语义（新鲜完成与对侧缓存笛卡尔组合逐条回调、fireOnly 仅发射不入保留集、every 按回调逐次派生），新鲜性判定排除早前已满足侧修复定时器回调双触发；or 保持永久完成分支杀死整体、every/not 支存活语义；timer:interval(0) 放行且部署时钟即到期；模式匹配流接入无分组 count(*) 聚合。
>
> 历史补充：Draft 3.61（2026-08-10），PatternOperatorFollowedBy suite 10/10 关闭：已完成序列在右支未退出时保留并经保留序列新鲜完成再触发（对齐 EvalFollowedByStateNode nodes 映射），派生右支起始即满足（not/or-not）立即以 null 右侧完成，外层 every 按血统顺序 continued/sibling 派生使 every (every a -> every b) B3 精确 4 触发（经 Java 引擎实测核实），新增 ThenMax 有界复合右支与 PatternQuery.Where（from pattern 后语句级 where）。
>
> 最新补充：Draft 3.60（2026-08-10），PatternOperatorNot suite 6/6 关闭：and/or 触发规则改为本次事件新完成子分支+各侧缓存满足（对齐 EvalAndStateNode eventsPerChild 与 EvalOrStateNode 回调语义），完成后保留含存活 every 腿的 and/or 状态，every 派生时吞没起始即满足的子树（or-not 零触发）。当前 279 mapped / 1 partial / 17 approved-difference。本轮不需要 MySQL Docker。


## 1. 文档信息

当前切片（Draft 3.87，2026-08-12）关闭 ViewIntersect 剩余 8 个 execution（FirstAndLength、Batch+Derived、Grouped+Sorted 矩阵）；修复 `compositeIteratorEvents` 的 post-flush snapshot 与 `addToWindow` CompositeWindow child removal propagation 对齐 Java IntersectDefaultView。下一步继续其他 capability 域。

| 项目 | 内容 |
|---|---|
| 文档状态 | Draft 3.87，Client domain 100% 关闭（281/281）；View domain 114 个唯一 runtime（新增 ViewIntersect 剩余 8：FirstAndLength、Batch+Derived、Grouped+Sorted；TimeAccum 10/10、TimeLengthBatch 9/9、FirstTime 3/3、ExternallyTimedWin 5/5、ExternallyTimedBatched 5/5、ExpressionWindow 13/13、ExpressionBatch 13/13、LengthBatch 3/8、Union 1 及既有 KeepAll/Length/First/Last/Time/TimeBatch/Sort/FirstUnique/MarketData 子集），Pattern domain 107 个；ExprFilter 32、ExprEnum 27、ResultSet 2、EPLOther 2、EPLInsertInto 1；manifest 当前 2,102 条关联、2,102/4,140 唯一 runtime（50.77%）、318 mapped / 1 partial / 17 approved-difference；下一步继续其他 capability 域 |
| 历史增量状态 | Draft 2.22：在 sorted aggregate access 的不可变导航快照基础上补充 Fire-and-Forget named-window 快照、重复 key 桶、边界事件、`EventsBetween`、descending/navigable map 访问和对照测试登记；新增 Table 按主键 selector 的 target-row 绑定、缺失分组 Null 投影和 grouped sorted table Java 对照；声明表达式已补充 Context initiating/pattern Event、多行 `SubqueryEvents` 参数和 Map keep-all/where/`NullOnMultiple` cardinality 对照；Java `TestSuiteExprDefine` 5/5 通过。 |
| 前序复核状态 | Draft 2.33（2026-08-07）：补充非分组聚合子查询 `SubqueryHaving` 的完整 inner-group 评估、outer-field 相关阈值、`SubqueryExistsValue` 以及带 options 的 IN/ANY/SOME/ALL；补充多列子查询结果的递归 fragment Schema、Row/Event 的标量 `GetFragment` 与 indexed `GetFragments` 运行时物化，并以 scalar/history/rows 三层对照测试固定 map/slice 结果不变。补充 TableColumn nested schema metadata 与 Table/Named Window representation 保留对照；本轮再补齐 `InfraOnMergeMatchNoMatch` 的 Go-native `CopyMatchingFields` wildcard 赋值、`InfraOnMergeInsertStream` 的 `ThenInsertInto`/`ThenInsertIntoWhen`/`ThenInsertIntoTarget` 有序 action-chain，以及 matched side-stream 读取 target-row 后继续 update 的边界，覆盖 Table/Named Window、source-only/target-only 字段、side-stream projection、条件 side-stream、目标字段作用域以及 new/old/target snapshot 语义；相关 case 已登记到 `testdata/compat/capability-manifest.json`。Java `TestSuiteInfraNWTable` 在 JDK 17/Maven 3.9.16 下 26/26 通过；Go 核心与本轮验证不依赖 MySQL，DB/SQL/connector 测试继续按需使用本机 `esper-java-mysql`。此前补充 `ClientExtendAggregationMultiFunction` 的 typed fluent provider、共享 `StateKey`、分组/窗口 Enter-Leave replay、过滤作用域、IntoTable/trigger 读取和 inline/invalid Build 对照；补充 `SortedMultiKey` 两级字典序及 alias/string/numeric 比较边界；补充 `InfraNWTableOnMerge` 的单侧 merge 分支、无条件 `WhenMatchedAny`/`WhenNotMatchedAny`/`WhenMatchedDeleteAny`、Table/Named Window new/old 对照及 Java runtime 映射。JDK 17（`C:\Program Files\Microsoft\jdk-17.0.20.8-hotspot`）、Maven 3.9.16（`C:\Users\baicai\AppData\Local\UniGetUI\Chocolatey\lib\maven\apache-maven-3.9.16`）可用；SQL/DB 相关测试按需使用本机 `esper-java-mysql`（MySQL 8.0，`127.0.0.1:3306`），核心/本轮测试不依赖 MySQL。 |
| 历史修订（Draft 2.70，已由 Draft 2.72 取代） | 继续对照 Context `ContextKeyedSegmentedTable`（`java-runtime-f9d69a7108debefc522e`），为 live Table trigger 的 `InsertIntoTable`、`UpsertIntoTable`、merge not-matched insert 增加首次 ownership 注册，并在 live delete 后清理 row identity；`TestContextTableLiveInsertOwnershipMatchesEsper` 覆盖 A/B 分区、三种插入入口、更新分区字段后按原分区 selector FAF 删除以及归属清理。此前的 FAF mutation 失败恢复仍覆盖 Table/Named Window 根、Context 分区状态、ownership 和 pending queues；完整跨 statement transaction、跨目标 routed side effect、listener/external resource rollback、更广 Context Table live mutation 组合和 transaction/concurrency trace 仍列后续项。Java `@Hint`/`SupportQueryPlanIndexHook` 不作为 Go 契约；Java `TestSuiteContext` 17/17 通过，本轮不需要 MySQL Docker。 |
| Java 对照项目 | D:/Code/soc/esper |
| 本轮增量 | Draft 2.63（2026-08-08）：Context inner FAF Join 已接入安全 equality/range candidate；首个已加载源负责枚举分区，索引结果按 context key 过滤，Named Window/Table、All/指定 key selector 和 index-hit 计数均有对照测试。Context 两流 outer 与相邻条件左深 mixed `Inner`/`LeftOuter` chain 已继续接入安全物理候选；Context right/full-preserving、非相邻条件和 unidirectional Join 与 Context FAF subquery 仍保持后续项；Java `TestSuiteInfraNWTable` 26/26 通过。 |
| 本轮增量（继续） | Draft 2.64（2026-08-08）：新增 Context FAF 子查询的 equality-prefix + B-tree range candidate。`OuterField`/参数可作为完整 equality prefix 与 lower/upper range bound；全局 Named Window/Table 和 Context-scoped Named Window 均按插入顺序返回候选，指定 partition selector 只访问目标 partition，无法证明安全边界时回退完整 snapshot。新增 `TestInfraFAFSubqueryContextRangeIndexCandidateParity` 与 `TestInfraFAFSubqueryContextBoundRangeIndexCandidatePartitionParity`；该切片关闭可观测 range candidate 子集，但不宣称 Java 10k/1M timing、index-sharing/cost/selectivity、JVM query-plan hook、representation、历史/方法源和完整 trace parity。 |
| 本轮增量（继续 2） | Draft 2.65（2026-08-08）：Context FAF Join 物理路径由 inner/all-inner chain 扩展到两流 `LeftOuter`/`RightOuter`。`fireAndForgetJoinEvaluationOrder` 保证 preserved side 先加载，optional side 的 equality-prefix + B-tree range candidate 可为空并由普通 `joinTuples` 生成 unmatched preserved row；Named Window/Table、全量/指定 partition selector、partition key 二次过滤和 index-hit 计数均有对照测试。FullOuter、混合/链式 outer、unidirectional、成本/选择性和完整 Java query-plan/performance trace 仍回退或保持 partial。 |
| 本轮增量（继续 3） | Draft 2.66（2026-08-08）：Context FAF Join 再放开相邻条件的左深 mixed `Inner`/`LeftOuter` chain candidate。仅当每条边引用紧邻前一源和新源时才允许后续 source equality/hash 或 equality-prefix + B-tree range lookup；缺失 optional 中间源不会错误恢复后续匹配。Named Window/Table、全量/指定 partition selector、稳定顺序和 middle/tail `indexLookups` 有对照测试；RightOuter/FullOuter、非相邻链、unidirectional、cost/selectivity 与完整 Java trace 继续 partial。 |
| 本轮增量（继续 4） | Draft 2.67（2026-08-08）：复核 Java `InfraUpdate` 后补齐 Go fluent `OnDemand().UpdateAll(...)`，覆盖 Named Window/Table 的无 `where` 全量更新、逐行 ordered assignment、`InitialNamedWindowField`/`InitialTableField` 以及 Named Window 返回更新后事件、Table 空结果契约；同时修复 Table 更新主键时的原子 re-key，保留原插入顺序并同步 hash/unique/B-tree index，主键或唯一 secondary index 冲突不改变原状态。`TestOnDemandUpdateAllMatchesInfraUpdate` 与 `TestTableUpdateRekeysPrimaryKeyInPlaceAndPreservesIndexes` 固定该切片；Java `TestSuiteInfraNWTable` 26/26 通过，本轮不需要 MySQL Docker。 |
| 本轮增量（继续 5） | Draft 2.68（2026-08-08）：继续审计 Context Table mutation。为每个 Table 行增加稳定内部 identity，并由 Engine 保存 table/context/row 到 partition 的 ownership 与首次代表 context properties；Context FAF 首次扫描绑定归属，后续更新 category/hash/key 字段或 primary key re-key 不重新分区。删除行和 DestroyContext 清理归属，回归测试强化了 negative category update 后仍由原 selector delete-all 的 Table 结果。普通 FAF mutation 的整体 rollback、live context insert 的首次 ownership 注册、跨 context row ownership 与 transaction/concurrency trace 仍开放；本轮不启动 MySQL Docker。 |
| 本轮增量（继续 6） | Draft 2.69（2026-08-08）：为 FAF mutation 增加失败恢复边界。执行前快照目标 Table 或 Named Window（含 Context partition runtime）、Engine 的 Context Table ownership/partition IDs/next IDs；action 或 `processPendingRoutedEventsLocked` 返回错误时恢复这些状态并清空 pending dispatch/routed queues。新增 `TestOnDemandTableMutationRollsBackAfterMidBatchFailure`、`TestOnDemandNamedWindowMutationRollsBackAfterAssignmentFailure`、`TestOnDemandMutationRollsBackAfterRoutedProcessingFailure` 与 `TestOnDemandContextTableMutationRollsBackAcrossPartitions`，验证第一行/第一 partition 已成功后第二行/第二 partition assignment/type、route cycle limit 或 primary-key collision 不会留下半成品状态。该实现覆盖当前 FAF 目标存储，不宣称跨 statement、跨目标 routed side effect、listener/external resource 的完整事务回滚；live context insert ownership、并发 transaction 与完整 Java trace 仍开放；本轮不启动 MySQL Docker。 |
| 本轮增量（继续 7） | Draft 2.70（2026-08-08）：对照 Java `ContextKeySegmentedInfra.ContextKeyedSegmentedTable`，新增 `context_table_live_test.go` 的 Go 链式测试，分别使用 `InsertIntoTable`、`UpsertIntoTable` 和 `MergeInsertIntoTable` 在 A/B Context 分区写入 Table；Engine 在 live action 成功后按稳定 row identity 保存首次 partition key、代表事件和 selector properties，后续 live update 改变分区字段不重新归属，Context FAF 用旧 A selector 仍可删除；live delete 同步回收 ownership。Java `TestSuiteContext` 17/17 通过；跨 context 相同主键隔离、完整 aggregate-into-table/Context Table transaction、listener/external resource 与更广 Java trace 仍保持后续项；本轮不启动 MySQL Docker。 |
| 当前修订（最新） | Draft 2.80（2026-08-08）：在 Draft 2.79 的 EsperIO DB executor 切片之上，继续对照 Java `EPLDatabaseFAF` 的 10 个 execution。`SQLHistoricalProvider` 修复多行结果跨行错误去重，新增 `SQLHistoricalRowConverter`/`RowConverter` 支持 SQLROW whole-row materialization 与 nil-row skip，并新增 `SQLHistoricalProviderOptions.ParameterTypes` 将 opaque SQL argument functions 纳入 Go Plan 的参数声明、canonical、缺参/额外参数/类型校验。`database_faf_parity_test.go` 覆盖 simple、column/row hook、prepared reuse/close、typed substitution、distinct、where 多行、variable late binding、fluent SODA-equivalent plan、SQL text subquery parameter snapshot 和 invalid SQL/closed query。Java `EPLDatabaseFAF` 10/10 runtime 已登记到 `case.historical-sql`；Java SQL+SQL Join/Context SQL FAF 仍是 Java invalid，而 Go typed historical/method Join/Context 是显式扩展；1000 次性能阈值、精确 SQL invalid 文本、完整 dialect/type-code/connection-pool/transaction/trace 仍未宣称等价。 |
| 本轮增量（继续 8） | Draft 2.72（2026-08-08）：关闭 Context-scoped Table 的 FAF/index candidate 遗漏。`lookupManyAllScopes`、`lookupPrimaryManyAllScopes`、`lookupRangeManyAllScopes` 让普通 FAF/Join 的 Table candidate 合并 root 与所有已物化 scoped state；相关子查询根据保留 Context 变量查当前 scope，root state 有行时回退 ownership-aware snapshot。新增 `TestInfraFAFSubqueryContextScopedTableIndexCandidateParity`、`TestInfraFAFSubqueryContextScopedTableRangeIndexCandidateParity`、`TestInfraFAFContextScopedTableIndexCandidateParity`，覆盖 duplicate primary key partition isolation、hash/range lookup、legacy root fallback、full selector 和 index-hit count。Java `TestSuiteContext` 17/17 与既有 `TestSuiteInfraNWTable` 26/26 对照保持通过；本轮不启动 MySQL Docker。 |
| 本轮实施（EsperIO DB executor） | Draft 2.79：Java `ExecutorServices`/`ExecutorSameThread`/`RunnableDML`/`RunnableUpsert` 已对应 Go `ExecutorServices`/`SameThreadExecutor`/`AsyncExecutor`/`Task` 与 DML/Upsert `WriteAsync`；覆盖 named queue、same-thread、FIFO drain、cancel、panic/error、retry、Destroy lifecycle re-check 和 `go test -race ./connectors/db`。Java XML/JNDI/config、connection factory、SQL type binding、duplicate/error handler、日志文本与完整 trace 仍开放。 |
| 本轮追加（SQL 参数边界） | 固定 `Plan.Canonical()` 中排序后的 provider 参数类型、缺参/额外参数/运行时类型错误，以及 `ParameterTypes` 的 blank/nil/trimmed-conflict 构造期诊断；这些是 Go fluent API 的稳定契约，不等同于 Java SQL vendor 的原始错误文本。 |
| Java 基线 | Esper 9.0.0，tag release_9.0.0，commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c |
| Java 要求 | Java 17 |
| Go 目标项目 | D:/Code/soc/bigsoc-esper |
| 目标仓库现状 | 规划起点为空；当前工作树已出现 Go 垂直切片实现，不能再按“空仓库”判断完成度 |
| Go 工具链现状 | 本机 go1.25.5；最低支持版本在阶段 0 固化，候选为 Go 1.25 |
| 核心 API 方向 | Flink DataStream 风格的 Go 链式 API，不以 EPL 字符串作为规则定义方式 |
| 规划原则 | 对照 Java 行为完整移植，采用 Go 架构与习惯，不逐类、逐包机械翻译 |
| 本轮补充复核 | Draft 2.34（2026-08-07）：新增 `InfraInsertOtherStream` bootstrap 对照，覆盖 Table route-before-merge、Named Window merge-before-route、deployment-order 语义和 struct/Map/ObjectArray target representation 保留；Avro/JSON/JSONCLASSPROVIDED/XML 仍为 partial。 |
| 本次增量复核 | Draft 2.35（2026-08-07）：对账 `InfraNWTableOnMerge` execution inventory 后登记此前遗漏的 6 个 runtime：`InfraPropertyEvalInsertNoMatch`/`InfraPropertyEvalUpdate` 的 4 个 Table/Named Window 变体已由 typed `Unnest` 测试映射；`InfraDeleteThenUpdate` 的 2 个变体登记为 `partial`，明确 Go terminal-delete 与 Java Named Window 观察结果的差异。 |
| 本次切片复核 | Draft 2.36（2026-08-07）：补齐 Java `InfraNamedWindowOnMerge` 的 10 个 runtime 对账：JavaBean setter/working-target 更新、Named Window 级联 dispatch 与 insert-stream-only、Map omitted-property Null 和 method projection，以及 `InfraSubselect`/`InfraDocExample`/contained insert/RHS Event 的 Map/ObjectArray/Avro/JSON/JSON-class-provided/struct 表示矩阵；新增 4 个 capability case 和 manifest mapping。Java `TestSuiteInfraNamedWindow` 17/17 通过，Go 定向切片通过；本轮不需要 MySQL Docker。 |
| 本次清单复核 | Draft 2.37（2026-08-07）：修正 6 个旧 Java runtime ID，区分 12 个静态候选 ID 与运行态 `javaRuntimeIds`，并增加 manifest 校验防止两类 ID 混用；当时唯一运行态引用为 1,179/4,136（28.51%）。全量 Go、race、vet 和 compat 门禁均通过；本轮仍不需要 MySQL Docker。 |
| 本次语义复核 | Draft 2.38（2026-08-07）：关闭 `InfraDeleteThenUpdate` 的 2 个 partial runtime：Go 现在复现 Java 的 Table 删除结果与 Named Window `A/10` 保留结果；同时保留 `InfraMultiactionDeleteUpdate` 的条件多 action terminal-delete 语义，避免扩大未被 Java 证据支持的规则。新增 target-specific evaluator 对照，当前不需要 MySQL Docker。 |

本轮 Variant 复核（2026-08-07）：重新执行 Java `TestSuiteEventVariant`，17/17 execution 通过；Go 新增 `variant_parity_test.go` 的 boxed/numeric common metadata、JavaBean getter cache、indexed/mapped/nested getter、Pattern/Subquery、LengthWindow old/new、Named Window Unique、ANY dynamic metadata、wrapper/derived、wildcard Join、late schema 和 Plan canonical 对照。实现上修复 PREDEFINED Variant getter 的成员 schema 分派，并修复 Variant 成员无投影进入 Named Window 时保留 routed member Event；对应 15 个 Java runtime 新增清单记录。本轮后续已补齐单列 method-backed member conversion：`InsertEventIntoNamedWindow` 接受 `Func1` 返回的 concrete member underlying，按 target Variant member schema 物化并保留 Named Window envelope；`TestVariantSingleColumnConversionMatchesEsper` 对照 Java `EventVariantSingleColumnConversion`。本轮不需要 MySQL Docker。

本轮继续对照 Java `ExprEnumDataSources`：新增 `enum_data_sources_parity_test.go`，用 Go 链式 `PrevWindow`/`EnumAllOf`/`EnumSelect` 复现普通 length window 和 sorted window 的 collection 顺序、view eviction 与元素访问；用 `SubqueryValues` 对照 Named Window/Subquery 的空集合与混合数据；用 `Parameter[T]`/`DeployWithParameters` 和 `VariableRef` 固化 late-bound collection source；用 typed nested `Property`/`EnumWhere` 验证 schema-backed array property 的 child metadata；并覆盖 method-backed enum object、mapped property、collection cast、sorted access、Go-native generic component，以及已有 Window access aggregate/Table source 测试。对应 15 个 Java runtime 纳入 `case.expr-enum-data-source-propagation`；Java `TestSuiteExprEnum` 28/28 通过。join/pattern、match-recognize/context property、Java Optional 精确 generic/serde、完整 invalid/type metadata 和共享 trace 仍保持 partial。本轮不需要 MySQL Docker。

本轮 FAF 复核对照 Java `InfraNWTableFAF`：新增 `infra_nwtable_faf_parity_test.go` 和 `faf_mutation_test.go`。Go 用 `FromNamedWindow`/`FromTable` 链式构造只读 FAF，并用 `OnDemand()` 构造 insert/update/delete/delete-all，不引入 EPL 字符串；覆盖两类目标的 wildcard/filter/distinct/count/sum/group/in、四种二流 Named Window/Table 组合、三流组合、join where、参数绑定、`InitialNamedWindowField` 和消费者 delta dispatch。修复 ungrouped row-for-event 聚合快照/FAF 输出，使 `theString + sum(...)` 对每个保留事件返回一行。Java `mvn -pl regression-run '-Dtest=TestSuiteInfraNWTable' '-DfailIfNoTests=false' '-Dgpg.skip=true' test` 为 26/26；新增 28 个 runtime 映射。Context partition selector、完整 prepared/positional parameter、index plan、representation、FAF subquery/resolve/historical source、invalid/transaction/concurrency 仍按 `query.fire-and-forget` 保持 partial；本轮不需要 MySQL Docker。

本轮继续对照 Java `InfraNWTableFAFInsertMultirow` 的 5 个 execution（Named Window/Table 成功、Named Window/Table rollback、invalid）：新增 `faf_multirow_test.go`，用 `OnDemand().InsertRows(InsertValues(...))` 覆盖两行 positional insert、独立 Named Window 子查询值、1000 行上限、行值数量诊断和 unique/primary-key 冲突整批回滚。Named Window 的 `NamedWindowUniqueIndex` 是严格约束，保留 `NamedWindowRetention(Unique(...))` 的重复 key replacement 语义；多行提交只排队一个合并 Named Window delta，避免消费者观察到中间半批状态。Go 的 immutable fluent Plan 对应 Java SODA/model 构造，未引入 EPL 字符串；本轮不需要 MySQL Docker。

本轮继续对照 Java `InfraNWTableFAFSubstitutionParams` 的 7 个 execution（`java-runtime-84f052c24b6691c4c9ba`、`java-runtime-839c641f3f76537dfc4f`、`java-runtime-1631538ca06e4cb6a7af`、`java-runtime-15505945766cf1e16463`、`java-runtime-b207ffe847bca6059b98`、`java-runtime-5ff95eb63b189816d771`、`java-runtime-84c9760295357bac5cc7`）：新增 `faf_substitution_params_test.go`，以 `ParameterAt[T](1)`/`Parameter[T]("p0")` 和 immutable positional/named binding 复现 Named Window/Table、slice-IN、重复引用、缺参、额外值、混用、跳号和类型错误矩阵。该 case 已登记为 `case.query-fire-and-forget-substitution-params`，并从 `query.fire-and-forget` 的剩余项中移除基础 positional/named breadth；跨 join/subquery/context 的参数组合、index plan、representation、resolve/historical source、事务/并发和完整 trace 仍保持开放。本轮不需要 MySQL Docker。

本轮新增 Java `InfraNWTableFAFSubquery` 对照切片（15 个功能 execution，4 个 performance/index execution 留作 partial）：`faf_subquery_parity_test.go` 使用 Go-native source/store fixture，覆盖 Named Window 与 Table 的 scalar subquery、二流 join、FAF insert、未相关 update/delete、`OuterField` 相关 select/update/delete、Context 两个 partitioned window 与一个 global window、inner where/group-by，以及 event-stream/source-filter/context-mismatch invalid。实现约束是：live on-trigger 仍以入站事件为 outer scope；FAF target-row mutation 仅在 on-demand 路径把目标事件放入 `OuterEvent`，子查询评估器在没有入站 `Event` 时回退到该显式作用域。该切片不依赖 MySQL Docker；索引选择/计划 hook、性能阈值、全表示/事务/并发与完整 Java diagnostic trace 继续留在 `query.fire-and-forget` 的 remaining。

本轮索引复核对照 Java `InfraNWTableFAFIndex` 的 12 个运行态 execution：新增 `faf_index_parity_test.go`，用 Go-native `IndexHash`/`IndexBTree`、`SecondaryIndex`/`UniqueIndex`、`NamedWindowIndex`/`NamedWindowBTreeIndex` 和 `UseIndex`/`UseIndexOn` 复现单源自动选择、普通内连接两侧选择、unique/non-unique backing、IN/equality/range、单数组/双数组/复合 slice key、null key 和错误 hint。`Plan.IndexPlan()` 提供可复制的 source/index/access/backing/matched-columns 摘要；`NamedWindow.Lookup` 维护声明索引并覆盖插入、更新、删除、Unique replacement、窗口淘汰和多行回滚恢复。随后把单源 Table/Named Window 的完整 hash equality/IN 接入物理 candidate lookup，把普通内连接及两流 outer optional side 的完整 equality/range key、Context 单源 Table/Named Window candidate 接入物理执行，并把 B-tree `Between`/单边比较接入 ordered candidate path；测试固定 automatic/explicit index、source filter、复合 equality-prefix、双边 inclusive range、反向操作数、`JoinWhere`、preserved-side unmatched 行、Context partition selector、重复 probe key、FullOuter/链式 outer/unidirectional/OR/UDF/Null/超大 probe 安全回退、prepared bounds、Null 边界 snapshot fallback 以及 update/delete/clear 后不残留陈旧成员。该路径保留最终谓词求值和稳定插入顺序；尚未宣称 FullOuter/链式 outer/unidirectional range Join、Context FAF Join/subquery 物理候选、真正的二分游标、成本选择、JVM backing class 或性能阈值，这些仍是后续优化/组合切片。本轮不需要 MySQL Docker。

本轮继续对照 Java `InfraNWTableFAF.InfraDeleteContextPartitioned`（`java-runtime-5a682b340bdb8f3f1fe5`、`java-runtime-489ad40b70745286f9cb`）：新增 `OnDemandStream.WithContext`，将 context 名称纳入 Query/Plan canonical；新增 `TestOnDemandContextPartitionMutationMatchesInfraDeleteContextPartitioned`、`TestOnDemandCategoryPartitionMutationMatchesInfraDeleteContextPartitioned` 和 `TestOnDemandContextPartitionUpdateAndDeleteAllRespectSelector`，覆盖 Context-bound Named Window/Table 的 hash bucket/descriptor-hash/category selector、选错分区 no-op、选中分区 delete/update/delete-all、`context.label` 条件、Named Window old-event 返回和 Table 空 FAF result。Named Window partition 保存首次 materialize 的 context properties，因此 update 改变事件字段后仍能按原 partition selector 定位；Table 侧当前按 mutation 时的行值计算逻辑 partition，持久化的 context-table row-to-partition ownership 仍是后续差异。Context mutations 在同一 engine lock 内完成，Table 不再用全表 `Clear` 绕过分区；本切片关闭的是 Java 该两项 delete runtime 及 Go-native update/delete-all 回归，不代表完整 Context selector/FAF prepared、index-plan、representation、subquery/resolve、invalid、transaction/concurrency 矩阵已完成。本轮不需要 MySQL Docker。

本轮复核补充：对照 Java `InfraUpdateNestedEvent` 的 `java-runtime-065003de88aca37795b8`/`java-runtime-2e8d691b5e2c927038d7`，新增 `TestTriggerNestedAssignmentsPreserveMapAndObjectArrayValues`，覆盖 Map/ObjectArray × Table/Named Window 的直接复合列赋值及 `cflat.c0`、`carr[0].c0`、`carr[1].c0` 读取。修复 Table materialization 遗漏：`TableColumn` 支持 `WithTableColumnNestedSchema`，Table schema 与 Plan canonical 保留复合列/数组元素 nested metadata。该 slice 已登记为 `case.trigger-nested-assignment` 并映射到 `trigger.table-named-window`；wildcard、多 action、mapped/nested path 写入、representation-specific conversion/metadata、事务/持久化和 `InfraNWTableOnMerge` 其余 execution 仍未完成。JDK 17/Maven 可用，当前验证不需要 MySQL。

本轮继续对照 Java `InfraNWTableOnMerge.InfraUpdateOrderOfFields`（`java-runtime-3034e5da517c1235da22`、`java-runtime-cb5eefe84a486b090e53`）和 `InfraNWTableOnUpdate.InfraUpdateOrderOfFields`（`java-runtime-fdc71005421b6648a890`、`java-runtime-06eb4a56203c92fe01a6`）：新增 `TestTriggerOrderedScalarAssignmentsMatchInfraUpdateOrderOfFields`，覆盖 Table/Named Window × Merge/Update 四种入口；按 Java 顺序验证 `intPrimitive=trigger.id`、`intBoxed=working.intPrimitive`、`doublePrimitive=initial.intPrimitive`，并固定 E1 的 `5/5/1.0` 与后续 `7/7/5.0` 结果。该 case 已登记到 `trigger.table-named-window`。这补齐的是 scalar ordered assignment 的对照证据，不代表 wildcard、多 action、mapped/representation、subquery/pattern 和其余 on-trigger execution 已完成。

本轮补充 Java `InfraOnMergeSimpleInsert`（`java-runtime-dbf13fb6d1ca3a37275a`、`java-runtime-df3d7a21bdd769f1acce`）：新增 `MergeInsertIntoTable`/`MergeInsertIntoNamedWindow` 两个 Go-native 链式便捷入口，明确表达只执行 not-matched insert 的 merge；`TestTriggerMergeInsertOnlyConvenienceMatchesInfraOnMergeSimpleInsert` 覆盖 Table/Named Window 的 A/B 插入、重复 A 不更新以及最终快照。该 case 已登记到 manifest。Java 的多 action insert-into stream、wildcard/select projection、representation-specific conversion 和完整 on-merge trace 仍待后续切片处理。

本轮继续对照 Java `InfraMultiactionDeleteUpdate`（Named Window `java-runtime-dfa83c0593d19172be7d`、Table `java-runtime-ffbda0563d50878dafd8`）：新增 `TableMergeAction`、`ThenUpdate`、`ThenDelete`、`WhenMatchedActions`，按 working target 顺序求值，后续动作可读取前一动作的更新，delete 终止动作链，最终只提交一次 old/new mutation；`TestTriggerMultiActionMergeMatchesInfraMultiactionDeleteUpdate` 覆盖 E1-E6 的 Java 轨迹、Table/Named Window、plan identity 和 terminal-delete 边界，已登记为 `case.trigger-merge-multiaction`。该切片仍不等于完整 on-merge：wildcard/select projection、insert-into stream、多表示转换、mapped/nested path 写入、事务/持久化、pattern/subquery 组合和共享 Java/Go trace 继续保持 partial；本轮不需要 MySQL Docker。

本轮继续对照 Java `InfraOnMergeInsertStream`（`java-runtime-2c687a68317caea3c148`、`java-runtime-081455731ccafbf6847c`）：新增 `ThenInsertInto`/`ThenInsertIntoWhen`、`ThenInsertIntoTarget`、`WhenNotMatchedActions`，把 not-matched 分支的多个 side-stream projection 和最终 Table/Named Window insert 保持在同一个有序 action chain 中；`TestTriggerMergeInsertStreamActionsMatchInfraOnMergeInsertStream` 覆盖 StreamOne/Two/Three、条件 StreamFour、K1/K2 及两类 infra target，已登记到 manifest。Go 使用预注册 event schema 和正常 route queue，不复制 Java 推断输出类型/EPL wildcard；同步 interleaving、rollback/transaction、representation conversion 和完整 on-merge trace 仍保持 partial。

并行复核补充 Java `InfraFlow` 的 wildcard assignment 子集（`java-runtime-403bba8c6b29e32b1a8f`、`java-runtime-ae05c015767242106de7`）：`CopyMatchingFields` 按 target schema 复制同名 source fields，insert 时 target-only optional 字段物化为 Null，matched update 保留原 target-only 值；`TestTriggerMergeWildcardCopiesMatchingFields` 已登记为 `case.trigger-merge-wildcard`。完整 InfraFlow 的 source-filter、delete/update、多表示和 route 生命周期仍未关闭。

本轮进一步对照 Java `InfraNamedWindowOnMerge`：`MergeIntoNamedWindowWhen` 对“无 where 且没有 matched 分支”的 insert-only 规则按每个输入事件执行 not-matched，修复 contained expansion 第二个 child 被错误丢弃的问题；Map-backed 新行会为声明但未赋值的字段物化 nil，使 `Event.Get` 返回 Null 而不是 Missing。新增测试覆盖 JavaBean setter 与 working/initial target 顺序、Named Window 级联 dispatch 的两段行为、Map property insert 与 method projection、A/B/C filtered `LastEvent` 子查询驱动的 merge，以及 DocExample 的六表示矩阵。该切片的 10 个 Java runtime 已登记为 4 个 mapped case；它只关闭这些 runtime 的行为证据，不代表整个 `trigger.table-named-window` capability、事务/持久化、XML/serde、完整 on-trigger action 及全量 Java trace 已完成。

本文以实施规划为主，不把当前 Go 原型的签名视为最终稳定 API。文中出现的方法名和调用链只表示目标 API 形态，除第 17 节外不代表对应功能已经完成。

本轮继续补齐子查询与 on-trigger 交界：`SubqueryRow`/`SubqueryRows`/`SubqueryGroupRows` 的静态 nested fragment metadata 已进入 projection result schema，`Row.GetFragment(s)`/`Event.GetFragment(s)` 可 materialize 标量、数组和递归嵌套 map/slice fragment；Join projection 同时完成 `JoinSelection` 到普通 `Selection` 的适配，避免 metadata schema 构建破坏全局编译。新增 `TestSubqueryMultiColumnRowsMaterializeScalarAndIndexedFragments`。另新增 `TestTriggerMergeNotMatchedAssignmentUsesCorrelatedSubquery`，对照 Java `InfraSubqueryNotMatched` 的两个 runtime（`java-runtime-905164d98662721e5509`、`java-runtime-d3e1f3aa50c4375f657f`），覆盖 Table/Named Window 的 not-matched assignment 中 `SubqueryValue + OuterField` 相关查询及 Named Window unique replacement；相关 case 已登记到 manifest。Java 的跨表示 fragment、iterator/cardinality/error、index-plan 和更广 merge/subquery 组合仍保持 partial。

本轮量词复核（2026-08-07）：对照 Java `EPLSubselectAllAnySomeExpr` 的 `EPLSubselectRelationalOpNullOrNoRows` 与 `EPLSubselectEqualsInNullOrNoRows`，补充 `TestSubqueryEmptyQuantifiersFollowEsperTruthTable`。固定空集合时 `ALL=true`、`ANY/SOME=false`、`IN=false`，包括 outer value 为 null 的情况；非空集合遇到 null candidate 时保留 Esper 的 Null 结果。该切片已登记为 `case.subquery-empty-quantifiers`，Java runtime 为 `java-runtime-ad44331ab7f0645a4e7a` 与 `java-runtime-29c66b744c3754d59ec7`。`go test ./...`、`go vet ./...` 与子查询/触发器竞态测试仍是本轮门禁；核心测试不需要启动 MySQL，数据库/SQL connector 测试继续按需使用本机 Docker MySQL。

本轮 JSON 标量复核对照 Java `EventJsonTypingVMClass` 与 `EventJsonParserLaxness`：新增 `URL`/`URI` Go-native 标量及 UUID、OffsetDateTime/LocalDate 对应的 typed JSON 转换，覆盖 scalar/一维数组/二维数组/collection、null、shape mismatch、非法标量和 JSON 渲染；新增全部 byte/short/int/long/float/double/BigInteger/BigDecimal（对应 `int8`/ `int16`/ `int`/ `int64`/ `float32`/ `float64`/`big.Int`/`big.Rat`）的 lax string conversion、非法值和 shape policy，以及 malformed/trailing/undeclared static/dynamic policy。修正 typed special scalar 的非法输入必须返回错误，避免错误值静默变成 Null；完整 dynamic JSON value matrix、递归 schema/metadata 和共享 parse-write trace 仍为 partial，本轮不需要 MySQL Docker。

本轮递归 JSON fragment 实施：`NewJSONSchema`/`NewJSONSchemaFor` 现在为 struct、pointer-to-struct 和 struct slice/array 字段自动构造稳定的嵌套 JSON schema；按 Go reflected type 缓存并在递归指针处回指共享 schema map，避免无限展开，同时保留深层 `Property`/`Getter` 的静态类型信息。`Schema.NestedSchema`/`NestedSchemaNames`/`FragmentSchema` 是 Go-native metadata 入口，`Event.GetFragment`/`GetFragments` 可直接导航 typed scalar/indexed fragment；显式 `WithNestedPropertySchema` 不被推断覆盖。`renderJSONNestedSchemaValueWithRaw` 对 nil nested slice 保留 JSON `null`，并有 class-array/partial/recursive 对照测试。该切片推进了 typed provided-underlying 的 fragment 证据，但不等于 Java named schema catalog、跨表示 fragment、module/path visibility 或全量 metadata parity 已完成。

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
| regression-lib | 回归场景与断言 | 1,387 | 场景代码位于 main |
| regression-run | JUnit 回归入口 | 0 | 82 |
| EsperIO 七个子模块 | AMQP、CSV、DB、HTTP、Kafka、Socket、Spring JMS | 144 | 58 |

额外基线：

- common/client/soda 下有 208 个 Java API 文件，是 Go 规则 AST 的重要结构参考。
- regression-lib 静态扫描到 3,848 条 RegressionExecution 候选；运行时按 801 个外层 suite 枚举得到 4,136 条可构造 execution 记录，另有 4 条明确 ignored，不能把两组数字混为同一覆盖率分母。
- regression-run 静态扫描到约 860 个 public test 入口方法。
- examples 下有 17 个示例项目、34 个 Java 测试源文件和 181 个 Java 主源码文件，需要按用例价值转换为 Go 示例或端到端测试。
- 回归标签包含多线程、性能、无效输入、即席查询、序列化、数据流、运行时操作、编译器操作和事件发送器等维度。
- 当前 capability manifest 已建立 1,387 条 runtime 关联、覆盖 1,391/4,136 个唯一 Java runtime（约 33.63% 的 Java runtime 对账/处置进度）；191 个 case 中 185 个 mapped、2 个 partial、4 个 approved-difference。该比例不是 Java/Go 行为 parity 通过率，也不是全量移植完成度。
- 除 regression-lib 外，common/compiler/runtime/common-avro/common-xmlxsd 共 371 个 Java 单元测试文件、regression-run 有 82 个入口源文件、EsperIO 共 58 个测试文件，也必须逐项分类；不能只迁移 RegressionExecution。
- 17 个示例为 autoid、benchmark、cycledetect、marketdatafeed、matchmaker、namedwinquery、ohlcpluginview、qos_sla、rfidassetzone、runtimeconfig、servershell、stockticker、terminalsvc、terminalsvc-jse、transaction、trivia、virtualdw。

当前已生成 `testdata/compat/source-test-manifest.json`（`esper-source-tests/v1`）作为静态源资产盘点：core-unit 371、regression-entry 82、esperio-unit 58、example-test 34、example-source 181，共 726 个 Java 源文件。另已生成 `testdata/compat/java-execution-inventory.jsonl`：静态候选 3,848 条、外层 suite 801 个、运行态记录 4,140 条，其中 4,136 条可枚举且 `runtimeId` 无重复、4 条明确 ignored、0 条探针错误。两类文件仍只证明“发现并固定了源/运行态资产”，尚未证明 Java 测试运行、Go 映射或差分通过；source-test disposition/mapping 仍是阶段 0 的退出条件。

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

- JSON：严格/宽松解析、数字精度、深度限制、原生表示、provided-underlying 等价能力、自定义 parser/field-adapter hook、schema-bound getter、自动/显式 nested fragment schema、特殊字段名、JSON/Map nested schema 校验、sender 原始字面量保留、模块/path visibility 和 Schema 演进。Go 的 adapter 采用显式注册函数，不复制 Java annotation/class-loader；JSON sender/getter/adapter/nested-fragment 的独立 Java runtime 映射必须与 core typed/laxness case 分开统计，不能因 `SendJSON` 已存在就漏记 EventSenderJson。
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

声明表达式的 Go 入口采用 `ExpressionParam[T]` 标记定义体参数、`DefineExpression[T]` 注册定义、`ExpressionRef[T](env, name, args...)` 调用；参数按首次出现顺序进入定义元数据，调用点做 arity/type 校验，局部绑定覆盖同名外层 prepared parameter 但不修改调用方上下文。值参数、嵌套声明表达式、参数错误和 Plan identity 必须有单测；事件值、子查询结果、Pattern/Context/Dataflow 作用域另列组合矩阵，不能因值参数通过而宣称声明表达式全量完成。

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
- Named Window/Table 子查询和索引复用；当前已补齐 `InfraNWTableFAFSubquery` 的 scalar/join/mutation/context/where/group-by 功能切片，以及 `InfraNWTableFAFResolve` 的显式 Module name-resolution/same-name isolation 功能切片；性能/index execution、计划复用和 Java path/visibility 语义仍待完成。
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
- Fire-and-Forget select/insert/update/delete、准备查询和参数化查询；Go 以 `FromNamedWindow`/`FromTable` 的 `OnDemand()` 链表达目标侧 mutation，返回值必须区分 Named Window old/new delta 与 Table 空结果，并保持同一 engine 事务边界；`WithContext` 与 `ExecuteFireAndForgetWithSelector` 还必须将 update/delete/delete-all 限定到选中的 Context partition。Module-bound `NamedWindow`/`Table` source 必须把 module identity 纳入 Build/canonical/runtime catalog lookup，允许同名对象隔离，并在未注册 module 时尽早失败。
- 依赖关系、作用域、部署卸载前置条件和资源回收；FAF 子查询的目标行 outer scope、Named Window/Table source 限制、source filter/context mismatch invalid 已在执行边界固定。
- Table 聚合重置、named/indexed 参数绑定，以及 prepared query 的幂等关闭；还必须覆盖更宽的 Context selector/FAF 生命周期、跨 join/subquery/context 的参数绑定、index/range/in 计划选择、representation conversion、invalid diagnostics、rollback 和并发一致快照。当前 `query.fire-and-forget` 已关闭基础 Named Window/Table snapshot、row-for-event aggregate、join、on-demand 单行及多行 mutation、`InfraDeleteContextPartitioned` 的 hash/category partition-local delete 子集和 `InfraNWTableFAFSubstitutionParams` 的基础 named/positional PreparedQuery 矩阵；Go-native update/delete-all 已有回归，但 Table row-to-context ownership 仍未持久化。
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
- structured audit、日志与异常回调；instrumentation 的固定 disabled execution 已处置，runtime/statement metrics 与 14 类 statement/dataflow audit 已有 typed management surface，剩余内部 callback/handle 精细度继续按 observable case 扩展。
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

当前 FAF 索引实施口径必须单独看待逻辑计划与物理执行：`IndexHash`/`IndexBTree`、`UseIndex`/`UseIndexOn` 和 `Plan.IndexPlan()` 先提供稳定的 Go-native 计划摘要；`index_runtime.go` 已把普通单源 Table/Named Window 的完整 hash equality/IN key（literal、变量、prepared named parameter、slice-IN）、普通内连接及两流 LeftOuter/RightOuter optional side、Context 单源 Table/Named Window 的完整 equality/range key，以及 B-tree `Between`/单边范围（含 equality-prefix）接入物理 ordered candidate path，并由状态层按插入顺序合并命中。索引探针无法安全提取（UDF、算术/属性链、Null/Missing、OR、FullOuter/链式 outer/unidirectional 或超大多流 probe）时必须回退完整 snapshot，不能猜测值；FullOuter/链式 outer/unidirectional range Join、Context FAF Join、FAF subquery、真正的 B-tree 二分游标、成本/selectivity 和 Java backing-class/query-plan hook 仍是后续实施项。

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
- 基于 `testdata/compat/source-test-manifest.json` 逐项盘点 371 个核心单元测试、58 个 EsperIO 测试、34 个示例测试文件和 17 个示例项目；regression-run 的 82 个入口源文件由回归 case manifest 关联，所有条目记录 Go 对应测试或 N 级理由。
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

当前已建立版本化、机器可读的 `testdata/compat/capability-manifest.json`（schema 由 `internal/compat/capability.go` 校验），包含 Capability、Case 两类实体及其多对多关联。它只登记当前已复核的首批映射、基础 rowrecog 映射和明确的 EsperIO/Serde 规划项，不代表 4,140 条 Java execution 已全部映射；新增能力必须先补 manifest，再进入 parity 看板。

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

模块单元测试、EsperIO 测试和示例进入同一数据库的 source-test 条目，记录 one-to-one、many-to-one、replacement 或 N disposition。当前 `testdata/compat/source-test-manifest.json` 只有静态发现字段和分类计数，不能被当作 disposition 已完成；后续必须在不改变稳定 ID 的前提下补充映射表或扩展 schema。many-to-one 必须列出覆盖断言，避免用一个宽泛 Go 测试虚假吞并多个 Java 测试。每个 S/G capability 至少关联一个正向和一个适用的边界/无效测试；每个适用 case 必须关联 capability，防止“有功能无测试”和“有测试无范围归属”。

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
| internal/compat | 仓库私有的清单校验、Java 源扫描和 parity trace 工具 |
| testdata/compat | 机器可读回归执行、源测试、连接器测试和示例映射资产 |
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

实际目录边界由 ADR-003 和 ADR-015 确定。目标是控制 import 环，避免 common 式超级包，也避免每个 Java 子包变成一个 Go 包。

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
3. **非 Regression 测试资产**：371 个核心模块单元测试、58 个 EsperIO 测试、34 个示例测试文件和 17 个示例项目不能只写在统计数字里；当前 `testdata/compat/source-test-manifest.json` 已固定 726 个 Java 源文件的静态分类，但每个文件/场景仍要补充 one-to-one、many-to-one、replacement 或 N disposition，以及 Go 断言映射。regression-run 的 82 个入口源文件必须与回归 case manifest 关联，不能漏算或重复计数。
4. **状态事务边界**：Variable、Table、Named Window、FAF、Context 分区和 `insert-into/on-trigger` 必须共享明确的版本/快照/提交边界。失败、取消、监听器错误或卸载不能留下半更新状态；不能让各子域各自定义一套不可组合的锁语义。
5. **动态注册与计划依赖**：Schema、Variable、Table、Named Window、Function、Serde、Context、Dataflow operator 和 Connector 的注册表版本必须进入 Plan dependency digest；Engine/Plan 绑定、部署卸载前置条件和 recompile/upgrade 需要单独测试。
6. **可观测性不是附加项**：每个 state/index/timer/partition/connector 都要能报告数量、容量、命中/未命中、积压、关闭状态和 owner；日志必须做 secret/PII 脱敏，并能用 statement、deployment、context、partition 和 parity case 关联诊断。
7. **资源配额与取消传播**：事件深度、递归 route、窗口/聚合基数、Pattern 状态、FAF 返回行数、JSON/XML/Avro 深度、正则预算、连接器重试和 goroutine/timer 数量都要有有界默认值，并测试 context 取消后的回收。
8. **发布工程**：必须补充 CI 矩阵、生成代码校验、API/Plan/State 兼容检查、可复现构建、依赖漏洞/许可证扫描、SBOM、CHANGELOG、迁移指南和回滚方案；Windows 当前环境能编译不等于 Linux/macOS/arm64 已支持。
9. **Plan/trace 保真度**：Builder 的分支顺序、条件、主键、端口、历史触发类型、配置和依赖都必须进入规范化 Plan/trace；必须用“语义不同但结构相近”的规则验证 canonical/hash 不碰撞，并在差分报告中保留分支和状态转移。此次条件式 Table Merge 复核已发现并修正该类遗漏，后续每个新 builder 都要复用这条门禁。
10. **on-trigger 结果语义**：Table 与 Named Window 的 insert/upsert/update/delete/delete-all/conditional-merge 不仅要改变状态，还必须按 Java 约定形成 new/old stream；无匹配 update/delete 不得伪造结果或误报错误，Named Window 的 matched/not-matched、无 `where` 与恒假匹配、保留策略驱逐、结果批次和 consumer dispatch 也要进入差分场景；Table schema、主键数值 coercion 和 listener 边界同样必须覆盖。
11. **Variant 事件契约**：PREDEFINED 必须只暴露成员共有且类型可兼容的属性，ANY 必须保留实际成员 identity 并支持动态属性；成员注册、Route、insert-into/derived stream/wrapper、late schema、supertype/interface coercion、Named Window/Join/Pattern/Dataflow/rowrecog/subquery/FAF 传播、identity/equality、metadata/getter/cache 与 Variant 结果类型都必须单独建 case。当前 Go 已补齐 boxed/numeric common metadata widening、按成员 Event/underlying 类型分派的 JavaBean/method-backed getter cache、indexed/mapped/nested getter、Pattern/Subquery、LengthWindow old/new、Named Window Unique、ANY dynamic metadata、wrapper/derived stream、wildcard Join、late Map/ObjectArray schema、Plan canonical identity 和 `InsertEventIntoNamedWindow` 单列 concrete member conversion；Java application/type-class metadata、完整 listener/表示矩阵和更宽组合仍未完成，不能将其记为 Variant 全量完成。
12. **InsertInto/Route 事件管道**：必须验证目标 Schema、new-stream-only 语义、无投影时原始 Event identity/underlying 保留、Row 到普通事件格式和 ANY Variant 的转换、Variant 逻辑流与实际成员类型隔离、稳定派发顺序、循环/递归路由上限、取消、监听器错误、部署卸载和 Dataflow 传播；Join、Aggregate、Pattern、Named Window/FAF 等结果路由必须分别定义支持矩阵，不能因普通流路由通过就默认全量支持。当前 Go 已开放 Join/Aggregate/Pattern 的基础结果路由与对应链式入口，补充 Named Window consumer、remove-stream、输出批次排序、循环上限和显式 FAF 结果路由/参数绑定测试；混合 new/old 顺序、完整事件表示/转换、可配置 precedence/loop policy、Named Window/FAF 全矩阵仍未完成。
13. **矩阵完整性与漏域**：`testdata/compat/capability-manifest.json` 当前是首批纵向 capability/case 映射，不是 4,136 个 Java runtime execution、726 个源测试文件的全量处置矩阵；当前已扩展到 35 个 capability，仍不能用这些 `mapped` 状态代表全量完成。必须另行登记 client configuration、Module/PathCache/Stage、Filter Service、完整 view/statistical view、spatial、script/UDF、serde/render、metrics/instrumentation、multithread、transaction/qos、compiler/runtimeconfig、benchmark、example 和剩余 Event/Expr/Infra/EsperIO 入口，并为每项给出 Go test、replacement 或 N 级理由。
14. **清单引用完整性**：每次变更 `capability-manifest.json` 后，必须机器校验所有 `javaRuntimeIds` 都存在于 `java-execution-inventory.jsonl`，`javaSourceFiles` 都存在于固定 Java 源树，`goTests` 都能解析到实际 Go 测试符号，且 capability/case/mapping 无重复或孤立引用。仅 JSON 语法和 `mapped` 状态校验不够；本轮复核已发现并修正一个尾部错误的 runtime ID 和一个过期的 Go 测试名。

### 17.2 当前工作树的事实状态（2026-08-08）

本轮对物理索引验收口径补充：Context inner FAF Join（包括全 inner 的 left-deep chain）、两流 `LeftOuter`/`RightOuter` 以及相邻条件左深 mixed `Inner`/`LeftOuter` chain，在已加载驱动侧能够提供完整 equality/range probe 时，Named Window/Table 目标源必须实际增加 `indexLookups`，并在 candidate lookup 后按当前 context key 过滤；没有可安全证明的 candidate、FullOuter/right-preserving 或非相邻条件链式 Join、unidirectional Join 和 Context FAF subquery 继续使用完整 snapshot fallback，不能用索引计划本身代替执行证据。

本轮新增的物理索引验收口径如下：单源 hash equality/IN、普通内连接及两流 LeftOuter/RightOuter optional-side、Context 单源 Table/Named Window hash equality/range 与 B-tree range 的逻辑 `IndexPlan` 必须分别与实际 `indexLookups` 计数同时验证，不能只断言计划对象；Table 与 Named Window 都要覆盖重复 key、IN 探针顺序与最终插入顺序、prepared named parameter、范围边界、反向操作数、复合 equality-prefix 和变更后的索引状态。Join 还要覆盖 automatic/explicit selection、source filter、复合 equality-prefix、双边 inclusive range、`JoinWhere`、preserved-side unmatched 行、重复 probe key，以及 FullOuter/链式 outer/unidirectional、OR、UDF、Null/Missing、超大 probe 的回退；Context 还要覆盖 All/指定 key/ID selector 和错误 selector 不泄漏结果。对 UDF 等值、Null/Missing range bound 和不安全动态表达式，要同时断言结果正确且索引计数不增长，证明运行时走了安全 snapshot fallback。该切片仍不关闭 Java `InfraNWTableFAFIndex` 的 FullOuter/链式 outer/unidirectional range Join、Context FAF Join/subquery、cost/selectivity、性能阈值或 JVM query-plan hook 差异。

| 项目 | 当前状态 | 结论 |
|---|---|---|
| Java 基线 | 已固定为 release_9.0.0 / `9e1b9f1cc9117fea4bf33ab043762c045d73839c` | 可作为 oracle 输入 |
| Java 源资产清单 | 已生成 `testdata/compat/source-test-manifest.json`：726 个源文件，分类计数已校验；首批 capability/case 处置已登记在 `testdata/compat/capability-manifest.json` | 仍只有首批映射；全量 source-test disposition、many-to-one 覆盖断言和 N 级理由尚未完成 |
| Java 运行时枚举 | 已接入 `tools/java-oracle/ExecutionInventory.java` + `run-probe.ps1`；3,848 个静态候选经 801 个外层 suite 枚举为 4,140 条记录，4,136 条 ok、4 条 ignored、0 条 probe error，唯一 `runtimeId` 已校验 | 运行态 inventory 已建立，但 Java 测试体尚未全量执行，JUnit 入口/源测试 disposition/Go 行为 trace 仍未完成 |
| Java regression-run 基线 | 已执行 78 个 suite、860 个 JUnit 入口；857 个无失败/错误，2 个性能/数据库诊断差异，1 个多线程上下文用例首跑波动；官方 SQL 夹具已在 MySQL 8.0.46 Docker 中加载，隔离重跑 `TestSuiteMultithread` 为 41/41；汇总固化于 `testdata/compat/java-regression-baseline.json` | 仍需将每个 Java execution 与 Go parity case 一一关联；2 个 approved difference 不能计为普通语义 pass，首跑波动必须保留环境/重跑记录 |
| Go 基础垂直切片 | 已有 Value/Schema/Expr/Filter/View/Clock/Plan/Deploy/Listener/Sink；事件模型新增 ObjectArray 与 Variant PREDEFINED/ANY 成员路由、Route 入口、普通流/RecordStream/Join/Aggregate/Pattern/Named Window 链式 InsertInto/RouteTo、new/remove stream 选择、输出排序后的有界路由队列和循环错误 | 仅代表增量实现，不代表阶段退出 |
| Expr/函数 | 已有字段、变量、常量、比较、逻辑、算术、字符串、少量日期/聚合表达式，以及基于窗口历史的基础 `Prev`/`Prior`、`Leaving`、`Cast`、`Exists`、`TypeName`、`InstanceOf`、`ArrayAt`；新增可分析的枚举/集合表达式：元素/索引/size、where/select/arrayOf、any/all/count、first/last、distinct、take/while/reverse、min/max/order、sum/average、except/intersect/union、sequenceEqual、aggregate、groupBy、toMap、most/least frequent；Build 会拒绝缺失 collection、selector/predicate、pair operand，以及 `FirstOf/LastOf` 多谓词声明 | 类型提升、完整 null/三值矩阵、枚举方法的完整 Java 类型/无效规则诊断、BigDecimal/任意集合类型、嵌套/子查询/访问聚合组合、UDF、脚本、正则/Unicode/collation、MathContext 和扩展函数仍未完成 |
| View/Window | 已有 length/time、batch、expression、group、union/intersect、sort/rank、unique/first-unique、time-order、TTL、externally-timed 等基础窗口原型；`Prev`/`Prior` 和 `Leaving` 已接入历史/移除流求值，并有 new/old stream 测试；新增 `Window().Aggregate()` 形式的 size/univariate/weighted-average 组合，以及 `Correlation`/`Correl`、`LinearRegression`/`Linest` 的链式统计表达式，长度窗口淘汰会同步重算 new/old 结果；新增 `UniqueBy`/`FirstUniqueBy` 多键唯一窗口、稳定 key 顺序、Snapshot/历史可见性和嵌套 GroupWindow 递归历史；`RankWindowBy(size, uniqueKeys, sortKeys...)` 已覆盖唯一键替换、满窗口 pass-through、最末事件淘汰、primitive/object/二维数组 key、array-key union/intersection 及 Sort/Rank 同分到达顺序差异 | 完整 view 目录、native derived-view event-type properties、组合窗口的全部边界、rank/sort 其余导航、union/intersection 的跨传播组合、group reclaim、窗口索引复用、批/移除/顺序语义及 Java 对照仍未完成 |
| Join | 已有内/外连接、复合/范围条件、N-way inner/left/right/full outer 基础 tuple 差分、Named Window join、Context-partitioned join 和活动 tuple 差分；新增显式 source-indexed `JoinField`/`JoinEventValue` 及 Join-to-Aggregate 的分组、访问聚合、窗口淘汰和 old/new 重算；FAF Named Window/Table Join 已支持一致快照，Context FAF Join 已支持 key/ID partition selector；对应 builder/运行时测试已加入 | 全量外连接顺序、复合/范围索引计划、outer/multi-way Join-to-Aggregate、与 Join/Context 的完整相关子查询组合和全量顺序语义仍未完成；同一事件类型的自连接已补充当前事件自配对、历史交叉配对和分组隔离测试；基础子查询单独登记为 `query.subquery-basic` |
| Historical/SQL | 新增 `FromHistorical` 外部拉取源、`HistoricalProvider` 请求契约、SQL `database/sql` prepared-query adapter、SQL 字节/数值/null 基础转换；支持独立历史查询、历史源 Join、历史源 FAF 快照和参数透传；历史查询结果现在通过统一表示边界物化到 typed Struct/JSON/ObjectArray/Avro 或 map/XML；新增参数键 LRU/expiry 缓存、缓存事件副本隔离、列名大小写归一化、调用方可插拔的问号占位符重写与 metadata/type 转换 hook、预置 `$n`/`:n`/`@pn` 方言重写器及 literal/comment/quoted-identifier 词法边界、Java `metadatasql` 对应的独立 `MetadataStatement`/`SQLHistoricalMetadataSample` 元数据来源、可选 PreparedStatement/Close（含 metadata statement）、`*sql.Tx` caller-owned queryer、可挂到链式 Query `.To(...)` 的参数化 `SQLSink`（new/old 顺序、取消/关闭、基础 DML/Upsert）、fixture 缓存测试和可选 `ESPER_MYSQL_DSN` 的真实 MySQL Docker 测试；有 fake provider 的触发、Join、参数化 FAF 与 DeployWithParameters 测试；本轮 Java `TestSuiteEPLDatabase` 在 MySQL Docker 下 15 个入口为 13 个语义通过、1 个非法 SQL 诊断文本差异、1 个性能阈值环境差异，不能计为全量 pass | 完整方言 adapter 矩阵、完整 SQL 类型码绑定策略、连接池事务语义、DML/Upsert 的事务/retry/connector 矩阵、SQL 多源 FAF/Context 约束、方法流和完整 Java database trace 仍未完成；EsperIO DB 另有 `connectors/db` 的 DML/Upsert lifecycle sink 与 MySQL round-trip，但 XML/config、异步 executor、连接工厂和完整 Java trace 仍未完成 |
| Aggregate | 已有 count/sum/avg/min/max、first/last/nth/count-distinct/median/stddev、avedev/variance/stddev-pop/weighted-avg/rate（含常量间隔/过滤与虚拟时钟）、`RateByTimestamp`/数量速率、min-by/max-by/window/set/sorted、`first-ever`/`last-ever`/`count-ever`、group-by/having 和 first/last/snapshot/every output 原型；新增 `FilterAggregate`/`CountIf`/`SumIf`/`AvgIf`/`MinIf`/`MaxIf` 的可分析谓词过滤、带可选过滤器的有状态 `Leaving`、`GroupByRollup`、`GroupByCube`、`GroupByGroupingSets`、subtotal Null 字段、`Grouping`/`GroupingID`、增量 old/new 维度状态和重复定义校验，并覆盖 ROLLUP 在 Fire-and-Forget Named Window 快照上的求值；本轮新增可选索引的 `First`/`Last`/`Nth`、`WindowEvents`/`EventValue`、`MinByEver`/`MaxByEver`、多条件 `SortedEvents`、可分析 `LocalGroupBy`，以及投影别名 `ResultField` 驱动的聚合 order-by/having；继续新增可链式 `SortedAccessBy`（重复 key 桶、first/last、lower/floor/ceiling/higher、submap、计数和事件列表）、`PluginAggregate`/`RegisterAggregatePlugin`/`PluginAggregateRef`、`PluginAggregateWithFactory`/`RegisterAggregatePluginFactory`/`PluginAggregateFactoryRef` Go 扩展（按组独立 state、`Enter`/`Leave`/`Value`/`Clear`）、无参数 Event 方法/属性链、`CountMinSketchAdd` 频率与 total 访问、`TableSink` 聚合落表、链式 `AggregateStream.IntoTable`（plain/rollup/cube/grouping-sets 目标快照、subtotal Null、原子 `Table.Replace`、窗口淘汰/整组移除同步、过滤 scalar/access 列、live FAF side effect 拒绝）、live table snapshot join 与 consumer-local filter/window 处理；新增 `OutputAfterEvents`/`OutputAfterTime`/`OutputAfterCalendar` 激活门、事件计数 every、虚拟时钟 `OutputEveryTime`/`OutputSnapshotEvery`、基础五字段 `CronSchedule`/`OutputAt`（含变量/参数字段、严格下一次触发、日月/周 OR），并补充可选秒/毫秒字段；`OutputAt(..., OutputSnapshot())` 在日历触发点读取普通窗口、分组聚合和 Join 的当前快照，`OutputSnapshotEvery(...)` 按虚拟时钟重建分组聚合当前状态；同时保留可组合的 `OutputWhenWith(OutputSnapshot(), ...)` 当前流快照触发和可缓冲的 `OutputWhen`/then 变量提交，支持基础 count_insert/count_remove/total 与 last-output-timestamp 条件，结果查询已有 distinct/order/limit/offset 原型 | 完整聚合族、math-context/decimal、into-table 的 context/属性链生命周期、table/navigable/plugin 全访问方法与属性链、组合/嵌套 grouping specification、Context/Named Window/FAF/output 全矩阵、table sink 批量原子回滚、Count-Min Sketch 的可配置宽深/碰撞策略与更广组合、cron 的微秒精度/特殊日历运算符、动态重排、时区/DST/无效规则矩阵、完整 when-then 上下文与更多条件函数、snapshot-after 的 Context 生命周期、Named Window/Table/custom-access 的高级组合、grouped/discrete delivery、全局结果窗口语义和精确移除语义仍未完成 |
| Variable | 已有全局注册、引用、类型校验、constant、原子快照；新增链式 `OnEvent(...).SetVariable/SetVariables`，支持按声明顺序求值、重复 assignment 最终提交、数字 coercion 和同事件后续 statement 可见；新增 `RegisterContextVariable`/`RegisterVariableInContext`、按 context+partition key 的共享状态、`GetContextVariable`/`SetContextVariable`/`SetContextVariables`/`SetContextVariablesByID`、全局 `VariableValues`、按 selector 返回 `ContextVariableStates`、一致快照、批量校验后一次性提交、`VariableChangeListener` 的 old/new 事件、作用域校验、on-trigger 更新和 initiated 分区释放重置 | 上下文变量的完整 Java iterator、变量版本/延迟可见性、按 deployment/name 的完整管理服务键模型、subquery/array/object assignment、output-then 与 Java 全量 on-set 差分仍未完成 |
| Table/Named Window/FAF | 已有内存 Table/Named Window、基础索引/保留、消费者和 prepared FAF；新增 `FromTable` 表源、Context-aware FAF selector、FAF Named Window/Table Join、类型化 `Parameter[T]`/`Param[T]`、同名参数类型一致性与执行时类型校验、`ExecuteFireAndForgetWithParameters`/`PreparedQuery.ExecuteWithParameters`/`DeployWithParameters`、切片 `InSlice`、`TableField`/`NamedWindowField` 目标行表达式、Table `UpdateTableWhere`/`DeleteFromTableWhere`/`SelectFromTableWhere` 批量触发、Named Window `InsertIntoNamedWindow`/`UpdateNamedWindow`/`SelectFromNamedWindow`/`DeleteFromNamedWindow`/`DeleteAllFromNamedWindow`/`MergeIntoNamedWindow`（含显式 `MergeIntoNamedWindowWhen`），以及 Table/Named Window mutation new/old 结果批次；Named Window 变更已接入 consumer dispatch，并有条件 Merge 的 matched/not-matched/delete、无 where matched、恒假插入、关闭、取消、生命周期、非法规则和 Plan hash 区分测试；新增 `ExecuteFireAndForgetAndRoute*`/`RouteFireAndForget`，显式区分只读 FAF 与路由副作用，覆盖 named-window/table、投影顺序和参数绑定；新增 `InsertRows` 多行 FAF、typed named/positional substitution parameter 和 `SubqueryValue`/`SubqueryGroupRows` 的 FAF scalar/grouped subquery，使用 `OuterField` 覆盖目标行相关 select/update/delete，使用 `OnDemand()` 区分 live on-trigger 与 FAF mutation，并覆盖 Context-local/global lookup、inner where/group-by 及 event-stream/source-filter/context-mismatch invalid；已有 `SubqueryExists`/`SubqueryExistsValue`/`SubqueryIn`/`SubqueryInWithOptions`/`SubqueryCount`/`SubquerySum`/`SubqueryAvg` 与 `SubqueryAny`/`SubquerySome`/`SubqueryAll` 的带 options 形式，支持 Named Window/Table 快照、相关过滤、非分组聚合 `having`、嵌套作用域、参数绑定、三值量词、标量排序/offset/limit 和多行 cardinality 选项；TableColumn 允许声明 nested schema 并把元数据带入 Table Event 物化 | Java `InfraNWTableFAFSubquery` 的 4 个 performance/index execution、复杂表访问/聚合列、谓词批量操作的原子回滚/索引计划、split stream、完整索引/迭代/持久化、Named Window/Table on-trigger 的全量 Java 差分、SQL/历史源的多源约束、FAF 全结果路由矩阵、分组/having/多行多列子查询和完整 Java 差分仍未完成 |
| Compiler/Deployment | 已有链式 AST、Build 校验、Plan、Deploy/DeployWithParameters、Undeploy 和依赖摘要原型；新增显式 `Environment.RegisterModule`/`Module` namespace 与 module-bound source 的 catalog resolution | Module/PathCache/uses/imports、Java protected/public visibility、rollout 原子性、recompile/upgrade、artifact 恢复、完整配置和部署锁策略仍未完成 |
| Pattern / Match Recognize | Pattern 已有单源 tag/filter、followed-by、every/every-distinct、within、`WithinOrMax`、`While` guard、MaxStates、and/or/not、match-until、until、timer-interval/timer-at/显式 timer-schedule，以及基础五字段和可选秒/毫秒精度 `TimerCron`；新增 `RowVar`/`RowSequence`/`RowAlternation`/`RowPermute`、`Optional`/`ZeroOrMore`/`OneOrMore`/`Repeat`（含有限和无限组合重复）、DEFINE、MEASURES、partition、all/first match、三种 skip、固定 duration/日历 `Interval`、`IntervalOrTerminated`、当前窗口 `Prev` DEFINE、typed `PrevTag`/`PriorTag`、`Statement` `Snapshot`、measure-alias `OrderBy`、MaxStates 基础运行时；并有连续序列、可选/重复、交替、有限排列、重复变量、有限和无限组合重复、reluctant、固定/日历 interval、interval-or-terminated、measure alias 排序、当前窗口 Prev、tag-aware previous field、skip 状态快照、长度窗口淘汰、Named Window 乱序删除重算、分区和校验测试；本轮新增 Java `RowRecogDataSet` 的 `A B C* D E* F+` 链式场景，逐事件对照 E7/E8/E9 的 1/3/5 条 listener 输出，并修复 `SKIP TO CURRENT ROW` 对重叠旧起点的裁剪；本轮又新增 Java `RowRecogRegex` 全部 12 组链式组合模式，对照 listener 与 Snapshot 的可选、交替、嵌套重复和重复捕获结果；本轮新增 Java `RowRecogVariantStream` 的 PREDEFINED Variant 成员类型场景，使用 typed `InsertInto` 路由和 concrete member type 谓词；本轮再新增 Java `RowRecogDataSet.RowRecogExampleFinancialWPattern` 的 E1–E60 `A W+ X+ Y+ Z+` 链式数据轨迹，覆盖全部终止 listener batches；本轮新增 Java `RowRecogMultikeyWArray` 的数组内容分区与 int/long 多键分区链式对照，验证 nil/空数组和完整 tuple 隔离；本轮补齐 Java `RowRecogAfter` 的 current/next/past-last 三种 skip 及重复变量、分区场景 listener/Snapshot 矩阵，并修复 `SKIP PAST LAST ROW` 快照保留已发出结果、排除被裁剪新起点的语义；本轮新增 Java `RowRecogInterval` 四个 execution 的 simple/partitioned/multiple-completed/month-scoped 虚拟时间链式对照，覆盖 pending Snapshot、绝对 deadline listener、分区 deadline、多起始匹配和日历月边界；本轮再新增 Java `RowRecogIntervalResolution` 的 A* exact flip 与 WConfig 微秒边界链式对照，使用显式 `time.Duration`/`time.Time` 验证 1ns/1µs before 与 exact deadline 的 listener/Snapshot；Context 新增事件驱动 pattern-init/pattern-term adapter，支持 tag capture、相关 end pattern、Every overlapping、termination snapshot 和纯 TimerAt/TimerInterval/TimerSchedule/TimerCron root Context 生命周期（部署时锚定虚拟时钟） | Pattern cron 微秒/ISO period 与特殊日历运算符、复杂 timer+event Pattern Context observer、完整时区/DST/动态调度矩阵、guard/observer 的完整组合、`WithinOrMax` 在 Until/Not/MatchUntil/多层组合中的传播、消费语义、复杂状态限制；Match Recognize 的高级嵌套 NFA、Variant/ANY/dynamic-member 高级矩阵、advanced tag-aware prev/prior variants 与高级聚合、Java 全局 TimeSource time-unit 配置 API、`IntervalOrTerminated` 更复杂嵌套/动态组合、完整日历终止与剩余高级 composition parity、完整 out-of-sequence delete 矩阵、完整 iterator/order-by 和完整 Java trace 仍未完成 |
| Context | 已有 key/hash/category/initiated-terminated builder、嵌套 segmented context、普通/Named Window/Join/aggregate/trigger 的分区路由、key/ID/category/hash/segmented/nested/descriptor-filter selector、Context-aware FAF aggregate 与 Context FAF Join、生命周期处理，并有 `ContextPartitionKeys`/count 查询；新增 context-partitioned rollup 聚合隔离和 FAF partition selector 测试，以及 `ContextPartitions` descriptor 快照（ID、key、context properties）、共享分区 ID、allocation/deallocation listener、Environment context 的 created/destroyed 管理、statement added/removed、activated/deactivated 通知、key/category/initiated 生命周期、initiated/terminated boundary-event `Property` 链、partition-scoped context variable 注册/读写、全局与分区变量一致快照和原子批量更新；本轮新增 `NewPatternInitiatedTerminatedContext`/`NewPatternInitiatedContext` 及显式 overlapping 变体，复用 PatternStream matcher 支持 start tag 保留、end pattern 关联、`ContextPatternEvent`/`ContextPatternField`、`Every` 并发分区、pattern termination snapshot 和纯 TimerAt/TimerInterval/TimerSchedule/TimerCron root Context 生命周期（部署时锚定虚拟时钟）和 temporal termination snapshot | selector 的完整 Esper 类型校验、Java hash/segmented/nested identifier 细节与非法组合诊断、复杂 timer+event Pattern Context、initiated parent/pattern child nested Context、ContextDeploymentID/RuntimeURI 与完整 Java 事件 payload 对等、终止/启动事件对象矩阵、变量 iterator/listener 与完整管理服务契约、IntoTable context materialization、跨 Context 事务和多租户仍未完成 |
| Dataflow | 已有保存定义、Beacon/Filter/Select/Emitter、EventBus source/sink (Event/underlying)、EPStatement source subscription、new/old batch collector、显式 `Connect/ConnectPorts` 分支图、拓扑校验、实例状态、取消和统计；新增 `FinalMarker`/`WindowMarker`/`CustomSignal`、`OnSignal` handler、图内传播、Emitter 观察、运行态 `SubmitSignal`、captive `Emitter` handle/`Submit` 事件注入、Dataflow Filter/Select 的独立 source-window subquery 状态，以及每个实例独立的 `Custom` factory/runtime、可选 Open/Close/OnSignal 生命周期、named emission port routing、实例 ID/user object、in-process saved configuration、结构化 exception handler、fail/continue policy 和 error/drop 统计，并有对应测试 | typed event/row 多端口约束、自定义 source 和完整 factory 参数/错误策略、并发背压、跨进程持久化 saved instance/configuration、完整 join/多端口 captive 组合和 Java trace parity 仍未完成 |
| EsperIO | 已实现 `connectors` 生命周期、CSV/File source/sink、unformatted line source、DB DML/Upsert sink、HTTP client/server source/sink、Socket TCP source、四种 Socket 协议、Kafka Reader/Writer、Kafka JSON processor/commit/retry/timestamp/key/header、AMQP RabbitMQ Consumer/Publisher、queue/exchange/binding、prefetch、JSON/GOB codec、auto/manual ack、reject/requeue、JMS provider-neutral Consumer/Publisher、Map/Text/Object/Bytes codec、Java event-type property、ack-after-process、重试、类型转换、loop/reset、定时回放、Esper Event/Row bridge、HTTP URI/query/JSON/retry/Engine bridge、Socket 并发/暂停/重启/object decoder；Java `esperio-csv` Maven 80/80 通过，`esperio-db` 4 个测试中 Upsert 通过、2 个环境/断言差异已记录，`esperio-http` 4 个测试中 3 个通过、1 个旧订阅/连接环境差异，`esperio-socket` 7/7 通过，`esperio-kafka` 在固定单节点 broker/topic 环境下 6/6 通过，Spring JMS 4 个测试中 1/4 通过且环境差异已记录，Go Kafka/RabbitMQ/JMS bridge round-trip/单测通过，映射已登记 `integration.esperio`/`case.esperio-csv-file`/`case.esperio-db`/`case.esperio-http`/`case.esperio-socket`/`case.esperio-springjms`/`case.esperio-amqp`/`case.esperio-kafka` | `partial`；DB XML/config、异步 executor/connection factory、HTTP XML/classic service/完整异步 delivery、Kafka group/rebalance/custom serializer/plugin/高级恢复、AMQP Java serialization/高级重连/Dataflow graph、JVM JMS provider/session/transaction/Spring XML、Socket XML/plugin/writable-property cache、Dataflow operator/signal-marker、AdapterCoordinator、bean population 和全量 connector delivery/trace 仍未完成 |
| Variant | 已有 `SchemaVariant`、`NewVariantSchema`/`NewAnyVariantSchema`、`RegisterVariant`/`RegisterVariantAny`、`Engine.Route`，PREDEFINED 共有字段/成员校验与 ANY 动态字段；普通源、Join、Pattern、Named Window consumer、Dataflow EventBus source 的成员路由和 Route identity 已有 Go 测试；新增 `Stream.InsertInto`/`RecordStream.InsertInto`、Join/Aggregate/Pattern 链式结果入口、Named Window consumer、`RouteTo`、Plan 目标校验、有界嵌套路由，以及 Event/Row 到普通 Schema 或 ANY Variant 的基础投影转换；本轮补充 boxed/numeric common metadata widening、成员 Event/underlying 分派的 JavaBean/method-backed getter cache、indexed/mapped/nested getter、wrapper/derived stream、Pattern/Subquery、LengthWindow old/new、Named Window Unique replacement、wildcard Join、ANY dynamic metadata、late Map/ObjectArray schema 和 Plan canonical identity | 单列 method-backed member conversion、Java application/type-class metadata、完整表示转换/FAF/Dataflow/Serde 矩阵、混合 new/old 顺序、完整 fragment/identity/equality、Named Window Variant source 与 Java/Go trace parity 仍未完成 |
| JSON/XML/Avro/Serde | 已有基础 JSON、严格基础 XML scalar/path 解析、Avro JSON datum 解析，以及 `SchemaObjectArray`/`RegisterObjectArray`/`SendObjectArray` 的位置字段访问、长度/类型校验、数值 coercion；本片新增 `array[index]`、`mapped('key')`、转义点路径、嵌套 `Property`、JavaBean getter fallback，以及带标题/缩进/深度上限和 XML attribute 选项的 `RenderJSON`/`RenderXML`，覆盖 schema/map/struct/Row/Event/嵌套数组的 Go 单测；随后补充 schema-directed JSON 对 nested struct/map/slice/array、enum-like string、time.Time、big.Int/big.Rat 的递归转换、大小/尾随数据/未知字段选项和精确数值渲染，并对照 Java `EventInfraGetterIndexed`/`EventInfraGetterMapped`/动态 getter、`EventRenderJSON`/`EventRenderXML`、`EventJsonTypingCoreParse`/`CoreWrite` runtime inventory 登记 case | ObjectArray 的命名嵌套事件类型/属性元数据、完整 EXPLICIT accessor registration、继承和 configuration、JSON provided-underlying/class/list adapter 与全量 laxness/动态值矩阵、XSD/XPath/namespace/DOM、二进制 Avro/logical type/schema evolution、provided-underlying/fragment 和 Java 精确格式 trace、Serde 和格式版本矩阵仍未完成；当前新增 case 仍是 Go 单测映射，不是 Java/Go trace parity |
| 全量差分与覆盖率 | Go 单测、vet、race、首批中立场景和结构化 `DiffTraces` 差异报告可运行；本轮新增 ROLLUP/CUBE/GROUPING SETS、subtotal Null/`grouping_id`、filtered aggregate、ever aggregate、stateful leaving、常量/时间戳/数量 rate、indexed/access aggregate、local group-by、聚合 order-by/having、`SortedAccessBy`、`AggregateStream.IntoTable` 快照替换/删除、维度 subtotal IntoTable、过滤 access 列物化、plugin、无参数 Method、Count-Min Sketch、derived view 的 `Correlation`/`LinearRegression`/`UnivariateStatistics`、multi-key `UniqueBy`/`FirstUniqueBy` 与唯一窗口 Snapshot/历史、primitive/object/二维 array-key、Rank/Sort 同分顺序和 TimeOrder 虚拟时钟到期、Dataflow signal/marker、Dataflow instance options/saved configuration/exception policy、子查询 count/sum/avg、any/all/some 量词和标量 order/offset/limit/cardinality、事件 indexed/mapped/nested getter、JSON/XML renderer、typed JSON parse/write、XML tree/attribute/repeated element parse、事件 property metadata/inheritance、Context created/destroyed replay、statement added/removed、activated/deactivated、descriptor/boundary-event/context variable management snapshot/atomic-batch/key-ID-category/hash/segmented/nested/descriptor-filter selector/shared lifecycle listener、EsperIO CSV/File/DB/HTTP/Socket/Kafka/AMQP/JMS bridge source/sink/lifecycle/replay/retry/Engine bridge 和非法定义测试，并定向执行 Java `TestSuiteResultSetQueryType` 17/17、`TestSuiteResultSetOutputLimit` 14/14、`TestSuiteResultSetAggregate` 14/14、`TestSuiteEPLSubselect` 19/19、`TestSuiteView` 27/27（含 ViewRank/ViewSort/ViewTimeOrder/ViewMultikeyWArray 与 `ViewDerived` 12 个 runtime execution）、`TestSuiteContext` 17/17、选定 EventInfra/EventRender/EventJson/EventXML 86/86、`esperio-csv` 80/80、`esperio-http` 3/4、`esperio-socket` 7/7、`esperio-kafka` 6/6；EsperIO AMQP 已完成 Java 编译/外部 broker 诊断，Spring JMS 已完成 Java 4-test 环境诊断，Go RabbitMQ/JMS bridge source/sink round-trip/单测通过；保留基础子查询相关/非相关/嵌套/参数化/FAF 一致快照、显式 FAF 结果路由/参数绑定、历史 SQL、Variant/InsertInto、Join/Aggregate/Pattern/Named Window、new/remove stream、CronSchedule、ExprEnum、output-after/when-then、OutputAt snapshot、Pattern TimerCron 和基础 Match Recognize 测试；本轮已通过 `go test ./...`、`go test -race ./...`、`go vet ./...`、compat 门禁、格式/JSON/diff 检查、MySQL Docker 集成测试和 Kafka/RabbitMQ Docker round-trip；最近一次 `go test ./... -cover` 核心包覆盖率 75.6%，compat 66.9%，connector 各包覆盖率以 coverprofile 为准 | Java/Go 全量 parity、source-test 100%、发布覆盖率门禁均未达成；当前覆盖率不是最终阈值通过 |

本轮又对照 Java `EventInfraGetterIndexed`、`EventInfraGetterMapped`、`EventInfraGetterDynamicIndexed`、`EventInfraGetterDynamicMapped` 以及 `EventRenderJSON`/`EventRenderXML` 的 runtime inventory，补充 Go 事件属性路径解析：`array[index]`、`mapped('key')`、可选 `?`、转义点和嵌套路径现在统一进入 `Event.Get` 与 `Property`；`Schema`/`Event` 同时提供 root/path property names、声明类型和 indexed/mapped/dynamic descriptor，struct schema 在缺少直接字段时支持安全的零参数 JavaBean 风格 getter fallback。新增 `RenderJSON`/`RenderXML` 及 functional options，支持标题、缩进、XML scalar attribute、嵌套 map/struct/array/Event/Row、Missing/Null 保留和最大深度拒绝，并建立正向、转义、空值、嵌套、属性元数据、属性输出和深度边界测试。固定 Java 17/Maven 3.9.16 环境下选定的 `TestSuiteEventInfra`、`TestSuiteEventRender`、`TestSuiteEventJson`、`TestSuiteEventXML` 共 86/86 通过；这只是 Java oracle 可复现证据，不代表 Go 与 Java trace 相等。该切片已登记为 `event.property-access-render` / `case.event-property-access-render`，仍不覆盖 Java 的 EXPLICIT accessor、fragment/metadata、DOM/XPath、XSD、完整 Avro/JSON provided-underlying 或双方共享 trace。

随后继续对照 Java `EventJsonTypingCoreParse`、`EventJsonTypingCoreWrite`、`EventJsonTypingClassParseWrite` 和 `EventJsonParserLaxness`，将 Go `ParseJSONWithOptions` 扩展为 schema-directed 递归转换：支持嵌套 struct/map/slice/array、string alias、`time.Time`、`big.Int`、`big.Rat`，保留 null pointer，拒绝尾随 JSON，并提供最大深度和 undeclared-field 选项；`RenderJSON` 对 arbitrary-precision 数字保持 JSON number 形态。Java `TestSuiteEventJson` 本次 13/13 通过，Go 新增 typed conversion、精度、深度、未知字段和 render 测试已登记为 `event.json-typed` / `case.event-json-typed`；provided-underlying/class/list adapter、完整 laxness、schema inheritance 和共享 parse/write trace 仍未完成。

随后又将 `ParseXML` 从标量扫描扩展为受深度限制的内存树：属性以 `@name` 暴露、重复子元素形成 `[]any`、嵌套 indexed path 可直接由 `Event.Get`/`Property` 读取，并保持无路径字段的 leaf fallback；`ParseXMLWithOptions` 对非法 XML、无根文档和超深输入返回显式错误。Java `TestSuiteEventXML` 本次 33/33 通过，Go 新增属性/重复元素/index path/深度测试登记为 `event.xml-tree` / `case.event-xml-tree`；XSD/XPath/namespace/DOM/fragment/transpose 和完整安全资源配额仍未完成。

本轮还增加显式 `WithSchemaParent`/`NewSchemaWithOptions`：父字段在子字段之前合并，动态属性策略继承，ObjectArray 位置顺序保持，`Schema.ParentNames`/property metadata 可查询，且父关系进入 Plan canonical hash。该切片关联 Java Map/ObjectArray/JSON inheritance runtime entries，登记为 `event.inheritance` / `case.event-inheritance`；多级/分支/跨模块可见性、supertype/interface coercion、provided-underlying/Avro evolution、fragment/route/query 传播和双方 trace 仍未完成。

随后对照 Java `ViewDerived` 的 12 个 runtime execution（并执行 `TestSuiteView` 27/27），补齐 derived/statistical view 中此前遗漏的线性回归和相关系数：Go 通过 `Window().Aggregate()` 组合 `CountAll`、sum/count/avg/variance/stddev/weighted average，并提供 `Correlation`/`Correl` 与 `LinearRegression`/`Linest(...).Slope()`、`YIntercept()`；长度窗口淘汰时 new/old 统计行都会重算，Go 对 Java 回归中的四组数值采用明确浮点容差。该切片登记为 `view.derived-statistical` / `case.view-derived-statistical`；Esper 原生 derived-view 输出事件类型属性、group/union/intersect/time/batch/iterator/reclaim 组合及 exact NaN/decimal/trace policy 仍未完成。

本轮进一步复核 Java `ViewMultikeyWArray`，修复 Go `Unique`/`FirstUnique` 只写入 keyed map、却没有进入 Snapshot/Prev/Prior 历史的问题：`UniqueBy`/`FirstUniqueBy` 现在支持多个 analyzable key，key identity 保留 Value state/type/content，稳定保存首次 key 顺序，并在 `GroupWindow(..., Unique(...))` 中递归生成分区历史。新增唯一窗口快照、嵌套 group、new/old replacement、多键 first-unique、primitive/object/二维 array-key、array-key GroupWindow 和 array-key union/intersection 测试；同时对照 Java `ViewTimeOrderAndTimeToLive`，补齐 TimeOrder 的外部时间排序、虚拟时钟到期和迟到边界事件的即时 old-stream。继续对照 Java `ViewRank`/`ViewMultiKeyRank`，新增 `RankWindowBy(size, uniqueKeys, sortKeys...)`，实现唯一键替换、满窗口时的 pass-through new+old、按排序键淘汰最末事件和快照顺序，并修正 Sort/Rank 同分时各自不同的到达顺序/淘汰策略；随后补齐 `ViewRankedPrev` 的 `PrevWindow`/`Prev(0..4)` 访问，以及 `ViewRankPrevAndGroupWin` 的 GroupWindow 分区唯一键替换和淘汰生命周期；本轮再补齐 `NamedWindowRetention(Unique(...))`，使 named window 支持 content-based array-key replacement、old-stream dispatch 和稳定 Snapshot，对照 `ViewMultiKeyLastUniqueArrayKeyNamedWindow` 的 E0–E16 序列；随后补齐 `MergeIntoNamedWindowWhen` 在 `Unique`/`FirstUnique` retention 下的 not-matched 插入、unique collision old/new replacement 和 duplicate suppression，新增 `TestNamedWindowArrayUniqueRetentionSupportsMerge`、`TestNamedWindowArrayUniqueMergeCollisionDispatch`、`TestNamedWindowFirstUniqueMergeIgnoresDuplicate`，登记为 `case.named-window-unique-merge`；随后补齐 named-window-backed 与裸 event-stream array-key 子查询的独立 source-window 状态、聚合投影和 count/filter：`SubqueryValue(..., WindowValues(...))` 在 retained group 上只生成一行，普通 Statement 为 raw stream subquery 持续维护 `Unique` 状态，新增 `TestNamedWindowArrayUniqueSubqueryWindowMatchesEsper`、`TestNamedWindowArrayUniqueSubqueryCountFilterMatchesEsper`、`TestEventStreamUniqueSubqueryWindowMatchesEsper` 与 `TestEventStreamUniqueSubqueryCountFilterMatchesEsper`，登记到 `case.view-window-core`/`case.subquery-basic`；随后继续对照 Java `ViewMultiKeyLastUniqueArrayKeyDataflow`，使 Dataflow instance 为每个 Select 中的 event-stream subquery 持有独立 source-window state，并通过 captive Emitter 的 `Submit` 传播事件，`TestDataflowArrayKeySubqueryWindowMatchesEsper` 覆盖 E1–E5 的 array-key Unique 替换与输出顺序；Dataflow 的其它 operator/captive graph 组合、context/pattern subquery、更多导航和完整 Java trace 仍未完成。

本轮继续对照 Java `ViewTimeOrderAndTimeToLive` 的 `#timetolive(timestamp)`：新增链式 `TimeToLiveAt(expr)`，把事件的 integral epoch-millisecond 字段解释为绝对到期时间，而不是把字段值当作插入后的相对时长；虚拟时钟在边界一次淘汰同时间事件，已过期事件在插入时产生同批 new/old，named window retention 也保留该语义。`TestTimeToLiveAtWindowUsesAbsoluteEventExpiry`、非法类型测试和 `TestNamedWindowTimeToLiveAtUsesAbsoluteEventExpiry` 固定 Java 用例的 E1–E6 生命周期。

随后复核 Java `ViewTimeOrderTTLPreviousAndPriorSceneOne/Two`：Go 新增 `PrevTail`、`PrevCount`、`PrevWindow` 链式表达式，并为 TimeOrder/Sort/Rank 维护独立的 view-access 与 arrival-order 元数据；`Prev`/tail/window 按排序头到尾访问，`Prior` 按到达顺序访问，过期 old-stream 只保留 Java 可见的 prior 值，Snapshot 也重建同一导航上下文。`TestPreviousViewNavigationUsesWindowAndArrivalOrders` 与 `TestTimeOrderPreviousNavigationMatchesExpiryLifecycle` 覆盖普通 length、TimeOrder 的 E1/E2/E3 乱序插入、E2/E4/E3 分阶段过期、old-stream Null/保留 prior 以及 Snapshot；`TestTimeOrderPreviousAccessMatchesEsper`、`TestSortedPreviousAccessUsesPostEvictionViewForNewRows` 和 `TestGroupedSortedPreviousAccessUsesPartitionView` 进一步固定 Java 的 Sort/TimeOrder 头尾顺序、即时淘汰新流仍使用 post-eviction 视图、new/old 同事件上下文隔离和 GroupWindow 分区历史。随后补齐 Java `ViewTimeOrderTTLMonthScoped` 的 calendar-month 语义：链式 `TimeOrderCalendar(timestamp, years, months, days)` 以 `time.Time.AddDate` 计算到期点，覆盖月边界前一毫秒保留、边界精确淘汰和非法周期；`TestGroupedTimeOrderWindowMatchesExpiryLifecycle` 对照 `ViewTimeOrderTTLGroupedWindow` 固定分区独立淘汰、迟到事件和 Snapshot 顺序。subquery/named-window/dataflow 传播和共享 Java/Go trace 仍待继续移植。

随后对照 Java `ViewTimeOrderTTLTimeOrderRemoveStream`，补上 `TimeOrder(...).Select(...).InsertInto(..., WithRemoveStreamOnly())` 的 Go 链路：窗口淘汰的 old-stream 结果经注册的 `OrderedStream` 投影后作为下游 new-stream 事件消费。`TestTimeOrderRemoveStreamRouteMatchesEsper` 逐时刻覆盖 E1–E9 的乱序时间戳、同时间戳稳定顺序、已过期事件即时移除和 31/31.3/32/37/38 秒边界；视图的更广表示、insert-into 组合与共享 Java/Go trace 仍待继续。

本轮又补齐 Java `ContextKeySegmentedSubqueryFiltered` 的基础 event-stream subquery：Context 每个分区持有独立的 `LastEvent` 子查询状态，内层事件只广播到已存在分区，新分区不回放创建前的内层事件；`TestContextEventStreamSubqueryKeepsPartitionLocalLastEvent` 固定 G1/G2/G3 分区的 null、更新和复用序列。Named Window index-sharing 的完整矩阵、Pattern Context/Dataflow 子查询及完整 context/subquery trace 仍保持部分对等；后续已补充 `InfraNWTableSubqCorrelIndex` 的 equality/hash shared-index 子集。

随后对照 Java `ContextStartEndSubselect` 与 `ContextStartEndSubselectCorrelated`，补充日历 Context 的 event-stream subquery 生命周期：09:00 打开分区时不回放非活动期的 `LastEvent`，17:00 关闭时释放状态，次日 09:00 新分区重新从 Null 开始，活动期内仍支持按 outer field 关联内层事件。`TestTemporalContextEventStreamSubqueryResetsAtCalendarBoundary` 覆盖首日/跨日边界和 inactive 期间事件；Pattern/initiated-context、Named Window shared-index 的剩余形状、复杂 temporal 组合和完整 Java/Go trace 仍未完成。

本轮再覆盖事件驱动 initiated Context 的 event-stream subquery 生命周期：`TestInitiatedContextEventStreamSubqueryReleasesPartitionState` 验证 start 创建分区、active 期间内层 `LastEvent` 可被 outer field 关联、end 释放状态，以及同一 key 再次 start 不回放两次 start 之间的内层事件。该测试对应 Java `ContextInitTermSubqueryInFilter` 的上下文子查询方向，但 Java 原用例还包含同 Context Named Window 与 `not exists` 过滤，不能把本轮 event-stream 证据解释为 Named Window/filter parity。

随后补上 Java `ContextKeyedSubqueryNamedWindowIndexUnShared`/`IndexShared` 的值语义切片：`TestContextNamedWindowSubqueryUsesGlobalState` 用全局 `NamedWindow` 作为子查询源，验证 G1/G2 Context 分区都读取当前全局快照、不同 inner key 的结果不会错误串联；当前已补充 `NamedWindowSubqueryIndexSharing` 的 equality/hash 自动共享、显式 index、consumer disable/no-index 和 Context partition inheritance，但 Context-scoped Named Window 创建/retention、B-tree/multikey shared index 和同 Context `not exists` filter 的完整矩阵仍开放。

### 17.3 后续每次提交的强制核对项

每实现一个 capability，提交必须同时更新四类证据：

- Go Builder/AST/Runtime 的位置和能力 ID；
- Java 来源入口、适用 Regression/Unit/EsperIO/Example case 及当前映射状态；
- 正向、无效、边界、取消/关闭、并发或资源配额测试（按适用性）；
- Plan/State/trace 规范化输出、已知差异、性能/内存数据和安全影响。

能力映射统一写入 `testdata/compat/capability-manifest.json`：`status=mapped` 只表示 Java 来源与 Go 测试已经关联，不表示 Java/Go trace 已相等；只有存在双方 trace 且差异处置完成时，才允许改为 `passing` 或 `approved-difference`。

在 Java oracle 恢复前，允许继续编写 Go 端单元和中立场景，但不允许把这些结果写成“Java 对照通过”；在许可证材料、完整 manifest 和全量差分恢复前，也不允许宣称“完整移植完成”。

### 17.4 Java 环境恢复后的实测补充

#### 本轮 Merge wildcard 与 insert-select action-chain 复核

本轮把前文仍写作“wildcard/multi-action projection”的已落地子集拆成可追踪 case，避免把一个粗粒度的 `trigger.table-named-window` 状态误读为完整 on-merge parity：

| Java 对照 | Go 链式 API/测试 | 当前结论 |
|---|---|---|
| `InfraOnMergeMatchNoMatch`（`java-runtime-240cb61eabb22eb2a215`、`java-runtime-aaa45c95e95a919bd0c0`） | `CopyMatchingFields()`；`TestTriggerMergeWildcardCopiesMatchingFields` | `mapped`：按目标 schema 的同名字段复制；source-only 字段忽略；not-matched 时 target-only 字段显式 materialize 为 Null；matched 时已有 target-only 值不被清空；Table/Named Window 均覆盖 |
| `InfraOnMergeInsertStream`（`java-runtime-2c687a68317caea3c148`、`java-runtime-081455731ccafbf6847c`） | `ThenInsertInto`、`ThenInsertIntoWhen`、`ThenInsertIntoTarget`、`WhenNotMatchedActions`；`TestTriggerMergeInsertStreamActionsMatchInfraOnMergeInsertStream` | `mapped`：有序 side-stream projection、条件 side-stream、最终 Table/Named Window insert 和目标快照均覆盖 |
| `InfraMultipleInsert`（`java-runtime-2c4851ea026cc835d39b`、`java-runtime-f075aa28d64b1ae78d2e`） | `WhenNotMatched` 分支；`TestTriggerMultipleInsertBranchesMatchInfraMultipleInsert` | `mapped`：四个按序 not-matched 分支的 source/constant/computed projection、无分支 no-op、重复匹配 key 抑制及 Table/Named Window 快照均覆盖；表示转换、事务/回滚和精确 Java 诊断仍为 `partial` |
| `InfraFlow`（`java-runtime-403bba8c6b29e32b1a8f`、`java-runtime-ae05c015767242106de7`） | 过滤 `InsertIntoTable`/`InsertIntoNamedWindow`、`DeleteAll`、有序 matched/not-matched merge、plan redeploy、`CopyMatchingFields()`；`TestTriggerInfraFlowLifecycleAndRedeploy` | `mapped`：E1-E3 的 insert/update/reset/delete old/new 生命周期、清空后 replay、Table/Named Window 和 E99 wildcard insert 均覆盖；EPL/SODA 差异、歧义列编译、模块依赖、表示转换、事务/持久化和完整 Java trace 仍为 `partial` |
| `InfraInsertOtherStream` 的 12 个 Table/Named Window × representation execution（`java-runtime-c91b3df37512cfac454f`、`java-runtime-b54b14998ff59e65f2fc`、`java-runtime-945ba0790a1cfa2b1033`、`java-runtime-fef4b3a46f9b7da65a72`、`java-runtime-d8cb24fc9e29421236e2`、`java-runtime-e22957dc02c3ac4b8bb7`、`java-runtime-284497e8a9729bb76ac2`、`java-runtime-f6718d2a96a3e3668239`、`java-runtime-51773f11823a58156447`、`java-runtime-b891f1f54423ccb8a628`、`java-runtime-dd95faed8e31c95bec9f`、`java-runtime-6e68e972f23cef8dde85`） | matched `ThenInsertInto` + `ThenUpdate`；`TestTriggerMatchedMergeInsertStreamReadsTargetRow`、`TestTriggerMatchedMergeInsertStreamReadsTargetRowTypedSource`、`TestTriggerMergeInsertOtherStreamRepresentationMatrix`、`TestTriggerInfraInsertOtherStreamBootstrapMatchesInfraInsertOtherStream` | `mapped`：side-stream projection 可读取当前 target row；side-stream-only 不产生伪造目标 old/new，后续 update 才提交目标变化；表示矩阵覆盖 ObjectArray/Map/Avro/JSON/JSONCLASSPROVIDED/DEFAULT × Table/Named Window，bootstrap 子集覆盖 struct/Map/ObjectArray 与 deployment-order-sensitive route；XML、跨表示 shared trace、完整 insert/delete flow 仍为 `partial` |

上表原有五个 case 加上本轮新增的 `case.trigger-merge-insert-other-stream-bootstrap` 已写入 `testdata/compat/capability-manifest.json`，并保留 `status=mapped` 的含义：它只表示 Java 来源、Go 测试和能力入口已关联；尚未有共享 Java/Go 中立 trace，因此不能改成 `passing`。本轮新增的 `plan.go` 遍历也必须收集 merge insert projection 的表达式依赖，避免参数依赖或 canonical/hash 漏失。

新增 bootstrap case 的 Java/Go 对照边界如下：Table 先部署 insert route 再部署 merge，首条同 key 事件的 side-stream status 读取已落表的 10；Named Window 先部署 merge 再部署 insert route，首条 status 为 0，后续同 key 事件依次读取 10、11。Go runtime statement 消费顺序已改为 deployment order，名称只作缺失顺序信息时的稳定兜底；该顺序是可观察的 target-row 生命周期契约。Go 本轮实际验证 struct、Map、ObjectArray，并在 Named Window snapshot 中检查 underlying representation；Java Avro、JSON、JSONCLASSPROVIDED 以及其它 representation conversion 仍未关闭。

API 语义决策：Go 不提供 `select "*"` 字符串入口，而使用显式的 `CopyMatchingFields()`；它是 schema-bound、可分析且能在 Build/Plan 阶段保留语义的 wildcard。side-stream insert 与当前 merge target 使用不同构造器，防止把 event type route 和 Table/Named Window mutation 混成一个不透明动作。not-matched 条件和 assignment 不能读取目标行；未知目标事件类型、重复 projection alias、目标不存在的列和非法 action shape 在 Build 阶段拒绝。

仍未关闭的遗漏必须单独建 case，而不能复用上面测试的名字：Java `InfraFlow` 除本节已覆盖的过滤 insert、DeleteAll、merge 生命周期、plan redeploy 和 wildcard insert 子集之外的 EPL/SODA 语义、歧义列诊断、模块依赖、表示转换、事务/持久化、split-stream 与完整 Java trace，`InfraMultipleInsert` 除本节四个 not-matched 分支之外的表示转换、rollback/transaction、split-stream 与完整 Java trace，以及 `InfraInsertOtherStream` 的 XML、InnerType/Variable、invalid/rollback、事务/持久化、索引计划、跨表示 shared trace 和精确 Java 诊断仍属于 `partial`。其中 `InfraFlow` 与 `InfraOnMergeInsertStream` 都涉及多 action，但前者还包含既有 `insert into`、delete、重部署和 full lifecycle，不能仅凭 action-chain 测试宣称覆盖。

验证记录：

- Java：`mvn -pl regression-run -Dtest=TestSuiteInfraNWTable -DfailIfNoTests=false -Dgpg.skip=true test`；PowerShell 调用 `mvn.cmd` 时将每个 `-D...` 参数整体加引号，JDK 17.0.20、Maven 3.9.16，`26/26` 通过。
- Go：定向 `go test -run '^TestTriggerMultipleInsertBranchesMatchInfraMultipleInsert$' .`、全量 `go test ./...`、`go vet ./...`、`go test ./internal/compat` 和 `go test -race -run 'TestTrigger|TestSubquery' .` 均通过；本轮已完成 trigger/subquery 竞态分片和 manifest 校验，完整 `go test -race ./...` 仍需按 CI 分片资源单独执行。

- 外部依赖：本轮没有新增 MySQL 语义；DB/SQL/EsperIO 测试才按测试标签启用已有 `esper-java-mysql`（MySQL 8.0，`127.0.0.1:3306`），不把“容器可用”当作 core parity 证据。

本机已发现 Java 17（`C:\Program Files\Microsoft\jdk-17.0.20.8-hotspot`），Maven 3.9.16 可执行文件位于 `C:\Users\baicai\AppData\Local\UniGetUI\Chocolatey\lib\maven\apache-maven-3.9.16\bin\mvn.cmd`。Chocolatey 因当前会话非管理员无法完成最后的环境变量写入，但不影响显式设置 `JAVA_HOME`/Maven 路径运行构建。随后在官方 `common/etc/regression/create_testdb.sql` 夹具和 MySQL 8.0.46 Docker 环境下，按 UTF-8、UTC、Java 17、Maven 3.9.16 执行 `regression-run` 全量基线：78 个 suite、860 个 JUnit 入口，857 个无失败/错误；`testEPLDatabaseJoin` 只出现 MySQL 诊断文本差异，`testEPLDatabaseJoinPerfNoCache` 触发容器/JDBC 性能阈值，均已记录为 approved difference；`TestSuiteMultithread` 首轮另有 `testMultithreadContextCountSimple` 波动，隔离重跑 41/41 通过，不能把它归入 Esper 语义失败。首轮计数和 disposition 固化在 `testdata/compat/java-regression-baseline.json`；Surefire 目录保留最近的全量分片/隔离报告，重现首轮结果以 JSON 基线中的命令和差异记录为准。

外部 connector 验证使用独立 Docker 实例：`esper-java-mysql`（MySQL 8.0，`127.0.0.1:3306`，数据库 `test`）、`esper-go-kafka-test`（Kafka 3.8.1 KRaft，`127.0.0.1:9092`）和 `esper-go-rabbitmq`（RabbitMQ 3 management，`127.0.0.1:5672`）。Go 集成测试分别通过 `ESPER_MYSQL_DSN`、`ESPER_KAFKA_BROKERS` 和 `ESPER_AMQP_URL` 显式启用；这些容器只用于可复现的 connector/对照测试，不替代 Java provider、事务、重连和完整 trace 门禁。

第一轮无数据库环境的 `common` 模块曾是 585 个测试、583 个通过、2 个环境错误；在 MySQL 8.0.46 Docker 和官方夹具下复跑后，`common` 为 585/585 通过，错误来自 `TestDatabaseDMConnFactory` 和 `TestDatabaseDSConnFactory` 的环境缺口已消除。随后 `common-avro` 运行 13 个测试、全部通过，`common-xmlxsd` 运行 10 个测试、全部通过；`compiler` 运行 69 个测试、全部通过，`runtime` 运行 50 个测试、全部通过。`regression-run` 已完成 8 个核心 reactor 模块的跳过测试 install；`TestSuitePattern` 实际运行 28 个测试且全部通过；带数据库的 `TestSuiteContext` 17/17 通过。阶段 0 仍需要把 Java 测试结果固定分为 `pass`、`fail`、`error-environment`、`skip`、`manual`、`N` 六类，并为性能阈值、厂商诊断文本和首跑并发波动留下明确 disposition。

本次分片还确认了两个容易遗漏的构建前置条件：Esper 的 `install` 生命周期默认触发 GPG 签名，本地测试安装必须显式使用 `-Dgpg.skip=true`，而发布构建仍必须保留签名门禁；源码未固定编码时 Maven 会按本机 GBK 编译并产生不可映射字符警告，因此 Java baseline 和 Go CI 都必须固定 UTF-8、locale、timezone 与 charset，不能依赖开发机默认值。

本轮又定向执行了 Java `regression-run` 的 `TestSuiteEPLJoin,TestSuiteEPLSubselect`：61 个 JUnit 入口全部通过。Join 入口实际覆盖了多流、外连接、范围/复合条件、数组多键、无条件/单向、coercion 和 query-plan 变体；Subselect 入口覆盖了相关/非相关、exists/not-exists、聚合、prior、Named Window、pattern、UDF、多子查询和无效规则。Go 当前的基础 Join/FAF 快照能力与显式 FAF 路由已经有中立测试，但尚未把这 61 个 Java execution 转为共享 Java/Go trace，不能据此把 `join.basic` 或子查询能力标记为 parity。

本轮新增的 Go `subquery_test.go` 将其中可先落地的语义拆成独立 case：Named Window 相关 `exists`、Named Window 标量子查询、Table `in`、参数化谓词、普通事件源非法校验、Fire-and-Forget 一致快照、嵌套子查询中 `OuterField` 指向外层子查询候选事件、Named Window `count/sum/avg` 与 `any/some/all` 比较量词，以及标量 `order/offset/limit` 和多行 cardinality 选项。随后补充非分组聚合 `SubqueryHaving`，让完整 inner aggregate group 接受 outer-field 阈值，并提供 `SubqueryExistsValue` 与带 options 的 `SubqueryInWithOptions`/`SubqueryAnyWithOptions`/`SubquerySomeWithOptions`/`SubqueryAllWithOptions`；`TestSubqueryUngroupedHavingCorrelatesAndTracksWindowState` 固定空组、低于阈值、达到阈值、窗口状态变化和非法非聚合 having。该映射已登记为 `query.subquery-basic` / `case.subquery-basic`；它证明聚合/量词/标量选项的基础 builder/runtime 形态和 null 处理可运行，不覆盖 Java 的动态 error/cardinality 传播、运行时 fragment materialization、上下文/模式/Dataflow 组合、迭代器和完整 query-plan/trace 语义。

本轮继续对照 Java `EPLSubselectMulticolumn` 的 `FragmentEventType` 形状，为 `SubqueryRow`、`SubqueryRows` 和 `SubqueryGroupRows` 增加静态 `SubqueryMetadata`：列别名、Go 类型、可空性、nested fragment、indexed/native 标记和 defensive-copy 查询接口均来自可分析 AST，不读取运行时状态，也不改变现有 Go-native map/slice 结果类型。随后将该 metadata 递归编译为结果 Schema 的 nested property，Row/Event 提供 `GetFragment`/`GetFragments` 物化标量和 indexed fragment；`TestSubqueryMultiColumnMetadataMatchesEsperFragmentShape` 与 `TestSubqueryMultiColumnRowsMaterializeScalarAndIndexedFragments` 覆盖多列聚合、多行结果、嵌套多行 fragment、单列结果不暴露 fragment、运行时 scalar/history/rows 三层 envelope 以及元数据副本隔离。Java 的全表示 fragment 转换、完整 iterator/enum-method/cardinality/error trace 与动态 representation 组合仍保持 partial。

本轮再对照 Java `EPLSubselectMultirowGroupedMultikeyWArray` 与 `EPLSubselectMultirowGroupedIndexSharedMultikeyWArray`：新增 `SubqueryGroupByAny` 和 typed `SubqueryGroupBucket`，允许 slice/array/map/struct 等不可比较 Go key，按 inner snapshot 首次出现顺序生成 bucket，使用 `KeyValue` 保留 Missing/Null/typed-nil 状态，聚合与逐行投影共用同一分组语义。`TestSubqueryGroupByAnySupportsArrayKeysAndStableBuckets` 固定重复数组内容合并、不同数组内容分桶、typed-nil key 和 Sum 聚合；旧的 `SubqueryGroupBy[K comparable]` map API 保持兼容。Java 的共享窗口索引、完整 multi-key 规划/性能和跨 Context/Pattern/Dataflow 组合仍保持 partial。

随后以固定 Java 17/Maven 3.9.16 环境直接执行 `mvn -pl regression-run -Dtest=TestSuiteEPLSubselect -DfailIfNoTests=false test`，该套件本次 19/19 通过；这只是 Java 侧回归入口可复现证据，Go 端仍按 capability manifest 的逐 case 差分口径推进，不能把整套子查询标记为已完成。

本次又定向执行了 Java `regression-run` 的 `TestSuiteRowRecog`：23 个 JUnit 入口全部通过，覆盖连续序列、reluctant/skip、interval、分区、measure aggregation、repetition、prev、variant stream、窗口和 Named Window delete 等入口。Go 已将其中基础连续序列、交替、可选/重复、有限和无限组合重复、reluctant、固定/日历 interval、Statement snapshot、长度窗口淘汰、Named Window 乱序删除重算、分区、重复变量、基础 measure aggregation 和无效规则切片映射到 `rowrecog_test.go`；这只是 Java case 到 Go 测试的关联证据，尚未执行共享场景 trace，因此 capability manifest 仍使用 `mapped` 或 `partial`，而不是 `passing`。

RowRecog 入口拆分如下，后续必须以此表逐项消项，不能以 `TestSuiteRowRecog` 这个外层 JUnit 名称代替：

| Java 入口/来源 | 当前 Go 证据 | 当前处置 |
|---|---|---|
| `RowRecogOps`、`RowRecogPermute`、`RowRecogRepetition` 的基础连续/交替/排列/可选/重复/分区/重复变量 | `rowrecog_test.go` 的基础序列、交替、有限排列、有限和无限组合重复、可选/重复、分区和 canonical/invalid 测试；新增 `rowrecog_repetition_test.go` 覆盖 exact、at-least、range、at-most、嵌套组、重复变量 PREV、等价展开和非法边界 | `mapped`，尚无共享 trace |
| `RowRecogAfter` | 已有三种 skip builder、固定 duration `Interval` 和 `Statement.Snapshot`；新增 `TestRowRecogAfterNextRowContinuation`、`TestRowRecogAfterSkipToNextRowDataSet`、`TestRowRecogAfterSkipToNextRowRepeatedVariable`、`TestRowRecogAfterSkipToNextRowPartitioned`、`TestRowRecogAfterSkipPastLastRow`，与既有 current-row 测试共同覆盖 Java 的三种 skip、重复变量、分区 listener/Snapshot 矩阵；`SKIP PAST LAST ROW` 的快照历史保留也已固定。interval 与 termination 的组合生命周期、完整 Java iterator/ordering 差异仍未对照 | `partial` |
| `RowRecogGreedyness` | `Reluctant` API、匹配器分支和 Go 正向用例已覆盖；新增 `TestRowRecogGreedynessReluctantZeroToOne`、`TestRowRecogGreedynessReluctantZeroToMany`、`TestRowRecogGreedynessReluctantOneToMany`，逐段镜像 Java 的 `A?? B?`、`A*? B? C`、`A+? B? C` listener/空输出序列 | `mapped` |
| `RowRecogInvalid`、`RowRecogClausePresence` | 已覆盖空模式、非法 DEFINE、未知 tag、重复 measure、负 MaxStates、旧流和 snapshot 限制的部分校验；新增 `TestRowRecogInvalidJavaMatrix` 对照 duplicate DEFINE、DEFINE 引用未来变量、MEASURES 聚合跨多个 tag/单事件 tag 和 DEFINE 中普通聚合的 Build 拒绝；新增 `TestRowRecogClausePresenceMeasures` 覆盖 `B.size()`、算术组合和 `B.anyOf` 的 interval measure，`TestRowRecogClausePresenceAllowsOmittedDefines` 覆盖省略 DEFINE 时的默认 true 语义；EPL 解析器专属 invalid syntax、property indexed 诊断和完整诊断文本仍未对照 | `partial` |
| `RowRecogEmptyPartition`、`RowRecogMultikeyWArray` | 有普通 `PartitionBy` 运行时切片；新增 `TestRowRecogEmptyPartitionLifecycle` 对照分区交替模式、空分区创建/释放和大量未完成分区 churn；多键数组高级组合仍未对照 | `partial` |
| `RowRecogInterval`、`RowRecogIntervalResolution`、`RowRecogIntervalOrTerminated` | 新增 `TestRowRecogIntervalSimpleTrace`、`TestRowRecogIntervalPartitionedTrace`、`TestRowRecogIntervalMultipleCompletedTrace`、`TestRowRecogIntervalMonthScopedTrace`，对照 Java `RowRecogInterval` 四个 execution 的 pending `Snapshot`、deadline listener batches、partitioned start times、multiple completed starts 和 calendar month boundary；新增 `TestRowRecogIntervalResolutionExactBoundary` 与 `TestRowRecogIntervalResolutionMicrosecondBoundary`，对照 Java `A*` 的 exact flip 和 `TestSuiteRowRecogWConfig` 的 10,000,000 微秒数值边界，检查 listener/Snapshot；新增十个 `RowRecogIntervalOrTerminated` execution 对照用例，覆盖 doc sample、`A B*` first/all、`A*` timer 后延展、`A B+`、三类 alternation、`A B C*`、fixed `A B` 和 parenthesized repeat 的 mismatch/finite terminal/timer listener；Go 使用显式 `time.Duration`/`time.Time`，Java 全局 TimeSource time-unit 配置 API 仍未提供；复杂动态/嵌套 NFA、全量日历终止、完整 iterator/order/skip 组合仍未对照 | `partial` |
| `RowRecogIterateOnly`、`RowRecogDataWin`、`RowRecogDelete` | 已有 `Statement.Snapshot` 基础 iterator 视图、长度窗口淘汰重算、time-window 到期重算、time-batch pending batch 迭代器及边界后状态清空、无数据窗原始流“listener 有结果但 Snapshot 为空”、Named Window 乱序删除重算和“删除不产生伪 listener batch”的测试；新增 `TestRowRecogNamedWindowConsumerStartsFromRetainedState`，对照 Java `RowRecogDataWinNamedWindow`，先用链式 `InsertIntoNamedWindow` 注入保留事件，再部署 Match Recognize 消费者并验证部署后的新事件能完成部署前已存在的识别分支；新增 `TestRowRecogNamedWindowDeleteOutOfSequencePrev`、`TestRowRecogNamedWindowDeleteOutOfSequence`、`TestRowRecogNamedWindowDeleteInSequence`，分别对照 Java 的 PREV 引用删除、无 PREV 乱序删除和顺序删除，覆盖 partial match 移除、PREV 到达顺序保留、重复变量、listener/Snapshot 结果；另有 `TestRowRecogIterateOnlyNoListenerMode`、`TestRowRecogIterateOnlyPrev`、`TestRowRecogIterateOnlyPrevPartitioned` 和 `TestRowRecogIterateOnlyDoesNotConsumeStatePool`，覆盖 iterate-only 只更新窗口/PREV、事件到达不触发 listener、Snapshot 重算、分区隔离和不占用状态池；完整 OOSD 状态库排序、delete/update 组合、Context-owned named-window replay 和 Java 输出排序仍未对照 | `partial` |
| `RowRecogEnumMethod`、`RowRecogAggregation`、`RowRecogArrayAccess` | 已新增 `TestRowRecogArrayAccessSingleMultiMix`、`TestRowRecogArrayAccessMultiDepends`、`TestRowRecogArrayAccessMeasuresClausePresence`、`TestRowRecogArrayAccessLambda`、`TestRowRecogArrayAccessLambdaAPlusB`、`TestRowRecogEnumMethodFirstOfEquivalent`、`TestRowRecogMeasureAggregationMatrix` 和 `TestRowRecogMeasureAggregationPartitioned`，覆盖重复 tag 的索引依赖、重复变量、array/single/constant measure presence、两段 Lambda `sumOf` 等价链式聚合、`firstOf` 等价 `TagFieldAt(0)`、max/min/first/last/count 和分区 sum 表达式；复杂动态枚举、聚合服务和完整 Java trace 仍未对照 | `partial` |
| `RowRecogPrev`、`RowRecogRegex`、`RowRecogVariantStream`、`RowRecogDataSet`、`RowRecogMultikeyWArray` | 已增加 Go 链式 `Prev`/`Abs`，并为每个 match-recognize 分区保存有限滚动 previous-access 快照；time window 淘汰后仍可按到达顺序取 PREV，已覆盖非分区、分区、双字段分区和 keep-all 场景。新增 `TestRowRecogDataSetFinancialPattern` 对照 Java `A B C* D E* F+`、DEFINE 中的普通 PREV/tag 引用、三段 E7/E8/E9 listener trace 和 `SKIP TO CURRENT ROW` 重叠分支；新增 `TestRowRecogFinancialWPatternDataSet` 对照 Java `A W+ X+ Y+ Z+` 的 E1–E60 无窗口流和全部终止 listener batches；新增 `TestRowRecogPartitionMultikeyWithArrayContent` 对照 Java 数组内容分区，验证 nil、空数组和同内容数组的分区身份；新增 `TestRowRecogPartitionMultikeyPlainTuple` 对照 int/long 完整多键 tuple；新增 `TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch` 对照 `RowRecogAfter` 的 A B* continuation；新增 `TestRowRecogRegexMatrix` 对照 Java `RowRecogRegex` 全部 12 组模式，并同时检查 listener 与 `Statement.Snapshot`；新增 `TestRowRecogVariantStreamPreservesMemberType` 对照 PREDEFINED Variant 的 typed member route、concrete type predicate、listener 和 Snapshot，另有非法成员/source/direct-payload 负例。完整外部 fixture trace、Variant/ANY/dynamic-member 高级矩阵和高级 tag-aware PRIOR/PREV 变体仍未映射 | `partial` |
| `RowRecogPerf` 及 MaxStates engine-wide 变体 | Go 已增加 `WithMatchRecognizeStateLimit`/`WithMatchRecognizeMaxStates`/`WithMatchRecognizePreventStart` 引擎级配置、按 statement.ID 汇总的不可变超限事件、prevent-start 分支拒绝、跨 statement/Context/Named Window 生命周期和 undeploy 释放测试；`TestRowRecogPerformanceBaseline` 对照 Java `A B*? C` 两分区终止匹配并保留可重复的 Go 基线日志，Java 25,000 行/分区的 `<2s` 硬阈值仍不直接作为普通单元测试门禁；精确 NFA 状态计数及 Java-scale 性能基线仍未完成 | `partial` |

本轮针对 `RowRecogDataWin` 增加了 Go 链式 API 对照场景：`TestRowRecogTimeWindowIteratorAndExpiry` 覆盖虚拟时钟下的 time window 到期、匹配后 iterator 视图和到期后的重算；`TestRowRecogTimeBatchWindowPendingIteratorAndBoundary` 覆盖 partition-by、pending batch 的 iterator、边界 listener 发射、边界后识别状态清空和下一批重新识别。实现将 time-batch 窗口保留的上一批与 match-recognize 当前识别状态分离，Java 的 unbound stream、PREV 到达顺序、Named Window time-batch 变体和完整输出顺序仍需继续对照。

随后补齐 Java `RowRecogDataWin.RowRecogDataWinNamedWindow` 的部署生命周期：`TestRowRecogNamedWindowConsumerStartsFromRetainedState` 先用链式 `OnEvent(...).InsertIntoNamedWindow(...)` 写入 named window，再部署 `FromNamedWindow(...).MatchRecognize(...)`；部署时回放保留事件建立识别状态但丢弃历史 listener batch，使部署后的新事件能够完成部署前已开始的序列。该回放当前限定为非 Context 的 Match Recognize named-window consumer，Context-owned named-window replay 及完整 Java trace 仍需单独对照。

本轮补充对照 Java `RowRecogOps.RowRecogUnlimitedPartition`：`TestRowRecogUnlimitedPartitionLifecycle` 使用 Go 链式 `PartitionBy`/`AllMatches` 在 64 个独立分区中先完成一批 `A B`，再批量建立 pending `A` 并逐分区完成 `B`，验证分区状态不会串扰、重复完成或丢失；512 分区试跑显示当前通用 NFA 路径复杂度明显上升，因此不把该规模作为默认单测门禁，后续仍需专门的高分区性能/回收优化与 Java 级阈值对照。

本轮继续对照 Java `RowRecogPrev`：Go 新增 `TestRowRecogPreviousHistorySurvivesTimeWindowEviction`、`TestRowRecogPreviousHistoryIsPartitionLocal`、`TestRowRecogPreviousHistoryForPartitionedSequence`、`TestRowRecogPreviousHistorySupportsMultiFieldPartitions` 和 `TestRowRecogPreviousHistoryOnUnpartitionedKeepAll`。实现对应 Java 的 `RowRecogStateRandomAccess`：新事件保存受最大偏移约束的滚动历史，旧事件从当前匹配输入移除时不回写滚动到达历史；因此匹配状态仍受窗口淘汰控制，而 DEFINE 中的 PREV 可复现 Java 的到达顺序语义。`PRIOR`、tag-aware previous access 和完整 Java/Go trace 仍待继续拆解。

随后补齐 Java `PREV(A.property, n)`/`PRIOR(A.property, n)` 的基础 tag-aware field 语义：Go 新增 `PrevTag` 与 `PriorTag` 链式构造器，并在 previous evaluator 中为嵌套 `TagField`/tag enumeration 重绑定被选中的到达顺序事件，而不是继续读取当前 match tag。`TestRowRecogTagAwarePrevAndPriorEvaluateAgainstPreviousEvent` 在 DEFINE 和 MEASURES 中验证 `PrevTag(1)` 与 `PriorTag(0)`、keep-all 的监听器/快照结果及未知 tag 构建期错误；新增 `TestRowRecogTagAwarePrevBindsRepeatedCapture` 对照 `A{3}` 重复捕获，在 DEFINE 中用 `PrevTag` 绑定前一个 A 事件，同时检查重复标签的首项、末项和 count。动态 offset、复杂嵌套 NFA、更多 tag-aware PRIOR/PREV 组合与完整 Java/Go trace 仍未完成。

本轮又对照 Java `RowRecogDataSet.RowRecogExampleWithPREV` 与 `RowRecogAfter.RowRecogAfterCurrentRow`：Go 新增 `TestRowRecogDataSetFinancialPattern`，用链式 `RowSequence(RowVar("A"), RowVar("B"), RowVar("C").ZeroOrMore(), RowVar("D"), RowVar("E").ZeroOrMore(), RowVar("F").OneOrMore())`、`Prev`、`TagField` 和 `SkipToCurrentRow` 复现 E1–E9 价格序列；测试不仅比较最终九条结果，还逐事件检查 E1–E6 无输出、E7/E8/E9 分别产生 1/3/5 条 listener rows。运行时同步修复 current-row skip：该策略保留已经被当前匹配接纳的旧起点，使 E8/E9 能继续产生重叠 NFA 结果；`TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch` 进一步固定 A1 初始结果在 B1 到达后扩展为 A1/B1。该测试中的 `Statement.Snapshot` 仍只表示当前可枚举/活跃识别分支，不被误当作 listener 历史结果全集；完整外部 fixture trace 和高级 NFA 仍需后续逐项移植。

本轮又对照 Java `RowRecogRegex`：Go 新增表驱动 `TestRowRecogRegexMatrix`，覆盖 Java 中全部 12 组 SupportTestCaseHolder，包括可选前缀/整组、交替、嵌套可选分支、`(A B)* C D`、嵌套组合重复、重复前缀/重复尾、嵌套 alternation 以及固定/重复 alternative；每个输入序列都通过链式 `RowSequence`/`RowAlternation`/`Optional`/`ZeroOrMore`/`OneOrMore` 构造，并对 listener 和 `Statement.Snapshot` 做无序结果比较。对照期间发现 current-row + ALL MATCHES 的 iterator 不能只保留每个 start 的最长 end，已修复为保留所有可完成路径；FirstMatch 和其他 skip 策略仍保留最长当前路径。Variant 输入和高级 NFA 仍需继续拆解。

本轮又对照 Java `RowRecogVariantStream`：Go 新增 `TestRowRecogVariantStreamPreservesMemberType`，注册 `SupportBean_S0`/`SupportBean_S1` 两个 struct schema 和 PREDEFINED `MyVariantType`，以两个 typed `From[T](...).InsertInto("MyVariantType")` 规则复现 Java 的成员路由；RowRecog 的 DEFINE 使用 `EventValue[Event]` 加命名 `Func1` 读取 concrete `Event.TypeName()`，因此只有 S0→S1 顺序能生成 A/B 结果。测试同时检查 listener 与 `Statement.Snapshot`，`TestRowRecogVariantStreamRejectsInvalidMemberRoute` 固定非成员 schema、未知源和直接 payload 的 Build/运行时错误。Java 的 Variant multikey-array、ANY/动态成员和完整 trace 仍需后续拆解。

本轮又对照 Java `RowRecogDataSet.RowRecogExampleFinancialWPattern`：Go 新增 `TestRowRecogFinancialWPatternDataSet`，使用无窗口原始流和链式 `A W+ X+ Y+ Z+`，以 `Prev` 在 DEFINE 中表达 W/X/Y/Z 的交替涨跌条件，并逐事件发送 E1–E60；测试对照 Java 的全部终止 listener batches（包括同一终点的多起始分支），同时验证累计输出数量。至此 `RowRecogDataSet` 的两组 Java execution 都有 Go 链式数据轨迹，完整外部 fixture trace、Variant multikey-array 和高级 NFA 仍需继续拆解。

本轮继续对照 Java `RowRecogMultikeyWArray`：Go 新增 `TestRowRecogPartitionMultikeyWithArrayContent` 与 `TestRowRecogPartitionMultikeyPlainTuple`，分别使用链式 `PartitionBy(array)` 和 `PartitionBy(intPrimitive, longPrimitive)`，复现数组分区的同内容/空数组/null 身份以及普通多键 tuple 的独立识别状态；两组测试都逐事件验证 A/B listener 输出。数组与普通多键分区的基础 Java trace 已映射，Variant 的 ANY/动态成员矩阵和更复杂的共享 trace 仍需继续拆解。

本轮继续对照 Java `RowRecogInterval`：Go 新增 `TestRowRecogIntervalSimpleTrace`、`TestRowRecogIntervalPartitionedTrace`、`TestRowRecogIntervalMultipleCompletedTrace` 和 `TestRowRecogIntervalMonthScopedTrace`，用虚拟绝对时间逐事件检查 `Snapshot` 中的 pending rows、deadline 到期的 listener batches、partition-local deadline、多起始匹配和 calendar month boundary。四个 Java `RowRecogInterval` execution 已有链式对照证据；`RowRecogIntervalResolution` 的精确/微秒边界以及 `RowRecogIntervalOrTerminated` 的十个 execution 已有对照证据，但高级终止组合仍需单独拆解。

本轮再对照 Java `RowRecogIntervalResolution` 与 `TestSuiteRowRecogWConfig.testRowRecogIntervalMicrosecondResolution`：Go 新增 `TestRowRecogIntervalResolutionExactBoundary` 和 `TestRowRecogIntervalResolutionMicrosecondBoundary`，均使用链式 `RowVar("A").ZeroOrMore()` + `Interval(10*time.Second)`，逐事件固定到达后 pending、1ns/1µs before 不发射、exact deadline 发射，并同时检查 listener 与 `Statement.Snapshot`。Go 不复制 Java 的全局数值 TimeSource 单位开关，而以 `time.Duration` 和纳秒精度 `time.Time` 表达同一可观察时间语义；该配置表面仍登记为差异。

本轮再对照 Java `RowRecogIntervalOrTerminated` 的十个 execution：Go 新增 `TestRowRecogIntervalOrTerminatedDocSample`、`TestRowRecogIntervalOrTerminatedABStar`、`TestRowRecogIntervalOrTerminatedAllMatchesAStarSuffix`、`TestRowRecogIntervalOrTerminatedAStarKeepsBranchAfterInterval`、`TestRowRecogIntervalOrTerminatedAStarBPlus`、`TestRowRecogIntervalOrTerminatedABStarOrCStar`、`TestRowRecogIntervalOrTerminatedABCStar`、`TestRowRecogIntervalOrTerminatedAB`、`TestRowRecogIntervalOrTerminatedABStarOrC` 和 `TestRowRecogIntervalOrTerminatedParenthesizedBStar`。这些链式用例逐事件对照 mismatch termination、finite terminal、timer snapshot 后继续延展、ALL MATCHES、alternation/repeated group、fixed sequence、doc sample 与 listener/Snapshot；运行时增加 terminal/branch path metadata、分支级关闭、同起始时间 timer 竞争和保留未关闭 NFA 分支的生命周期处理。十个 Java execution 已有 Go 对照证据，但复杂嵌套 NFA、完整 iterator/order、全量日历终止和高级 skip/组合矩阵仍保持 `partial`，不能视为 RowRecog 全部 parity。

本轮继续对照 Java `RowRecogAfter`：Go 新增 `TestRowRecogAfterNextRowContinuation`、`TestRowRecogAfterSkipToNextRowDataSet`、`TestRowRecogAfterSkipToNextRowRepeatedVariable`、`TestRowRecogAfterSkipToNextRowPartitioned` 和 `TestRowRecogAfterSkipPastLastRow`，配合既有 `TestRowRecogAfterCurrentRowKeepsTheRecognizingBranch` 覆盖 `A B*` continuation、`A B` 数值比较、重复变量、partition-by 和 past-last 的逐事件 listener/Snapshot 结果。为保持 Java iterator 语义，运行时新增已发出 match 的快照历史；past-last 新起点被 skip 后不再被 Snapshot 重新计算，而已发出结果继续可见。interval/termination 组合与完整排序仍需后续拆解。

随后补齐 `RowRecogDataWin.RowRecogUnboundStreamNoIterator`：无 `Window` 的原始事件流仍按链式 `MatchRecognize`/`Define`/`Measures` 产生 listener match，但 `Statement.Snapshot` 不再把识别分支当作可枚举数据窗；只有带 data window、Named Window、Table 或 historical source 的输入链才建立 iterator 视图。`TestRowRecogUnboundStreamListenerAndEmptyIterator` 固定复现 Java 的 `s1/s2/s1/s3/s2/s1/s1` 序列，验证最后一条重复字符串产生一条监听结果且快照为空；iterate-only 已另以窗口/PREV/分区三项用例固定，性能基线和完整 Java trace 仍属于后续工作。

本轮又以固定环境在 `regression-run` 模块专项执行 `mvn -pl regression-run -Dtest=TestSuiteExprEnum -DfailIfNoTests=false test`，Java `TestSuiteExprEnum` 的 28 个 JUnit 入口全部通过。该结果证明 Java oracle 的 ExprEnum 基线可复现，不等于 Go 端已经覆盖这 28 个入口；Go 当前只映射了首批可分析枚举算子和中立/运行态测试，嵌套、子查询、访问聚合、UDF、BigDecimal、完整无效规则与 Java trace 仍需逐项关联。

本轮补充了枚举表达式的 Go 类型/精度边界：新增 `EnumCollect[T](Expr)`，将数组、slice 和命名 slice 归一化为可链式枚举输入；新增公开 `EnumOrdered`/`EnumNumeric` 约束，`EnumSum`/`EnumSumOf`/`EnumMin`/`EnumMax`/`EnumMinBy`/`EnumMaxBy`/`EnumOrderBy`/`EnumAverageOf` 可处理 `math/big.Int` 与 `math/big.Rat`，求和和排序不再经过 float64；新增 `EnumAverageExact`/`EnumAverageExactOf` 返回精确 `big.Rat`。`TestEnumerableSupportsExactBigNumbersAndArrayCollections` 固定大整数、分数、字段选择器、数组和命名 slice。Java 28/28 基线通过；Go 仍需补 Java collection/generic component metadata、嵌套/子查询/访问聚合/UDF source 组合以及完整无效规则诊断，不能将本切片解释为全量 ExprEnum parity。

随后对照 Java `ExprEnumDataSources.ExprEnumProperty` 的 array/iterable 来源，`EnumCollect` 增加标准库 `iter.Seq`、`maps.Values` 适配和显式 `EnumIterator[T]` pull iterator；`TestEnumerableCollectsMapValuesAndIterators` 验证 map-backed source 的计数/求和、可复用 sequence 的 index 保序以及 pull iterator 的反转结果。实现只把 map 当作无序 source adapter，不把 Go map 迭代顺序伪装成 Java `Collection` 的顺序；Java 的集合泛型元数据、method/UDF/property/subquery/access-aggregate 组合和完整 invalid 诊断仍保持未完成。该切片登记为 `expr.enum` / `case.expr-enum-sources`，并保留 Java `TestSuiteExprEnum` 28/28 作为 oracle 基线。

本轮继续补齐 ExprEnum 的构建期 invalid 边界：枚举 AST 记录 collection 输入和 selector/predicate/右侧集合是否必需，`Environment.Build` 现在拒绝 `where/select/anyOf/allOf/countOf/takeWhile` 缺少 lambda、`minBy/orderBy/sumOf/averageOf/toMap/groupBy` 缺少 selector，以及 union/except/intersect/sequenceEqual 缺少右侧集合；对 nil collection 也不再延迟到运行时；`FirstOf/LastOf` 的 Go 可变参数若超过一个谓词会标记为 InvalidRule，避免静默忽略多余参数。`TestEnumerableBuildRejectsMissingRequiredExpressions` 同时断言代表性错误片段。Go 泛型已经在编译期拒绝多数类型不匹配，Java 的表达式体属性解析、完整 invalid 矩阵与错误文本仍需继续映射；该切片继续登记为 `case.expr-enum-invalid`。

本轮继续对照 Java `ExprEnumMinMax`、`ExprEnumOrderBy` 与 `ExprEnumTakeAndTakeLast` 的 method footprints：Go 增加 `EnumMinOf`/`EnumMaxOf` 返回 selector 值、`EnumOrderByNatural` 覆盖 scalar `orderBy()`/`orderByDesc()`，以及由 `Expression[int64]` 驱动、可读取运行时变量的 `EnumTakeExpr`/`EnumTakeLastExpr`；`TestEnumerableSelectorExtremesNaturalOrderAndDynamicTake` 固定事件 selector 极值、字符串自然升降序、负数/边界截取和变量更新后的运行时结果。Java 的 lambda 参数个数、null/type invalid 文本、集合泛型元数据与剩余 method footprint 仍未宣称对等；该切片登记为 `case.expr-enum-footprints`。

本轮继续对照 Java `EnumMethodEnumParams`/`DotMethodFP` 的编译期契约：Go 为每个内置枚举节点附加不可变 `EnumMethodMetadata`，显式记录 `CollectionType`、`ElementType`、`ResultType`、输入形态和无 lambda/1/2/3 参数 lambda footprint；`aggregate` 另外登记 initialization + 2/3/4 参数 accumulator footprint，`groupBy`/`toMap` 登记双 selector footprint，`EnumCollect` 对 array/slice/`iter.Seq`/pull iterator 统一报告归一化 `[]T` component type。`EnumerationMetadata` 只读复制由 `TestEnumerableMetadataMirrorsJavaFootprints` 验证。与此同时 UDF 入口扩展到 `Func3`/`Func4`，保留每个参数节点，缺少 name/function/argument 的规则在 Build 阶段诊断，UDF panic 物化为 Null；`TestUDFArityThreeAndFourRemainChainableAndSafe` 固定集合 UDF 链接、四参数求值和 panic 边界。Java 的动态 representation generic descriptor、表达式体精确类型推导、完整 null/invalid 文本与 Context/Pattern/FAF/Dataflow 组合仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprEnumDocSamples.ExprEnumSubquery`/`ExprEnumNamedWindow` 与 `ExprEnumDataSources.ExprEnumUDFStaticMethod`：Go 增加 `SubqueryValues`/`SubqueryEvents`，将 named-window 多行快照（含 inner predicate）转换成可继续 `EnumSelect` 的 typed slice；同时提供类型安全的 `Func0`/`Func1`/`Func2`/`Func3`/`Func4`，使零至四参数 UDF 返回的集合可以直接进入 `EnumSum` 等枚举链；UDF 的 nil function/argument 在 Build 阶段报告，运行时 panic 归一化为 Null。`TestEnumerableSubqueryAndZeroArgumentUDFFSources` 与 `TestUDFArityThreeAndFourRemainChainableAndSafe` 固定 E1/E2 named-window 过滤、投影求和、多参数 UDF 和 panic 边界。事件流 subquery、访问聚合/Context/Table/Pattern source、声明表达式、dynamic representation generic metadata 和完整 invalid 矩阵仍保持开放；该切片继续登记为 `case.expr-enum-subquery-udf`。

本轮继续对照 Java `ExprEnumGroupBy` 的 one/two-parameter execution：Go 增加 `EnumGroupBySelect`，把 key selector 与 value selector 分开建模为 `map[K][]V`，同时保留 `EnumGroupBy` 的原元素桶；`TestEnumerableSetFoldGroupMapAndFrequencyMethods` 固定重复 key 的稳定分组和值 selector 结果，`TestEnumerableBuildRejectsMissingRequiredExpressions` 固定缺少 value selector 的 Build 拒绝。Java 的 lambda 多参数解析、完整泛型集合 metadata、null-key descriptor 与精确 invalid 文本仍未宣称对等；该切片登记为 `case.expr-enum-groupby`。

随后以相同 Java 17/Maven 3.9.16 环境执行 `mvn -pl regression-run -Dtest=TestSuiteResultSetOutputLimit -DfailIfNoTests=false test`，该结果集套件的 14 个 JUnit 入口全部通过，其中包含 `ResultSetOutputLimitAfter` 的事件数 after、持续时间 after、after+every、月份和变量/when-then 场景。Go 当前映射的是固定 `time.Duration` after、非负 years/months/days 日历 after、事件计数 every、变量/基础计数/last-output-timestamp when/then、虚拟时钟 time every，以及来自 `ResultSetOutputLimitCrontabWhen` 的基础五字段 output-at：`CronEvery`/`CronRange`/`CronWildcard`、变量/参数字段、下一次触发和日月/周 OR；本轮又补充了普通窗口、分组聚合、Join、Named Window、Table、Table aggregate/join 和 context-partitioned Named Window 的当前状态快照，以及 `OutputSnapshotEvery`。Java 的 cron 秒/毫秒字段、特殊日历运算符、完整时区/DST/无效规则矩阵、更多 when/then 上下文/计数函数、完整 row-for-all/custom-access snapshot-after 和其他输出组合仍未宣称对等。

本轮又从同一 Java 输出套件的 `ResultSetOutputLimitRowPerEvent`、`ResultSetOutputLimitRowPerGroup` 和 `ResultSetOutputLimitRowForAll` 增加三条 Go 对照轨迹：`TestOutputRowPerEventJoinAllAndLastMatchesEsper` 固定 Join + `all`/`last` 的 interval 新行与最终聚合行，`TestOutputRowPerGroupAllAndLastMatchesEsper` 固定重复分组的 all 更新、last 去重和稳定 group identity，`TestOutputRowForAllAggregateOldNewBatchMatchesEsper` 固定无分组 length-window 聚合的 old/new 批次与滚动淘汰。三条测试均通过；Java 报告仍为 14/14，且 row-per-event/group/for-all 的 none/default/first/snapshot/time/count、unidirectional、Named Window、iterator、hint、SODA 和完整 old/new 组合继续保持 `partial`。

本轮修正了 `OutputWhen` 的一个运行时语义遗漏：Java 的 `then set` 赋值表达式与 `when` 条件共享同一输出上下文，因此 `count_insert`、`count_remove`、`count_insert_total`、`count_remove_total` 以及 `last_output_timestamp` 在赋值阶段必须仍可见；Go `finishOutput` 现在先捕获计数再清零，并按赋值顺序以工作变量和完整输出上下文求值。`TestOutputWhenThenAssignmentsSeeOutputCounterContext` 与 `TestOutputWhenThenAssignmentsSeeRemoveCounterContext` 固定了 current/total insert/remove 结果；Java `TestSuiteResultSetOutputLimit` 14/14 与 Go `TestOutputWhen*` 均通过。更大的 when/then 条件函数、Context/Named Window/Table/custom-access 与 output-after 组合矩阵仍保持 `partial`。

同一环境下又执行了 `mvn -pl regression-run -Dtest=TestSuiteResultSetAggregate -DfailIfNoTests=false test`，Java `TestSuiteResultSetAggregate` 的 14 个 JUnit 入口全部通过，覆盖 filtered aggregate、first/last/rate/nth、sorted/window access、table/join、invalid 和 aggregate-on-delete 等分支。Go 当前已映射可复用 `FilterAggregate` 的标量/访问聚合过滤和非法 nil predicate、带过滤器的有状态 `Leaving`、常量间隔速率的 ever-point/过滤语义、时间戳/数量速率的窗口淘汰和过滤语义、sorted/window access 的一组链式导航、Named Window 删除后的空聚合行、Named Window/FAF 的 first/window/last 快照、命名及环境注册式 `PluginAggregate` 扩展、按组隔离并能接收 `Enter`/`Leave`/`Value`/`Clear` 的 `AggregatePluginFactory`、`PluginAggregateAccess` 事件列表与显式 named-filter 链、无参数 Event 方法/属性链、确定性 Count-Min Sketch 频率/total 与表触发器访问、`TableSink` 适配器、链式 `AggregateStream.IntoTable` 的目标校验/原子快照替换/窗口淘汰删除以及 live table snapshot join；named filter parameter 的全部 API 形态、factory 的配置/serde/HA 生命周期、math-context/decimal、完整属性链/导航方法、context/rollup/FAF 组合和 table/join 全矩阵仍需继续拆解。

本轮新增的 Go `aggregate_test.go` 已覆盖 filtered aggregate 的 true/null 排除、Count/Sum/Avg/Min/Max/Distinct 包装、`first-ever`/`last-ever`/`count-ever` 跨长度窗口淘汰，以及 dimensional grouping 的 subtotal Null、`Grouping`/`GroupingID`、cube/explicit grouping set、Fire-and-Forget Named Window 快照和非法定义；对应 capability/case 已登记为 `resultset.aggregate-filtered`、`resultset.aggregate-ever`、`resultset.aggregate-dimensional`。

本轮继续对照 Java `ResultSetAggregateBlackWhitePercent`、`ResultSetAggregateCountVariations`、`ResultSetAggregateAllAggFunctions` 与 `ResultSetAggregateInvalid`：新增链式 `TestFilteredAggregateAllFunctionsJavaTrace` 和 `TestFilteredAggregateDistinctAndNullPredicateJavaTrace`，覆盖过滤后的 `avedev/avg/min/max/median/stddev/sum`、`max/min-ever`、distinct、窗口淘汰及 Null predicate 排除；同时修正普通 `StdDev` 单个有效样本返回 Null（Java sample standard deviation 未定义），并保留 nil predicate 的构建期拒绝。Java 的 named filter parameter 全 API 形态、`BigDecimal` math-context/decimal、plugin/join/table/access 全矩阵与精确 invalid 诊断仍为 `partial`。

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
2. 已接入并运行 Regression execution inventory 探针；后续以 `testdata/compat/java-execution-inventory.jsonl` 的 `runtimeId` 核对 `executions()` 动态生成的参数化/配置变体与静态 `3,848` 候选项，并继续关联 JUnit 入口、源测试条目和实际 Java/Go trace。
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

本轮继续对照 Java `PatternObserverTimerInterval` 的 `PatternIntervalSpec`、`PatternIntervalSpecVariables`、`PatternIntervalSpecExpression`、`PatternIntervalSpecPreparedStmt`、`PatternMonthScoped` 和 property-array 变体：Go 增加 `DurationDays`/`Hours`/`Minutes`/`Seconds`/`Milliseconds`/`Microseconds`/`Nanoseconds` 与可分析的 `DurationSum`，以及 `TimerIntervalExpr`、`TimerIntervalCalendar`。动态 interval 在 observer branch armed 时解析，可读取前序 Pattern tag、重复 tag 的 `TagFieldAt`、变量或 `DeployWithParameters` 绑定；根 timer、普通 Pattern 组合、Pattern Context 和月周期 recurrence 共用同一虚拟时钟 deadline 内核，calendar occurrence 使用 `time.Time.AddDate`。后续的 `TestPatternTimerIntervalExpressionUsesCapturedPropertyArray` 补齐重复 tag 上 indexed property，`TestPatternTimerIntervalExpressionUsesVariableReconfigurationForNextPeriod` 固定 callback 后变量变更只影响下一周期，`TestPatternTimerIntervalPeriodPreservesCalendarAndFixedPrecision` 固定日历 period 与固定精度 duration 叠加；原有测试继续覆盖 just-before/exact boundary、组件参数、微秒精度和 Context 分区。Java 的 ISO period 文本解析、动态 calendar component、observer 与 guard/consumption 的嵌套组合和双方共享 trace 仍未完成。

本轮继续对照 Java `PatternGuardTimerWithin.PatternOp` 与 `PatternObserverTimerInterval.PatternOp` 的独立 guard/observer 组合：新增 `TestPatternIndependentWithinGuardsComposeWithEveryAndMatchesEsper`，用两个各自 `Within` 的事件分支构造 `And().Every()`，固定虚拟时钟在部署/Context callback 中预装的 branch 必须由首个匹配事件复用，避免同一 `B/D` 组合重复输出；分支完成后下一次 Every 尝试仍独立持有 deadline，过期左 guard 后的迟到右事件不输出。runtime 为 timer/guard 预装匹配增加 `prearmed` 生命周期标记，并为 transition 区分普通匹配与事件消费（`matched`/`consumed`）；普通 Pattern 与 Context 路径共同修复。Java `TestSuitePattern` 本轮 28/28 通过；更广的 nested observer/guard/consume、Not/Until/MatchUntil、动态日历 component、SODA 和完整共享 trace 仍保持 `partial`。

本轮再补上 Java `PatternMicrosecondResolution` 对 timer interval 的纳秒级内部表示验证：`DurationMicroseconds` 在 `TimerIntervalExpr` 中可将 1µs deadline 精确落到 Engine 虚拟时钟，`TestPatternTimerIntervalMicrosecondPrecision` 覆盖 1ns-before 与 exact boundary。该测试只证明 Go clock/kernel 的精度切片；Java 的 ISO period、全部 observer 解析形式、DST/时区和共享 Java/Go trace 仍需单独映射。

本轮还修正了 Context 参数依赖的计划遍历遗漏：`queryParameterTypes` 现在递归扫描 context key/category/start/end、cron 和 start/end Pattern AST，确保 Context 内的 `TimerIntervalExpr`/guard 参数必须通过 `DeployWithParameters` 绑定，并复用同一类型冲突校验。`TestPatternTimerIntervalContextRequiresDeploymentParameters` 覆盖无绑定拒绝、参数化部署、虚拟时钟 deadline 和分区事件派发；Context 的变量动态重配置、跨 statement 参数快照和完整 Java deployment trace 仍未完成。

随后补上 Java `PatternIntervalSpecVariables` 的 Go 对照切片：`TimerIntervalExpr` 的 duration 表达式现在以 `VariableRef` 组合分钟/秒变量，并通过 `TestPatternTimerIntervalExpressionUsesVariables` 验证构建期变量依赖、变量初值、just-before/exact deadline 和虚拟时钟输出。这里仅证明变量表达式在 observer armed 时可求值；Esper Java 的变量版本快照、已排队 observer 对变量修改的重调度语义、跨 Context/statement 的可见性和共享 Java/Go trace 仍需单独对照，不能把一次初值测试解释为动态重配置 parity。

本轮继续对照 Java `EventBeanJavaBeanAccessor`、`EventBeanExplicitOnly` 和 `EventBeanPublicAccessors`：Go 事件 schema 新增 `WithPropertyGetter`/`WithTypedPropertyGetter`、`WithPropertyPath`、`WithPropertyMethod` 与 `WithNestedPropertySchema` 链式选项。`AccessorJavaBean` 会在 schema 构造时把 `GetX`/`IsX`（含 acronym decapitalization 和 `(value,error)` 形式）纳入 `PropertyNames`/`Properties`/`PropertyType`，`AccessorExplicit` 对 `StructSchema` 只发布注册的字段、路径和方法；方法既支持零参数 getter，也支持 indexed/mapped path 直接传参并在失败时回退到返回 slice/map 后再索引。注册的 nested schema 现在同时驱动 `GetFragment`、nested JSON/XML render 和 nested property metadata，显式 accessor/path/method/nested 信息也进入 Plan canonical identity。`TestJavaBeanAccessorPublishesGetterMetadataAndRenders`、`TestExplicitAccessorFieldMethodPathAndNestedSchema`、`TestExplicitAccessorRejectsInvalidRegistration`、`TestPublicAccessorKeepsFieldsAndAddsExplicitMethods` 和 `TestAccessorRegistrationEntersPlanCanonicalIdentity` 覆盖正向元数据、字段/方法/嵌套 schema、索引/映射、fragment、非法注册、未注册属性 Missing 与 plan hash 差异，并已登记为 `event.property-access-render` / `case.event-property-access-render`。Java `TestSuiteEventBean` 与 `TestSuiteEventBeanWConfig` 本轮固定为 19/19 通过。XML/Avro/JSON 全表示的 fragment/metadata、DOM/XPath、Java provided-underlying 和双方共享 render trace 仍未完成；本轮不把 accessor slice 宣称为事件模型完整 parity。

本轮继续对照 Java `RowRecogArrayAccess.RowRecogLambda`、`RowRecogClausePresence`、`RowRecogDataSet`、`RowRecogMultikeyWArray`、`RowRecogAfter`、`RowRecogInterval`、`RowRecogIntervalResolution`、`RowRecogIntervalOrTerminated` 和 `RowRecogRegex`：Go 在链式 Match Recognize 表达式层新增 `TagSum`、`TagAvg`、`TagMin`、`TagMax`、`TagFirst`、`TagLast`、`TagAny`、`TagAll`、`TagEvents` 和 `TagSize`。这些构造器把 tag 名与元素表达式保留在 AST 中，在 DEFINE 候选分支和 MEASURES 结果阶段按重复捕获顺序求值；可选 tag 的聚合返回 Null、`TagSize` 返回 0、`TagAll` 遵循空集合为 true，未知 tag 在 Build 阶段报 `ErrorUnknownName`。`TestRowRecogTagAggregatesAndEnumeration` 覆盖 A* B 的重复/可选捕获、数值聚合、枚举 any/all、事件数组复制、边界值和未知 tag 负例；`TestRowRecogRegexMatrix` 现在覆盖 RowRecogRegex 全部 12 组组合模式及 listener/Snapshot 结果；RowRecogAfter 的三种 skip、重复变量、分区和 past-last listener/Snapshot 矩阵也已新增；`TestRowRecogIntervalSimpleTrace`、`TestRowRecogIntervalPartitionedTrace`、`TestRowRecogIntervalMultipleCompletedTrace` 和 `TestRowRecogIntervalMonthScopedTrace` 已映射 RowRecogInterval 的四个 execution，固定 pending/deadline、分区和月边界语义；`TestRowRecogIntervalResolutionExactBoundary` 与 `TestRowRecogIntervalResolutionMicrosecondBoundary` 已映射 A* exact flip 和 WConfig 微秒边界，验证显式 typed time 的 listener/Snapshot 结果；十个 `RowRecogIntervalOrTerminated` execution 已新增链式对照，覆盖 mismatch termination、finite terminal、timer 后 branch continuation、ALL MATCHES 和 alternation/repeated-group 生命周期；新增 `TestRowRecogIterateOnlyNoListenerMode`、`TestRowRecogIterateOnlyPrev` 和 `TestRowRecogIterateOnlyPrevPartitioned` 对照 Java `RowRecogIterateOnly` 的窗口/PREV/分区 iterator 行为。Java `TestSuiteRowRecog` 本轮 Maven 运行 23/23 通过。RowRecog 的完整差异仍包括 time/time-batch row window、engine-wide max-states/prevent-start、variant/multikey array、复杂嵌套 NFA、invalid diagnostics、tag-aware PREV/PRIOR、Java 全局 TimeSource time-unit 配置 API、`IntervalOrTerminated` 更复杂嵌套/动态组合与完整日历终止、性能及共享 Java/Go trace；因此清单中的 `case.rowrecog-inventory` 只提升为 partial，不视为完整回归覆盖。

本轮继续对照 Java `ConfigurationRuntimeMatchRecognize`、`RowRecogStatePoolRuntimeSvc`、`ConditionMatchRecognizeStatesMax` 以及 `TestSuiteRowRecogWConfig`：Go 增加 `MatchRecognizeRuntimeConfig` 与链式 Engine option，默认关闭全局上限并保持 `PreventStart=true`；当状态池超限时，`MatchRecognizeStateLimitEvent` 在 engine lock 外按 registration order 回调，并携带 MaxStates 与 deployment/name 组合键对应的 statement 计数。RowRecog 运行时按 admitted start branch 接入池计数，`preventStart=true` 会保留被拒绝起始行但禁止它成为新的匹配起点，`false` 会记录超限后继续接纳；完成、窗口删除、Context 分区释放和 statement undeploy 都归还计数。`TestRowRecogEngineWideMaxStatesPreventStart`、`TestRowRecogEngineWideMaxStatesNoPreventStart`、`TestRowRecogEngineWideMaxStatesAcrossStatementsAndContext`、`TestRowRecogEngineWideMaxStatesNamedWindowRemoval` 与 `TestRowRecogEngineWideMaxStatesUndeployReleasesPool` 已覆盖这些 Go 边界。Java `TestSuiteRowRecogWConfig` 在 JDK 17/Maven 3.9.16 下运行 4/4 通过；Go 当前仍需补齐 Java NFA 内部每个 transition/重复分支的精确 state-count、microsecond configuration、完整 3/4-instance trace 和性能回归，因此该切片是 engine-wide resource-policy 映射，不宣称完整 RowRecog parity。
本轮继续对照 Java `PatternObserverTimerAt` 的 `PatternTimerAtSimple`、`PatternOp`、`PatternCronParameter`、`PatternPropertyAndSODAAndTimezone` 与 `PatternWMilliseconds`：Go 新增链式 `TimerAtSchedule`，复用类型安全的 `CronSchedule` 解析 wildcard、range、step、秒、毫秒、微秒、`last`、`lastweekday`、`N weekday`、`N last` 和固定/IANA 时区，在普通 Pattern 与 Pattern Context 中从部署/分区启动时计算严格下一次日历 occurrence，并在首次触发后清空 observer 状态，保证跨日跳时也不会重复创建。`TestPatternTimerAtScheduleEmitsNextCalendarOccurrenceOnly`、`TestPatternTimerAtScheduleSupportsStepAndMilliseconds`、`TestCronScheduleSupportsMicrosecondPrecision`、`TestCronScheduleSupportsSpecialCalendarOperators`、`TestCronScheduleSpecialOperatorsFlowThroughTimerCron`、`TestCronScheduleUsesConfiguredTimeZoneAndDST` 和 `TestCronScheduleRejectsInvalidSpecialCombinations` 覆盖 just-before/exact 边界、单次终止、分钟 step、微秒精度、月末/最近工作日/最后星期日、时区/DST 与 invalid 组合；Java `TestScheduleComputeHelper` 1/1、`TestSuitePattern` 28/28、`TestSuitePatternWConfig` 6/6 均通过，runtime ID、Go 测试和差异已补入 `case.pattern-timer-cron`/`case.output-crontab-static`。动态特殊运算符表达式、完整 SODA/property 语法、未知时区诊断策略、DST gap/overlap 精确策略及完整 observer/consumption 组合仍未完成，不能将该切片视为 Pattern timer 全量 parity。
本轮继续对照 Java `PatternObserverTimerSchedule` 与 `TestTimerScheduleISO8601Parser`：Go 增加类型化 `PatternTimerScheduleSpec`/`TimerScheduleWithPeriod`，支持当前虚拟时间或显式日期锚点、日历加固定精度周期、有限/无限 repetitions、past-date catch-up；同时提供迁移用 `TimerScheduleISO`/`TimerScheduleISOExpr`，覆盖 ISO date、period、date/period、`R/period`、`R/date/period` 及 captured-tag 动态表达式。普通 Pattern 的多 due occurrence、动态 ISO、以及 recurring Pattern Context 已有 `TestPatternTimerScheduleWithPeriodFiniteAndCalendarAnchor`、`TestPatternTimerScheduleISOForms`、`TestPatternTimerScheduleISOExpressionUsesCapturedTag`、`TestPatternTimerSchedulePeriodContextCreatesRecurringPartitions`；ISO 解析/调度差异仍包括微秒字段、DST/timezone 规则、特殊 calendar operator、动态 repetitions/date/period typed expression、嵌套 observer 的完整每次 occurrence trace 与精确 Java diagnostics，仍保持部分对等。
本轮继续对照 Java `RowRecogIterateOnly` 的 `RowRecogNoListenerMode`、`RowRecogPrev` 和 `RowRecogPrevPartitioned`：Go 新增链式 `RowRecogQuery.IterateOnly()`，事件到达时只维护输入窗口、分区和 PREV 历史，不推进 NFA、不发 listener 结果，并由 `Statement.Snapshot` 在查询时重新计算当前识别结果；同时避免为 IterateOnly 起始事件占用 MatchRecognize 状态池。`TestRowRecogIterateOnlyNoListenerMode`、`TestRowRecogIterateOnlyPrev`、`TestRowRecogIterateOnlyPrevPartitioned` 和 `TestRowRecogIterateOnlyDoesNotConsumeStatePool` 覆盖长度/last-event 窗口、PREV、分区隔离、空 listener、Snapshot 结果和资源配额边界。Java `TestSuiteRowRecog` 本轮 23/23 通过；性能计时、复杂 pattern/interval、完整 iterator/order 和 Java hint 诊断仍需 trace 级对照，不能将该切片视为 RowRecog 全量 parity。

本轮继续对照 Java `RowRecogArrayAccess`、`RowRecogAggregation` 和 `RowRecogEnumMethod`：Go 以链式 `TagFieldAt`、`TagEvents`、`TagSum`/`TagMin`/`TagMax`/`TagFirst`/`TagLast` 和 `TagCount` 映射重复 tag 的数组访问与 measure 聚合。`TestRowRecogArrayAccessSingleMultiMix` 固定 B[0]/B[1]、D[0]/D[1] 在 DEFINE 中的依赖；`TestRowRecogArrayAccessMultiDepends` 覆盖重复变量和 `(A B)* C` 的多重索引；`TestRowRecogArrayAccessMeasuresClausePresence` 覆盖 array/single/constant measure 形态；`TestRowRecogArrayAccessLambda` 覆盖 A* B 的 tag sum predicate 与索引结果；`TestRowRecogEnumMethodFirstOfEquivalent` 用 `TagFieldAt(0)` 固定 Java `firstOf()`；`TestRowRecogMeasureAggregationMatrix` 与 `TestRowRecogMeasureAggregationPartitioned` 覆盖未分区/分区的 max/min/sum/first/last/count、表达式聚合和 Snapshot。高级动态枚举、聚合上下文/删除矩阵、完整 ordering 和共享 Java/Go trace 仍保持 `partial`。

本轮继续对照 Java `RowRecogRepetition`：Go 的 `Repeat(min,max)` 链式模式与 Java exact、lower-only、upper-only、range 和 nested repeat 对齐；`TestRowRecogRepetitionDocSamples` 覆盖 TemperatureSensor 的 exact-two、at-least-two、between-two-and-three、up-to-two 文档样例，`TestRowRecogRepetitionPrev` 覆盖 `A{3}` 中基于到达顺序 PREV 的递增判断，`TestRowRecogRepetitionNestedAndRanges` 覆盖嵌套 group，`TestRowRecogRepetitionEquivalentExecution` 比较量词与显式展开的结果，`TestRowRecogRepetitionInvalidBounds` 固定负边界和 minimum>maximum 的 Build 拒绝。动态量词表达式、SODA 模型字符串、全部 Java 等价展开 trace 和复杂 NFA 状态计数仍待后续对照。

本轮继续对照 Java `RowRecogClausePresence`、`RowRecogEmptyPartition` 以及 `RowRecogArrayAccess.RowRecogLambda` 的第二段 `A+ B`：Go 以 `TagEvents` + `ArrayAt[Event]` 表达整事件 measure，以 `TagSize`/`TagAny` 和普通算术表达式表达 clause presence；`TestRowRecogClausePresenceMeasures` 覆盖 1 分钟 interval 到期后的 `B.size()`、`100+B.size()`、`B.anyOf`，`TestRowRecogClausePresenceAllowsOmittedDefines` 固定省略 DEFINE 的默认匹配，`TestRowRecogEmptyPartitionLifecycle` 固定 value 分区下的两种交替方向和 10,000 个无配对分区，`TestRowRecogArrayAccessLambdaAPlusB` 补齐 A+ B 的 first/second array measure、非匹配 B、后续匹配和尾部 invalid trace。Java `TestSuiteRowRecog` 的 23 个 execution 仍全部通过；EPL 专属 invalid parser 诊断、动态枚举和完整复杂 NFA 仍保持 `partial`。

本轮继续对照 Java `RowRecogDelete` 的三个 Named Window execution：Go 新增 `TestRowRecogNamedWindowDeleteOutOfSequencePrev`、`TestRowRecogNamedWindowDeleteOutOfSequence` 和 `TestRowRecogNamedWindowDeleteInSequence`，使用链式 `DeleteFromNamedWindow` 与 `NamedWindowField` 分别复现 PREV 依赖事件删除、重复变量 `A+ B* C` 的乱序删除以及 `A* B` 的顺序删除；测试逐阶段检查删除 partial match、保留 arrival-order PREV、重排后的重复 tag 和 listener/Snapshot 结果。对照期间修正了非 ALL MATCHES 的 `SKIP PAST LAST ROW` 快照起点边界；完整 OOSD 状态库排序、delete/update 组合与共享 Java/Go trace 仍待继续。

本轮补登记 Java `RowRecogPerf`：Go 的 `TestRowRecogPerformanceBaseline` 保留 `partition by value`、`A B*? C`、ALL MATCHES 和两分区终止输出，当前运行采用可在普通 `go test` 中完成的 25,000 行/分区基线并记录耗时；Java 的 `<2s` performance assertion、instrumentation exclusion 和完整 benchmark harness 仍单独列为差异，不把单元测试的通过误记为性能 parity。

本轮继续对照 Java `RowRecogInvalid` 的可由链式 API 表达部分：`RowRecogQuery.Define` 记录不同谓词的重复注册并在 Build 阶段拒绝；线性 sequence 中 DEFINE 只能引用当前或前置变量 tag，未来 tag 报 `ErrorInvalidRule`；普通 `Sum`/`Avg` 等聚合在 DEFINE 中被拒绝，MEASURES 聚合跨多个 tag 或作用于 singleton tag 也被拒绝。相同描述的重复注册保持 Go builder 的幂等性，以支持重复 measure 索引生成器；Go 的 `CountAll`/`Sum(Field(...))` 继续表示当前匹配组，EPL 专属 parser/Join/property-indexed 诊断仍保持差异。

本轮继续对照 Java `ResultSetAggregateFirstEverLastEver`：`TestAggregateFirstLastEverWindowAndFilteredTrace` 逐事件复现 length(2) 窗口中的 current first/last、first-ever/last-ever、`countever(*)`、非空表达式计数和 bool 过滤计数；`TestAggregateFirstLastEverNamedWindowDeleteRetainsHistory` 复现 Named Window 插入、E2/E3/E1 删除后 ever-state 保持 E1/E3/3；`TestAggregateCountEverRejectsMultipleExpressions` 固定链式 `CountEver` 的多表达式 Build 拒绝。Java SODA 两种构造、table/join/output 组合和完整 ever 生命周期仍保持 `partial`。

本轮再对照 Java `RowRecogMaxStatesEngineWide3Instance`/`4Instance`：新增两条链式 Java 事件轨迹，覆盖跨 statement pool overflow、prevent-start、终止释放、partition+length window 和 undeploy 释放；并在固定变量 sequence 上增加 dead-start 裁剪，使不再可转移的旧 NFA start 在下一次状态池决策前释放。复杂重复/交替 NFA 的每 transition 精确计数仍保持 `partial`。

本轮继续对照 Java `ResultSetQueryTypeLocalGroupBy`：运行时为聚合语句维护按到达顺序去重的 statement-level current/ever event scope，使 `LocalGroupBy` 在外层 `GroupBy(group, level)` 下可以正确复现 `group_by:(group)`、`group_by:(level)` 和 `group_by:()` 的跨外层组汇总，而不是错误地局限在当前 outer group；scope 会随窗口/Named Window 删除收缩，ever scope 保留历史。新增 `TestLocalGroupByMultiKeyTrace`、`TestLocalGroupByOuterGroupsShareCrossGroupState`、`TestLocalGroupByNamedWindowDeleteRecomputesOuterAndLocalState` 和 `TestLocalGroupByRejectsInvalidKeys`，覆盖多键累计、局部 WindowValues 访问、删除重算与非法 key。Java `TestSuiteResultSetQueryType` 17/17 通过，数组多键、join/context/table/plugin、planning 与完整 invalid/trace 矩阵仍保持 `partial`。

本轮继续对照 Java `EventJsonParserLaxness` 与 `EventJsonTypingCoreParse/CoreWrite`：`ParseJSON` 现在对声明字段执行可分析的 scalar/array/map/struct 转换，支持 number/bool 到 string、字符串到 bool/number 及数组元素递归转换；声明标量收到 object/array 时按 Java lax parser 置为 Null，合法形状但值无法转换（包括数组元素和 map value）返回解析错误，不再静默保留错误原始值。新增 `TestJSONParserLaxScalarArrayAndShapeMatrix`，与既有 nested typed、big.Int/big.Rat、time、unknown/trailing/depth 和 renderer 测试共同固定边界。Java `TestSuiteEventJson` 13/13 通过；provided-underlying class/list adapter、dynamic JSON 全矩阵、schema inheritance 和共享 parse/write trace 仍保持 `partial`。

本轮补齐 Java `EventJsonTypingCoreParse`/`CoreWrite` 的动态 JSON 值矩阵：Go 动态 schema 现在递归保留 string/number/bool/null/object/array、混合数组和多层嵌套数组；整数动态值继续以 `int` 暴露，带小数或指数的值保留为 `json.Number`，因此 `42.0`、`4.2E+1` 在 `RenderJSON`、iterator 和 property access 路径中不会被错误折叠为 `42`。新增 `TestJSONDynamicValueMatrixAndNumberLexemesMatchEsper` 并登记 `case.event-json-typed`；Java class/list adapter、递归 inheritance/metadata、所有 nested/dynamic BigDecimal/BigInteger 精确格式和共享 Java/Go trace 仍未完成。

本轮继续对照 Java `RowRecogNFAView` 的 transition/state-pool 生命周期：新增轻量 Thompson 风格 NFA 计数器，在启用 engine-wide MaxStates 时按当前节点消费、后继边申请、终态释放维护每个起点的实际活动状态；覆盖 `A* B` 的循环 fan-out、`(A|B)* C` 的交替分支、PreventStart/no-prevent、skip、窗口删除与 undeploy 释放，并禁用一对一 fast path 以避免状态池只看起点。`TestRowRecogStatePoolCountsRepeatedNFAStates` 与 `TestRowRecogStatePoolCountsAlternatingNFAStates` 固定重复/交替 successor overflow 和完整释放；NFA 节点扩展、嵌套动态量词、部分分支被 PreventStart 拒绝后的精确结果过滤、Java global TimeSource 配置和性能基线仍保持 `partial`。

随后补充同一起点的终态/续接分支竞争：当 `A*` 的终态允许输出而循环 successor 因 `PreventStart` 被拒绝时，状态池会按 `start/end/captures` match key 过滤结果，保留当前终态并阻断后续重建。`TestRowRecogStatePoolAllowsAcceptedTerminalWhenContinuationIsBlocked` 固定该 Java transition 边界；更复杂的分支级结果排序、interval/terminated 延迟终态、嵌套动态量词和完整 global configuration 仍待继续对照。

本轮继续补齐 Java `RowRecogVariantStream` 之外的 Variant ANY 路由场景：`TestRowRecogVariantAnyStreamMatchesDynamicMembers` 注册 ANY Variant，通过两个 typed `InsertInto` 将不同 concrete member Event 路由到同一逻辑流，在链式 `MatchRecognize` 的 DEFINE 中按 `Event.TypeName()` 区分动态成员，并同时对照 listener 与 Snapshot。该切片证明 ANY Variant 可进入 RowRecog 的成员类型路由，但 Variant 的完整 dynamic property getter/cache、fragment/metadata、late schema、supertype coercion、mixed new/old order 和 rowrecog/subquery/FAF/serde 全矩阵仍未完成。

本轮继续对照 Java `RowRecogNFAView.step`、`RowRecogHelper.buildStartStates` 与 `RowRecogPatternExpandUtil.expand`：新增 `TestRowRecogStatePoolCountsFiniteOptionalPermutationAndNestedBranches`，分别固定 `A? B? C` 的 optional bypass、`permute(A,B)* C` 的 permutation 回边、`(A B)* C` 的 nested repeat 回边，以及 `A{2,3} B` 的 Java finite expansion。测试按 Java 先消费当前状态、再为每条 successor 边申请状态、下一行才执行 successor predicate 的顺序检查活动计数、超限事件和终态释放；完整动态量词、branch ordering、interval/terminated 组合与 Java plan introspection 仍保持 `partial`。
本轮继续对照 Java `ViewRank.ViewRankRemoveStream`（runtime `java-runtime-0652854f960214bc4473`）：Go 为 Named Window 增加可链式 `RankWindowBy` 保留策略，支持 unique-key 当前值替换、按 sort key 稳定排序、同值 rank 淘汰、容量 old stream、`DeleteFromNamedWindow` 外部删除和 consumer `Snapshot` 迭代顺序。`TestNamedWindowRankRemoveStreamMatchesEsper` 逐步复现 E1/E2/E3/E4 的排序与淘汰、E3/E4/E1 删除、同 key 的 E3 100→101 替换以及最终删除，并已登记为 `case.view-rank-remove-stream`；更广的 rank 与 Context/Pattern/Subquery 组合及共享 Java/Go trace 仍需继续拆解。
本轮补齐 Java `ViewRank.ViewRanked`/`ViewRankRanked` 的完整 rank replacement trace：新增 `TestRankWindowRankedSceneMatchesEsper`，用链式 `RankWindowBy` 复现降序排名、unique-key 原地替换、相同 sort 值的到达顺序、满窗 pass-through 和尾部淘汰，并逐事件检查 new/old stream 与 Snapshot 顺序；该 execution 已登记为 `case.view-rank-ranked-scene`。这只是 ViewRank 家族的独立排序轨迹，rank 的更多 navigation/context/subquery 组合和共享 Java/Go trace 仍保持开放。
本轮继续对照 Java `ViewRank.ViewRankMultiexpression`：新增 `TestRankWindowMultiexpressionMatchesEsper`，以链式 `RankWindowBy` 表达双 unique key 和双 sort key，逐步验证多字段替换、满窗新事件立即淘汰、旧/新流相同事件和多字段 tie 淘汰；该 execution 已登记为 `case.view-rank-multiexpression`。更广的 Rank navigation、Context/Subquery 组合和共享 Java/Go trace 仍保持开放。
本轮继续对照 Java `EPLSubselectWithinPattern.EPLSubselectCorrelated`（runtime `java-runtime-3d9d3a714bd642e784bd`）：新增 `TestPatternSubqueryCorrelatedExistsMatchesEsper`，以链式 `PatternFrom(...).Every()`、`SubqueryExists`、内层 `Field` 与外层 `OuterField` 表达 `SupportBean_S0` 对 `SupportBean_S1#keepall` 的相关匹配，逐事件复现 Java `tryAssertionCorrelated` 并固定输出 id 5、6、9。Pattern 子查询的聚合、Named Window/UDF、Named Window IN、invalid 诊断，以及 Context/Dataflow 组合仍保持开放，不能据此宣称 Pattern 子查询全量 parity。
本轮再对照同一 Java 类的 `EPLSubselectFilterPatternNamedWindowNoAlias`（runtime `java-runtime-0db35509697665c0960e`）：新增 `TestPatternSubqueryInLastEventMatchesEsper`，以链式 `SubqueryIn` 和 `LastEvent()` 复现 `tryAssertion` 的替换语义，固定仅在 LastEvent(A) 后的 id 5 与 LastEvent(E) 后的 id 10 输出，并覆盖 Pattern `Every` 对非候选事件的隔离。Pattern 子查询聚合、Named Window/UDF、invalid 诊断和更宽的 Context/Dataflow 组合仍保持开放。
本轮补上 Java `ViewRank.ViewRankInvalid` 的链式负向映射：`TestRankWindowRejectsInvalidDefinitions` 固定非正 size、缺失/nil unique key 和 nil sort key 在 Build 阶段拒绝，并在 `case.view-rank-invalid` 中注明 EPL parser 的原始错误文本不属于 Go API 契约；其余 Rank family 的组合 trace 仍需继续对照。
本轮继续对照 Java `EPLSubselectWithinPattern.EPLSubselectAggregation`（runtime `java-runtime-9a15ec8213060ac0185c`）：新增 `TestEventStreamSubqueryAggregationWindowMatchesEsper`，用链式 event-stream 子查询和 `SubquerySum` 表达 `length(2)` 内层窗口，逐事件验证空窗口、滚动替换、未匹配和精确 sum 命中；该 execution 已纳入 `case.subquery-pattern`。Pattern 与 Named Window/UDF 聚合组合、完整 cardinality/iterator/trace 仍未完成。
本轮补齐 Java `EPLSubselectWithinPattern.EPLSubselectSubqueryAgainstNamedWindowInUDFInPattern`（runtime `java-runtime-3c1d7cc144c168f6a0d6`）：新增 `TestPatternSubqueryNamedWindowFunctionMatchesEsper`，用链式 `PatternFrom`、Named Window `SubqueryCount` 和 `Func1` 表达 UDF 接收子查询结果的触发条件，并验证 Pattern 读取到 Named Window 当前行；Pattern 子查询的 invalid、Context/Dataflow 组合仍待继续对照。
本轮继续对照 Java `EPLDataflowInputOutputVariations.EPLDataflowFanInOut`（runtime `java-runtime-6309a6b7e0f0ba981a97`）：Dataflow 增加 `DataflowPortOf[T]` 与 `CustomTypedPorts`，在 Build 阶段校验输出/输入端口的 Go 类型可赋值关系，在图运行时校验实际值类型；`TestDataflowTypedPortsFanInAndFanOutMatchesEsper` 以双文本/双数字输入和双命名输出复现 fan-in/fan-out，并固定类型不匹配边拒绝。自定义 source/operator 生命周期、内建 Event/Row 类型推导、并发/背压、深图/反馈和跨进程配置仍保持开放。
本轮继续对照 Java `EPLSubselectWithinPattern.EPLSubselectFilterPatternNamedWindowNoAlias`（runtime `java-runtime-0db35509697665c0960e`）：新增 `TestPatternSubqueryFilterAndNamedWindowVariantsMatchesEsper`，补齐 event-stream 顶层 Filter、Named Window 顶层 Filter、Named Window Pattern 三条原有用例分支，并复用 Java `tryAssertion` 的 A/C/E LastEvent 替换序列。为支持 Java `#lastevent` Named Window，`NamedWindowRetention(LastEvent())` 现在替换旧行、发出 old/new delta，并可被 Pattern/Filter 子查询读取；Pattern invalid、聚合及 Context/Dataflow 组合仍待继续对照。
本轮先执行 Java `TestSuiteEPLDataflow` 门禁：23 个 Dataflow execution 全部通过（Failures=0、Errors=0），覆盖 `EPLDataflowAPIRunStartCancelJoin` 的 start/run/join/cancel、异常、并发 runnable 与快速完成序列。Go 随后补充 `DataflowInstance.Run`/`Join`、完成信号、context 驱动取消、可重复 `Cancel` 和 source 错误传播，保持链式 API，不引入 EPL 字符串。
本轮继续对照 Java `EPLDataflowAPIRunStartCancelJoin`（runtime `java-runtime-893c8283ee90019f37fa`、`java-runtime-e1bdb19779e28fa49bc2`、`java-runtime-69dbb4e0d879421b52f7` 等）：Dataflow 增加 `CustomSource`/`CustomTypedSource` 与 `DataflowSourceRuntime`，每个实例独立创建 source，支持 raw/typed/named port 输出、Build/runtime 输出类型校验、Open/Run/Close 生命周期、有限 source 自动完成、阻塞 source Cancel/Join，以及 source 运行错误的 `DataflowError` 传播。`TestDataflowCustomSourceRunAndJoinMatchesEsper`、`TestDataflowCustomSourceCancelAndJoin`、`TestDataflowCustomSourceErrorPropagatesThroughRun`、`TestDataflowCustomSourceRejectsWrongOutputType`、`TestDataflowCustomSourceNamedPortsRouteAndTypeCheck` 和 `TestDataflowJoinRequiresStart` 已登记为 `case.dataflow-lifecycle`；Java 多 source runnable、内建 source/operator port 推导、并发投递/背压、完整 exception recovery 和跨进程保存实例仍保持开放。
本轮继续对照 Java `EPLSubselectWithinPattern.EPLSubselectInvalid`（runtime `java-runtime-57f90dfd1960035b1517`）：新增 `TestPatternSubqueryInvalidDefinitionsMatchesEsper`，覆盖三条 Build 阶段负向边界——无窗口且非聚合的 event-stream 子查询、Named Window 子查询再次声明数据窗口、以及 `IN` 两侧类型不兼容；同时将原有无窗口 event-stream 子查询断言修正为拒绝。Go fluent API 对外保持结构化 `ErrorCode` 契约，不复制 EPL parser 的原始文本；Pattern 聚合、Context/Dataflow 组合、完整 invalid 矩阵和执行 trace/cardinality parity 仍待继续对照。
本轮继续对照 Java `EventMapInheritanceRuntime`、`EventObjectArrayInheritanceConfigRuntime` 和 `EventJsonInherits` 的多级/分支执行：Schema 构造现在对多个父级共享祖先字段去重并拒绝冲突定义，运行时父类型 source 递归接收所有已注册后代事件、兄弟分支保持隔离；`TestSchemaInheritanceRoutesMultiLevelAndBranchedEvents` 覆盖 Map 的 Root/Left/Right/Leaf 路由、JSON 多级 `SendJSON` 和 ObjectArray 位置值。跨 module/configuration-time 注册、provided-underlying/Avro evolution、supertype/interface coercion、动态嵌套属性和完整 Java/Go trace 仍开放。
本轮继续对照 Java `ContextKeySegmentedNamedWindow` 的 6 个 execution（`java-runtime-18f8400337cdcd1dffd3`、`java-runtime-d96f78d8cfbbd5d871be`、`java-runtime-7ba3e00d3cf7571829de`、`java-runtime-73bbdb8596de168d6d94`、`java-runtime-7347c7d16d52e5ea0d30`、`java-runtime-af7bcf071474f57227fb`）：Named Window 新增 `NamedWindowContext` 分区存储与 `SnapshotContext`，动态记录源新增 `PatternFromRecord`；`TestContextScopedNamedWindowKeepsPartitionLocalRetentionAndSubqueryState` 覆盖多分区 Unique 替换、Context consumer、Named Window Pattern、普通/Context FAF 和相关标量子查询。全局/分区快照及未共享/共享索引的值语义已固定；Context-scoped temporal/initiated 生命周期、索引性能和完整 Context/Named Window trace 仍开放。

本轮补齐 Context-scoped Named Window 的 on-trigger 分支：`UpdateNamedWindow`、`DeleteFromNamedWindow`、`MergeIntoNamedWindowWhen` 和 `SelectFromNamedWindow` 现在读取 statement 的保留 context partition key，只访问当前分区；不存在的分区对读/删/改保持空结果，Merge 的 not-matched 分支可以在当前分区创建状态。`TestContextScopedNamedWindowTriggerMutatesOnlyCurrentPartition` 以 A/B 两个 key 逐步验证 update、delete、matched/not-matched merge、old/new stream 和本地 select，避免根窗口快照导致兄弟分区串写。Context-scoped temporal/initiated 生命周期、索引共享性能、跨 statement 事务和完整 Java trace 仍保持开放。

同一切片又补上 `InsertIntoNamedWindow` 的 Context 分区路由、on-trigger 与 Named Window context 不一致时的 Build 阶段拒绝，以及 `TestContextScopedNamedWindowOnTriggerMutationsStayPartitionLocal` / `TestContextScopedNamedWindowTriggerRequiresMatchingContext`；Java `ContextKeySegmentedInfra` 的 `OnDeleteAndUpdate`、`OnSelect`、`NWConsumeAll`、`NWConsumeSameContext`、`OnMergeUpdateSubq` 已登记，`mvn -pl regression-run -Dtest=TestSuiteContext -DfailIfNoTests=false test` 结果为 17/17。Context-scoped temporal/initiated 生命周期、索引共享性能、跨 statement 事务和完整 Java trace 仍保持开放。

本轮继续对照 Java `ContextStartEndNWSameContextOnExpr`（`java-runtime-229872ae86cacee2694c`）与 `ContextStartEndNWFireAndForget`（`java-runtime-e7e977b7827820a24082`）：`processNamedWindowContext` 现在支持 temporal Context 的当前周期 key，以及 initiated/terminated Context 的现有活动分区；Named Window 分区在 temporal 边界或 initiated Context 最后一个 statement 引用释放时同步删除，下一周期/下一次 initiation 从空状态重建。`TestTemporalContextNamedWindowReleasesPartitionAtBoundary` 与 `TestInitiatedContextNamedWindowReleasesPartitionOnTermination` 覆盖 inactive 丢弃、边界清空、`SnapshotContext` 失效和 restart；nested temporal/initiated、FAF 事务、old-stream 生命周期通知和完整 Java trace 仍保持开放。

本轮补齐 PREDEFINED Variant 的继承成员判定：Variant 声明父类型成员时，Leaf/branched descendant 现在可通过普通 `RouteTo` 进入 Variant，运行时保留实际 member identity 和动态字段；Build、Engine route、Dataflow EventBus source 及 Variant source acceptance 共用 Environment 的递归父类型判断。`TestVariantPredefinedAcceptsInheritedMemberTypes` 用 Map Root/Leaf 对照 RouteTo、FromAny、Filter 和 Leaf 字段读取；Java `EventVariantStream` 的 interface method/property coercion、wrapper/derived-stream、完整 metadata/getter cache 和共享 trace 仍保持开放。

本轮对照 Java `EventVariantSuperTypesInterfaces`（`java-runtime-a9cfea1ab05a48fc46fa`）：`commonVariantFields` 对 PREDEFINED Variant 的共有属性改为选择可容纳全部成员的最宽可赋值 Go 类型，避免成员声明顺序把子接口/具体类型泄露到 Variant 元数据；`TestVariantCommonPropertiesWidenInterfaceTypes` 以 JavaBean 风格 getter 返回父/子接口，验证 `p0`/`p1` 的共有接口类型、实际 member identity 和 getter 值都能通过普通 Variant 流读取。Java 的 collection/indexed/mapped/fragment metadata 及 wrapper/derived-stream matrix 仍保持开放。

本轮继续对照 Java `EPLDataflowAPIRunStartCancelJoin` 的多 runnable 分支与 `EPLDataflowOpEPStatementSource`：Dataflow graph 现在用独立的 `DataflowOperatorOpener`/`DataflowOperatorCloser` 支持只实现一侧生命周期的 source/operator，多个 custom source 可并发启动并在每个 source 完成后保持实例运行，`Join` 只在全部有限 source 完成后返回；EPStatement source 同时接受 Event 和投影 Row，`TestDataflowMultipleCustomSourcesJoinAfterEachCompletesMatchesEsper` 与 `TestDataflowEPStatementSourceConsumesPatternRowsMatchesEsper` 固定两条行为。Java 的完整 event/row 端口推导、动态 statement/filter 变体、异常恢复和持久化 graph 仍保持开放。

本轮继续对照 Java `EPLDataflowAPIRunStartCancelJoin` 的多 source execution（`java-runtime-827b0af4ea14c59da2bc`、`java-runtime-955a5d67e2668f50f0f5`）并补组合语义：Dataflow 图投递增加实例级同步门，多个 CustomSource 可并行运行但同一图的 operator 访问按输入队列串行，慢下游会同步阻塞 source 提交；同一实例内 `EventBusSink` 回送另一个 `EventBusSource` 时使用当前投递上下文直接重入，避免串行门自锁。`TestDataflowMultipleCustomSourcesJoinAfterEachCompletesMatchesEsper` 验证先完成一个 source 时实例仍为 Running，所有 source 返回后 Join 才完成；`TestDataflowEventBusSinkReentersAnotherSource` 固定 Trade → EventBusSink(TradeCopy) → EventBusSource(TradeCopy) 的重入结果。`EPStatementSource` 现在同时转发 Event 与 projection Row，`TestDataflowEPStatementSourceConsumesPatternRowsMatchesEsper` 用链式 `PatternFrom(...).FollowedBy(...).Every()` → `EPStatementSource` → `Emitter` 固定 Pattern Row 的形状和顺序。Java 的 built-in source/operator 背压、异常文本、深图/反馈、多端口 captive/join 及跨进程配置仍需继续对照。

本轮补充 Dataflow 同实例同步投递的重入回归：`EventBusSink` 在处理 Trade 时同步发送 TradeCopy，另一个 `EventBusSource` 能在当前投递上下文中继续完成到 Emitter 的链路，不会因实例级串行门重复加锁而死锁；`TestDataflowEventBusSinkReentersAnotherSource` 连续运行验证了该边界。该修复只允许当前 Dataflow 实例在已持有投递门时处理嵌套队列，跨实例仍保持独立串行边界；内建 source/operator 的完整端口推导、异常恢复、多端口 captive/join 和跨进程配置仍保持开放。

本轮继续对照 Java `EPLDataflowOpSelect.EPLDataflowAllTypes`（`java-runtime-2d7b6229c7a3b2bee40b`）：Dataflow 内建 `Filter`/`Select` 不再只接受 `Event`，也可消费 `EPStatementSource` 产生的 projection `Row`；链式 `ResultField` 作为行作用域表达式进入统一 `EvalContext`，可完成 Row 过滤和再投影。`TestDataflowBuiltinsProcessPatternRowsMatchesEsper` 固定 `PatternFrom(...).FollowedBy(...).Every()` → `EPStatementSource` → `Filter(ResultField)` → `Select(ResultField)` → `Emitter` 的结果。Java 的多输入 Select join、窗口/输出速率触发、全 representation 类型推导和完整 invalid 矩阵仍保持开放。

本轮继续对照 Java `EPLDataflowOpSelect.EPLDataflowOutputRateLimit`/`EPLDataflowTimeWindowTriggered`（`java-runtime-64f78048eb114d0e9cf9`、`java-runtime-553841d9498fc8d6d1c7`）：Go 增加 `SelectWithOptions`、`SelectTimeWindow` 与 `SelectSnapshotEvery` 链式入口，Select 实例独立维护当前/ever Event 聚合状态；`Engine.AdvanceTime` 驱动 snapshot tick 和时间窗精确到期，结果继续沿 graph 或线性下游投递。`TestDataflowSelectSnapshotEveryMatchesEsper` 固定 5s 到达 E1/E2/E3 后在 60s tick 输出 14，再在 120s tick 输出 23；`TestDataflowSelectTimeWindowExpiresAtBoundaryMatchesEsper` 固定 5s/15s 到达、65s/75s 精确移除和逐次重算 5/null。Java 的复杂 output-rate、完整视图组合、全 representation 与 invalid 矩阵仍未完成。

本轮补齐 Java `EPLDataflowIterateFinalMarker`（`java-runtime-59ce9d5f4b9ca475c272`）：Go 增加 `SelectIterate`/`SelectIterateOnFinalMarker` 链式入口，按 `GroupBy` 保存有限批次状态，在 `FinalMarker` 到达时按 `SortKey` 输出聚合 Row；`TestDataflowSelectIterateFinalMarkerMatchesEsper` 覆盖 captive graph、分组/求和/排序与 marker 前静默，`TestDataflowSelectIterateFinalMarkerLinearPathMatchesEsper` 覆盖 Beacon 有限线性路径，`TestDataflowSelectIterateRejectsRateLimitAndNilOrdering` 覆盖 invalid。并修复线性 signal 投递中 final-marker 分支的 operator index 传递，避免编译/下游投递错误。Java 的完整 Select invalid、representation、signal fan-out 和更广 final-marker 生命周期仍保持开放。

本轮补齐 Java `EPLDataflowOpSelect.EPLDataflowAllTypes`（`java-runtime-3c7ac0077e118cf1cccb`）的首段表示矩阵：Go 用同一条链式 `BeaconSource -> Select(Sum) -> Emitter` 分别消费 Struct、Map、ObjectArray、XML 四种 Event，验证 `Field[any,...]` 字段读取、累计状态和输出顺序；`TestDataflowSelectConsumesAllRegisteredRepresentationsMatchesEsper` 同时覆盖 `RegisterMap`/`RegisterObjectArray`/`RegisterXML`、`ParseObjectArray`/`ParseXML` 与 Struct Event。另以 `EventBusSource -> Select -> Emitter` 配合 `Engine.SendEvent`/`SendRecord`/`SendObjectArray`/`SendXML` 固定 EventBus 的四表示输入，测试名为 `TestDataflowEventBusSourceConsumesAllRegisteredRepresentationsMatchesEsper`。随后补齐 Java `EPLDataflowOpSelectWrapper` 的值形状：`SelectPassThrough` 保留 Event/Row 输入，`SelectEvent` 以注册事件 schema 合并原字段并覆盖额外选择（对应 select-star/wrapper 的字段与 underlying 语义），由 `TestDataflowSelectPassThroughPreservesEventMatchesEsper`、`TestDataflowSelectEventProjectionPreservesInputAndAddsPropertiesMatchesEsper` 和负例测试固定。Java 的动态 source/operator、representation-specific invalid 及全算子组合仍保持开放。

本轮继续对照 Java `EPLDataflowFromClauseJoinOrder`/`EPLDataflowOuterJoinMultirow`（`java-runtime-323dd1ec14f5ed5c0586`、`java-runtime-21dcd981de8124bf387c`）：Go 增加 `DataflowJoinOptions`、`SelectJoin`、`ConnectInput`，以 `in0..inN` 显式表达多输入，不引入 EPL；`DataflowJoinLastEvent` 的内连接等待所有输入后输出最新 tuple，`DataflowJoinKeepAll` 的 left/right/full outer 在单侧到达时按外连接边以 Null 填充缺失输入并生成只包含新到达事件的 Cartesian row，`JoinField` 读取 tuple source，Build 阶段要求每个 join input port 都有边。`TestDataflowSelectInnerLastEventJoinWaitsForAllInputsMatchesEsper` 覆盖三路齐备前不输出及 latest replacement，`TestDataflowSelectFullOuterJoinEmitsMissingInputMatchesEsper` 覆盖 full outer 单侧 Null、避免旧 unmatched row 重放和后续 Cartesian 补齐，`TestDataflowSelectLeftOuterJoinEmitsOnlyLeftUnmatchedRowsMatchesEsper`/`TestDataflowSelectRightOuterJoinEmitsOnlyRightUnmatchedRowsMatchesEsper` 固定左右外连接只保留各自外侧未匹配输入，`TestDataflowSelectLeftAndRightOuterLastEventJoinsMatchEsper` 固定两种定向 outer 的 latest-event 到达顺序，`TestDataflowSelectLeftAndRightOuterKeepAllJoinsMatchEsper` 固定两种定向 outer 的多行 Cartesian，`TestDataflowSelectJoinOrderPermutationsMatchEsper` 固定三路输入的 source-order permutations，`TestDataflowSelectJoinCaptiveEmittersMatchEsper` 固定 Java 的 captive Emitter → 多输入 Select 形状，`TestDataflowSelectJoinRejectsInvalidShape` 固定 Build 负例。Java 这些 Dataflow 用例本身不带谓词；Go 同时以 `DataflowJoinOptions.On` 提供无 EPL 的 `OnSourcesEqual`/`OnSourcesCompare` 条件及 `AllJoin`/`AnyJoin` 组合，`TestDataflowSelectInnerJoinConditionFiltersAndComparesMatchesEsper`、`TestDataflowSelectInnerJoinAnyConditionMatchesEsper`、`TestDataflowSelectJoinConditionsMatchEsper`、`TestDataflowSelectFullOuterJoinConditionEmitsUnmatchedInput` 与 `TestDataflowSelectJoinConditionRejectsInvalidSource`/`TestDataflowSelectJoinRejectsInvalidCondition` 固定条件过滤、比较/组合、全外无匹配输入保留和 source 范围校验。Java 的 multi-row old/new、view 组合、全部 representation 与 invalid 矩阵仍未完成。

本轮补齐内建端口的首段静态推导：EventBusSource/Sink、无 projection/projection 的 EPStatementSource 分别暴露 `Event`/`Row`，Filter/Select 使用封闭的 `DataflowRecord`（Event 或 Row）端口，并在上游已知时收窄为具体形状；`Row -> EventBusSink` 在 Build 阶段拒绝，`Event -> EventBusSink` 保持合法。`TestDataflowBuiltinPortInferenceMatchesEsper` 固定这些 metadata 与边类型校验；Beacon 异构值、Java EventBean/underlying/XML/ObjectArray/Map 多表示形式、Dataflow join 的完整端口传播与生命周期仍保持开放，条件 Join 的基础组合已在上一段登记。

本轮对照 Java `EPLDataflowAPIConfigAndInstance`（`java-runtime-67b54e408ca6f86b4ca5`）：Go 增加 `SaveDataflowConfigurationAs`，允许配置名与 Dataflow 名分离并拒绝重复保存；Engine 增加同进程 saved instance 的保存、按序列列举、读取和删除，以及 `DataflowInstance.DataflowName()`。`TestDataflowSavedConfigurationAndInstanceMatchesEsper` 覆盖缺失、重复、实例 identity、配置实例化和清理。Java 这部分本身是 runtime 内存服务；真正跨进程的定义/Go factory 序列化、恢复和 operator state 持久化仍属于后续扩展，不再与 Java saved registry 混为一项。

本轮补齐 Java `EPLDataflowAPIStatistics`（`java-runtime-c8cbd0bc1aeb3eba6006`）：Go 增加 `DataflowInstance.OperatorStats()`，按定义顺序返回稳定的 operator 名称/编号、PrettyPrint、已提交数量、按输出端口计数和耗时快照；提交统计只计算至少有一个下游连接的非 signal 值，终端 sink 保持零提交，与 Java source/capture 的统计形状一致。`TestDataflowOperatorStatsMatchEsperSubmissionShape` 覆盖有限 CustomSource 的两次提交、端口计数、终端 operator、快照拷贝隔离和最终输出。Java 的 CPU 统计开关、完整 operator metadata/pretty-print、异常/取消期间统计与跨模块统计生命周期仍保持开放。

本轮对照 Java `EPLDataflowAPIInstantiationOptions` 的 `EPLDataflowParameterInjectionCallback`/`EPLDataflowOperatorInjectionCallback`（`java-runtime-1dd9223830c5409fc252`、`java-runtime-1092d7e9660b84bc6e6d`）：Go 增加 `DataflowOperatorOptions`、`CustomWithOptions`/`CustomSourceWithOptions`、`DataflowParameterProvider`、`DataflowOperatorProvider` 和 `DataflowSourceProvider`；定义属性与参数名在 Build 时校验，在实例化时按稳定顺序回调并可由实例值覆盖，provider 可替换默认 factory，同时向 Go factory 传递独立复制的 `DataflowOperatorContext.Properties`。`TestDataflowParameterProviderMatchesEsperInstantiationOptions`、`TestDataflowOperatorAndSourceProvidersOverrideFactories` 和 `TestDataflowOperatorOptionsRejectDuplicateParameters` 固定参数上下文、覆盖优先级、source/operator 注入及 invalid。Java 的 ExprNode 参数求值、注入 factory 真实类型、部署级 property 元数据和完整 provider 生命周期仍保持开放。

本轮补齐 Java `EPLDataflowOpFilter` 的双输出分支：Go 增加 `FilterWithPorts` 链式入口，谓词命中值从 `passPort` 输出，未命中值从 `rejectPort` 输出；图模式下 `WindowMarker`/其他 Dataflow signal 向两个下游端口扇出，内建端口类型跟随上游 `Event`/`Row` 表示并拒绝无图边的双输出线性定义。`TestDataflowFilterWithPortsRoutesMatchedAndRejectedValuesMatchesEsper`、`TestDataflowFilterWithPortsFansOutSignalsMatchesEsper` 和 `TestDataflowFilterWithPortsRejectsLinearDefinition` 固定值路由、signal fan-out 与 invalid 边界。Java Filter 的多端口异常恢复、与所有内建 source/operator 的组合以及更广 representation coercion 仍保持开放。

本轮继续对照 Java `EPLDataflowOpEventBusSource` 的 `filter` 参数：Go 增加 `EventBusSourceWithFilter` 链式入口，在 EventBus 图入口和线性入口先执行结构化 Expr，未命中事件不进入下游；`TestDataflowEventBusSourceWithFilterRoutesOnlyMatchingEventsMatchesEsper`、`TestDataflowEventBusSourceWithFilterWorksOnLinearPathMatchesEsper` 和 `TestDataflowEventBusSourceWithFilterRejectsNonBooleanPredicate` 覆盖两种执行形态及类型 invalid。Java 的 EventBean/object-array collector、动态 source 参数和完整表示/诊断矩阵仍保持开放。

本轮继续对照 Java `PatternOperatorFollowedByMax` 的 `PatternMultiple`/`PatternMixed`/`PatternSingleMaxSimple`（`java-runtime-ab2b08d8274d0609885d`、`java-runtime-862a2f5893ecc893cee3`、`java-runtime-22ffb66490ffc2d07f4c`）：Go 增加 `PatternStream.FollowedByMax` 链式入口，在显式 sequence edge 上保存正数最大并发数；运行时只计算进入该 edge 右侧且仍等待匹配的 progress，嵌套 edge 独立限流，完成后容量可复用，普通 `MaxStates` 语义保持不变。`TestPatternFollowedByMaxLimitsActiveSubexpressionsMatchesEsper` 覆盖嵌套 B→C edge、容量释放和新一批起点，`TestPatternFollowedByMaxSimpleMatchesEsper` 覆盖 A→B 简单分支，`TestPatternFollowedByMaxRejectsNonPositiveMaximum` 固定 invalid。Java 的表达式/变量最大值、ConditionHandler 回调和 runtime subexpression 统计诊断仍待实现；消费策略及更复杂 pattern 组合仍未收口。

本轮补齐 Java `EPLDataflowOpEPStatementSource.EPLDataflowStatementFilter`/`EPLDataflowInvalid`（`java-runtime-8c8f6f03b11605a16d11`、`java-runtime-ef085a37ed45eb35a13f`）：Go 增加 `EPStatementSourceWithFilter` 链式入口，针对 EPStatement 产生的 `Event` 或投影 `Row` 在进入 Dataflow 图/线性链路前执行结构化布尔 Expr；未命中结果被源边界丢弃，非布尔谓词在 Build 阶段拒绝。`TestDataflowEPStatementSourceWithFilterRoutesOnlyMatchingRowsMatchesEsper` 与 `TestDataflowEPStatementSourceWithFilterRejectsNonBooleanPredicate` 固定正向及 invalid；JDK 17/Maven 下 Java `TestSuiteEPLDataflow` 23/23 通过。Java 的动态 statement 名称、完整 representation/filter 参数和 parser 诊断仍保持开放。

本轮补齐 Java `PatternConsumingPattern.PatternFollowedByOp`/`PatternCombination`（`java-runtime-d473b7cc639e11aa5776`、`java-runtime-917b60f248cb3507337c`）的查询级消费策略：Go 以 `DiscardPartialsOnMatch()` 和 `SuppressOverlappingMatches()` 两个 QueryOption 表达 `@DiscardPartialsOnMatch`/`@SuppressOverlappingMatches`，前者在一个事件导致完成后清空仍存活的 partial branches 并阻止同事件重新起根，后者保留活跃状态，但只在当前 result batch 内按捕获 Event identity 去重重叠结果，后续事件批次允许复用此前事件；Context、join、on-action 和非 Pattern 查询在 Build 阶段拒绝，两个策略也进入 Plan canonical/hash identity。`TestPatternDiscardPartialsOnMatchClearsOverlappingStartsMatchesEsper`、`TestPatternSuppressOverlappingMatchesHidesRepeatedEventMatchesEsper`、`TestPatternConsumptionPoliciesRejectNonPatternQueries` 与 `TestPatternConsumptionPoliciesRejectContextQueriesAndChangePlanIdentity` 固定该切片；JDK 17/Maven 下 Java `TestSuitePattern` 28/28、`TestSuitePatternWConfig` 6/6 通过。事件级 `@consume(N)`、match-until/and/or/guard/observer 的完整消费矩阵、排序/重复 tag cardinality 和共享 Java/Go transition trace 仍保持开放，不能把本轮查询级策略登记为完整 consumption parity。

本轮继续对照 Java `PatternConsumingFilter` 的 `PatternFollowedBy`/`PatternAnd`/`PatternOr`/`PatternFilterAndSceneTwo`/`PatternInvalid`（`java-runtime-db22761aeeae234aa70f`、`java-runtime-09fdeb1428f1772fd38c`、`java-runtime-fa260698edee4fba58ed`、`java-runtime-6cc7b65fc365ca99568d`、`java-runtime-5742c44f5fc51b6adfa1`）：Go 增加链式 `PatternStream.Consume(level ...int)`，将 consumption level 绑定到最近追加的 Event filter；无参数等价 Java 裸 `@consume`（level 0），显式 `Consume(0)` 也属于消费过滤器。同一输入 Event 命中一个或多个消费 filter 时，先求最高 level，只推进该 level 的消费分支，未标注 filter 保持原 partial state；只有没有消费 filter 命中时，未标注 filter 才正常推进，并覆盖普通/Context 路径、FollowedBy/And/Or、filter-scene 生命周期、同级扇出、混合未标注分支、level 0、负数/参数个数校验及 Plan hash identity。`TestPatternFilterConsumeFollowedByMatchesEsper`、`TestPatternFilterConsumeAndPreservesSuppressedBranch`、`TestPatternFilterConsumeAndSceneTwoMatchesEsper`、`TestPatternFilterConsumeOrSelectsHighestLevelAndFansOutTies`、`TestPatternFilterConsumeContextKeepsHighestLevelSemantics`、`TestPatternFilterConsumeUnannotatedFiltersYieldOnlyWithoutConsumptionMatch` 与对应矩阵固定上述行为。Java 的 SODA/表达式文本、精确 parser 诊断、重复 tag cardinality 和更广泛嵌套 transition trace 仍需继续对照；本轮不宣称 `@consume(N)` 全量 parity。

本轮继续对照 Java `EPLDataflowInputOutputVariations.EPLDataflowLargeNumOpsDataFlow`/`EPLDataflowFactorial`（`java-runtime-bddc8c72bd9d2a2c8869`、`java-runtime-98c5bdc6afa84705b9db`）：Go Dataflow 增加链式 `ConnectFeedback`/`ConnectFeedbackPorts`，在 `DataflowEdge.Feedback` 上显式声明可进入运行时 work queue 的反馈边；普通未标记环路仍在 Build 阶段拒绝，避免把 accidental cycle 误当成可终止图。`TestDataflowDeepLinearGraphMatchesEsper` 复现 17 段线性深图，`TestDataflowFeedbackEdgeFactorialMatchesEsper` 以 `input`/`temp`/`final` typed ports 和自环反馈计算 5! = 120，`TestDataflowUnmarkedCycleStillRejected` 固定负向边界。该切片只收口图拓扑与队列反馈；Java 的异常上下文/恢复、内建 source/operator 背压、任意反馈终止诊断、多端口 captive/join、跨进程保存及更广 custom operator method-binding 仍需继续对照。

本轮继续对照 Java `EPLDataflowAPIExceptions`（`java-runtime-76175156292855f373cf`）：Go `DataflowError` 增加 `OperatorNum`/`OperatorPrettyPrint` 与原始错误的 `Unwrap`，pretty-print 采用 Go 链式端口定义生成；Fail 策略下 operator/source 异常只调用一次 handler、保留 `LastError`，并将实例终态置为 `Complete`，避免把 Java 的异常完成态误映射为 `Canceled`；Continue 策略继续丢弃失败 work item。`TestDataflowOperatorExceptionCompletesWithEsperContext`、`TestDataflowEventBusOperatorExceptionCompletesInstance`、`TestDataflowSourceExceptionContextMatchesEsper` 与已有 source/Continue 用例固定 Beacon、持久化 EventBusSource 和 custom source 分支。Java 的精确 wrapper 文本、异步 handler 调度、内建 source/operator 背压及更宽恢复矩阵仍保持开放。

本轮继续对照 Java `EPLDataflowOpBeaconSource` 的四个 execution（`java-runtime-5438f7be9b56ca119b0c`、`java-runtime-f54de0ae037b90b24551`、`java-runtime-aac755fc7592bf62718d`、`java-runtime-f1fc7e290467bab36366`）：Go 增加链式 `BeaconSourceWithOptions` 和 `DataflowBeaconOptions`，支持有限/无限迭代、initial delay、interval、循环静态值、Go factory 上下文、有限源自动提交 `FinalMarker` 并在 context cancel 时退出；`TestDataflowBeaconOptionsIterationsAndFinalMarkerMatchesEsper`、`TestDataflowBeaconFactoryReceivesStableIterationContext`、`TestDataflowBeaconWithoutIterationLimitRunsUntilCanceled` 与负向参数用例固定行为。Java 的 EPL property evaluator、变量参数、Bean/Map/ObjectArray/Avro 制造和 additional field writer 尚未转成 Go 结构化字段映射，后续需继续补齐。

本轮对照 Java `PatternOperatorMatchUntil.PatternExpressionBounds`：Go 增加 `PatternStream.MatchUntilExpr(minimum, maximum)`，用 typed 链式表达式替代 EPL 的 `[lower:upper]` 文本；上下界表达式参与 Build 阶段的字段/变量/部署参数依赖收集、类型检查与 Plan canonical/hash，运行时在分支首次激活时读取变量、部署参数和外层 captured tag，并支持 `nil` 表示 Java 的省略下界/无上界。`TestPatternMatchUntilExpressionBoundsUseVariablesAndParameters` 覆盖闭区间、部署参数和空上界，`TestPatternMatchUntilExpressionBoundsUseCapturedTag` 覆盖外层 tag 传入重复分支，`TestPatternMatchUntilExpressionBoundsRejectInvalidRuntimeValues` 覆盖上下界反转时不产生结果；固定 `MatchUntil` 的重复 tag/数组投影保留。Java 的带 terminator 的动态 bounded-until、followed-by 动态上限、timer/Not 组合、零/负运行时值精确诊断及完整 PatternExpressionBounds trace 仍保持开放，不能把本轮动态表达式入口登记为全量 match-until parity。

本轮继续对照 Java `EPLDataflowOpEventBusSource`/`EPLDataflowOpEventBusSink` 的 collector 与动态事件类型 execution（`java-runtime-dfb59d3bd4798d57cc0c`、`java-runtime-37aed9aedcbbb25c4826`、`java-runtime-e6b4bb618f70b384cf03`、`java-runtime-1ecb769e10b4a818772c`、`java-runtime-11a7b321e6901dbad740`）：Go 增加链式 `EventBusSourceWithCollector`、`EventBusSourceWithFilterAndCollector` 与 `EventBusSinkWithCollector`。Source collector 可抑制、转换或复制输入，并返回已有 `Event` 或注册的 Go 事件值；组合入口保证 source filter 先于 collector 执行；Sink collector 可返回零/多条带目标事件类型的发送项，覆盖运行时动态类型分支；静态/collector EventBusSink 均作为终端 operator，不再向下游转发；`TestDataflowEventBusSourceCollectorTransformsAndDuplicatesEvents`、`TestDataflowEventBusSourceFilterRunsBeforeCollector`、`TestDataflowEventBusSinkCollectorSendsDynamicEventTypes` 与 `TestDataflowEventBusSinkIsTerminalAndRejectsOutputEdges` 固定图/线性两种路径。Java 的 EventBean/object-array/Beacon collector 上下文、全部 representation 组合和精确诊断仍保持开放。

本轮继续对照 Java `EPLDataflowOpLogSinkSettings`/`EPLDataflowOpLogSinkWWrapper`（`java-runtime-1320a4f248f4d8b28332`、`java-runtime-fa99678e315c32c80c59`、`java-runtime-0f763eb2cfa9c4677afc`）：Go 增加终端链式 `LogSinkWithOptions`，支持 summary/JSON/XML 三种格式、`%df/%p/%i/%t/%e` layout 占位符、title、默认/显式 linefeed、Event 与 projection Row 形状以及可注入的 Go writer；原有 `LogSink` 回调保持兼容，LogSink 输出边在 Build 阶段拒绝。`TestDataflowLogSinkStructuredFormatsAndLayoutMatchEsper`、`TestDataflowLogSinkRendersProjectionRowsAndDefaultsLineFeed` 与 `TestDataflowLogSinkRejectsUnsupportedFormatAndOutputEdges` 固定格式、布局、包装行和 invalid 边界。Java 的 SLF4J/stdout `log` 选择、EPL 参数求值、EventBean summary/toString 细节与更宽 renderer/property 矩阵仍保持开放。

本轮继续对照 Java `PatternOperatorMatchUntil.PatternExpressionBounds` 的 terminator 分支：`MatchUntilExpr(...).Until(terminator)` 在 Go AST 中保留动态上下界和 terminator，运行时在达到上界后停止重复分支但继续监听 terminator；同一事件同时命中重复分支与 terminator 时 terminator 优先，terminator 在下界前到达则该 partial 终止且不输出。`TestPatternMatchUntilWithTerminatorUsesBounds` 覆盖闭上下界、上界截断、同事件 terminator 优先，`TestPatternMatchUntilWithTerminatorBelowMinimumExpires` 覆盖下界失败；重复 tag 数组和 terminator tag 同时保留。Java 的 followed-by 动态上限、timer/Not 组合、零/负运行时值精确诊断以及完整 PatternExpressionBounds trace 仍保持开放。

本轮继续对照 Java `PatternOperatorFollowedByMax.PatternSingleMaxSimple` 的变量最大值与 invalid 矩阵：Go 增加 `PatternStream.FollowedByMaxExpr(maximum, tag, predicate)`，在左侧匹配完成、右侧 edge 激活时解析变量或部署参数表达式并将正数上限绑定到该条 active sequence；容量检查继续按 edge 独立统计，完成后释放。`TestPatternFollowedByMaxExpressionUsesVariableAndParameter` 覆盖变量/部署参数注入与容量上限，`TestPatternFollowedByMaxExpressionRejectsInvalidFieldAndRuntimeValue` 覆盖 Java 禁止的事件字段/tag 表达式及运行时非正值。Java 的 condition-handler callback、动态 subexpression diagnostics 与完整 SODA/trace 仍保持开放。

本轮补齐 Java `ConditionPatternSubexpressionMax` 的宿主诊断边界：Engine 新增 `PatternSubexpressionLimitListener`/`PatternSubexpressionLimitListenerFunc`，当 `FollowedByMax` 拒绝候选分支时，在 Engine 锁释放后报告 deployment ID、statement name、canonical edge、解析后的 maximum 和 attempted branch count；同一输入转换对同一 edge 的重复拒绝合并为一次通知，回调可安全重入 Engine。`TestPatternFollowedByMaxReportsRejectedSubexpressionMatchesEsper` 复现 `-[2]>` 的第三分支，`TestPatternFollowedByMaxNestedEdgeDiagnosticIsCoalescedPerInput` 复现 Java `PatternMultiple` 的嵌套 `-[3]>` 超限，另以 typed-nil/函数 nil 固定监听器校验。随后同一诊断路径接入 Context start 与各 partition end Pattern，额外携带 context name、start/end phase、partition key/ID，并由 `TestPatternContextFollowedByMaxReportsStartAndEndOwnership` 固定归属和跨分区去重边界。Java 的 runtime URI、factory/static hook 及更广的 engine-wide subexpression accounting surface 仍保持 partial。

本轮继续对照 Java `PatternOperatorMatchUntil.PatternBoundRepeatWithNot`（`java-runtime-f10e941aae4f3b003593`）：Go 将负分支命中禁止事件后的状态标记为 terminal，并从 enclosing `And`/`MatchUntil` partial 与 `MaxStates` 并发计数中立即释放；非 `Every` 根在该失败路径上停止，避免下一事件错误重启已失效 pattern。`TestPatternNotSuppressesNegativeBranch` 与 `TestPatternMatchUntilNotBranchExpiresAndReleasesState` 固定负分支失败、停止和槽位释放语义。Java 的 timer/Not observer 重启、`Every` 完成事件边界、嵌套 guard/observer/consume trace 与精确诊断仍保持开放。

本轮补齐 Java `EPLDataflowOpEPStatementSource.EPLDataflowStmtNameDynamic`（`java-runtime-950696d7aa6356acb9d4`）：Go 增加 `EPStatementSourceByName` 与带过滤器的对应链式入口；数据流可以在目标 statement 尚未部署时启动，statement 部署、撤销和同名重部署会自动订阅/解绑，取消 dataflow 后不再接收事件。`TestDataflowEPStatementSourceByNameTracksDeploymentLifecycleMatchesEsper`、`TestDataflowEPStatementSourceByNameWithFilterMatchesEsper` 和 `TestDataflowEPStatementSourceByNameRequiresName` 固定等待、生命周期重连、源边界过滤和 invalid；Go 使用 Engine 内的稳定 statement name lookup，未复制 Java deployment-id/class-filter/collector 反射机制，其他 statement filter/collector 与全表示矩阵仍保持开放。

本轮继续对照 Java `EPLDataflowOpEPStatementSource.EPLDataflowStatementFilter`（`java-runtime-8c8f6f03b11605a16d11`）：Go 增加 `DataflowStatementSourceContext` 与 `EPStatementSourceWithStatementFilter`，selector 可按 statement name、deployment ID 等元数据决定订阅对象；数据流启动时扫描已有语句，后续部署自动评估，撤销和重部署同步解绑/重连。`TestDataflowEPStatementSourceWithStatementFilterTracksAllMatchingStatements` 与 `TestDataflowEPStatementSourceWithStatementFilterRequiresSelector` 固定当前/未来 statement 选择、生命周期和 invalid；该 selector 与结果值级 `EPStatementSourceWithFilter` 明确分层，Java 的 statement collector、全表示和反射式 filter 配置仍保持开放。

本轮继续对照 Java `PatternOperatorMatchUntil.PatternExpressionBounds` 的 exact-1 与嵌套 timer/Not execution（`java-runtime-3e3d29bbec8e40af6e05`）：Go 统一在 `Not`/`Until`/`MatchUntil` 组合入口物化定义级 `Every`，并让 `Every` 在子分支因禁止事件 terminal 时重建 child、从失败时刻重新 arm timer；因此 `timer:interval(10) and not B` 在 6 秒收到 B 后于 16 秒完成，嵌套 `Until` 仍保留重复 B tag 数组。`TestPatternEveryMatchUntilNotRestartsTimerAfterForbiddenEvent` 与 `TestPatternUntilNestedEveryMatchUntilNotRetainsRepeatedEvents` 固定边界时刻、重启 deadline 和 repeated-tag 投影。Java 的完整 observer/guard/consume 矩阵、变量重配置与精确 SODA/诊断仍保持开放。

本轮继续对照 Java `EPLDataflowOpEPStatementSource` 的 collector 形态：Go 增加 `DataflowStatementSourceCollector`、`EPStatementSourceByNameWithCollector` 与直接 statement 变体，collector 可对每个 new Event/Row 结果返回零个、一个或多个图值；`TestDataflowEPStatementSourceCollectorTransformsAndDuplicatesMatchesEsper` 与 `TestDataflowEPStatementSourceCollectorCanSuppressValues` 固定 statement 上下文、转换/复制和抑制语义。Java 的 statement filter 与 collector 同时配置、old-stream collector、全 representation 和反射式参数装配仍保持开放。

本轮补齐 statement filter/collector 组合入口：Go 增加 `EPStatementSourceWithStatementFilterAndCollector`，先按部署语句元数据选择，再按 new Event/Row 值执行 collector；`TestDataflowEPStatementSourceStatementFilterAndCollectorCompose` 固定组合顺序和复制结果。Java 的 old-stream collector、全表示与精确 collector 上下文仍保持开放。

本轮对照 Java `EPLDataflowAPIOpLifecycle` 的 operator initialize context：Go 的 `DataflowOperatorContext` 现在向自定义 factory/provider 提供实例 `UserObject` 以及稳定的输入/输出 `DataflowPort` 描述（名称和可用 Go 类型），并保持属性 map 与参数回调的独立副本；现有 `TestDataflowParameterProviderMatchesEsperInstantiationOptions` 同时固定这些元数据。Java 的 forge-time annotations、完整 statement context 和更细粒度 lifecycle callback 仍保持开放。

本轮补齐 Java `EPStatementSource` 的 deployment-scoped lookup：Go 增加 `EPStatementSourceByDeployment` 及其 filter/collector 变体，以 `deploymentID + statementName` 共同匹配，避免同名 statement 跨 deployment 被误订阅；`TestDataflowEPStatementSourceByDeploymentMatchesDeploymentScope` 与空 statement-name 负例固定范围和校验。Java 的 deployment filter/collector 反射配置仍由 Go 函数式 API 替代，完整 representation 矩阵继续保持开放。

本轮补齐 Java `EPDataFlowIRStreamCollector` 的 new/old 批量语义：Go 增加 `DataflowStatementSourceBatchCollector` 及 `EPStatementSourceWithBatchCollector`、按名称/按 deployment、按 statement filter 的链式入口；回调收到独立复制的 `ResultBatch`，可以同时观察 new/old 结果并返回零个或多个图值，现有逐 new 值 collector 保持兼容。`TestDataflowEPStatementSourceBatchCollectorReceivesNewAndOldStreamsMatchesEsper` 固定长度窗口的首批 new、后续 new+old 以及输出顺序。Java 的 EventBean/object-array collector representation、port-aware emitter、精确异步队列和完整 collector 参数矩阵仍保持开放。

本轮补齐 Java `EPLDataflowOpEventBusSource` 的 underlying/EventBean 表示选择：Go 增加 `EventBusSourceWithUnderlying` 及 filter/collector 组合入口；source filter 仍针对 `Event` envelope 求值，接受后可在图路径或线性路径提交注册事件的原始 Go 值，默认 `EventBusSource` 继续提交 `Event`。`TestDataflowEventBusSourceWithUnderlyingMatchesEsper` 固定过滤、struct underlying 类型和两种执行路径。Java 的 Map/ObjectArray/XML/EventBean collector 上下文及完整 representation coercion 矩阵仍保持开放。

本轮继续补齐 Java `EPStatementSourceFactory.submitEventBean` 的表示选择：Go 增加 `EPStatementSourceWithUnderlying`、按名称/按 deployment 及 statement selector 变体；事件结果提交注册 underlying，投影 Row 提交稳定的 `map[string]any`，source filter 仍在 Event/Row envelope 上求值，默认入口保持 Event/Row。`TestDataflowEPStatementSourceWithUnderlyingMatchesEsper` 同时固定 direct Event、named Row、线性链路和输出顺序。Java 的 EventBean/object-array/Map/XML collector 参数组合和完整 representation coercion 仍保持开放。

本轮继续对照 Java `PatternOperatorEvery` 的完成边界（`java-runtime-96878ce28620e660d0b2`）并结合 `PatternOperatorMatchUntil` 的 `every [2]` 形态：定义级 `Every` 在当前事件完成一个根分支后不再用同一事件重新起根，新实例从后续事件接收；`TestPatternEveryMatchUntilStartsAfterCompletionEvent` 固定 `[2]` 的 1/2、3/4 分组，避免错误的 2/3 重叠结果。Java 的多层 Every、observer/guard 失败重启和完整 consumption trace 仍保持开放。

本轮继续对照 Java `PatternObserverTimerInterval.PatternIntervalSpecExpressionWithPropertyArray`（`java-runtime-6155a2b0181e2169f2a4`）与 `PatternMicrosecondResolution`：Go 用 `TagFieldAt + ArrayAt` 表达重复 tag 上的 indexed property，并增加 `PatternTimerPeriod`/`TimerIntervalPeriod` 支持日历 period 与固定精度 duration 的组合，例如 `1 month + 10ms`；`TestPatternTimerIntervalExpressionUsesCapturedPropertyArray`、`TestPatternTimerIntervalPeriodPreservesCalendarAndFixedPrecision` 固定属性数组求和、月边界和纳秒级 deadline。另对照变量型 timer observer，根 timer 在首个 callback 后记录变量快照；变量在 callback 之后变更时，下一周期从前一 callback 时间重新读取，`TestPatternTimerIntervalExpressionUsesVariableReconfigurationForNextPeriod` 固定 2 秒到 5 秒的重配置边界。Go 仍未覆盖 Java 的 ISO period 文本解析、完整动态 calendar component、timezone/DST、嵌套 observer/guard/consume 矩阵与精确 SODA/诊断 parity。

本轮继续对照 Java `EPLDataflowOpEventBusSource.EPLDataflowAllTypes`、`EPLDataflowSchemaObjectArray` 与 `EPLDataflowOpEventBusSink.EPLDataflowAllTypes`（`java-runtime-37aed9aedcbbb25c4826`、`java-runtime-dfb59d3bd4798d57cc0c`）：Go 将静态 `EventBusSink` 的链式输入从单一 `Event` 扩展为目标 schema 的原生表示边界，支持 Struct/POJO、Map、ObjectArray 和 XML tree；建图阶段按目标 schema 校验 source 输出类型，运行时交给 `Engine.Send` 完成字段/位置归一化和类型转换，投影 `Row` 仍明确拒绝作为事件发送。`TestDataflowEventBusSinkSendsAllRegisteredRepresentationsMatchesEsper` 固定 Beacon 原生值到静态 sink 再回到 statement 的四表示闭环，`TestDataflowEventBusSourceWithUnderlyingAllRegisteredRepresentationsMatchesEsper` 固定 source underlying 的四表示输出；Java 的 EventBean/object-array collector context、完整 multi-row join 生命周期和精确 representation diagnostics 仍保持开放，不能将 Dataflow 全量能力登记为完成。

本轮继续对照 Java `PatternObserverTimerAt.PatternCronParameter`、`ScheduleComputeHelper` 与 `PatternPropertyAndSODAAndTimezone`：Go 将 `last`、`lastweekday`、`N last`、`N lastweekday` 和 `N weekday` 映射为 `CronLastDay`、`CronLastWeekday`、`CronLastDayOfWeek`、`CronLastWeekdayOf`、`CronNearestWeekday` 等类型安全构造器；`CronSchedule.InTimeZone` 支持固定偏移、常用固定区时区和 IANA 时区，日历搜索、OutputAt、TimerCron、DST 边界及 Java 回归中的跨月/闰年序列均有覆盖。`TestCronScheduleSpecialOperatorsMatchEsperSequences`、`TestCronScheduleSpecialOperatorsFlowThroughOutputAt`、`TestCronScheduleSpecialOperatorsFlowThroughTimerCron`、`TestCronScheduleUsesConfiguredTimeZoneAndDST` 与 `TestCronScheduleConfiguredTimeZoneFlowsThroughTimerCron` 已登记到 `case.output-crontab-static`/`case.pattern-timer-cron`。Java 的动态特殊运算表达式、完整 SODA/property 形态、非法时区诊断、DST gap/overlap 策略及嵌套 observer trace 仍保持开放，不能据此宣称 cron 全量完成。

本轮继续对照 Java `EventJsonTypingClassParseWrite`、`EventJsonTypingClassWArrayAndColl` 与 `EventJsonTypingNestedRecursive`：Go 增加泛型 `NewJSONSchemaFor[T]`/`RegisterJSONFor[T]`，在保留 JSON schema kind 的同时把 declared JSON 字段递归物化为 typed struct、嵌套 struct、slice/array、map 和 nullable pointer；`ParseJSON`、`SendJSON`、property access 与 `RenderJSON` 共享同一 typed underlying。`TestTypedJSONSchemaMaterializesStructAndNestedCollections`、`TestTypedJSONSchemaSendJSONUsesStructUnderlying` 和非 struct invalid case 已登记到 `case.event-json-typed`。Java 的 provided-class serialization convention、dynamic-property 全矩阵、recursive schema inheritance 和共享 parse/write trace 仍保持开放。

本轮继续对照 Java `PatternObserverTimerAt.PatternPropertyAndSODAAndTimezone`：Pattern 的日历 TimerAt 现在区分未绑定的事件字段与已捕获的 Pattern tag 字段，允许在 `Then`/observer 分支激活后用 `TagField` 计算 Cron 字段，例如 `2*a.intPrimitive`，并在 arm observer 时把当前 tag/tag-values 传入 Cron 表达式求值；直接引用源事件字段仍在 Build 阶段拒绝。`TestPatternTimerAtScheduleUsesCapturedTagExpression` 覆盖 06:00 捕获属性、06:39:59.999999999 前不触发、06:40 one-shot 输出和 tag 投影，`TestPatternTimerAtScheduleRejectsUnboundEventField` 覆盖负例。Java SODA 重建、完整 timer:at property/timezone 变体、动态特殊运算符、DST gap/overlap 策略和嵌套 observer trace 仍未完成。


本轮继续对照 Java `EPLInsertIntoJoinWildcard`：Go 增加 `SelectSourceEvent` 链式入口，以 `EventValue[Event]` 保留 join 每个 source 的完整 Event envelope，并验证 InsertInto 下游的 source 类型、字段和 underlying 值在 Struct/Map/ObjectArray/JSON/XML/Avro 表示中保持稳定。`TestInsertIntoJoinWildcardPreservesSourceEventsAcrossRepresentations` 已登记到 `case.insert-into-chain`。Java 的 wildcard column 语法、join alias 自动推导、wrapper/recast、mixed-stream ordering 和更广的多表示转换诊断仍保持开放。

本轮继续对照 Java `EPLInsertIntoAssertionWildcardRecast`：Go 对无 projection 的链式 `InsertInto` 增加 Struct/Map/ObjectArray/JSON/XML/Avro 源到同组六种目标表示的 36 条 recast 对照矩阵，固定 `p0`/`p1` 的值、目标字段顺序和缺失 `c0` 的显式 null；`TestInsertIntoWildcardRecastAcrossRepresentations` 已登记到 `case.insert-into-chain`。Java 的 wrapper bean 构造、provided-class JSON、Avro union/metadata、split/transpose 与完整异常诊断仍保持开放。

本轮继续对照 Java `EPLInsertIntoChain`：Go 以四条独立链式 `Select(...).InsertInto(...)` 复现 `ChainMarketData -> S0 -> S1 -> S2 -> S3` 的稳定传播，逐段替换 `val` 并在最终消费者固定 `E1/3` 单条结果；`TestInsertIntoChainPropagatesThroughFourChainedRoutes` 已登记到 `case.insert-into-chain`。Java 的多模块部署时序、mixed new/old stream、output policy 叠加和更大 chained graph 仍保持开放。

本轮继续对照 Java `EPLDataflowOpBeaconSource` 的 `EPLDataflowBeaconWithBeans`、`EPLDataflowBeaconVariable`、`EPLDataflowBeaconFields` 与 `EPLDataflowBeaconNoType`，并以 `TestSuiteEPLDataflow` 23/23 通过作为 Java 基线。Go 增加链式 `BeaconEventSource`/`BeaconEventSourceWithUnderlying`：source-less typed 字段可使用 literal、注册变量及 Go 表达式，按目标 schema 物化 Event envelope 或 Struct/Map/ObjectArray/JSON/XML/Avro 原生值；factory 可逐次提供 Event/Row/map/native base 后由字段表达式覆盖，输出端口携带稳定 Go 类型，并可直接进入 `EventBusSink`。`DataflowBeaconOptions.IterationsExpression` 在实例化时读取变量快照，补齐 Java 变量控制迭代次数的 execution；Build 阶段拒绝空/未知/variant 类型、未知或重复字段、输入字段/查询参数依赖及不兼容字段类型。`TestDataflowBeaconEventFieldsMaterializeAllRepresentationsMatchesEsper`、`TestDataflowBeaconEventFactoryMaterializesPerIterationBaseValues`、`TestDataflowBeaconIterationsExpressionUsesVariableAtInstantiationMatchesEsper`、`TestDataflowBeaconEventFieldsFlowIntoEventBusSinkMatchesEsper` 与负向定义矩阵固定行为。终端 `LogSink`/`EventBusSink` 现在只向可选 signal handler 暴露 FinalMarker，不再从未声明输出端口转发。`GenericData.Record` Avro underlying、反射式 operator injection metadata 与完整 invalid diagnostics 仍保持开放。

本轮补齐同一 Java `EPLDataflowBeaconFields` 中 `EPDataFlowInstantiationOptions.addParameterURI("BeaconSource/theString", "E1")` 的实例级字段写入：Go 在 `DataflowBeaconOptions.FieldParameters` 中显式声明可覆盖字段，并复用统一的 `DataflowParameterProvider`，不引入 EPL/URI 字符串解析。参数名会归一化、去重并参与 Plan canonical；Build 拒绝无类型 Beacon、空/重复/未知字段，Instantiate 在启动 source 前校验 provider 值与目标 schema 的类型。每个实例持有独立 override，provider 未返回值时继续使用链式字段表达式默认值，provider 值最终优先于 factory base 和字段表达式。`TestDataflowBeaconFieldParametersOverridePerInstanceAndRouteEventBusMatchesEsper` 固定两个实例的独立值、确定性 provider context/order、定义不可变性及 EventBusSink 闭环，负向用例覆盖 untyped/unknown/duplicate/type mismatch。Java `TestSuiteEPLDataflow` 仍为 23/23；`GenericData.Record` Avro underlying、反射式 operator injection metadata 与完整 invalid diagnostics 继续保持开放。

本轮继续补齐 Java `EPLDataflowBeaconWithBeans` 的可写 Bean 属性：`AccessorJavaBean` 除 `GetX`/`IsX` 外自动发现指针接收者 `SetX(value)`/`SetX(value) error`，并将 setter 名称、类型和来源纳入 schema metadata 与 Plan canonical；对照 Java `EPLBeanWriteOnly`，只有 setter、没有 getter/字段的 Bean 仍暴露 0 个事件属性。Go 另提供 `WithPropertySetter`、`WithTypedPropertySetter`、`WithPropertySetterMethod`，让 EXPLICIT schema 使用类型化 callback/method writer；通用 `projectMapToSchema`/`mergeSchemaUnderlying` 先经 setter 写入新分配的 addressable struct，因此 Beacon、InsertInto、route 和 EventBus 共用同一物化语义。setter 返回错误、panic、getter/setter 类型冲突、错误 method 签名及非 struct schema 均有明确错误。`TestJavaBeanSetterMaterializesGetterOnlyStruct`、`TestJavaBeanWriteOnlySetterIsNotReadableProperty`、`TestExplicitPropertySetterCallbackMaterializesStruct` 和 `TestDataflowBeaconJavaBeanSetterMaterializationMatchesEsper` 固定私有字段、getter 读取、setter 写入、write-only metadata 及 Beacon underlying。Java 的“无默认构造器”通过 JVM 对象制造策略解决；Go 没有构造器选择，使用零值分配后调用 setter，行为结果对等。`GenericData.Record` Avro underlying、反射式 operator injection metadata 与完整 invalid diagnostics 仍保持开放。

本轮复核 Java `EPLDatabaseJoinOptionUppercase` 的 `metadatasql` 机制：Go `SQLHistoricalProvider` 增加 `SQLHistoricalMetadataSample` 与独立 `MetadataStatement`/`MetadataArguments`，在 live query 前读取样例结果的 `ColumnTypes`，再用于 converter/type metadata；Default/Required/Skip 语义保持兼容，live 与 metadata prepared statement 均由 `Close` 释放。`TestSQLHistoricalProviderUsesSampleMetadataStatement` 用自定义 driver 验证 metadata→live 查询顺序、类型来源、参数复用、prepared 生命周期和非法配置；真实 MySQL 历史查询与 SQL sink round-trip 继续通过。Java 数据库套件本轮 15 个入口仍保留 13 个语义通过、非法 SQL 诊断文本差异和性能阈值环境差异两项 approved difference；完整方言/type-code/连接池事务和全量 database trace 仍不得标记为完成。

同一 SQL 切片继续补齐方言边界：`NewSQLHistoricalPlaceholderRewriter` 提供 question、`$n`、`:n`、`@pn` 四种常见 driver convention，占位符扫描跳过单/双引号、反引号、方括号标识符、`--/#` 行注释、可嵌套块注释和 PostgreSQL dollar-quoted 文本，并继续在构造阶段报告参数数量不一致。`TestSQLPositionalPlaceholderRewriterRejectsMismatch` 扩展为词法/方言矩阵；这降低了方言适配误替换风险，但不等同于完整数据库 dialect/DDL/transaction parity。

本轮补齐 Java `EPLDatabaseFAF` 的剩余 9 个 runtime：Go 以 `FromHistorical` + `Select`/`Filter`/`WithDistinct`/`OrderBy`/`PreparedQuery`/`ExecuteFireAndForgetWithParameters` 构造 SQL FAF，不解析 SQL 文本中的规则语义。`SQLHistoricalProvider` 将 SQL 每一行重新初始化列别名去重状态，避免多行结果在第二行被错误判为重复列；`SQLHistoricalProviderOptions.ParameterTypes` 显式声明 argument function 所消费的 named parameters，使参数进入 Plan canonical 和执行时 missing/extra/type validation；`SQLHistoricalRowConverter` 对应 Java SQLROW hook，接收已经按 SQL metadata 转换的整行 map，可返回目标 schema underlying 或 nil 跳过该行。`TestSQLHistoricalFireAndForgetSimpleMatchesJava`、`TestSQLHistoricalFireAndForgetHooksMatchesJava`、`TestSQLHistoricalFireAndForgetPreparedQueryMatchesJava`、`TestSQLHistoricalFireAndForgetSubstitutionParametersMatchesJava`、`TestSQLHistoricalFireAndForgetDistinctMatchesJava`、`TestSQLHistoricalFireAndForgetWhereMatchesJava`、`TestSQLHistoricalFireAndForgetVariableMatchesJava`、`TestSQLHistoricalFireAndForgetFluentPlanMatchesJava`、`TestSQLHistoricalFireAndForgetSQLTextSubqueryMatchesJava` 和 `TestSQLHistoricalFireAndForgetInvalidPathsMatchJavaBoundary` 固定 10 个 execution 的正向、边界、关闭和错误传播。Java `EPLDatabaseFAFInvalid` 中 SQL result-set Join/Context SQL FAF 仍与 Go 的显式 historical/method Join/Context extension 保持 approved difference；SODA 以不可变 Plan 表达，1000 次性能门槛和精确 vendor/Java diagnostic 不作为 Go 契约。本轮 Go 定向测试使用自定义 database/sql driver，真实 MySQL 仍由 `TestSQLHistoricalProviderMySQLDocker` 门禁复核，不把 fake driver 结果写成真实数据库兼容性结论。

本轮继续对照 Java `EPLDataflowOpBeaconSource.EPLDataflowBeaconFields` 中强制转换为 `GenericData.Record` 的 Avro underlying，以及 `EventInfraPropertyUnderlyingSimple` 的表示身份：Go 新增 schema-bound `AvroRecord`，通过 `NewAvroRecord`/`NewAvroRecordFromMap` 提供 typed `Set`、`Get`、`Lookup`、`Clone` 与 JSON connector 输出；`Engine.SendAvro` 保留原生 record identity，既有 `SendRecord` 与 `ParseAvroJSON` 作为输入边界统一归一化为 `*AvroRecord`。Schema property access、Beacon typed underlying port、EventBus source/sink underlying、partial InsertInto 和 Struct/Map/ObjectArray/JSON/XML/Avro 36 路 recast 均传播目标 Avro schema identity，不再把 Avro 与 `map[string]any` 混同。Build/runtime 负例覆盖 undeclared field、类型不兼容、dynamic Avro schema、nil/cross-schema record 和未知 route projection；Java `TestSuiteEPLDataflow` 本轮仍为 23/23。该切片只关闭 `GenericData.Record` native-underlying 缺口；Avro binary datum/container codec、union/logical/fixed/enum、schema text/default/alias/evolution、完整 EventAvro/serde/fragment trace 仍保持开放。

本轮继续对照 Java `EPLDataflowAPIInstantiationOptions` 的 `EPLDataflowParameterInjectionCallback` 与 `EPLDataflowOperatorInjectionCallback`：Go 新增 `DataflowFactoryMetadata`，`DataflowParameterContext` 和 `DataflowOperatorContext` 都携带稳定 `DataflowOperatorKind`；自定义 operator/source 还携带定义中的原始 typed factory，provider 替换 runtime 时仍能识别原 factory，内建 Beacon 则以 `Kind=BeaconSource` 且 `IsBuiltin=true` 表示。测试固定 parameter provider 的 factory identity、operator/source provider override 不调用原 factory但 context 保留 identity、Beacon built-in metadata、确定性参数顺序/default value，以及空白/带首尾空格 property/parameter name 的 Build-time 拒绝。Go 继续用显式 `DataflowOperatorOptions`、factory context 和函数式 provider 代替 Java `@DataFlowOpParameter` 反射；Java `@DataFlowOpPropertyHolder`/`@DataFlowContext` 的任意字段注入、built-in operator runtime 替换和完整诊断矩阵仍保持开放。

本轮对照 Java `TestSuiteEPLSubselect` 的 grouped/multirow 子查询入口，新增 `SubqueryGroupBy` typed chain API：以显式 key、projection 和可选 `SubqueryGroupWhere`/`SubqueryGroupHaving` 构造 Go 原生 `map[K][]V` bucket；标量 projection 按组保留每个事件值，`Sum` 等 aggregate projection 按组物化单值，having 在组上下文中求值。`TestSubqueryGroupByAggregateAndHaving` 覆盖 Named Window 快照、where 后分组、aggregate/scalar 两种 projection、having 淘汰和未知/聚合分组键 Build-time 拒绝；Java `TestSuiteEPLSubselect` 本轮 19/19 通过。多列 row-shape、完整多行 cardinality/iterator/error 诊断、Context/Pattern/Dataflow 组合仍保持开放。

本轮继续对照 Java `EPLSubselectMultirow`、`EPLSubselectMulticolumn` 与 `EPLSubselectAggregatedMultirowAndColumn`：Go 新增 `SubqueryRow`/`SubqueryRowWithOptions`、`SubqueryRows`/`SubqueryRowsWithOptions` 和 `SubqueryGroupRows`，用显式 `Selection` alias 物化 Go-native `map[string]any` 行，支持单行 scalar、全量多行、全聚合多列、分组 scalar+aggregate 多列、where/having，以及 offset/limit/order/cardinality 复用；Build 拒绝空列、重复 alias、未分组 scalar+aggregate 混合、外层引用分组键和非分组键 scalar 列。`TestSubqueryRowsAndGroupedRows` 覆盖四种正向结果与 Java 对照负例；Java `TestSuiteEPLSubselect` 继续为 19/19。Fragment event type metadata、完整 iterator/enum method/error/cardinality、Context/Pattern/Dataflow 组合和共享 trace 仍保持开放。

本轮继续对照 Java `BeaconSourceForge`/`BeaconSourceFactory` 的 `iterations`、`initialDelay`、`interval` 三个 `@DataFlowOpParameter` 及 `DataFlowParameterResolution.resolveNumber` 顺序：Go 在 `DataflowBeaconOptions` 增加 `InitialDelayExpression`/`IntervalExpression`，以 `time.Duration` 保持 Go-style 类型安全；三个时序参数从同一次实例化变量快照解析，并先询问 `DataflowParameterProvider`，被 provider 覆盖的表达式不会求值。内建时序名按稳定顺序进入参数上下文、从事件字段参数中保留，表达式描述进入 Plan canonical。`TestDataflowBeaconTimingExpressionsUseInstantiationSnapshot`、`TestDataflowBeaconTimingParameterProviderOverridesDefaults`、`TestDataflowBeaconTimingParameterProviderPrecedesExpressions`、`TestDataflowBeaconTimingExpressionsHaveStablePlanIdentity` 与非法值矩阵覆盖快照、覆盖优先级、哈希差异、字段/查询参数依赖、类型和负值；Java `TestSuiteEPLDataflow` 仍为 23/23。Java 数值秒在 Go API 中有意替换为 `time.Duration`，不接受 EPL 式数值 coercion；反射式 property-holder/context 注入和剩余精确诊断矩阵继续开放。

本轮对照 Java `EPLDataflowCustomProperties` 的 simple、array/map、nested settings 与 catch-all property：Go 保持 `DataflowOperatorOptions.Properties` 的显式配置风格，并新增泛型 `DataflowProperty[T](context, path...)`，让 factory 以类型安全路径读取基础值、数组、嵌套 string-key map 和 catch-all map，不复制 Java 注解字段注入。定义构建、`Operators()` 元数据和每次实例化均递归复制 map/slice/array 容器，外部、首个实例或返回视图的修改不会污染定义及后续实例；属性树和排序后的参数名进入 Plan canonical，map 插入顺序不影响哈希，而属性值/参数集合变化必然改变哈希。`TestDataflowCustomPropertiesAreTypedAndInstanceIsolated`、`TestDataflowCustomPropertiesHaveStablePlanIdentity` 已登记到新 case。Java class-name 动态构造、setter numeric coercion、ExprNode 注入及 EPL parser 文本诊断由直接 Go 值/factory 取代，仍作为明确 API 差异保留。

本轮继续对照 Java `ExprEnumSelectFrom`：当前 Go 的单 selector `EnumSelect` 之外新增 `EnumSelectMap`，用 `Selection` 别名构造 Go-native `[]map[string]any` 多列行，支持 `EnumElement`/`EnumIndex`/`EnumSize` 在每个元素上的链式求值，并保持 null collection、empty collection 和 selector 顺序；空列、重复 alias、nil selector 在 Build 阶段拒绝。`TestEnumerableSelectFromProjectsNamedRowsWithIndexAndSize` 与 `TestEnumerableSelectFromBuildAndRuntimeProjection` 对照 Java 的 `v0/v1` 及 `i+100*s` 结果，Java `TestSuiteExprEnum` 28/28 通过；Java 的匿名行参数化 metadata、更广 source representation、declared/access-aggregate/context/pattern/table 组合和精确 lambda diagnostics 仍保持 partial。

本轮继续对照 Java `ResultSetAggregateFiltered` 与 `ResultSetAggregateFilteredWMathContext` 的 BigInteger/BigDecimal 分支：Go 新增 `ExactNumeric` 约束和 `SumExact`、`AvgExact`、`MinExact`、`MaxExact`、`LessExact` 等链式 API，以 `big.Int`/`big.Rat` 保持任意精度，支持过滤聚合、长度窗口淘汰和大整数边界，`AvgExact` 明确返回精确 `big.Rat` 以避免隐式舍入。`TestFilteredExactAggregatesMatchJavaBigNumberTrace` 复现 Java 的 bigint<100、avg/sum BigDecimal/BigInteger 长度二窗口轨迹，`TestExactAggregatesDoNotRoundLargeIntegers` 固定超过 float64 安全整数范围的结果。Java named filter-parameter 注册/语法、MathContext 与 BigDecimal scale/rounding、插件/表/Join 组合和完整 invalid/trace 仍保持 partial，不能将该切片解释为完整 aggregate parity。

本轮对照 Java `EPLDataflowAPIStartCaptive` 与 `EPDataFlowInstanceImpl.startCaptive()`：Go 新增 `DataflowInstance.StartCaptive`，一次返回 `DataflowCaptive` 的命名 Emitter 快照和按定义顺序排列的 `DataflowCaptiveRunnable`。Captive 启动只执行一次 operator/source Open 和被动 statement/event source 装配，不自动启动 Beacon/custom source；调用方可同步或自行放入 goroutine 执行每个 single-use runnable。source 正常返回、Beacon 发出 FinalMarker 后实例仍保持 `DataflowRunning`，可继续从 Emitter 提交下一批，直到显式 Cancel；Cancel 关闭 runtime 并让后续提交失败。`TestDataflowStartCaptiveEmitterMatchesEsper` 复现 E1/E2→FinalMarker flush→E3 与 Running/Cancel 生命周期，另外两项测试固定 custom source 不自动运行、单次执行、Open/Close，以及 Beacon runnable/final marker。Java 底层 completion listener、并发 runnable 调度诊断、EventBean/object-array 精确 coercion 和异常恢复矩阵仍保持开放。

同一 captive runnable 切片继续对照 Java `GraphSourceRunnable`：Go runnable 现在同时提供同步 `Run`、异步 `Start`、`Wait`/`Done`、source-only 幂等 `Shutdown`/`IsShutdown` 和 `AddCompletionListener`。完成快照携带 source 名、终止错误与 canceled 标志；正常、异常、caller context、source-only shutdown 和 instance Cancel 均只完成一次并唤醒所有等待者，完成后新增 listener 立即收到保留结果。`TestDataflowCaptiveRunnableAsyncShutdownAndWait` 固定异步调度、Wait timeout、source-only cancellation 和实例仍 Running，`TestDataflowCaptiveRunnableStopsWhenInstanceIsCanceled` 固定实例取消向 source context 传播。Java 的 `next()` 单步/audit provider、线程命名/中断文本、typed EventBean/object-array coercion 与更广异常恢复仍保持开放。

本轮补齐 Java `ExprEnumDataSources.ExprEnumTableRow`、`ExprEnumAccessAggregation` 及其 method/property/nested 组合的 Go 链路：先用 `WindowAccessBy` 将长度窗口物化到 Table，再通过 `TableField[WindowAccessValue[T]]` 和 `Method("Values")` 取得表中窗口元素，或直接用 `WindowValues` 取得有序 `[]T`，最后交给 `EnumAnyOf`/`EnumAllOf`/`EnumField` 做可分析链式判断；`Method`/`Property` 也可作为外层或内层枚举的集合 source，并保留外层事件字段用于嵌套比较。`TestEnumerableTableWindowSourceMatchesJava` 覆盖首批命中及窗口淘汰后的不命中结果，`TestEnumerableWindowAccessAggregateSourceMatchesJava` 固定 window(*) 轨迹，`TestEnumerableMethodPropertyAndNestedSourcesMatchJava` 固定方法返回集合、嵌套属性和 nested anyOf。Java `TestSuiteExprEnum` 28/28 通过；同一 source capability 中的 UDF/event-subquery、generic component metadata、context/pattern 和精确 invalid diagnostics 仍保持 partial。

本轮补齐 Java 多行 subselect 后继续调用 enumeration method 的组合证据：`SubqueryRows` 先将 Named Window 快照物化为有序 `[]map[string]any`，再由 `EnumWhere` 用 `Property` 过滤，最后由 `EnumSelectMap` 以 `EnumIndex` 生成 Go-native 行；`TestSubqueryRowsContinueIntoEnumerableChain` 固定 A/B 的过滤结果和 0/1 顺序位置。Java `TestSuiteEPLSubselect` 19/19 通过；Go 仍以 slice/map 取代 Java iterator/anonymous-row metadata，fragment/error/cardinality 及更宽 Context/Pattern/Dataflow 组合继续保持 partial。

本轮复核 Java `EPLDataflowInvalidGraph` 的 compile/instantiate 两个 execution（`java-6163dd659f409b462303`、`java-c883534b279f4d80c0b6`）：Go `DataflowBuilder.Build` 新增数据流/operator/port 首尾空白拒绝、所有 source-only operator 输入边拒绝、忽略 feedback 标记的路由唯一性，以及 feedback 必须确实闭合一条普通有向路径的校验，防止把任意前向边标成 feedback 绕过环检测；普通边与 feedback 同一路由也不会造成重复投递。`Plan.Canonical` 现在记录每条边的 feedback 位，图语义会进入稳定哈希。`TestDataflowInvalidGraphTopologyMatchesEsper`、`TestDataflowFeedbackMustCloseOrdinaryPathAndEntersPlanIdentity` 连同既有 unknown operator/port、typed edge mismatch、duplicate/cycle 测试固定这些边界。Java EPL parser、forge/class 查找、注解 property injection 和反射式 `onInput` 方法匹配在 Go 类型化 builder/factory/`Process`/`Run` 接口中属于无运行时对应物的编译期约束，不复制其文本语法错误；完整 Java 诊断文本仍保持明确差异。

本轮继续对照 Java `EPDataFlowEmitterOperator.submitPort`/`submitSignal` 与 `EPLDataflowInputOutputVariations` 的多输入多输出语义：Go 新增链式 `EmitterPorts(name, outputs...)`，沿用稳定命名 `SubmitPort` 代替 Java 数字端口下标；多输出时普通 `Submit` 明确拒绝歧义，单个命名输出仍可直接 `Submit`，未连接但已声明端口提交为空操作，信号则对 captive Emitter 和 custom source 的全部连接输出广播。每个 operator 的有效输入/输出端口及 Go 类型现进入 `Plan.Canonical`/hash，未连出的端口定义变化也不会再共享计划身份。四项新测试固定值路由、无效端口、信号扇出、单端口便利语义和计划身份。Java 的数字端口异常文本及反射式 stream schema 由 Go 命名 fluent descriptor 有意替换；并补上先前遗漏的 `case.dataflow-invalid-graph -> dataflow.graph` 能力映射。

本轮补齐历史 SQL 结果的 typed event 表示边界：`SQLHistoricalProvider` 不再把 Struct/typed JSON/ObjectArray/Avro schema 限制为 map-backed 查询，逐列转换后统一复用 `projectMapToSchema` 再创建 Event；Variant 仍在构造期拒绝，因为它需要已路由的 member Event 而不是数据库列集合。`TestSQLHistoricalProviderMaterializesTypedStructRows` 与既有 MySQL historical/sink round-trip 固定 Struct underlying、数值/布尔转换和真实 driver 行为；完整 SQL dialect/type-code、连接池/事务、DML/Upsert retry 与 SQL multi-source/context 组合仍保持 partial。

本轮对 `testdata/compat/capability-manifest.json` 做反向完整性审计，发现 9 个已经标记为 mapped/passing 的 case 只有测试证据而没有 capability 映射，导致能力汇总静默漏计：Named Window unique merge、Variant supertype/interface、Dataflow lifecycle、Dataflow EPStatement/Pattern、aggregate rate、Every completion boundary、两个 MatchUntil/Not 组合和 RowRecog unlimited partition。现已分别挂入 `trigger.table-named-window`、`event.variant`、`dataflow.graph`、`resultset.aggregate-filtered`、`pattern.basic` 与 `rowrecog.match-recognize`；清单达到所有非 `unmapped` case 均有映射。`CapabilityManifest.Validate` 现在会拒绝任何非显式 `unmapped`、却没有 capability mapping 的 case，并由 `TestCapabilityManifestRejectsMappedCaseWithoutCapabilityMapping` 固定，防止后续新增覆盖证据时再次漏入汇总。

本轮补齐 Java `EPLFromClauseMethod` 的首个 Go-native 链式入口：`FromMethod`/`FromMethodOn` 通过显式 `MethodProvider`/`MethodProviderFunc` 返回已物化 `Event`，不引入 Java 反射、类名查找或 EPL 字符串。方法源可绑定注册的 trigger type，在 live statement 中携带 `Trigger`、变量快照和参数快照，复用普通 Filter/Join/Select 链，并在 direct FAF 中以无 trigger 的一次 snapshot 运行；`TestMethodSourcePollsOnTypedTriggerAndCarriesParameters`、`TestMethodSourceParticipatesInJoin`、`TestMethodSourceFireAndForgetUsesEmptyTriggerSnapshot`、`TestMethodSourceValidationRequiresProviderAndKnownTrigger` 固定三条闭环和负例。Java `TestSuiteEPLFromClauseMethod` 3/3、`TestSuiteEPLFromClauseMethodWConfig` 5/5 已在 JDK 17/Maven 下通过。Java 的多方法返回形状、N-stream/outer-join、method cache、Context、UDF/script、EventBean/反射类型转换及完整 invalid/性能矩阵仍保持 partial。

本轮继续对照 Java `ConfigurationCommonMethodRef.setLRUCache`/`setExpiryTimeCache`：Go 新增 `MethodCacheConfig`、显式 `MethodCacheKey` 和 `CachedMethodProvider`，按 provider 调用参数计算稳定 key，支持互斥的 LRU/expiry 策略、虚拟 `MethodRequest.Now` 失效判断、配置校验以及命中结果的深拷贝；不复制 Java method reference、反射签名或字符串 method name。`TestCachedMethodProviderLRUReusesAndEvictsByExplicitKey`、`TestCachedMethodProviderExpiryUsesRequestTime`、`TestCachedMethodProviderClonesHitResultsAndValidatesConfig` 复现 Java 的 3-entry LRU、100ms expiry、命中/逐出顺序和结果隔离；完整方法返回类型/N-stream/outer-join/Context/cache performance 矩阵仍保持 partial。

本轮继续收敛 Dataflow `Select` Join：`DataflowJoinOptions.WithLength`/`WithTime` 为指定输入提供 Go-native 长度/虚拟时间窗口，`SelectJoin` 在窗口逐出后按当前组合做增量 diff，支持多行 Cartesian 与 Full/Left/Right Outer 的未匹配转移，并将窗口/条件/保留策略纳入 Plan canonical identity。`TestDataflowSelectJoinLengthWindowsEmitOnlyNewCartesianRows`、`TestDataflowSelectFullOuterLengthEvictionEmitsUnmatchedTransitions`、`TestDataflowSelectJoinTimeWindowExpiresAtExactVirtualBoundary`、`TestDataflowSelectJoinWindowsRejectInvalidDefinitions` 和 `TestDataflowSelectJoinWindowsEnterStablePlanIdentity` 固定正例、虚拟时钟边界、非法配置与 hash 稳定性；Java `EPLDataflowFromClauseJoinOrder`/`EPLDataflowOuterJoinMultirow` 的更完整 representation、old/new 与性能矩阵仍保持 partial。

本轮补齐 Dataflow Join 的输出 representation：`SelectJoinEvent` 以注册 event type 接收 Join projection，将结果物化为 Struct、Map、ObjectArray、JSON、XML 或 Avro 的原生 Event，并在 Build 阶段校验空/未知输出类型。审计同时发现此前 `DataflowSelectOptions` 未进入 Plan canonical，时间窗、snapshot、iterate/group/order、输出事件类型和 preserve-input 策略可能错误共享 hash；现完整编码这些选项并将 artifact schema 提升到 `esper-go-plan/v2`，旧 v1 artifact 会明确拒绝。`TestDataflowSelectJoinEventMaterializesRegisteredRepresentations`、`TestDataflowSelectJoinEventRejectsInvalidOutputType` 和 `TestDataflowSelectOptionsEnterPlanIdentity` 对照 Java `EPLDataflowAllTypes`/`EPLDataflowOpSelectWrapper` 的 representation/wrapper 边界；多 representation 端口推导、完整 invalid/old-new trace 仍保持 partial。

本轮扩展 method source 的返回形状边界：`NewTypedMethodProvider` 将 Go slice 返回按声明 `Schema` 物化为 Event，`NewSingleMethodProvider` 覆盖单值返回，`NewSequenceMethodProvider` 消费 `iter.Seq` 并保持 yield 顺序；因此 struct、map、object-array、JSON 等表示复用同一 Schema 校验和字段访问路径，不复制 Java EventBean/反射方法解析。`TestTypedMethodProviderMaterializesDeclaredReturnShapes` 与 `TestTypedMethodProviderSupportsSingleAndSequenceReturns` 对照 Java `EPLFromClauseMethodDifferentReturnTypes` 的单行/多行形状；Java 的完整泛型元数据、错误诊断、N-stream/outer-join/context/UDF/script 组合仍保持 partial。

method source 的组合矩阵再补两条 Go-native 证据：多个 `FromMethodOn` 可按同一个 typed trigger 独立返回并参与 `JoinMany` 三路条件组合，两个 method source 也可直接做 `FullOuter` 并保留 Null 缺侧；`TestMethodSourceIndependentNStreamJoinMatchesEsperShape` 与 `TestMethodSourceIndependentFullOuterJoinEmitsNullSides` 对照 Java `EPLFromClauseMethodNStream`/`EPLFromClauseMethodOuterNStream` 的独立 method 分支。依赖前一 subordinate method 结果作为下一次调用参数的动态绑定、Context 组合和 UDF/script 仍保持 partial。

本轮补齐 method/historical source 的 Context Fire-and-Forget 组合：分区型 Context 先对 provider 做一次无 trigger 快照，再把已物化事件接入分区内的普通 filter/window/projection，避免按每一行重新轮询 provider；`TestMethodSourceContextFireAndForgetPartitionsRows` 与 `TestHistoricalSourceContextFireAndForgetPartitionsRows` 固定多行、多分区和单次调用边界。initiated-terminated 生命周期 Context、Context Join、依赖前一 subordinate method 结果的动态绑定以及 UDF/script 组合仍保持 partial。

反向 capability 审计又发现 `join.basic` 已是 partial 且已有完整 Go 实现/测试，却没有任何 case 证据；现新增 `case.join-basic`，关联 Java `EPLJoin2StreamSimple`、无 on/where 基础 Join、三流 unique Join 和三流 Full Outer Join 的 6 个 runtime execution，并登记 Go 的 inner/left-outer/N-way/range、aggregate old/new、Context、FAF 和 InsertInto representation 测试。清单校验进一步要求所有非 `planned` capability 至少拥有一条 case mapping，只有 `serde.full` 这类明确 planned 项允许暂时无 case；`TestCapabilityManifestRejectsNonPlannedCapabilityWithoutCaseMapping` 固定该反向约束。

本轮补齐 Java `EPDataFlowEmitterOperator.getName()` 的公共 handle 元数据：Go `DataflowEmitter.Name()` 返回定义级 operator 名称，普通和多端口 captive emitter 均可脱离 map key 自描述，nil receiver 稳定返回空串。该入口并入多端口 captive emitter 对照测试；Java deployment ID 与运行时反射参数 map 仍由 Go 的 Environment ownership、`InstanceID`、`DataflowParameterProvider` 和显式 factory context 替代。

本轮继续对照 Java `EventJsonInheritsTwoLevelWArrayAndObject`、`EventJsonInheritsDynamicPropsParentOnly` 与 `EventJsonInheritsDynamicPropsChildOnly`：`ParseJSON` 现在递归归一化动态 JSON 数值，整数动态属性按 Go `int`/`int64` 暴露，浮点保留 `float64`，超出机器整数范围的整数保留为 `big.Int`；声明的 `big.Int`/`big.Rat` 精度路径不受影响。新增 `TestJSONSchemaInheritancePreservesNestedFragmentsAndDynamicProperties`，覆盖父级嵌套 JSON fragment、子级嵌套 fragment、继承数组索引、父级/子级单独开启动态属性，以及 descendant `SendJSON` 到 parent source 的运行时路由。事件继承的跨 module/configuration 注册、provided-underlying/Avro evolution、supertype/interface coercion、child override validation 和完整 metadata/route/join/query trace 仍保持 partial。

本轮补齐 Java `EPLFromClauseMethodNStream`/`EPLFromClauseMethodOuterNStream` 的 subordinate/lateral method 依赖核心：Go method stream 新增链式 `DependingOn(sourceName...)`，provider 通过 `MethodRequest.Dependency` 获取本次依赖行；Build 对依赖图做稳定拓扑排序，同时保持 `JoinMany` 声明顺序和 projection 下标不变，并拒绝空白、重复、未知、歧义、自依赖、环及不能由 left/right outer 保证的依赖。runtime 按每个兼容上游组合轮询下游 method，并以内部行身份传播传递性血缘，因此即使上游返回内容完全相同的重复行，也不会把不同 invocation 的结果错误扩成笛卡尔积；依赖 map 每次调用独立复制。`TestMethodSourceDependentJoinFollowsTopologicalOrderAndLineage`、`TestMethodSourceDependentChainUsesTopologyAndLineage`、left/right/full outer 测试及 invalid/Plan identity 测试固定反向声明三段链、普通 event→method→method、重复值、触发轮次替换、outer 缺侧和 canonical hash。Java 逐边混合 left/right/full outer 的完整 N-stream join tree 当前仍受 Go 全局 `JoinKind` 模型限制，Context lifecycle、UDF/script 和完整 invalid/性能 trace 继续保持 partial。

本轮补齐 Go method provider 对 Esper `EPLMethodInvocationContext` 的可用元数据边界：`MethodRequest.Invocation` 提供 deployment、statement、source 和 Context partition ID；live Context method source 会在每个分区内携带稳定 statement/deployment 身份和独立 partition ID，FAF method snapshot 则明确标记为非 live partition。`TestMethodSourceInvocationContextCarriesStatementAndPartitionMetadata` 固定两个 Context 分区的调用次数、身份和分区隔离。更完整的 initiated/terminated method lifecycle、Context variable/UDF/script 组合和 Java 反射诊断仍保持 partial。

本轮继续补齐历史 SQL 与 method source 的多源 FAF 组合：独立 historical/method source 各执行一次空 trigger snapshot，再把结果替换为普通 source 进入原有 Filter/Window 链；dependent method 按稳定拓扑顺序对依赖 side 做笛卡尔组合，给每次调用传空 trigger、依赖事件快照，并用 lineage 防止重复值造成伪笛卡尔结果；Context FAF 也在独立 partition runtime 内完成同样调度，并向 provider 暴露 ContextName/partition ID。`TestHistoricalAndMethodFireAndForgetJoinPollsEachSourceOnce`、`TestHistoricalAndMethodContextFireAndForgetJoinPartitionsRows`、`TestHistoricalAndMethodDependentContextFireAndForgetPartitionsRows`、`TestHistoricalAndMethodFireAndForgetSupportsDependentMethodSource`、`TestHistoricalAndMethodContextFireAndForgetSupportsDependentMethodSource` 和 `TestHistoricalAndMethodFireAndForgetDependentChainPreservesLineage` 固定调用次数、过滤/window 保留、依赖绑定、重复值 lineage 和分区结果。Java `EPLDatabaseFAFInvalid` 明确拒绝 SQL result-set Join 与 Context SQL FAF，因此这里属于 Go typed historical/method API 的显式扩展，不计为 Java 共享 trace parity；历史 SQL 更完整的事务、缓存与 selector 矩阵仍保持 partial。

本轮补齐逐边混合 N-stream outer join 的结构能力：新增 `JoinChain(first).InnerJoin/LeftOuterJoin/RightOuterJoin/FullOuterJoin(next, conditions...)`，以 Go/Flink 风格构造左深 join tree，每条边独立保存 kind 与 ON 条件；Build 校验边数、kind、空条件、未来 source 引用及条件必须关联新引入 source，完整 topology 进入 Plan canonical/hash。runtime 按边扩展 partial tuple，保留中间 unmatched 行，而不是把所有源先做全局笛卡尔积后只补首尾。`TestJoinChainMixedOuterPreservesIntermediateRowsMatchesEsper` 固定 left→full 的中间缺侧和后续匹配，`TestJoinChainFourStreamOuterRootsMatchEsper` 覆盖 Java `EPLOuterJoinChain4Stream` 的 S0/S1/S2/S3 四种根方向，`TestMethodSourceJoinChainMixedRightLeftMatchesEsper` 对照 Java `EPLFromClauseMethodOuterNStream` 的 `h1 RIGHT base LEFT h0(base-dependent)`，另有 invalid 测试复现 `S0 FULL H0 LEFT H1(H0-dependent)` 与反向 left dependency 的拒绝，同时确认 `S0 LEFT H0 LEFT H1(H0-dependent)` 合法；Java `TestSuiteEPLJoin` 在 JDK 17/Maven 下通过 42/42。非左深/bushy join tree、剩余 mixed-edge 排列、unidirectional/pattern join、索引计划和完整性能 trace 仍保持 partial。

本轮补齐 Java `EPLJoinUnidirectionalStream` 的核心 Join 语义：两流 API 用 `Join(...).Unidirectional(JoinLeft/JoinRight)` 标记 transient driver，N-stream/JoinChain 用 `JoinSource(stream).Unidirectional()` 标记源；driver 只参与当前事件 probe，不进入持久 side，passive source 仍保留并可在下一次 trigger 改变结果，过滤未命中不触发输出。runtime 覆盖 inner、full-outer、右侧 driver、N-stream、left-deep outer chain、all-driver full-outer、Context partition 隔离、当前 trigger 聚合重算及 iterator 明确拒绝；Build 拒绝 driver 上的 window view，并把 unidirectional flags 纳入 Plan canonical/hash。`TestUnidirectionalJoinRetainsOnlyPassiveSourceAndRejectsSnapshot`、`TestUnidirectionalJoinAggregateRecomputesCurrentTriggerProbe`、`TestUnidirectionalMultiJoinAndFullOuterProbe`、`TestUnidirectionalJoinContextPartitionsPassiveState`、`TestUnidirectionalJoinChainOuterProbe` 和 `TestUnidirectionalJoinValidation` 提供 Go 证据。Java `TestSuiteEPLJoin` 仍为 42/42；Pattern-based unidirectional、完整 output-rate/OM/诊断/性能 trace、非左深/bushy topology 和完整 ordering/self-join/index-plan 矩阵继续保持 partial。

本轮继续补齐 Join 内的关联子查询作用域：JoinSelection 现在把当前 selection 对应的 outer event 与完整 tuple 一起传给 `EvalContext`，JoinCondition 的左右表达式也保留各自 outer side 和所有 source；`JoinField`/`JoinEventValue` 在子查询候选、聚合、分组和嵌套 scope 中继续携带 tuple，因此三源 Join 可从未被 selection 选出的 source 做相关比较。Join→Aggregate 的 group key 与 projection 也补上 tuple scope，支持在聚合子查询中用 `JoinField` 关联外层 source。新增 `TestJoinSelectionCorrelatedSubqueryUsesCurrentOuterSide`、`TestJoinConditionCorrelatedSubqueryUsesLeftOuterSide`、`TestMultiJoinSelectionCorrelatesSubqueryToAnyOuterSource`、`TestJoinAggregateCorrelatedSubqueryCarriesTupleScope` 和 `TestContextJoinSelectionCorrelatedSubqueryKeepsPartitionLocalState`，覆盖 selection/condition、aggregate/group、multi-source tuple 与 Context 新 partition 不回放既有 LastEvent 的边界。Java `TestSuiteEPLSubselect` 19/19 与 `TestSuiteEPLJoin` 42/42 作为本轮 oracle 基线；Join aggregate 的 old/new/outer/cardinality/fragment 矩阵、更宽 Context lifecycle、Pattern/Dataflow 组合和完整共享 trace 仍保持 partial。

本轮补齐 Join 的 post-ON 过滤链：`JoinQuery.Where(predicate)` 以 Flink 风格在 `Select(...)` 后声明可分析的 tuple 级谓词，Build 阶段要求使用显式 `JoinField`/`JoinEventValue` 作用域，Plan canonical/hash 纳入过滤表达式；runtime 对 new/old、LeftOuter 未匹配/匹配转移、普通 Join Snapshot 和 Named Window FAF 快照统一先过滤再投影，并让 Where 内的关联子查询携带完整多源 tuple。`TestJoinWhereFiltersPostJoinOuterRowsAndOldNewTransitions`、`TestJoinWhereFiltersMatchedTuplesAfterOn`、`TestFireAndForgetJoinWhereFiltersNamedWindowSnapshot`、`TestJoinWhereCorrelatedSubqueryUsesCompleteTupleScope` 和 `TestJoinWhereRejectsUnscopedFieldExpression` 固定空侧三值语义、old/new/FAF 过滤、子查询 scope、非法作用域和 Plan identity。该切片对照 Java `EPLOuterJoinLeftWWhere`、`EPLSubselectFiltered` 的 Where/outer-filter 方向；Java 的复杂 Where coercion/index-plan、pattern/unidirectional Where、聚合 Join old/new/cardinality/fragment 与完整 trace 仍保持 partial。

本轮再补齐无 `ON` 条件的 Cartesian Join：`JoinMany(...).Select(...)` 在没有调用 `On` 时将条件集合解释为恒真，而不是构造一个非法零值条件；两流 `Join(left, right)` 的 condition 参数改为可选 variadic，旧的带条件调用保持兼容，且无条件/等值条件进入不同 Plan identity。两源多行交叉组合的 tuple 数量、顺序和 projection 由 `TestMultiJoinWithoutOnProducesCartesianTuples` 与 `TestTwoStreamJoinWithoutConditionProducesCartesianTuples` 固定，对照 Java `EPLJoinJoinWInnerKeywordWOOnClause`/`EPLJoinNoWhereClause` 的基础方向。随后补齐 `JoinQuery.Where` 的 Context live 与 selector FAF 组合：`TestContextJoinWhereKeepsOuterNullFilteringPartitionLocal` 固定两个 partition 的外连接空侧与 old-only 转移，`TestContextFireAndForgetJoinWhereHonorsPartitionSelector` 固定 All/B partition 的过滤 cardinality。多源 FullOuter 也补齐所有 source 的 unmatched 行（包括中间 source），并由 `TestMultiJoinFullOuterEmitsUnmatchedEverySource` 固定 middle/right unmatched 到 complete-match 的 2-old/1-new 转移；多源 outer/cartesian 的完整顺序、索引计划和完整 Java trace 仍保持 partial。

本轮继续补齐聚合 Join 的前置过滤语义：`AggregateStream.Where` 以链式 API 表达 `where ... group by`，Build 要求返回 bool，并对 Join 规则强制使用 `JoinField`/`JoinEventValue` 作用域；Plan description/hash、普通聚合和 Join 聚合增量状态均纳入该 predicate。`TestJoinAggregateWhereFiltersTuplesBeforeGrouping` 固定左外连接的 unmatched/低阈值 tuple 不建组、达标 tuple 建组以及后续 old/new 累计；`TestAggregateWhereFiltersOrdinaryEventsBeforeGrouping` 覆盖普通流过滤；`TestFireAndForgetJoinAggregateWhereFiltersSnapshotTuples` 覆盖 Named Window FAF snapshot，`TestContextFireAndForgetJoinAggregateWhereHonorsPartitionSelector` 覆盖 Context All/B selector；`TestUnidirectionalFullOuterWhereFiltersCurrentDriverTuple` 覆盖全源 transient driver 的 tuple 过滤；`TestInitiatedTerminatedContextJoinAggregateResetsState` 固定 Context 终止后 Join/aggregate state 不泄漏到下一分区。对照 Java `EPLJoinUnidirectionalStream` 的 where/group-by 与 outer/unidirectional 方向；Join aggregate 的 cardinality/fragment、pattern source、输出率/索引计划和完整 shared trace 仍保持 partial。

本轮补齐同一事件类型作为两个逻辑流的自连接对照：`TestSelfJoinUsesIndependentLogicalSidesMatchesEsper` 使用两条独立 `KeepAll` 链验证新事件在两侧同时可见、当前事件自配对、历史事件交叉笛卡尔配对以及不同分组隔离，结果顺序固定为 A1/A1、A1/A2、A2/A1、A2/A2、B1/B1。该证据关闭 Join 看板中的“自连接边界”遗漏，但不代表已经完成完整 self-join 的输出排序、索引计划、聚合/cardinality、Pattern/Context 组合矩阵。

并行完成的下一项 Join 补充对照了 Java `EPLJoinPatterns`/`EPLOuterInnerJoin3Stream` 的 Pattern source：新增 `JoinPatternSource`/`JoinPattern`，Pattern 的多个 typed input 可在 `And`/`Or` 组合后作为 Join 侧，匹配结果以具名 tag Event 物化，Join 条件通过 `JoinPatternField` 读取 tag 属性，Join 侧窗口保留 completed match 并产生 old-stream 驱逐。`TestPatternFilterJoinMatchesEsper` 覆盖过滤 Pattern、AnyJoin、窗口淘汰和 old row；`TestTwoPatternJoinProjectsTagEventsMatchesEsper` 覆盖两组跨事件类型 Pattern、wildcard Event 和 tag projection；`TestPatternJoinRejectsUnmaterializablePattern` 固定无可物化 tag 在 Build 阶段拒绝。

本轮继续对照 Pattern Join 的 timer/unidirectional 组合：`TestPatternUnidirectionalTimerJoinMatchesEsper` 使用 `TimerInterval(...).Unidirectional()` 作为左外 Join 的瞬时 driver，验证部署时钟初始化、无匹配时 count、被动 KeepAll 事件只进入 passive state、下一 tick 的 matched aggregate 以及旧/新聚合行不会跨 tick 累加。运行时将 timer-only Pattern 的无 tag match 以隐藏内部 marker 通过 Pattern 计划校验，但不向 Join schema 暴露 marker；带显式 tag 的 Pattern 仍使用 tag Event 物化。`TestPatternFilterJoinRedeployClearsStateMatchesEsper` 另固定完整过滤/淘汰序列、undeploy 后事件隔离和重新部署空状态。复杂 guard/observer/consumption、Pattern Join aggregate/cardinality 的多行/fragment 矩阵和完整性能/索引矩阵仍保持 partial。
`TestPatternUnidirectionalTimerJoinOutputRateMatchesEsper` 进一步固定 timer driver 经过 `OutputEveryTime` 的双分钟批量刷新，`TestPatternUnidirectionalRejectsPatternResultWindow` 固定 transient Pattern 侧不能再声明结果窗口的 Build-time 诊断；这两项补充不改变复杂 Pattern/Join 矩阵仍为 partial 的结论。

本轮补齐 Java `EPLJoinMultiKeyAndRange` 的五个 execution 对照：`TestJoinArrayCompositePredicateMatchesEsper` 复现数组深值相等与 value 范围组合、`TestJoinTwoPropertyEqualityMatchesEsper` 复现两字段复合等值和同一类型的 A/B 逻辑流、`TestJoinRangeNullDuplicateAndStringBoundsMatchesEsper` 复现可空范围端点、反向范围不匹配、重复 probe 行、字符串边界和 key 约束。Go 的 `numericValue` 现在会安全解引用非 nil 指针/接口后参与数值比较，nil 仍按 Null/不可匹配处理；Java `EPLJoinMultiKeyAndRange` runtime IDs 与三条 Go 对照已登记到 `case.join-basic`。Join unique-index、完整物理 index plan/性能阈值和更宽多键/范围组合仍保持 partial。

同一 Pattern Join 对照继续补上 Java `EPLJoinPatternFilterJoin` 的部署生命周期：`TestPatternFilterJoinRedeployClearsStateMatchesEsper` 覆盖初次部署的完整匹配/过滤/长度淘汰序列、undeploy 后事件不被旧语句接收，以及重新部署后状态从空开始并只产生当前匹配。该测试关闭了 Pattern Join 的部署状态隔离遗漏，但不改变 Pattern unidirectional、复杂 guard/observer/consumption、Pattern Join aggregate/cardinality 和完整性能/索引矩阵仍为 partial 的结论。

本轮继续对照 Java `EPLJoinUnidirectionalStream` 的 Pattern timer execution：`JoinPatternSource(timerPattern).Unidirectional()` 在 `AdvanceTime` 中按部署时虚拟时钟锚点推进，timer-only Pattern 用内部空 tag Event 作为 transient driver，不进入 passive state；Join aggregate 每个 tick 重新计算当前 passive probe，`OutputEveryTime` 可在虚拟时钟边界批量输出。`TestPatternUnidirectionalTimerJoinMatchesEsper` 覆盖 left outer 无匹配/匹配聚合，`TestPatternUnidirectionalTimerJoinOutputRateMatchesEsper` 覆盖两分钟批量，`TestPatternUnidirectionalRejectsPatternResultWindow` 覆盖 invalid view 诊断。非 timer Pattern driver、all-driver Pattern full outer、复杂 Pattern Join aggregate/cardinality 与完整 SODA/OM/诊断/性能 trace 仍保持 partial。

本轮复核 Java `EPLJoinPropertyAccess` 与 `EPLJoinEventRepresentation` 的遗漏并补齐 Go 对照：新增 Go-native `ArrayAt`/`MapAt`/`Property` 组合，覆盖嵌套、索引、映射条件和 projection，以及左外 Join 未匹配侧的 Null 语义；新增 Map/ObjectArray/JSON/XML/Avro 五种 schema 的直接字段 Join，另以 InsertInto 保留 source Event 的 wildcard 证据覆盖 representation 结果。针对 Java 的非唯一索引回归，`TestJoinMapRepresentationSupportsNonUniqueKeys` 固定 100 个重复 key 的 2500 行交叉结果，`TestJoinInsertedWrapperRepresentationSupportsNonUniqueKeys` 固定 InsertInto wrapper 流的高基数 Join 不丢行不崩溃。5 个新增 Java runtime IDs 已登记到 `case.join-basic`；复杂 JSON provided-class、Map/Wrapper 的完整序列化 class identity、Join 索引计划/性能与更宽 representation 组合仍保持 partial。

本轮补齐 Java `EPLJoinUniqueIndex` 的运行时语义对照：`UniqueBy(d2, i2)` 在 Join 被动侧保留每个复合 key 的最新事件，`LastEvent` 与无窗口单向 driver 均可在两流/三流 Join 中按不同条件声明顺序得到同一匹配结果；`TestJoinUniqueIndexRetainsLatestCompositeKeyAcrossDriverVariants` 还覆盖额外等值条件与第三路无条件笛卡尔扩展。Java 的 STATICHOOK 物理索引唯一性断言没有在 Go API 中伪造，物理 query-plan hook、索引选择和性能阈值仍保持 partial。

本轮继续补齐 Java `EPLJoinSelectClause`/`EPLJoinStartStop`：`TestJoinSelectClauseTypesArithmeticAndSnapshotMatchEsper` 验证链式 Join 的字段类型、显式数值提升/除法 projection 和 iterator snapshot；`TestJoinDeployLifecycleResetsWindowStateMatchesEsper` 验证 undeploy 后事件不进入旧语句、重新部署不回放旧窗口且只对当前部署状态 Join。Java 的精确无 view 诊断文本仍未强行映射，保持为 approved difference。

本轮由 Java `EPLJoinCoercion` 对照发现并修复等值数值提升遗漏：表达式级 `EqualValues` 现在对有符号/无符号整数做精确比较，对整数与浮点做数值提升；严格的 `Value.Equal` 仍保留底层类型 identity。`TestJoinRangeAndEqualityCoercionMatchesEsper` 覆盖 int 与 long range 边界、滚动窗口和复合 key，`TestJoinIntegralEqualityCoercionMatchesEsper` 覆盖 long=int Join；`TestExpressionNumericEqualityCoercion` 额外锁定大整数精度和负 signed/unsigned 边界。

本轮同时使用已启动的 `esper-java-mysql` Docker（3306）执行真实数据库门禁：`TestSQLHistoricalProviderMySQLDocker`、`TestSQLSinkMySQLDocker` 和 `TestDBConnectorMySQLDocker` 全部通过，覆盖历史 SQL 读取、SQL sink 以及 DB connector 的 MySQL upsert；测试使用独立临时表并由测试清理，未把 Docker 可用性误写成默认依赖。

本轮补齐 Java `EPLJoinSingleOp3Stream` 的 3 个运行入口：EPL、ObjectModel 和 compile/deploy 变体统一映射到 Go 的 `TestThreeStreamSingleOperationJoinMatchesEsper`。测试使用 Go 链式 `JoinMany`，三侧分别使用 `length(3)`，同时声明 A-B、B-C、A-C 三个等值条件，复现 Java 的乱序补齐、未完成 tuple 不输出、窗口滚动后仍只匹配当前 key、以及三侧 wildcard Event 投影。三个 Java runtime ID 已全部登记；Go API 不复制 Java 的 EPL/OM/编译入口，而以同一语义的静态链式构造作为替代。更宽的三流外连接 cardinality、非左深/bushy 拓扑和索引计划矩阵仍保持 partial。

本轮补齐 Java `EPLJoin2StreamInKeywordPerformance`/`EPLJoin3StreamInKeywordPerformance` 的谓词语义：`TestJoinInKeywordPredicateMatchesEsper` 以 Go `In[string]` + Join tuple `Where` 覆盖两流左右两个单向 driver 方向，以及三流 `p00 in (p10, p20)` 的多行 Cartesian 组合。两个两流 runtime 与一个三流 runtime 已登记；这里确认的是 `in` 的匹配、触发方向和 cardinality，不把 Java 的单索引/多索引物理计划与性能阈值伪装成 Go 语义完成。Go 当前仍通过 `Where` 表达任意 tuple 谓词，显式 `OnSourcesCompare` 保留可分析的二元等值/范围条件。

本轮再补齐 Java `EPLJoinInheritAndInterface` 与 `EPLJoinNoTableName`：`TestJoinInterfaceAndInheritedEventTypesMatchesEsper` 用显式 parent schema 和 `AccessorExplicit` concrete struct 验证接口/继承事件可进入父类型 Join，并保持 `a=b` 的属性解析；`TestJoinUnqualifiedPropertyMappingMatchesEsper` 用 Go 链式双条件 Join 对照 Java 的未限定 `symbol/theString` 与 `volume/longBoxed` 映射，覆盖非匹配候选和匹配 projection。Java 两个 runtime ID、源文件和 Go 测试已登记；Go 不复制 EPL 的“无表名”文本规则，而以 source-indexed、类型可检查的字段表达式提供同一语义。派生视图的专用 source/批次语义已在后续切片补齐。

本轮完成 Java `EPLJoinDerivedValueViews` 的专用派生视图 source/批次触发模型：新增 `LengthBatch`、`AggregateStream.AsJoinSource`/`JoinAggregateSource`，把普通聚合结果物化为可参与链式 Join 的 schema-bound derived Event；`TestJoinDerivedLinearRegressionBatchSourceMatchesEsper` 使用两个独立的 `length-batch(3)`/`length-batch(2)` + `Linest` source，确认批次未齐时不输出、批次齐备后 projection 类型与 signum 结果正确。`AsJoinSource` 会克隆输入 node，确保同一个 Go `From` 构造的多个 derived source 拥有与 Java 多视图实例等价的独立窗口状态；原 Java 的两事件 no-output 边界也由该测试覆盖。派生 source 当前限定为普通 aggregate（不包装 Join aggregate/副作用），更宽的 derived method/Context/outer/serialization 组合仍保持 partial。

本轮补齐 Java `EPLOuterInnerJoin3Stream` 的一个关键混合拓扑：`TestJoinChainFullThenInnerPreservesIntermediateOptionalRowsMatchesEsper` 用 `JoinChain(...).FullOuterJoin(...).InnerJoin(...)` 复现 `s0 FULL OUTER s1` 后以 `s1 INNER s2` 收敛的行为，覆盖 `s1+s2` 先到时保留 Null 的 `s0` 中间行、`s0` 到达后的替换、三侧不同到达顺序以及缺少 `s1` 时不提前输出。该 Java 类的 6 个 full/left/right 变体 runtime 已全部登记到同一语义切片；Go 仍保持链式、source-indexed 构造，完整 4/5/6/7 流 outer cardinality、bushy 拓扑及物理计划矩阵继续保持 partial。

本轮补齐 Java `EPLJoin20Stream` 的高路数 readiness 边界：`TestTwentyStreamLastEventJoinMatchesEsper` 用 Go `JoinMany` 构造 20 个独立的 `LastEvent` source，每路先按固定 ID 过滤，前 19 路到达时保持静默，第 20 路到达后才产生一条完整 tuple。该 runtime 以独立 `case.join-20-stream` 登记，避免把生成 EPL 文本或 Java 物理计划细节误写成 Go API；更高路数的 outer/cardinality、索引选择和性能矩阵仍保持 partial。

本轮补齐 Java `EPLOuterInnerJoin4Stream` 的 6 个四流拓扑 runtime：`TestFourStreamMixedOuterVariantsMatchesEsper` 用 Go `JoinChain` 按 Java 的 middle/sided/star 变体复现 source-indexed edge 条件，覆盖正向/反向链和四流全部匹配的 projection。该类的 6 个 runtime、源文件和 Go 测试已登记；每次到达时的中间 Null 行、old/new cardinality、全事件序列和 bushy/物理索引矩阵仍保持 partial。

本轮补齐 Java `EPLOuterJoin2Stream` 的两流 outer 语义切片：`TestTwoStreamOuterJoinVariantsMatchesEsper` 覆盖 left/right/full 的 Null 行与 matched old/new transition，`TestTwoStreamOuterCompositeAndCoercionMatchesEsper` 覆盖多列条件及 int/long/float 数值提升，`TestTwoStreamOuterRangeAndArrayMatchesEsper` 覆盖 range + where 边界和 typed `[]int` full-outer unmatched/matched cardinality。该类 11 个 runtime、源文件和 Go 测试已登记到独立 case；完整 order/iterator/group-by、多列 OM/compile、event-type 和更宽 coercion 组合仍保持 partial。

本轮补齐 Java `EPLOuterJoinUnidirectional` 的全 driver full-outer readiness：`TestUnidirectionalFullOuterAllDriversEmitTransientRowsMatchesEsper` 以 2/3 流分别验证 A/B/C 每次到达都只产生当前 transient tuple，不把任何 driver 事件错误保留到下一次；并关联已有的单 driver N-way/full outer、tuple Where、timer Pattern 和 invalid validation 对照。该 Java 类 6 个 runtime 已登记；Pattern + Named Window 混合 full outer、精确 SODA/诊断和更宽四流 cardinality 仍保持 partial。

本轮补齐 Java `EPLOuterJoin6Stream` 的六个 root 方向：`TestSixStreamOuterRootVariantsMatchesEsper` 用 Go `JoinChain` 复现六路 source order、left/right edge 条件和完整 keyed tuple 收敛，覆盖 `s0` 到 `s5` 的 root 变体。六个 Java runtime、源文件和 Go 测试已登记；中间 unmatched old/new cardinality、更多到达序列、七流/笛卡尔 outer 及物理索引计划仍保持 partial。

本轮补齐 Java `EPLOuterJoin7Stream` 的 7 个 root runtime：`TestSevenStreamOuterRootVariantsMatchesEsper` 用 Go `JoinChain` 复现 S0–S6 的 source order、left/right edge 和依赖树，并验证七侧同 key tuple 收敛。`EPLJoinKeyPerStream` 的双属性 key 条件、各阶段 Null/old/new cardinality、多行组合、更多到达序列及物理索引计划仍保持 partial。

本轮补齐 Java `EPLOuterJoinCart4Stream`/`EPLOuterJoinCart5Stream` 的 13 个 Cartesian hub/root runtime：`TestCartesianOuterHubVariantsMatchEsper` 以 Go `JoinChain` 覆盖四流 4 个 root、五流 9 个 root/order 变体及对应 left/right edge。当前证据锁定拓扑和同 key 最终 tuple；多行 Cartesian cardinality、中间 unmatched old/new、完整到达顺序和物理索引计划仍保持 partial。

本轮补齐 Java `EPLOuterJoinVarA3Stream`/`EPLOuterJoinVarB3Stream`/`EPLOuterJoinVarC3Stream` 的 14 个三流 outer runtime：`TestThreeStreamOuterVarRootVariantsMatchEsper` 用 Go `JoinChain` 覆盖 VarA/VarB/VarC 的 9 个 root source order、left/right edge 组合，并让每侧两行同 key 收敛为 8 行完整组合，验证多行 cardinality。Java 的 Map 未排序属性、精确复合列矩阵、SODA/compile 入口与 invalid 诊断文本仍作为 approved difference，后续补齐更宽的事件到达序列。

本轮补强 Java `EPLOuterJoinChain4Stream` 的四流 cardinality 对照：`TestFourStreamOuterChainCardinalityMatchesEsper` 覆盖 S0–S3 四个 root、单行/多行/中间缺侧/尾侧多行七种场景，并验证每个新批次的唯一组合数。该测试补充了原有 root 收敛测试没有锁定的多行笛卡尔基数；完整 old-stream、Bushy 拓扑和物理索引计划仍保持 partial。

本轮补齐 Join 性能类 runtime 的 Go 语义基线：`TestJoinPerformanceBaseline` 分别覆盖两流高/低选择率、范围与数值 coercion、三流 inner/outer coercion、五流 readiness 以及 unidirectional range probe，并在有界样本上记录耗时与结果 cardinality。Java 的 10k–100k 负载、JVM 毫秒阈值、merge/nested/index hint、静态 UDF/变量表达式和物理计划不跨语言硬等价，全部作为 approved difference 保留在清单中。

本轮补齐 Java `EPLOuterFullJoin3Stream` 的 full-outer hub cardinality：`TestThreeStreamFullOuterStarCardinalityMatchesEsper` 覆盖单键与复合键、hub 两侧多行组合、下游先到后由 hub 行替换 unmatched 结果，以及复合键不误合并的 mismatch 行；统计同时按行内 source key 出现次数校验 4/2/3/12 与复合 mismatch cardinality。更完整的 Java representation、old/new listener 序列和 iterator 矩阵仍保持 partial。

本轮补齐 Java `ResultSetOutputLimitRowLimit` 的批次 row-limit 语义：修正普通事件、聚合和 Join/Pattern 共用的输出路径，使 `OutputEvery`/`OutputEveryTime` 在整个输出区间合并候选结果后再按链式 `OrderBy`、`Limit`、`Offset` 排序截取；快照输出继续从当前窗口/分组状态重建后应用 row limit。`TestOutputEveryAppliesOrderLimitAfterBatchMatchesEsper` 对照五事件窗口的 order+offset，`TestOutputEveryTimeAppliesOrderLimitToAggregateInterval` 对照未分组聚合时间批次，`TestOutputSnapshotEveryAppliesGroupedOrderLimitMatchesEsper` 对照分组 snapshot 的 top-N。Java 的变量 limit/offset、ObjectModel 入口、context termination 优化、row-per-group/row-for-all 全矩阵和 invalid diagnostics 仍未完成，不能把本轮解释为完整 output parity。

随后复核 `ResultSetOutputLimitAfter` 的执行清单，发现事件数 after、固定时间 after、月历 after、after+every 和 after+when/then 五个 runtime 已由 `TestOutputAfterEventCountMatchesEsperActivation`、`TestOutputAfterTimeAndTimedEveryUseVirtualClock`、`TestOutputAfterCalendarUsesCalendarBoundaries`、`TestOutputAfterComposesWithEventEvery`、`TestOutputWhenGatesAndUpdatesVariables` 覆盖，但此前遗漏了 capability 映射，现已补登记。snapshot-variable activation 仍是明确未覆盖项，继续保持 partial。

本轮实现 Java `ResultSetOutputLimitFirstHaving` 中缺失的 `output first every` 语义：新增 Go 链式 `OutputFirstEveryEvents` 与 `OutputFirstEveryTime`，首次满足 HAVING 的结果立即输出，随后分别按已接收输入事件数或虚拟时间间隔节流；`TestOutputFirstEveryEventsWithHavingMatchesEsper` 覆盖隐藏/可见聚合结果交替与每两个输入事件，`TestOutputFirstEveryTimeWithHavingMatchesEsper` 覆盖长度窗口淘汰、2 秒虚拟时钟节流和 HAVING 重现。Java 的原始事件、两流 Join 变体、ObjectModel/诊断和完整 first/last 组合仍保持 partial。

本轮继续补齐 `ResultSetAfterWithOutputLast`：新增 `OutputLastEveryEvents`/`OutputLastEveryTime`，前者在 after 激活后按输入事件周期保留最新可见结果，后者在虚拟时钟 tick 刷新周期内最后一行；`TestOutputLastEveryEventsAfterActivationMatchesEsper` 对照 after 4 events + last every 2 events 的聚合和最新值，另有时间 tick 回归固定普通 last-every 行为。对应 Java runtime 已登记；输出 hint 全矩阵、snapshot-variable activation、ObjectModel/精确诊断仍未完成。

随后补齐 grouped output 的 group-isolation 边界：ResultBatch 现在保留聚合 group key 的内部元数据，`OutputFirstEveryTime` 对每个 group 独立节流，`OutputLast`/`OutputLastEveryEvents`/`OutputLastEveryTime` 对每个 group 保留最新行；`TestOutputFirstEveryTimeIsIndependentPerGroupMatchesEsper` 覆盖新 group 在同一时间段仍能首行输出，`TestOutputLastEveryEventsKeepsLatestRowPerGroupMatchesEsper` 覆盖 3-event 窗口的 IBM/ATT 最新值，`TestOutputLastEveryTimeKeepsLatestRowsPerGroupAndSupportsMultikey` 覆盖时间刷新、排序和复合 key 不合并。Java 的 grouped Join、row-remove、iterator、default/all/first/last/snapshot 全矩阵仍保持 partial。

本轮补上此前 capability manifest 完全遗漏的 contained-event 入口：新增 Go-native `Unnest[Parent, Child](stream, Property[[]Child](EventValue[Parent](), "children"))`，要求子类型显式注册 schema，并把父事件数组按原顺序展开为可继续 `Filter`、`Window`、`Aggregate`、`Select`、Join 的事件流；live runtime 和 Fire-and-Forget source snapshot 都支持该节点，窗口淘汰会产生对应 child old-stream。`TestUnnestProjectsContainedStructsMatchesEsper`、`TestUnnestAggregateCountMatchesEsper`、`TestUnnestLengthWindowEmitsContainedOldAndNewRows` 对照 Java `EPLContainedEventSimple` 的属性访问、alone count 与 IR-stream array item 三个 runtime。Java `EPLContainedEventNested/Array/SplitExpr/Example`、多级展开、标量 split/@type、Contained Named Window/Table/FAF、fragment 元数据和完整诊断仍保持 `event.contained` partial。

在此基础上补齐一条 nested 基础证据：`TestUnnestNestedStructsPreserveParentArrayOrder` 将 `Order → []Book → []Review` 两级 `Unnest` 串联，覆盖 Java `EPLContainedEventNested` 的 simple/column-select 形态；这只证明结构体数组的递归组合，不改变 `event.contained` 对 Named Window、Pattern、Subquery、scalar split 和 `@type` materialization 的 partial 结论。

随后增加 scalar-array 入口：`ContainedValue[T]` 与 `UnnestValues` 将 `[]T` 包装成显式注册的 typed child event，`TestUnnestValuesProjectsScalarArrayElements` 对照 Java `EPLContainedEventIntArray` 的有序 int 数组展开。该设计保持 Go 的类型/Schema 边界清晰；String-array `where`、`@type` 事件 materialization、split expression 和标量 contained join 仍未宣称完成。

另外以 `TestUnnestParticipatesInJoin` 验证展开后的 child stream 可以作为普通 Join 左侧，与后续事件流按 child 字段匹配；该证据登记到 Java `EPLContainedJoin`，但不等价于 Java 的 unidirectional、Named Window、outer/self/full-join 全矩阵。

随后补齐 Java `EPLContainedUnidirectionalJoin` 与 `EPLContainedUnidirectionalJoinCount`：`TestContainedUnidirectionalJoinMatchesEsper` 用同一父事件的 `books` 与 `orderdetail.items` 两个 `Unnest` 源构造三路 `JoinMany`，父流以 `Unidirectional()` 作为瞬时 driver，验证 Java 的 `3+1+1` 投影结果；`TestContainedUnidirectionalJoinCountMatchesEsper` 验证每个父事件分别得到 `3,1,1`，不把上一父事件的 child rows 累加到下一次 probe。为保持原规则的 `order by book.bookId, item.amount`，Join 查询现在支持可分析的 `JoinField`/`ResultField` 排序，Join Row 保留 tuple scope，批量输出在最终 flush 时仍可按 Join 字段排序。runtime 仅对同一 logical driver root、未显式挂窗口的 contained expansion 使用 current-event working side，普通 unidirectional Join 的 passive stream 和显式 contained view 仍保持原有保留语义；Contained Named Window/Table/FAF、scalar split/@type、fragment metadata 及完整 TestSuiteEPLContained trace 继续保持 partial。

本轮补齐 Java `EPLContainedNamedWindowPremptive` 的遗漏：`TestContainedNamedWindowUpdatesBeforeNextChildProbeMatchesEsper` 按 Java 的两段语义构造 `Unnest(...).InsertInto("ContainedBookStream")`，再由 `ContainedBookStream.Filter(...)` 与链式 `InsertIntoNamedWindow` 消费；测试同时验证 child 数组顺序和逐 child 路由。前一个 child 的 Named Window mutation 在下一个 child 的关联 `SubqueryExists` 谓词执行前已经可见，最终 `LastEvent` 窗口只保留最高价 `B35`，且只产生一次插入；`TestContainedNamedWindowDirectTriggerTraversalIsPreemptive` 另锁定 Go-native 直接 `OnEvent(Unnest.Filter(...)).InsertIntoNamedWindow` 的 callback 路径。runtime 对无状态的 contained/filter 链采用 callback 式逐 child traversal；含 window/pattern/derived 等持久状态算子的输入继续走批量路径。Java `TestSuiteEPLContained` 在 JDK 17/Maven 3.9.16 下本轮 5/5 通过，`case.event-contained-simple` 已登记对应 runtime ID、Go 测试与 named-window 标签。Nested Named Window filter/subquery/on-trigger、contained Table/FAF、scalar split/type、fragment materialization 和完整共享 trace 仍保持 partial。

本轮补齐 Java `EPLContainedEventNested` 的三个 Named Window 入口：新增 `FromNamedWindowAs[T]` typed source 与 `Stream.AsRecord` 转换，允许 `OrderWindow → Unnest(books) → Unnest(reviews)` 继续沿用 typed chain；`subqueryRootSource` 让 named-window contained source 在验证、快照和 event-stream registry 中回溯到逻辑父源。`TestUnnestNestedNamedWindowFilterMatchesEsper` 覆盖 LastEvent named-window 替换及 child 顺序，`TestUnnestNestedNamedWindowSubqueryMatchesEsper` 覆盖 `SubquerySum` 的 contained snapshot，`TestUnnestNestedNamedWindowOnTriggerMatchesEsper` 覆盖按每个 contained book 触发 Named Window selection。对应 Java runtime `EPLContainedNamedWindowFilter`、`EPLContainedNamedWindowSubquery`、`EPLContainedNamedWindowOnTrigger` 已登记到 `case.event-contained-named-window-nested`；Pattern、column/underlying fragment projection、invalid-expression、contained Table/FAF 和完整共享 trace 仍保持 partial。

output-when 另外补了一条表达式矩阵：`TestOutputWhenExpressionLikeAndThenAssignmentMatchesEsper` 使用 Go `And`/`Like` 变量表达式，在 `OutputAll` 待输出批次满足条件时 flush，并通过 then assignment 读取 `OutputCountInsert`；该用例登记 Java `ResultSetOutputWhenExpression`/`ResultSetOutputWhenThenExpression`，SODA round-trip、同一变量多语句时间协调和精确 invalid diagnostics 仍保持 partial。

本轮补齐 Java `EPLContainedEventArray.EPLContainedStringArrayWithWhere`：`Event` 现在为 contained child 保留 immediate parent 链，新增 `ContainedParentField`/`ContainedAncestor*` 等显式 Go 作用域表达式，并在 Build 阶段校验父层级与字段类型；`TestUnnestStringArrayWhereMatchesEsper` 使用 `UnnestValues`、`InSlice` 与 `Not` 对照 `idsAfter` 排除 `idsBefore` 的四组有序结果。另以 `TestUnnestSplitWordsMatchesEsper` 和 `TestUnnestArrayPropertyCarriesParentFieldsMatchesEsper` 覆盖 Java `EPLContainedSplitWords`/`EPLContainedArrayProperty` 的字符串拆分与父字段投影。普通 contained/filter、preemptive trigger 和 subquery candidate 已传递父作用域；返回 EventBean 的 split、String-array `@type` materialization、scalar contained join、Table/FAF 及完整 contained trace 仍保持 partial。

并行补充的 `EPLContainedEventNested` 对照已纳入同一 nested case：`TestContainedNestedWhereMatchesEsper` 覆盖逐层 child filter 与父级字段作用域，`TestContainedNestedPatternSelectMatchesEsper` 覆盖 contained child 作为 Pattern followed-by 输入，`TestContainedNestedSubselectMatchesEsper` 覆盖保留窗口上的 contained subselect，`TestContainedNestedUnderlyingParentSelectionMatchesEsper` 覆盖显式 immediate parent/grandparent 字段选择，`TestContainedNestedInvalidRules` 覆盖未知字段与越界父层级诊断；Pattern 输入现在按 stream output schema 匹配 contained child，而 statement 输入边界仍接受父事件。Named Window、property-selection alias/fragment materialization、scalar split/@type、Table/FAF 及完整 contained trace 仍保持 partial。

本轮继续对照 Java `EPLContainedSplitWords` 与 `EPLContainedArrayProperty`：新增可分析的 `Split` 表达式，允许 `Split(Field(...), Literal(" "))` 的 `[]string` 结果直接进入 `UnnestValues`；`TestUnnestSplitWordsMatchesEsper` 覆盖句子到有序单词 child 的展开，`TestUnnestArrayPropertyCarriesParentFieldsMatchesEsper` 覆盖标量数组 child 与 `ContainedParentField` 的 `topId/id` 投影。多态 `@type`、脚本/UDF 返回 EventBean 数组、fragment alias、contained Table/FAF 和完整 Java split-expression 表示矩阵仍保持 partial。

本轮再对照 Java `EPLContainedWithSubqueryResult`：`TestContainedSubqueryResultArrayMatchesEsper` 先从 KeepAll Named Window 以 `SubqueryValues[Room]` 取得 typed `[]Room`，通过 `Select(...).InsertInto(...)` 路由到 `PersonAndRooms`，再用 `Unnest` 输出 `personId/roomId`，验证子查询结果集合在事件管道中的类型、顺序和父字段作用域。Java 的 property-selection alias/fragment、多态 `@type`/EventBean 数组、contained Table/FAF 及完整 split-expression 表示矩阵仍保持 partial。

本轮补齐 contained fragment alias 的 Go 对照：`TestContainedNestedParentFragmentsMatchEsper` 同时投影 `ContainedAncestorEvent(2)`、`ContainedParentEvent()` 和当前 child `EventValue[Event]()`，逐行验证 order/book/review 三层 Event envelope 的字段与数组顺序。该证据收窄了 property-selection alias/fragment 的差异范围，但 exact raw representation metadata、跨 Map/JSON/ObjectArray/Avro 的 fragment conversion、`@type` 多态 EventBean 数组和 contained Table/FAF 仍保持 partial。

本轮继续对照 Java `EPLContainedEventSplitExpr.EPLContainedSplitExprReturnsEventBean` 与 `EPLContainedSingleRowSplitAndType`：新增 `UnnestAs[T,V]`/`UnnestEvents[T]`，以显式注册的 target schema 替代 EPL `@type(...)`，并在 Build 阶段校验 target 存在、property element 类型和 source schema。Event-valued split 通过 `NewEvent` 创建 A/B concrete member，`UnnestEvents(..., "ContainedSplitVariant")` 保留具体 `TypeName`、member-only 字段和 immediate parent；raw `map[string]any`、JSON 文本、ObjectArray、Avro record 和 XML 文本分别走目标 Struct/JSON/ObjectArray/Avro/XML schema 的 materialization，未知、空 target 和不兼容 concrete Event 有明确 Build/runtime 错误；`TestContainedTypeMaterializesMapElementsInFireAndForgetMatchesEsper` 另外锁定 named-window FAF snapshot 的 raw map 展开顺序和只读结果。新增 `TestContainedSplitChainPreservesNestedParentScopeMatchesEsper` 用两个 Go `UnnestAs` 串联 sentence→word→character，验证多级 splitter 的输出顺序、Unicode rune、immediate parent 和 grandparent fragment；新增 `UnnestSeqAs`/`UnnestSeqEvents` 适配有序 Go `iter.Seq`，`TestContainedSplitSequenceMaterializesGoIterSeqInOrder` 覆盖 map yield 与 Event yield 两种形态；新增 `validateContainedPropertyExpression` 和 `TestContainedSplitRejectsUnsupportedExpressionForms`，在 Build 阶段拒绝 contained property 中的 subquery、aggregation、previous/prior，保留普通 UDF/Split 表达式。相关证据为 `TestContainedSplitEventArrayPreservesVariantMembersMatchesEsper`、`TestContainedTypeMaterializesMapAndJSONElementsMatchesEsper`、`TestContainedTypeMaterializesMapElementsInFireAndForgetMatchesEsper`、`TestContainedTypeMaterializesObjectArrayAvroAndXMLRows`、`TestContainedSplitChainPreservesNestedParentScopeMatchesEsper`、`TestContainedSplitSequenceMaterializesGoIterSeqInOrder`、`TestContainedSplitRejectsUnsupportedExpressionForms`、`TestContainedTypeRejectsUnknownAndIncompatibleTargets`，并登记为 `case.event-contained-split-type`。Java 的脚本/UDF 执行、wildcard method adapter、其它 collection adapter、精确 EventBean metadata、contained Table/FAF 全矩阵和完整 split/type trace 仍保持 partial；因此 `event.contained` 仍不是全量完成。

本次复核从 `testdata/compat/java-execution-inventory.jsonl` 补出同一 Java 类的第三个 execution：`EPLContainedScriptContextValue`（`java-runtime-e5e8e7be2d9e28ca1fcf`）。该 runtime 现有 Go 等价的 `ScriptProvider` context/argument/contained expansion 测试，但这只是 provider contract，不是 JavaScript/MVEL/JVM 脚本执行器；因此 `case.event-contained-split-type` 和 `event.contained` 仍保持 partial。contained 语义不需要 MySQL，数据库 Docker 仍只在 SQL/connector 门禁显式启用时启动。

本轮新增 Go-native CASE 表达式 builder，对照 Java `ExprCoreCase` 的 24 个 execution：`CaseWhen[T]`/`Case[T]` 表达 searched CASE，`CaseValue[T]` 表达 simple CASE，`When` 按声明顺序追加，`Else` 或 `Build` 结束构造。表达式树保留每个条件、匹配值、结果和默认分支，支持字段依赖、Plan canonical/hash、数组/枚举结果以及包含 `Sum`/`Count`/`Avg` 的 aggregate context；runtime 只求值到命中的分支，Null/Missing 条件不命中，simple CASE 的 Null 与 Null 匹配，数值匹配沿用 `EqualValues` 的跨数值类型比较。Go 结果类型由 `T` 固定，Java 中不同结果类型的隐式 numeric/string coercion 需要调用方显式 `Cast`，无 EPL/SODA 文本编译入口；空分支、nil 条件/结果/value/match 和重复 ELSE 在 Build 阶段拒绝。`TestCaseExpressionsMatchJavaNullAndShortCircuitSemantics`、`TestCaseExpressionsBuildFluentQueriesAndPlanIdentity`、`TestCaseExpressionsPreserveAggregateContext`、`TestCaseExpressionsRejectInvalidBuildersAtBuild` 已登记为 `expr.core`/`case.expr-core-case`，并将 Java OM/compile 与参数化返回类型差异保留为 partial。

本轮二次复核补齐两项此前容易漏记的声明能力。第一，针对 Java `EPLSubselectMultirowGroupedUncorrelatedIteratorAndExpressionDef`（`java-runtime-fe88aa9f7b78478355ae`），Go 增加无参数命名表达式 `DefineExpression[T]` 与 `ExpressionRef[T]`：引用在当前事件、变量、参数、聚合组和 Engine scope 下求值，可继续接 `EnumTake` 等链式枚举操作；Build 阶段校验未知引用、跨 Environment 引用、返回类型、重复定义和循环依赖，定义/引用描述进入 Plan canonical/hash。`TestNamedExpressionReferenceEvaluatesAndEntersPlanIdentity`、`TestNamedExpressionReferenceRejectsInvalidDependencies`、`TestNamedExpressionGroupedSubquerySnapshotAndEnumChainMatchEsper` 固定运行、依赖诊断、Named Window grouped subquery 快照和同一表达式多次引用。Java 参数化 declared expression、模块/部署可见性、生命周期/替换和完整 iterator/cardinality/fragment trace 仍未完成。

第二，针对 Java `EPLScriptExpression`、`EPLScriptExpressionConfiguration`、`EPLScriptExpressionDisable`、`EPLScriptSandbox*` 及 contained 的 `EPLContainedScriptContextValue`，Go 增加 `ScriptContext`、typed `RegisterScript[T]`/`DefineScript[T]`、dynamic `RegisterValueScript`、`ScriptCall[T]`，保留 dialect、参数 Value 状态、当前 EvalContext、provider-local attributes 和 Build-time 结果/参数/环境校验；provider 是显式 Go callback，不伪造 JavaScript/MVEL/JVM runtime。`TestScriptProviderContextAndContainedExpansionMatchEsper`、`TestScriptProviderPlanIdentityAndRuntimeNull`、`TestScriptProviderAttributesPersistAndPreserveNull`、`TestScriptProviderRejectsMissingAndMismatchedDefinitionsAtBuild` 覆盖 contained 返回值、异常 Null、attribute 状态、canonical identity 和 invalid path；Java `TestSuiteEPLSubselect`、`TestSuiteEPLScript`、`TestSuiteEPLScriptWConfig` 本轮共 24/24 通过。脚本仍是 `partial`：尚缺 Java dialect/runtime/parser、参数化声明表达式、事件返回/泛型 metadata、异常传播策略、取消/超时/内存配额、sandbox/process isolation、脚本与 UDF/Pattern/Context/Dataflow/FAF 的组合及完整 Java trace；不能把 provider contract 当作脚本语言兼容。

外部依赖口径再次明确：命名表达式、CASE、contained、脚本 provider 和普通子查询不需要 MySQL；只有 `connectors/db`、历史 SQL、SQL sink 及 Java `TestSuiteEPLDatabase`/官方 SQL fixture 需要真实数据库时，才启动 `esper-java-mysql`，通过 `ESPER_MYSQL_DSN` 显式启用测试。MySQL 不进入核心包默认测试依赖，容器启动、fixture 导入、ready 检查、版本/digest、清理和失败 disposition 必须记录在测试报告；没有数据库时 SQL integration test 应明确 `Skip`，不能静默改成 fake pass。

本轮复核又补了三处容易遗漏的实施细节：`ScriptContext` 的 provider-local attribute 读写现在有持久化 Null/状态回归，`ScriptCall` 保留 nil AST 参数并在声明了参数类型时给出明确的 required 诊断，nil 参数类型也能安全进入 canonical；命名表达式首次展开会修改 AST，因此 Environment 的 Build 入口增加了同环境串行化保护，保留并发构造 Plan 的安全承诺。能力清单补全了 Java 脚本 configuration/disable/sandbox execution 的 runtime 资产，并明确它们仍属于待实现的 JavaScript/MVEL/JVM、sandbox、资源限制与诊断差异；Go `ScriptProvider` 只承诺 Go callback contract。以上验证不需要 MySQL，后续 SQL/connector 门禁仍按上一段的 Docker/DSN 规则执行。

本轮继续对照 Java `ExprCoreBitWiseOperators` 的 3 个 execution（`java-runtime-b0d354033a204970cb26`、`java-runtime-7819faeecbb3e817d49d`、`java-runtime-e1ecced8a0b539b7ba84`）：新增 `BitwiseAnd`/`BitwiseOr`/`BitwiseXor` 与 `Binary*` aliases，另提供 `Bitwise*Of`/`Binary*Of` 适配 primitive/boxed 指针和接口值，统一支持整数和布尔类型；整数结果保留 Go 声明宽度/符号，布尔值按 Esper 的二元 AND/OR/XOR 求值，Null/Missing/空 boxed 值传播为 Null，泛型约束在编译期排除字符串/浮点/对象，nil operand 在 Build 阶段拒绝。`TestBitwiseExpressionsPreserveIntegerWidthAndBooleanSemantics`、`TestBitwiseExpressionsPropagateNullAndRejectInvalidBuilders`、`TestBitwiseExpressionsHandlePrimitiveAndBoxedOperands` 已加入 `case.expr-core-bitwise`；Java `TestSuiteExprCore` 27/27 通过。Java EPL/SODA syntax、完整数值 promotion 和 ExprCore type/diagnostic 矩阵仍保持 `expr.core` partial。

本轮继续对照 Java `ExprCoreCurrentTimestamp` 的 3 个 execution（`java-runtime-c1c1fd3dc31af4864a50`、`java-runtime-96c8b8cb4cf36a523669`、`java-runtime-5b126fe7fb865be8b293`）：新增 Go-native `CurrentTimestamp()`，返回 `int64` epoch milliseconds，读取统一 `EvalContext.Now`，因此虚拟时钟下同一事件内多次引用一致，`Add(CurrentTimestamp(), Literal[int64](1))` 保持 Java 的毫秒整数算术。`TestExpressionArithmeticConditionalAndTimeFunctions` 固定直接表达式截断语义，`TestCurrentTimestampExpressionUsesVirtualClockAndStablePlan` 固定 live statement 的 100/999ms 输出、结果类型和 canonical/hash 稳定性，并登记为 `case.expr-core-current-timestamp`。Java 的 EPL/SODA/compile 文本入口由 Go 链式 API 取代，精确事件属性命名/编译诊断仍保持差异；当前切片不需要 MySQL。

本轮继续对照 Java `ExprCoreCoalesce` 的 7 个 execution（`java-runtime-6464df255cd4e23a89e7`、`java-runtime-0a352fd0c30e1b3a175c`、`java-runtime-0ee4f2c3f1d9129e2598`、`java-runtime-7cac5278b06087f8e7f2`、`java-runtime-9341a1983c1f4fb3fdda`、`java-runtime-c9475d47ffbc6275e330`、`java-runtime-32ea122af7f0205bcb19`）：新增安全的 `Coalesce[T]` 与显式结果类型 `CoalesceOf[T](...Expr)`，按第一个非 Null/Missing 值返回；nullable numeric pointer 会解引用后按 byte/short/int/long/float/double 只向上转换，typed-nil boxed 值被跳过，Bean/Pattern Event 保留原始身份，`CoalesceOf[any](null,null)` 覆盖 Java all-null 的动态结果。`TestCoalesceExpressionsMatchJavaNumericAndNullSemantics` 固定 long/double promotion、全 Null 与 all-null 类型，`TestCoalesceExpressionsPreserveBeansAndRejectInvalidBuilders` 固定 Bean、Missing、typed-nil、零/单 operand、nil operand、字符串/布尔/窄化/未知字段诊断，`TestCoalesceExpressionsPreservePatternEventIdentity` 固定 Pattern 两分支的事件身份。Build 阶段覆盖普通流、Pattern、source-less、join/context/aggregate 复用的 AST 校验；Java 的 EPL/SODA/compile 文本入口、隐式结果类型推断、精确 boxed metadata 和诊断文本仍保持差异。本轮不需要 MySQL。

本轮补齐 Java `ExprCoreCurrentEvaluationContext` 的 2 个 execution（`java-runtime-2efbbb4fce55aa6ce513`、`java-runtime-ca0799a6d2f163a49f7f`）：新增 Go-native `ExpressionEvaluationContext`、`CurrentEvaluationContext()`、`WithRuntimeURI` 和 `WithStatementUserObject`，普通 live projection 返回 runtime URI、statement name、statement user object 与非 context 的 `ContextPartitionID=-1`，重复引用保持同一 typed metadata，直接表达式求值也规范化为 `-1`。`TestCurrentEvaluationContextMatchesJavaMetadataContract` 覆盖结果 schema 类型、虚拟 Engine metadata 和 user object，`TestCurrentEvaluationContextNormalizesDirectEvaluationPartitionID` 固定脱离 statement 的默认语义；普通 Select 与 Join projection 使用同一 metadata path。Java EPL/SODA model 入口、aggregate/pattern/filter/FAF/trigger 的完整 metadata 传播、精确 context lifecycle payload 仍保持 partial；本轮不需要 MySQL。

本轮补齐 Java `ExprCoreAndOrNot` 的 3 个 execution（`java-runtime-e48bf14356e3aeb838b5`、`java-runtime-c63d599754acde7bb4dc`、`java-runtime-b9d938f2dc52682b77c0`）：补充逻辑表达式的三值 truth-table 对照，`And`/`Or`/`Not` 在 primitive、boxed、Null/Missing 值上保持 Esper 传播规则，`Property[T]` 对 nullable bean pointer 在 typed boundary 解引用，变量更新仍在后续事件中原子可见。`TestLogicalExpressionsMatchJavaThreeValuedTruthTables` 覆盖组合逻辑、boxed pointer、Null、Plan identity 和 schema type，`TestLogicalExpressionVariableUpdatesMatchEsper` 覆盖 Java `not thing.contains(theString)` 的变量更新序列。本轮不需要 MySQL；Java EPL/SODA 文本入口、完整表达式诊断与更多上下文组合仍保持 partial。

本轮继续对照 Java `ExprCoreInstanceOf` 的 5 个 execution（`java-runtime-f57ec2f2f04aad28961d`、`java-runtime-fc4c87ba7677b72cab9a`、`java-runtime-c5af420270464e5633f7`、`java-runtime-0423d81802ef0e7eb910`、`java-runtime-f446c95462162907b0c6`）：以 `InstanceOf[T](Expr)` 覆盖具体值、Null、interface/pointer 实现和动态 interface 属性，并用 live projection 固定 bool result schema、三种 item 类型及同一 Plan 的 canonical/hash；Java 多目标 `instanceof` 由 Go `Or` 组合表达。Java EPL/SODA/compile 文本入口、primitive-wrapper alias、optional dynamic property 与精确诊断矩阵仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreExists` 的 4 个 execution（`java-runtime-d77c035088ce635538c7`、`java-runtime-4202039fb2f1ea65fd49`、`java-runtime-8fa7f5076dde9d791d08`、`java-runtime-afd826c7d955eb538001`）：新增动态 Map schema 的 `Exists` 对照，覆盖声明字段、动态字段、缺失字段、显式 Null、嵌套 Map、Plan canonical/hash 与 live projection；并将现有 `Cast` 数值/字符串转换和 Null 保留测试纳入同一 Java case。Go 明确 `ValuePresent`/`ValueNull`/`ValueMissing` 的 presence contract，Java `?` optional-property 语法、SODA/compile 文本入口、完整 nested/indexed/mapped getter 和 exact compiler diagnostics 仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreTypeOf` 的 5 个 execution（`java-runtime-eeeacbe2c7669e1e1207`、`java-runtime-e2b1dfb02a3ef7fb8ba3`、`java-runtime-7811d3bb8fcadbda3937`、`java-runtime-a4ad7a90bf65d6ae817f`、`java-runtime-c3573f2b2f4f979ab963`）：`TypeName(Expr)` 对 schema-bound `Event` 返回 Esper 事件类型名，对普通动态值返回 Go reflect type string，Null 保持 Null；`TestTypeNameExpressionsMatchDynamicValuesAndEventTypeIdentity` 覆盖 Map 动态 int/string/null、Event envelope、声明字段、结果类型和 Plan identity。Java fragment/variant/POJO simple-name、representation-specific metadata 与 invalid parser diagnostics 仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreMath`、`ExprCoreMathDivisionRules` 和 `ExprCoreBigNumberSupportMathContext` 的 17 个 execution（`java-runtime-405fa04b5f1af20f1e13`、`java-runtime-a3016beb7537aaf1c737`、`java-runtime-babbfc6755a26b62b22c`、`java-runtime-6af819ef980958c5a674`、`java-runtime-3cc6c889919a4bf72ace`、`java-runtime-dd0e4d664e434a26b9eb`、`java-runtime-3896bd31e1788b516b8b`、`java-runtime-3d7b82c49ee5cf63d30b`、`java-runtime-64cc57ddcc9ffb00533c`、`java-runtime-548f20325f0a97b516be`、`java-runtime-521ac552224e56149c6d`、`java-runtime-3421f39650853c11b513`、`java-runtime-7bb6729ae48117fe847c`、`java-runtime-5d1b5d5b573bc6b7f0c7`、`java-runtime-3ff86e431005db558919`、`java-runtime-1b8818c11b408cc0be72`、`java-runtime-937dc1691e1ff83298d7`）：新增 mixed numeric `AddOf`/`SubtractOf`/`MultiplyOf`/`ModuloOf`、可配置 `DivideWithOptions`、精确 `AddExact`/`SubtractExact`/`MultiplyExact`/`DivideExact` big.Int/big.Rat，并修正枚举数值转换对 nullable pointer/interface 的解引用。`TestMathExpressionsMatchJavaPromotionAndDivisionPolicies`、`TestExactMathExpressionsPreserveBigIntegerAndDecimalValues`、`TestMathExpressionsBuildTypedPlanAndLiveProjection` 覆盖 promotion、integer/floating division、divide-by-zero、Null、BigInteger/BigDecimal、短/字节、invalid builder 和 Plan/live 结果类型；随后又把 configurationError 校验提升为递归 AST 门禁，避免嵌套 invalid math 被有效父表达式遮蔽，并将 DivisionOption 进入 canonical。Java 的默认类型推断、EPL/SODA compile path、MathContext DECIMAL32 rounding/scale、BigDecimal 序列化和完整 BigNumberSupport comparison/aggregate/join matrix 仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreMath`、`ExprCoreMathDivisionRules` 和 `ExprCoreBigNumberSupportMathContext` 的 17 个 execution（`java-runtime-405fa04b5f1af20f1e13`、`java-runtime-a3016beb7537aaf1c737`、`java-runtime-babbfc6755a26b62b22c`、`java-runtime-6af819ef980958c5a674`、`java-runtime-3cc6c889919a4bf72ace`、`java-runtime-dd0e4d664e434a26b9eb`、`java-runtime-3896bd31e1788b516b8b`、`java-runtime-3d7b82c49ee5cf63d30b`、`java-runtime-64cc57ddcc9ffb00533c`、`java-runtime-548f20325f0a97b516be`、`java-runtime-521ac552224e56149c6d`、`java-runtime-3421f39650853c11b513`、`java-runtime-7bb6729ae48117fe847c`、`java-runtime-5d1b5d5b573bc6b7f0c7`、`java-runtime-3ff86e431005db558919`、`java-runtime-1b8818c11b408cc0be72`、`java-runtime-937dc1691e1ff83298d7`）：新增 mixed numeric `AddOf`/`SubtractOf`/`MultiplyOf`/`ModuloOf`、可配置 `DivideWithOptions`、精确 `AddExact`/`SubtractExact`/`MultiplyExact`/`DivideExact` big.Int/big.Rat，并修正枚举数值转换对 nullable pointer/interface 的解引用。`TestMathExpressionsMatchJavaPromotionAndDivisionPolicies`、`TestExactMathExpressionsPreserveBigIntegerAndDecimalValues`、`TestMathExpressionsBuildTypedPlanAndLiveProjection` 覆盖 promotion、integer/floating division、divide-by-zero、Null、BigInteger/BigDecimal、短/字节、invalid builder 和 Plan/live 结果类型。Java 的默认类型推断、EPL/SODA compile path、MathContext DECIMAL32 rounding/scale、BigDecimal 序列化和完整 BigNumberSupport comparison/aggregate/join matrix 仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreMath` 的 10 个 execution、`ExprCoreMathDivisionRules` 的 5 个 execution 和 `ExprCoreBigNumberSupportMathContext` 的 2 个 execution（runtime ID 已登记在 `case.expr-core-math`）：新增 Go-native `AddOf`/`SubtractOf`/`MultiplyOf`/`ModuloOf`，允许显式选择混合 native 数值结果类型；`DivideWithOptions`/`DivideFloat` 将 integer-division 与 division-by-zero policy 固化为链式 option，默认浮点除零保留 IEEE `Inf/NaN`，显式 `WithDivisionByZeroReturnsNull(true)` 返回 Null。`AddExact`/`SubtractExact`/`MultiplyExact`/`DivideExact` 使用 `big.Int`/`big.Rat` 的有理数中间结果，覆盖 Java BigInteger/BigDecimal 的大数加减乘除并避免 float64 精度损失；`enumRatFromValue` 同时支持 nullable pointer/interface 数值。`TestMathExpressionsMatchJavaPromotionAndDivisionPolicies`、`TestExactMathExpressionsPreserveBigIntegerAndDecimalValues`、`TestMathExpressionsBuildTypedPlanAndLiveProjection` 覆盖 short/byte promotion、long/float division、modulo、integer division、除零、typed-nil、exact big number、Build diagnostics、schema type、Plan identity 与 live projection。Java `MathContext` 的 DECIMAL32 rounding/scale、默认编译器按 operand 推导结果类型、BigDecimal serialization、EPL/SODA/compile 文本入口与完整混合类型诊断仍保持 partial；本轮不需要 MySQL。

本轮对照 Java `ExprCoreMinMaxNonAgg` 的三个 execution（`java-runtime-26aacdff1f3bf613a79f`、`java-runtime-7483301bc5adaf192da1`、`java-runtime-5b4de929979c2a7c9abf`）：新增 Go-native `MinOf[T](...Expr)`/`MaxOf[T](...Expr)`，用显式结果类型承载 Java `longBoxed`、`intBoxed`、`shortBoxed` 等混合有序操作数，字符串仍按字典序比较；任一 Null/Missing/typed-nil 操作数使标量结果为 Null，数值比较优先使用任意精度有理中间值，结果物化会解引用 nullable 指针并保留目标类型。`TestScalarMinMaxExpressionsMatchJavaNullAndPromotionSemantics`、`TestScalarMinMaxExpressionsBuildTypedLiveProjectionAndPlanIdentity`、`TestScalarMinMaxExpressionsRejectInvalidBuilders` 覆盖三操作数 promotion、字符串、Null、窗口无关 live projection、结果 schema、canonical/hash 和 too-few/nil/unordered Build 负例。聚合 `Min`/`Max` 仍与标量 `MinOf`/`MaxOf` 分离；Java EPL/SODA/compile 的隐式结果推导、BigDecimal/BigInteger 标量规则及完整表示/诊断矩阵仍保持 partial。本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreAnyAllSome` 的 12 个 execution（runtime ID 已登记在 `case.expr-core-any-all-some`）：新增 Go-native `AnyOf`/`SomeOf`/`AllOf` 与 `QuantifierEqual`/`QuantifierNotEqual`/`QuantifierGreater...`，候选可以是标量表达式、array/slice 或 map key，显式展开后按 Java 的 equality/relational 三值规则处理 Null、Missing、空集合和 definite true/false 短路；numeric candidate 使用共享有理比较，因此 BigInteger 与 native integer 可对照。Build 阶段拒绝数组/collection/map 左操作数、空候选、nil candidate 和未知 operator；`TestQuantifiedExpressionsMatchJavaAnyAllSomeTruthTables`、`TestQuantifiedExpressionsExpandArraysCollectionsAndMaps` 覆盖标量/数组/collection/map、short-circuit、空集合、Null、BigInteger、live projection、结果 schema、Plan identity 和 invalid 诊断。Java EPL/SODA/compile 入口、隐式 common coercion、非 numeric collection item 的完整过滤矩阵和表示/类型诊断仍保持 partial；本轮不需要 MySQL。

本轮对照 Java `ExprCoreRelOp` 的两个 execution（`java-runtime-615cb125ab25e488f40c`、`java-runtime-402d95bd69d700735100`）：新增 Go-native `GreaterOf`/`GreaterOrEqualOf`/`LessOf`/`LessOrEqualOf`，允许不同 native numeric 类型、nullable boxed pointer、字符串和 `big.Int`/`big.Rat` 参与同一关系表达式；数值比较优先走有理数中间值，Null/Missing/typed-nil 返回 Null，Build 阶段拒绝 bool、slice 和缺失 operand。`TestMixedRelationalExpressionsMatchJavaNumericStringAndNullSemantics`、`TestMixedRelationalExpressionsBuildLiveProjectionAndPlanIdentity`、`TestMixedRelationalExpressionsRejectInvalidBuilders` 覆盖 Java 的字符串、int/long/float/double、BigInteger/BigDecimal、Null 表格、结果 schema、Plan identity 和 invalid 诊断。现有同类型 `Greater`/`Less` 及 `*Exact` 形式继续保留；Java EPL/SODA/compile 文本入口、隐式 parser 推导和数组/object equality 矩阵仍保持 partial。本轮不需要 MySQL。

本轮继续对照 Java `ExprCoreArray` 的 6 个 execution 与 `ExprCoreArrayAtElement` 的 15 个 execution（runtime ID 已登记在 `case.expr-core-array`）：新增 Go-native `ArrayOf[T]`/`ArrayLiteral[T]` 数组字面量、`ArrayAt[T]`/`ArrayElementAt[T]` 通用索引和 `ArraySize`/`ArrayLength`，支持动态 slice、固定长度 array、嵌套 array、整数表达式索引、显式 numeric element promotion、nullable pointer/interface 元素以及 Null/Missing/越界安全返回；构建阶段拒绝 nil element、错误 element/index 类型和 scalar operand。`TestArrayExpressionsMatchJavaLiteralAndIndexedAccessSemantics`、`TestArrayExpressionsBuildTypedLiveProjectionAndPlanIdentity`、`TestArrayExpressionsRejectInvalidBuilders` 覆盖字面量顺序/空值、固定/二维索引、live projection、结果 schema、canonical/hash、越界和 invalid 诊断。Java EPL/SODA/compile 的隐式 heterogeneous component inference、Avro array 物化、静态方法/UDF array 返回、bean/variable property-path 发现及精确 representation/diagnostic trace 仍保持 partial；本轮不需要 MySQL。

本轮对照 Java `ExprCoreLikeRegexp` 的 10 个 execution（runtime ID 已登记在 `case.expr-core-like-regexp`）：保留 typed `Like`/`RegexpMatch` 并新增 `LikeOf`、`LikeWithEscape`、`LikeOfWithEscape`、`RegexpMatchOf`，支持字符串与 native numeric/dynamic operand 的 Java-style text conversion、全字符串 `%`/`_` wildcard、单 rune ESCAPE、动态 pattern、Null/invalid regex 返回 Null 和 Build 期 bool/non-text/Null escape 诊断。`TestLikeAndRegexpExpressionsMatchJavaWildcardNumericEscapeAndNullSemantics`、`TestLikeAndRegexpExpressionsBuildLiveProjectionAndPlanIdentity`、`TestLikeAndRegexpExpressionsRejectInvalidBuilders` 覆盖 wildcard、numeric LIKE/REGEXP、转义字符、正则转义、Null、结果 schema、canonical/hash 和 invalid builder。Java EPL/SODA/compile 文本入口、Java regex engine 的精确诊断、locale/collation/Unicode 与完整 parser type inference 仍保持 partial；本轮不需要 MySQL。

本轮同时扩展 Java `ExprCoreCast` 对应的 Go cast 边界：`Cast[A,B]` 现在通过统一的 checked conversion 支持 native numeric/boolean/rune、`big.Int`/`big.Rat`、递归 array/slice、`time.Time` RFC3339/ISO、epoch milliseconds、duration 和稳定 string formatting；新增 `CastWithLayout` 与动态 `CastWithFormat`，layout/format 进入 Plan canonical，并保留 Null/Missing、范围溢出和 typed result schema。`TestCastExpressionsMatchJavaNumericBooleanBigNumberAndDateSemantics`、`TestCastExpressionsBuildLiveProjectionAndPlanIdentity`、`TestCastExpressionsRejectInvalidBuilders` 覆盖运行时/投影/负例。Java EPL/SODA/compile cast syntax、精确 Java date parser/diagnostic、timezone/locale 和完整 generic/representation matrix 仍保持 partial；本轮不需要 MySQL。

本轮对照 Java `ExprCoreInBetween` 的 22 个 execution（runtime ID 已登记在 `case.expr-core-in-between`）：新增 Go-native `InOf`/`NotInOf` 与 `BetweenOf`/`NotBetweenOf`；IN 候选支持 scalar、array/slice 和 map key 展开，使用 shared rational numeric coercion 并保留 scalar Null、Null collection/no-row、empty collection、definite match 与 negation 的 Esper 语义；BETWEEN 支持 native mixed numeric、`big.Int`/`big.Rat`、string、反向边界归一化和 Null/Missing false 规则，现有 typed `Between` 统一复用该实现。`TestInBetweenExpressionsMatchJavaCollectionNumericBigNumberAndNullSemantics`、`TestInBetweenExpressionsBuildLiveProjectionAndPlanIdentity`、`TestInBetweenExpressionsRejectInvalidBuilders` 覆盖数组/Map/BigNumber/字符串/布尔/Null、live projection、结果 schema、Plan identity 与 invalid 诊断。Java range/substitution syntax、EPL/SODA/compile path、collection metadata、完整 object/representation 和精确 diagnostic matrix 仍保持 partial；本轮不需要 MySQL。

本轮对照 Java `ExprCoreConcat` 的 execution `java-runtime-1de90a5ae31c0d929e6e`：保留 typed `Concat` 并新增 Go-native `ConcatOf`，按声明顺序拼接 string 与显式 text-compatible numeric 操作数，空字符串不产生字符，Null/Missing/typed-nil 操作数传播 Null；结果类型、操作数顺序、separator literal 和 Build-time empty/nil/bool/collection 诊断均进入测试。`TestConcatExpressionsMatchJavaNullEmptyAndMixedTextSemantics`、`TestConcatExpressionsBuildLiveProjectionAndPlanIdentity`、`TestConcatExpressionsRejectInvalidBuilders` 覆盖 Java 的 `p00 || p01`、三段表达式、`'|'` 分隔和 Null 矩阵。Java `||` 语法、EPL/SODA/compile 入口及精确 compiler diagnostic 仍由 Go fluent builder 差异项保留；本轮不需要 MySQL。

本轮对照 Java `ExprCoreEqualsIs` 的 5 个 execution（`java-runtime-fdef3bed6ec0b16d36db`、`java-runtime-1eaef3b328c31a863b26`、`java-runtime-2fb582ea3ac2dc026c82`、`java-runtime-bc33c9283b85c18481fb`、`java-runtime-d6084d5b7a191cbde6cd`）：新增 Go-native `EqualOf`/`NotEqualOf` 与 Null-safe `Is`/`IsNot`，支持 mixed native/BigNumber numeric coercion、深数组/二维数组 equality、跨类型 Null literal、typed-nil 和三值比较；`TestEqualityExpressionsMatchJavaCoercionArrayAndNullSemantics`、`TestEqualityExpressionsBuildLiveProjectionAndPlanIdentity`、`TestEqualityExpressionsRejectInvalidBuilders` 覆盖同类型/混合类型、数组组件诊断、Null、live projection 和 Plan identity。

本轮同时对照 Java `ExprCoreEventIdentityEquals` 的 5 个 execution（`java-runtime-292f25dc225cb23b7225`、`java-runtime-615c5592f1cdd37d56bf`、`java-runtime-ab2a4ffa3ad0439c39b1`、`java-runtime-746e0698b45d8a884302`、`java-runtime-eeb8e631e5f2bcbe9cd5`）：新增 `EventIdentityEquals`，为每个 ingested Event envelope 分配稳定私有 identity token，使同一 envelope 的复制保持 true、相同 payload 的独立事件为 false，variant routing 保留原事件身份，并覆盖 current/prior live projection 与非 Event Build 诊断。Java `event_identity_equals` 的 EPL/subquery/enumeration aggregate 文本入口及精确 compiler diagnostics 仍保持 fluent API 差异；本轮不需要 MySQL。

本轮对照 Java `ExprCoreNewStruct` 的 5 个 execution（`java-runtime-2c3672932bf32483e86c`、`java-runtime-11e47c2fd7484a672dee`、`java-runtime-9f33182692f1caccb0f9`、`java-runtime-36381f3719c42338b36f`、`java-runtime-2ffbc080e6d6087dadcd`）：新增 Go-native `StructOf`/`StructField` 匿名 map 构造器，字段表达式按声明顺序进入 AST，Null/Missing 仍 materialize 为显式 nil 字段，支持嵌套 struct、特殊字段名、CASE 分支、live projection 与稳定 Plan identity；Build 阶段拒绝重复/空字段名、缺失字段表达式和嵌套 invalid expression。Java 的 Map/Avro/JSON representation-specific fragment metadata、`new {}` EPL/SODA/compile 入口及精确 heterogeneous CASE diagnostics 仍保持 partial；本轮不需要 MySQL。

本轮继续对照 Java `ExprCorePrevious`/`ExprCorePrior` 的 42 个 execution（runtime ID 已登记在 `case.expr-core-previous-prior`）：在已有 `Prev`/`Prior`、`PrevTail`/`PrevCount`/`PrevWindow`、sorted/time-order/grouped view history 和 tag-aware previous 访问基础上，新增 `PrevOf` 动态 integer offset expression；offset 在当前事件上下文求值，负数、分数、Null/Missing、越界返回 Null，offset 子树进入 Plan canonical。`TestDynamicPreviousExpressionsMatchJavaOffsetAndNullSemantics`、`TestDynamicPreviousExpressionsEnterPlanIdentityAndRejectInvalidBuilders` 与既有 previous/view navigation 测试覆盖 length window、arrival/view order、Plan identity 和 Build 诊断。Java time-batch/ext-timed/variable/FAF/join/statistical-view 全矩阵、Prior 的 constant-only 校验及 EPL/SODA/compile 表示仍保持 partial；本轮不需要 MySQL。

本轮补齐 Java `ExprCoreBigNumberSupportMathContext` 的 2 个 execution：新增 `DecimalMathContext`、`MathContextDECIMAL32`、`AddExactWithContext`/`SubtractExactWithContext`/`MultiplyExactWithContext`/`DivideExactWithContext` 与 `RoundDecimal`，以 `big.Rat` 保留 BigDecimal 的精确值，再按 significant-digit precision 和 HALF_EVEN/HALF_UP/HALF_DOWN/UP/DOWN/CEILING/FLOOR/UNNECESSARY 逐项舍入；`TestDecimalMathContextMatchesJavaDecimal32Division`、`TestDecimalMathContextSupportsRoundingModesAndExactNumbers`、`TestDecimalMathContextBuildsTypedLiveProjectionAndPlanIdentity`、`TestDecimalMathContextRejectsInvalidBuilders` 覆盖 Java DECIMAL32 的 `1.6/9.2 = 0.1739130`、精确整数、负数舍入、除零、结果类型、Plan identity 和无效配置。Java compiler-wide MathContext 配置、BigDecimal scale/trailing-zero metadata、序列化、隐式 operand 结果类型以及完整聚合/过滤 MathContext 矩阵仍保持 partial；本轮不需要 MySQL。

本轮补齐 Java `ExprCoreDotExpressionDuckTyping`：新增显式 Go `DuckMethod[T]`，对动态 interface/any receiver 逐次解析导出方法，缺失方法、缺失 receiver 和不兼容返回值按 Java duck typing 物化为 Null；`Method[T]` 保留 Missing 语义用于强类型链，并把 method kind/receiver/参数树纳入 Build 校验和 Plan identity。`TestDuckMethodExpressionsMatchJavaDynamicDotSemantics` 对照 One/Two 两类动态对象及 common 返回对象的五列结果和 Object/float64 metadata，`TestDuckMethodExpressionsPreservePlanIdentityAndRejectInvalidBuilders` 覆盖 method name/receiver 负例。Java 的 compiler duck-typing 配置开关、完整 dot property/enum/collection/array/toArray/overload 矩阵和精确 EPL/SODA/compile diagnostics 仍保持 partial；本轮不需要 MySQL。

本轮补齐 Java `ExprCoreBigNumberSupport` 的 10 个 execution 对照登记，并新增 `MinExactOf`/`MaxExactOf` 标量大数表达式：BigInteger/BigDecimal 现在可参与精确 equality、relational、BETWEEN、IN、算术、过滤聚合、标量 min/max、cast、join 与 quantifier 测试；`TestBigNumberScalarExpressionsMatchJavaSupportMatrix` 和 `TestBigNumberExactScalarMinMaxRejectInvalidBuilders` 固定混合 native/exact 数值、Null 与 Build 负例，既有 exact aggregate/math/relop/in-between/equality/cast 测试作为同一 Java case 的跨域证据。Java median/stddev/avedev 的结果类型矩阵、compiler 推导的 BigDecimal scale/context、representation/serialization 和完整 BigNumber trace 仍保持 partial；本轮不需要 MySQL。

本轮补齐 Java `ExprCoreNewInstance` 的 13 个 execution：Go 以命名 `ValueConstructor[T]` 和 `Construct[T]` 显式替代 Java 的类加载、重载解析与泛型构造器；构造参数的 Missing/Null 状态进入工厂，工厂错误或 panic 安全物化为 Null，构造器名称、参数 AST 和结果类型进入 Plan identity。`MakeArray[T]`/`MakeArray2D[T]` 覆盖动态一维/二维零值数组，`ArrayOf[T]` 继续负责初始化数组字面量；负长度、非整数维度、nil 工厂/参数和嵌套无效表达式在 Build 或求值边界有明确诊断。`TestConstructExpressionsMatchJavaFactoryAndArraySemantics`、`TestConstructExpressionsBuildLiveProjectionAndPlanIdentity`、`TestConstructExpressionsRejectInvalidBuilders` 已登记到 `case.expr-core-new-instance`；本轮以固定 Java 17/Maven 3.9.16 环境复跑 `TestSuiteExprCore`，27/27 通过，其中包含 NewStruct 5 个和 NewInstance 13 个 execution。Java 的反射式任意 class constructor、精确 generic collection descriptor、固定数组类型推导、异常文本以及 EPL/SODA/compile 入口仍保持 partial；本轮不需要 MySQL。

本轮关闭 Java `ExprEnumAggregate` 剩余 Events/Invalid 两个 execution，并扩展已有 `case.expr-enum-aggregate`。`EnumAggregate[T,R]` initialization 从裸值升级为 `Expression[R]`，与 accumulator 一起进入 AST/Plan identity，并在每次 invocation 求值；动态字段/参数 seed、空集合返回 initialization、Null 输入/accumulator 与 2/3/4 参数 `EnumAccumulator`/`EnumElement`/`EnumIndex`/`EnumSize` fold 均由 `expr_enum_aggregate_parity_test.go` 固定。Build 明确拒绝缺失/null-typed initialization 和缺失 accumulator；Java 不兼容 accumulator 类型由 Go generic signature 在调用侧更早拒绝。Events/Invalid 两个 runtime 已新增登记，Scalar 保留原 runtime；Java `TestSuiteExprEnum` 28/28 oracle 沿用本轮实跑结果。

本轮关闭 Java `ExprEnumToMap` 的 3 个 execution：新增 `expr_enum_to_map_parity_test.go`，以 `EnumToMap[T,K,V]` 覆盖事件/标量 element-index-size 双 selector、重复 key last-write-wins、空/Null 输入、部署态 typed map 元数据、pointer K/V 的 null key/value 与 typed builder invalid。具体 K/V 的 Null 按 Go 零值物化，需区分 Null 时使用 pointer/interface 类型。Java `TestSuiteExprEnum` 在 JDK 17/Maven 3.9.9 下 28/28 通过并包含 ToMap 三条 execution；Java key/value lambda arity EPL 诊断、精确 compiler text 和 parameterized Map metadata 保持 fluent API 差异。3 个 runtime 已登记到 `case.expr-enum-to-map`。

本轮关闭 Java `ExprEnumMostLeastFrequent` 的 4 个 execution：新增 `expr_enum_most_least_frequent_parity_test.go`，以 `EnumMostFrequent`/`EnumLeastFrequent`（no-param）和 `EnumMostFrequentBy`/`EnumLeastFrequentBy`（带 selector）覆盖标量/事件/map 投影/invalid 足迹。修复 By 变体返回类型从 `Expression[T]` 改为 `Expression[K]`（返回 key 值而非集合元素），对齐 Java LinkedHashMap map-key 结果语义；tie-breaking 保留插入序。Java null-typed lambda 编译拒绝和精确 compiler error text 保持差异；4 个 runtime 已登记到 `case.expr-enum-most-least-frequent`。

本轮关闭 Java `ExprEnumExceptIntersectUnion` 的 7 个 execution：新增 `expr_enum_except_intersect_union_parity_test.go`，以 `EnumExcept`/`EnumIntersect`/`EnumUnion` 覆盖标量/contained/string-array/union-where/invalid 足迹。修复 set 操作 null/empty right 行为：右侧 Null 或空集合时返回左侧不变（对齐 Java removeAll/retainAll/concat）。Java 事件类型诊断、schema inheritance、SetLogicWithEvents subquery+length-window 组合及精确 compiler error text 保持差异；7 个 runtime 已登记到 `case.expr-enum-except-intersect-union`。

本轮关闭 Java `ExprEnumSequenceEqual` 的 3 个 execution：新增 `expr_enum_sequence_equal_parity_test.go`，以 `EnumSequenceEqual` 覆盖有序 `selectFrom` 投影、标量序列长度/空集合/Null/Null 元素语义、部署态 map 属性投影及 typed builder invalid；Go 的 Null 采用显式 `NullLiteral`，map-backed property 保留事件属性 null 语义。Java 事件集合类型诊断、EPL 输入入口、完整 generic/representation metadata 与精确 compiler error text 保持差异；3 个 runtime 已登记到 `case.expr-enum-sequence-equal`。

本轮关闭 Java `ExprEnumArrayOf` 的 6 个 execution：新增 `expr_enum_array_of_parity_test.go`，以 `EnumArrayOf[T]`/`EnumArrayOfSelect[T,R]` 覆盖标量和事件集合的 arrayOf、元素/索引/size lambda、`selectFrom(...).arrayOf()` 组合、空/Null 输入、部署态 typed projection 及 invalid builder。Go 使用显式切片元素/结果类型和 `EnumSelect` 组合，不复制 Java 的数组 component descriptor、null-typed lambda inference、representation-backed source breadth 或精确 compiler diagnostic；6 个 runtime 已登记到 `case.expr-enum-array-of`。

本轮继续对照 Java `ExprCoreDotExpression` 的 15 个 execution：新增 Go-native `ToArray[T]`，并以 `Property`、`Method`、`MapAt`、`ArrayAt`、`ArraySize`、`EnumSelect` 和显式 typed literal/factory receiver 覆盖对象 equality、枚举方法/嵌套返回、根化 map/index、无参及参数化方法链、数组 size/get、嵌套属性实例、named-window 嵌套方法、集合 select/get/size、toArray、聚合值方法、对象工厂属性和 map-to-string length。`TestDotExpressionsMatchJavaObjectEqualityAndEnumChains`、`TestDotExpressionsMatchJavaRootedMapArrayAndParameterizedChains`、`TestDotExpressionsMatchJavaArrayNestedWindowAndCollectionSemantics`、`TestDotExpressionsRejectInvalidChainsAndPreservePlanIdentity` 已登记到 `case.expr-core-dot-expression`；Java 的静态类方法/枚举常量、反射重载与完整 collection/array/dynamic property matrix、精确无效编译诊断及 EPL/SODA compile path 仍保持 partial，本轮不需要 MySQL。

本轮继续对照 Java `ExprDefineValueParameter` 的值参数 execution 以及 `ExprDefineEventParameterNonStream` 的事件参数边界：Go 新增 `ExpressionParam[T]`/`DeclaredExpressionParam[T]`，`DefineExpression[T]` 自动按定义体首次出现顺序登记参数，`ExpressionRef[T](env, name, args...)` 在调用点绑定参数并保留当前 Event、Join/aggregate scope、Variables、prepared Parameters、Engine 和 subquery correlation。参数化定义的 arity、类型、嵌套调用、Null/Missing 传播和参数元数据均进入 Build 诊断或 Plan canonical/hash；`TestParameterizedNamedExpressionReferenceMatchesJavaValueParameterSemantics`、`TestParameterizedNamedExpressionReferenceRejectsArityAndTypeMismatch` 与既有 named-expression/subquery 测试提供 Go 证据。Java `TestSuiteExprDefine` 在 JDK 17/Maven 3.9.16 下 5/5 suite 通过；本轮只关闭值参数核心切片，事件值/Pattern/Context/subquery multi-row 组合、overload/name collision、module visibility/lifecycle replacement 和完整 iterator/cardinality/fragment trace 继续保持 partial。

本轮同时修复三项由对照测试暴露的运行时遗漏：`Concat` 现在将已收集的操作数写入 AST children，避免字段/参数依赖和 Plan 分析丢失；Engine 在 Environment 先注册 Named Window、Engine 后创建的合法顺序下首次运行时延迟 materialize runtime window；普通投影中的 `First(...).Method(...)` 将当前 Event 视为单行 access-aggregate，而显式空 aggregate group 在 Named Window 删除后仍返回 Null。Go `go test ./...`、`go test -race ./...`、`go vet ./...` 和 `go test ./internal/compat` 均通过。以上修复属于跨切片基础设施，不改变 Java 已验证的空组、移除流和状态回收语义。

本轮继续补齐命名表达式的事件参数边界：`ExpressionParam[EventType]`/`ExpressionParam[Event]` 可作为定义体中的 typed receiver，调用点分别接收当前 `EventValue`、Join `JoinEventValue`、Pattern `PatternEvent` 和 LastEvent 子查询 `SubqueryValue`；`Property` 在参数替换后仍保留字段读取和 Null 传播。`TestParameterizedNamedExpressionReferenceSupportsEventValueParameters`、`TestParameterizedNamedExpressionReferenceSupportsJoinedEventValues`、`TestParameterizedNamedExpressionReferenceSupportsPatternEventValues`、`TestParameterizedNamedExpressionReferenceSupportsSubqueryEventValues` 对照 Java `ExprDefineValueParameterEV`、`VEVE`、`ExprDefineEventParamPatternPOJO` 和 `ExprDefineEventParamSubqueryPOJO`。Map-backed event、Context property、subquery 多行 cardinality、Pattern/Context 组合与完整事件参数 trace 仍保持 partial；本轮不需要 MySQL。

本轮复核补充（Draft 2.19，2026-08-07）：对照 Java `EnumMethodBuiltin`/`EnumMethodEnumParams` 逐项检查当前 Go 枚举构造器的声明元数据，补齐 `countOf` 的无参数 footprint、`groupBy` 的双 selector 三种 lambda arity，并将集合运算和 `sequenceequal` 的参数类型校准为 Java `ANY`；`EnumerationMetadata` 现在覆盖所有现有 Go enumeration constructor 的 collection/element/result type、no-lambda、1/2/3-parameter lambda、双 selector及 aggregate 2/3/4-parameter footprint，返回值的嵌套 slice 不可变副本也纳入测试。新增 `TestEnumerableMetadataCoversAllGoEnumerationConstructors`，并将 Java common 层 footprint 源文件登记到 `case.expr-enum-footprints`。命名表达式新增 Map-backed Pattern 与 LastEvent-subquery 事件参数对照（Java runtime `java-runtime-f99da8bbca4256d388b7`、`java-runtime-9159bf72b2622a56931f`），对应 `TestParameterizedNamedExpressionReferenceSupportsMapPatternEventValues` 和 `TestParameterizedNamedExpressionReferenceSupportsMapSubqueryEventValues`；Map 事件参数的更广 Context/多行/cardinality/fragment 组合仍为 partial。该切片只依赖 Go/Java 对照，不需要启动 MySQL Docker。

并行复核补充：新增 `ContextPatternEvent` 与 `ContextInitiatingEvent` 作为 Context 内声明表达式的 typed Event 参数来源，以及 `SubqueryEvents` 作为多行事件集合参数；另补齐 Map keep-all + where + `NullOnMultiple` cardinality。`TestParameterizedNamedExpressionReferenceSupportsContextPatternEventValues`、`TestParameterizedNamedExpressionReferenceSupportsContextInitiatingEventValues`、`TestParameterizedNamedExpressionReferenceSupportsMultirowSubqueryValues` 和 `TestParameterizedNamedExpressionReferenceSupportsMapSubqueryWhereAndCardinality` 固定当前 Context/窗口/过滤/cardinality 边界，对照 Java `ExprDefineEventParamContextProperty`（`java-runtime-58539b9c66bae857d0e6`）与 `ExprDefineEventParamSubqueryMapWithWhere`（`java-runtime-97b23e465870d3b1d72d`）。Context/多行/fragment 的完整 iterator/cardinality/trace 矩阵仍明确列为未完成差异，本轮仍不需要 MySQL Docker。

本轮继续对照 Java `ClientExtendEnumMethod` 的 11 个 execution（runtime ID 已全部登记到 `case.expr-enum-footprints`）：新增 `RegisterEnumPlugin`/`EnumPluginRef` 与可直接嵌入规则的 `EnumPlugin`，用 typed state factory 映射 Java `EnumMethodState`，用 `EnumArgument`、`EnumLambda1/2/3/4` 和 `EnumPluginStateValue` 保留普通参数、lambda arity、早停和 state-getter 语义；Build 阶段校验输入 footprint、参数类型/arity、结果类型、插件注册依赖和重复/未知定义，注册 footprint 进入 Plan canonical；另补上 `[]string` scalar-any footprint 和 Java 的“仅非 lambda 参数占 state parameter slot”编号规则。`TestEnumPluginScalarMedianFootprintsAndLambdaArity`、`TestEnumPluginEventLambdaAndChainableEventResults`、`TestEnumPluginSingleEventTwoLambdaAndValueIndexParity`、`TestEnumPluginParametersEarlyExitAndTwoLambdaState`、`TestEnumPluginScalarAnyAndStateParameterSlots`、`TestEnumPluginStateGetterDirectRegistrationAndInvalidRules` 覆盖 scalar/event 输入、median、无 lambda 参数、early exit、predicate 返回事件集合/单事件、双 lambda、value+index、state+value 和非 lambda 参数编号。Go API 明确采用 analyzable chain builder，不实现 Java EPL/反射类加载入口；Java 的完整 forge/service method overload、编译器自动 lambda 参数绑定、动态 collection/iterator descriptor、插件配置/生命周期及精确诊断仍保持 partial。本切片不需要 MySQL Docker。

本轮 Java 对照命令 `mvn -pl regression-run '-Dtest=TestSuiteClientExtension,TestSuiteResultSetAggregate' '-DfailIfNoTests=false' test` 已通过：两个 suite 合计 28/28，分别覆盖 `ClientExtendEnumMethod` 的 11 个枚举插件 execution 和 `ResultSetAggregateAccessAggPlugIn` 的 `eventsAsList(..., filter:...)`；Go 对应的枚举插件与 `PluginAggregateAccess` 测试已纳入 capability manifest。该结果只固化 Java oracle 可复现性，不代表其余 forge/configuration/serde/HA、完整 table/context/FAF/join 组合已完成。

本轮继续对照 Java `ClientExtendDateTimeMethod` 的 3 个 execution（`java-runtime-de17ce40295dbb11a09b`、`java-runtime-8daf1b5f51ec9f929f60`、`java-runtime-f86504ad2aafb13202f4`）：新增 `RegisterDateTimePlugin`/`DateTimePluginRef`/`DateTimePlugin`，以 `DateTimePluginOps` 和按输入/结果类型选择的 factory 映射 Java date-time forge，支持 `time.Time`、epoch-millisecond receiver、transform/reformat、footprint、Build/Plan canonical 和 inline extension；`TestDateTimePluginTransformAndReformatMatchJava`、`TestDateTimePluginInvalidRulesAndInlineForm` 固定 2002-05-30 的 roll 与 `day/month/year` 数组结果及 invalid/unknown/duplicate/no-op 边界。Go 仍明确采用 typed fluent API，不实现 Java 类加载/EPL dot 语法；Calendar.roll 的完整字段/溢出矩阵、全部 Java 时间表示 metadata、配置生命周期、精确 compiler diagnostics 和完整 date-time method family 继续保持 partial。本切片不需要 MySQL Docker。

本轮继续对照 Java `ResultSetAggregateFilterNamedParameter.ResultSetAggregateAccessAggPlugIn` 与 `TestSuiteResultSetAggregate` 的配置式 access multi-function 注册：Go 新增 `RegisterAggregateAccessPlugin` 与 `PluginAggregateAccessRef`，将 access 注册类别写入 Environment/Plan canonical，并以普通 factory ref 不能引用 access 插件、access ref 不能引用普通 factory、结果类型不匹配、多 filter、重复注册为 Build/配置负例；`TestRegisteredAggregateAccessPluginMatchesJavaAndPreservesCategory` 覆盖事件保留、`filter` 谓词、稳定 Plan identity 和四行 A/X 输入轨迹。普通方法插件和 access 插件仍采用显式 Go factory/注册 API，不实现 Java classloader、EPL named-parameter parser、serde/HA、完整 iterator/property/table/FAF/context 组合；本轮不需要启动 MySQL Docker。

本轮补齐 Java `ResultSetAggregationMethodSorted` 的 `ResultSetAggregateSortedCFHL` 与 `ResultSetAggregateSortedCFHLEnumerationAndDot`（`java-runtime-c0cf53fe4f33bd12d71f`、`java-runtime-259bd8b7e63eb47539e8`）：Go `SortedAccessExpression` 新增 `LowerEvents`/`FloorEvents`/`HigherEvents`/`CeilingEvents`，保留重复 key 桶的到达顺序，并可直接交给 `EnumLastOf` 做桶内末元素访问；`TestSortedAccessEventBucketNavigationMatchesEsper` 覆盖 10/10/20/30 四事件轨迹、四种边界导航和 duplicate-bucket last。Java 的完整 navigable map collection/iterator/property、table/context/FAF/alias/type/invalid 矩阵仍保持 partial；本轮不需要 MySQL Docker。

本轮继续对照 Java `ResultSetAggregateSortedNavigableMapReference`：`SortedAccessValue` 现在提供 defensive-copy 的 `Keys`/`Buckets`、`IsEmpty`、`HeadMap`/`TailMap`/`Descending`、四类 Entry/Key 导航和 `SortedAccessIterator`，迭代结果与子视图均不共享可变桶；`TestSortedAccessValueNavigableSnapshotAndIteratorMatchesEsper` 固定 10/10/20/30 的 key/bucket、上下界 Entry、视图顺序、迭代和副本隔离。该 Go API 是 typed immutable snapshot，不伪装成 Java 可变 `NavigableMap`；table 侧方法解析的空值/属性链和完整 FAF/context/alias/type/invalid trace 仍保持 partial，本轮不需要 MySQL Docker。

本轮补充 Fire-and-Forget 路径的 sorted access 证据：`TestFireAndForgetSortedAccessSnapshotAndNavigation` 先向 named window 写入 10/20/20/30 轨迹，再通过链式 FAF 查询读取完整 `SortedAccessValue` 快照、`LowerEvent(25)`、`EventsBetween(10,true,20,true)` 和 `NavigableMapReference`；断言快照与执行时窗口状态一致、重复 key 保留同桶事件、descending 视图及 bucket 数量稳定，并明确 FAF 只读查询不产生表/窗口副作用。`testdata/compat/capability-manifest.json` 已将该测试登记到 `case.resultset-aggregate-access`。这只关闭 sorted access 的 FAF snapshot/navigation 子集；Java 的 table/property method chain 空值语义、Context/rollup/FAF/subquery 组合、全量 alias/type/invalid/iterator trace 和可变访问器兼容仍为 partial，不能据此宣称结果集聚合全量对等；本轮不需要 MySQL Docker。

本轮补充 descending snapshot 的 comparator 语义：`SortedAccessValue.Descending()` 及其 `lower/floor/higher/ceiling`、`HeadMap`/`TailMap`/`SubMap` 均按视图方向导航，并支持再次反转回升序；`TestSortedAccessValueNavigableSnapshotAndIteratorMatchesEsper` 增加多候选边界验证。同时补充 `TestFireAndForgetSortedAccessSnapshotAndNavigation`，覆盖 Named Window Fire-and-Forget 的 sorted access、事件桶、between、navigable snapshot 和 map 视图。Java 的可变 collection mutator/stream API、完整 table/context/alias/type/invalid trace 仍保持 partial，本轮不需要 MySQL Docker。

本轮补齐 Java `ResultSetAggregateSortedTableAccess`/`ResultSetAggregateSortedTableIdent` 的 Table 访问子集：`SortedAccessValue` 增加 Go-style 的 `GetEvent(s)`、`Lower/Floor/Higher/CeilingEvent(s)`、`EventsBetween`、`Sorted`/`ListReference` 和 detached `NavigableMapReference`，可由 `TableField` + `Method` 组成可分析链式投影；`Method` 对 `(value, bool)` accessor 的 false 结果物化为 Null，避免空 access 返回 Go 零值。`IntoTable` 部署时为无分组 aggregate 建立空快照行，因而空 `first/last/key/navigation` 与 Java 一样可通过触发查询读到 Null，分组表仍在首个分组事件后建行。`TestSortedAccessTableMethodChainMatchesJavaAndPreservesNulls` 覆盖空/非空、重复 key、first/last/key、exact/get、lower/higher、between、submap、descending、values/count，并已登记到 `case.aggregate-access`；Java 完整 table key/property chain、multi-criteria、grouped table selector、invalid/type/alias/iterator trace 和所有访问方法组合仍保持 partial。本轮不需要 MySQL Docker。

本轮再补齐 `SortedAccessValue` 快照的 `FirstEntry`/`LastEntry`/`Entry`，均返回独立桶副本，覆盖 Java `NavigableMap.firstEntry`、`lastEntry` 和 exact `get` 的 Go-native 映射；现有 `TestSortedAccessValueNavigableSnapshotAndIteratorMatchesEsper` 同时校验重复 key 桶与副本隔离。Java 的可变 entry/values/key-set collection API 仍不直接暴露，继续保持 immutable typed snapshot 设计，本轮不需要 MySQL Docker。

本轮继续对照 Java `ResultSetAggregateSortedMultiCriteria`：新增 `SortedMultiKey`、`NewSortedMultiKey` 和 `SortedAccessByMulti`，以 Go 泛型表达式构造两级 lexicographic sorted key，保留重复完整 key 的到达顺序，并复用 `SortedAccessValue` 的 first/last/lower/higher 导航；`TestSortedAccessMultiCriteriaMatchesEsper` 使用 `E1a/E1b/E4b/E6a/E6b/E8/E9` 轨迹固定 `firstKey=E1a/1`、`lastKey=E9/9`、`lower(E4,1)=E1b/1`、`higher(E4b,-1)=E4b/4`，同时校验 key 的 detached parts。Java 的任意长度 multi-key、完整 table/property alias/type/invalid/iterator/FAF 组合仍保持 partial；本轮不需要 MySQL Docker。

本轮对照 Java `ClientExtendAggregationFunction` 的 13 个 execution（`TestSuiteClientExtension` 14/14 通过）：新增 Go-native `AggregatePluginInputs(...)`，把常量、字段、typed 当前事件和数组参数组成保留 Missing/Null 的 `[]Value` 参数向量，复用现有按组隔离的 `AggregatePluginState`/registered factory；零参数向量、length-window replay、注册引用和 nil input Build 拒绝由 `TestAggregatePluginInputsMatchJavaMultiParameterAndNoParameter` 覆盖，并登记为 `case.expr-aggregate-plugin`。Java 的 EPL plug-in name/classloader、SODA/codegen forge、distinct/star 编译糖、dot-method 返回 descriptor、配置/serde/HA 生命周期和精确 invalid diagnostics 仍保持 partial；本轮不需要 MySQL Docker。

本轮补充 Java `ClientExtendAggregationManagedDistinctAndStarParam` 的链式表达能力：新增 `DistinctAggregate[T](aggregate, input)`，对字段/参数值按 `Value` presence 保留 Null/Missing 的 distinct identity，对 nil input 按 Event identity 表达 wildcard/star；它作为普通 aggregate combinator 参与 AST、字段依赖、分组/窗口 replay、Filter/IntoTable 组合和 Build 诊断。`TestDistinctAggregatePluginMatchesJavaDistinctAndStarSemantics` 固定重复字段去重、窗口淘汰后的重新出现、wildcard 同一底层值的事件身份和嵌套 aggregate 输入拒绝，并补入 `case.expr-aggregate-plugin`。Java distinct/star 的 EPL 编译糖、join/subquery stream-wildcard 限制、forge metadata、codegen/serde/HA 和精确诊断仍为 partial。

本轮复核并补齐 Java `ResultSetAggregationMethodSorted` 的 grouped table selector 缺口：`SelectFromTable` 的按主键投影现在把实际表行绑定到 `EvalContext.Group`，因此 `TableField` 与 `Method` 链能够读取 grouped `SortedAccessValue`；主键不存在时仍生成一行结果，并以 present-but-null 的目标列承载 Java 的 Null 访问语义，触发事件保留为显式 outer scope。新增 `TestSortedAccessGroupedTableSelectorMatchesJavaAndPreservesMissingGroupNulls`，覆盖空 A 组、A 组更新、未存在 B 组、B 组创建后的 first/last/sorted 读取；`testdata/compat/capability-manifest.json` 新增 `case.aggregate-sorted-table-selector`，并把 `ResultSetAggregateSortedDocSample`、`ResultSetAggregateSortedInvalid`、`ResultSetAggregateSortedGrouped`、`ResultSetAggregateSortedFirstLast` 四个此前漏登记的 Java execution 纳入 inventory 映射。Java 的完整 table property/alias/type/invalid/iterator 矩阵仍保持 partial，本轮不需要 MySQL Docker。

本轮继续对照 Java `ClientExtendAggregationMultiFunction` 的 8 个 execution（`java-runtime-dddc14fa4fcbc5c299c9`、`java-runtime-502d46e9a79b152e9f2b`、`java-runtime-79ca5047f6bd47562f38`、`java-runtime-1a634a9ffc6da29055f2`、`java-runtime-f31078232e4c8225676c`、`java-runtime-bbce8948de4e027393fd`、`java-runtime-6e89910ea4ec2fce1869`、`java-runtime-ef9c3e22146d334ec0be`）：Go 新增 `AggregateMultiMethod[T]`、`RegisterAggregateMultiPlugin`、`PluginAggregateMultiRef`、inline `PluginAggregateMulti` 与 `AggregateMultiPluginState`，把 Java 的 multi-function provider 拆为显式方法元数据、typed result、provider-defined `StateKey` 和每个 aggregate group 的共享 state；同一显式 `StateKey` 的 `se1/se2` 共享 single-event state，未指定 key 的独立表达式实例隔离，过滤/局部分组 scope 隔离，state 仍经过 replayable Enter/Leave/Clear 生命周期。`TestAggregateMultiPluginMatchesJavaSharedAccessorAndTypeFamilies` 覆盖 scalar、array、collection、single-event、event-collection、group isolation、shared accessor、factory context、Plan canonical、duplicate/unknown/type invalid，`TestAggregateMultiPluginReplaysLengthWindowLeaveLifecycle` 补齐 length-window 的 old-state replay 与 Enter/Leave 回收；`TestAggregateMultiPluginRegisteredLifecycleAndSharing` 增加参数向量、显式共享/默认隔离、FilterAggregate、分组窗口淘汰和事件 accessor，`TestAggregateMultiPluginInlineIsolationAndPlanIdentity` 覆盖 inline provider 的默认实例隔离与 Plan hash，`TestAggregateMultiPluginValidationBoundaries` 覆盖 provider/method/factory/result type/environment/嵌套 aggregate 的 Build 边界，`TestAggregateMultiPluginTableAccessMatchesJava` 补齐 IntoTable 后的 `[]Event` state materialization 与 trigger 读取；对应 capability case 为 `case.expr-aggregate-multifunction`。Java 的 input-dependent return descriptor、table agent/accessor mode、inlined-class deployment/dependency graph、codegen、serde/HA 生命周期、完整 invalid diagnostics 和 shared Java/Go trace 仍是 partial；本轮不需要 MySQL Docker。

清单复核补充：同一 multi-function capability case 现在同时登记 Java `ClientExtendAggregationMultiFunctionInlinedClass` 的 `ClientExtendAggregationMFInlinedOneModule`/`ClientExtendAggregationMFInlinedOtherModule` 两个 runtime execution（`java-runtime-c7807955cdfdac25a284`、`java-runtime-8eb7eb1afc410849d750`）。Go 的 `PluginAggregateMulti` inline provider 与 Plan identity 已有对照测试；Java 的跨 module inlined-class dependency graph、class-provided deployment 和 `Trie` table agent 仍保持 partial，不能把 inline builder 等同于 Java class injection。

本轮继续对照 Java `InfraNWTableOnMerge`，补齐 Go fluent merge 的单侧分支边界：新增无条件 `WhenMatchedAny(...)`、`WhenNotMatchedAny(...)` 和 `WhenMatchedDeleteAny()`，允许 Table merge 只声明 matched 或只声明 not-matched 分支，同时保留至少一个 clause、条件类型、删除分支和 assignment/schema 校验。`TestSingleSidedMergeBranchesAndConvenienceConstructors` 分别验证 Table 的 insert-only 重复 key no-op、matched-only update、matched-only delete，以及 Named Window 的新事件插入、匹配更新、匹配删除和缺失匹配不产生伪造 new/old 批次；已有条件 merge、无 `where` 和非法规则测试继续作为回归。对应 Java `InfraInsertOnly` 的 10 个 runtime ID 与 `InfraNoWhereClause` 的 2 个 runtime ID 已登记为 `case.trigger-merge-single-sided`/`case.trigger-merge-no-where`，但 Java 的 no-primary-key Table/no-where、wildcard/多重 insert projection、representation metadata、数组/嵌套字段写入、subquery/pattern 组合和完整共享 trace 仍保持 partial。本轮只依赖 Go/Java 静态与运行态对照，不需要启动 MySQL Docker。

本轮继续完成 Java `InfraNoWhereClause` 的 Table 侧：`MergeIntoTableWhen` 对无主键 Table 接受空 key slice，把 Table 的单行空-key状态作为 no-where 匹配；`TestTableMergeWithoutPrimaryKeyUsesExistingRowAsMatch` 覆盖空表 fallback insert、已有行阻止 not-matched 分支、A/B 分支在 reset-event delete-all 后的重新插入，以及 C matched update 的 old/new 结果。带 key 表达式指向无主键 Table 会在 Build 阶段拒绝；Java 的 wildcard/多重 insert projection、representation metadata、嵌套/数组字段写入和完整 shared trace 仍保持 partial。本轮不需要 MySQL Docker。

本轮补齐 Java `InfraOnMergeMatchNoMatch` 的 target-row 作用域：Table merge 的 matched 条件与 assignment 现在将旧行物化为 `EvalContext.Group`，可通过 `TableField` 参与 `Add` 等链式表达式；not-matched 条件/assignment 若引用 TableField 则在 Build 阶段拒绝。`TestMergeMatchedBranchReadsTargetRow` 同时覆盖 Table 与 Named Window 的旧值累加、负值 matched delete、old/new 批次及重新插入，并登记两个 Java runtime ID。Java 的 wildcard select、representation-specific conversion、更多多重 clause 与完整 shared trace 仍保持 partial。本轮不需要 MySQL Docker。

本轮补齐 Java `EPLSubselectMultirowGroupedUncorrelatedIteratorAndExpressionDef` 的 iterator 侧：`TestSubqueryGroupedSnapshotExposesIteratorView` 用 `SubqueryGroupRows`、`EnumTake` 和 `Statement.Snapshot` 对照 `getGroups()`/`getGroups().take(10)` 的分组结果、空集合、listener 与当前快照一致性。Go 保持 immutable typed slice，而不暴露 Java 可变 iterator；完整 iterator/error/cardinality/fragment 诊断仍保持 partial。本轮不需要 MySQL Docker。

本轮继续对照 Java `EPLSubselectMultirow`（`java-runtime-29c2087cc4243e9b7a50`、`java-runtime-64eb1701d14bdbcefc86`）：新增 `TestSubqueryMultirowWindowValuesMatchesEsper`，覆盖事件流 keep-all 的 `WindowValues` 多行属性集合、空 aggregate 的 Null 和延迟部署后 length(3) named-window 的保留顺序；新增 `TestSubqueryMultirowUnderlyingCorrelatesAndPreservesEvents`，用 `WindowEvents` + `SubqueryWhere` 对照 `window(sb.*)` 的外层相关过滤，保留 Go typed `[]Event` 及原始 underlying；新增 `TestSubqueryMultirowEmptyCollectionsRemainEnumerable`，固定 `SubqueryValues`/`SubqueryEvents` 空 slice 与 EnumSelect/EnumCount 链的可组合行为。`testdata/compat/capability-manifest.json` 已登记该 Java source、2 个 runtime ID 和 3 个 Go 测试；Java `TestSuiteEPLSubselect` 在 Maven 下 19/19 通过。Java 的 iterator/error/cardinality/fragment 全矩阵及 Context/Pattern/Dataflow 组合仍保持 partial。本轮不需要 MySQL Docker。

本轮继续修复多行子查询的迭代器快照缺口：`Statement.Snapshot`/`SnapshotWithSelector` 之前重建变量时丢失 statement-owned subquery registry，导致监听器中的无窗口 grouped subquery 有数据而 iterator 视图为空；现在普通语句保留自身 registry，Context 分区通过 `contextPartitionVariables` 保留分区 registry。`TestSubqueryGroupedSnapshotExposesIteratorView` 对照 Java `getGroups().take(10)` 的 listener/iterator 一致性，`TestContextSubquerySnapshotPreservesPartitionRegistry` 验证 Context selector 的 A/B 分区状态和 Null 隔离；对应测试已登记到 `case.subquery-basic`/`case.subquery-context`。错误/cardinality/fragment 全矩阵及更广组合仍保持 partial，本轮不需要 MySQL Docker。

本轮继续对照 Java `EPLSubselectMulticolumnGroupedContextPartitioned`（`java-runtime-43ca7182e3e88cd6cbd3`）：新增 `TestContextGroupedMultirowSubqueryKeepsPartitionLocalGroups`，用 Context 内 unbounded inner stream、`SubqueryGroupRows`、`EnumTake` 和 `LastEvent` outer stream 固定 A/B 分区的分组累计、空分区不回放、后续 inner event 只进入已激活分区，以及稳定 group/aggregate 行顺序。该测试补入 `case.subquery-basic`；Context/Pattern/Dataflow 的完整多列 fragment/cardinality/termination trace 仍保持 partial，本轮不需要 MySQL Docker。

本轮补齐 Java `EPLSubselectMulticolumnInvalid` 的 previous/prior group-key 边界：`validateSubquery` 现在递归识别 `prev`、`prior`、`prev-tail`、`prev-count`、`prev-window` 和动态 previous 变体，在 Build 阶段拒绝它们作为 grouped subquery key；`TestSubqueryGroupByAggregateAndHaving` 增加对应负例。Go 仍保留结构化错误而不复制 EPL 文本诊断，其他 invalid/cardinality/fragment 矩阵继续保持 partial。

本轮继续对照 Java `InfraSetArrayElementWithIndex`（`java-runtime-b0318353bbb7db697423`、`java-runtime-5f3c511e9e701f2720e1`、无效规则 `java-runtime-e52e9c3cfbf7b4923300`）：Go 新增 `SetArrayElement(column, index, value)`，用可分析的链式表达式表达动态 slice/array 下标写入；on-trigger assignment 按声明顺序刷新 working target row，`TableField`/`NamedWindowField` 可读取前一条 assignment 的结果，`InitialTableField`/`InitialNamedWindowField` 保留更新前快照，并允许重复普通列以符合 Java ordered set 语义。`TestTriggerArrayAssignmentsPreserveOrderedWorkingAndInitialValues` 覆盖 Table 与 Named Window 的直接下标、事件下标、重复计数更新和 initial 值；`TestTriggerArrayAssignmentsRejectInvalidBuildersAndRuntimeBounds` 与 `TestTriggerArrayAssignmentsRejectInvalidDefinitionsAndBounds` 覆盖缺失列、非数组列、非整数下标、元素类型不兼容、null array 和运行时越界。Java `TestSuiteInfraNWTable` 本轮 26/26 通过；Java 的精确诊断、wildcard/多重 insert、nested/mapped property、representation、事务/持久化与更广 on-trigger trace 仍保持 partial；本轮不需要 MySQL Docker。

本轮继续对照 Java `InfraInnerTypeAndVariable` 的 6 个 Table/Named Window × ObjectArray/Map/DEFAULT execution（`java-runtime-3303d0922bd2d722fa3c`、`java-runtime-f20972a347aabfcc0260`、`java-runtime-484a8e636d3734b87673`、`java-runtime-76d16e1334c83e6d4002`、`java-runtime-462d190f20742a0c266d`、`java-runtime-8495f57749c2105b15a8`）：新增 `TestTriggerInnerTypeAndVariableBranchesMatchInfraInnerTypeAndVariable`，用 typed `VariableRef`/`IsNull` 表达 null/true/false 三态 not-matched 分支，保留嵌套 inner event 的 `in1/in2` 值，覆盖 matched delete、Table/Named Window new/old、目标 representation 和 merge undeploy/redeploy。Go 将 Java DEFAULT 映射为 struct、MAP/ObjectArray 保留对应 dynamic schema；Avro/JSON、完整 variable service/transaction/persistence 和共享 Java/Go trace 仍保持 partial，本轮不需要 MySQL Docker。

本轮验证：Java 使用 JDK 17/Maven 3.9.16 执行 `mvn -pl regression-run '-Dtest=TestSuiteInfraNWTable' '-DfailIfNoTests=false' '-Dgpg.skip=true' test`，26/26 通过；Go 的 `go test ./...`、`go vet ./...`、`go test ./internal/compat`、`go test -run '^TestTriggerInfraInsertOtherStreamBootstrapMatchesInfraInsertOtherStream$' .`、`go test -run '^TestTriggerMergeInsertOtherStreamRepresentationMatrix$' .` 与 `go test -race -run 'TestTrigger|TestSubquery' .` 均通过，manifest 当前为 190 cases/190 mappings 且 runtime/case/test 引用无缺失。本轮核心与对照测试均不需要启动 MySQL。

本轮补充对账：`testdata/compat/java-execution-inventory.jsonl` 中 `InfraNWTableOnMerge` 的 ordinal 52–57 之前未进入 capability manifest。`case.trigger-merge-property-eval` 覆盖 Table/Named Window 的 contained property evaluation、插入顺序和 repeated update；`case.trigger-merge-delete-then-update` 现由 `TestTriggerInfraDeleteThenUpdateMatchesEsperTargetSemantics` 覆盖 Table 删除与 Named Window `A/10` 保留两种 Java 轨迹，状态从 `partial` 提升为 `mapped`。该提升只关闭这两个 runtime 的已观察行为，不代表完整 merge 事务、未定义 winner 组合或全量 on-trigger trace 已完成；上述切片仍不需要 MySQL Docker。

补充门禁：`go test -race -run 'TestTrigger|TestSubquery' .` 也通过；完整 `go test -race ./...` 仍保留为 CI 分片级资源门禁，不能用本轮定向竞态结果替代。

本轮继续对照 Java `InfraPatternMultimatch`（`java-runtime-d28146beedb9cfdf9dbb`、`java-runtime-ac7258306c8be9b66f8f`）：`TestTriggerPatternMultimatchMatchesInfraPatternMultimatch` 用 `PatternFrom(...).FollowedBy(...).Every().Select(...).InsertInto(route)` 再接 `OnRecord` Merge，覆盖 A1/A2→B1 的双匹配、A3/A4→B2 的双匹配、Table/Named Window composite key 快照和重复结果抑制。该 Go-native 路由链关闭了 pattern multimatch 的行为子集；直接 `OnPattern` 作为 trigger source、跨 statement 原子事务、observer/deployment 生命周期和完整 Pattern/Infra trace 仍为 partial，本轮不需要 MySQL Docker。

本轮补齐 Java `InfraInsertOtherStream` 的表示转换子集（12 个 runtime：ObjectArray/Map/Avro/JSON/JSONCLASSPROVIDED/DEFAULT × Table/Named Window）：`TestTriggerMergeInsertOtherStreamRepresentationMatrix` 验证 not-matched status=0、matched target-row status、Named Window unique replacement 及 side-stream 输出。Go 将 Java DEFAULT 映射为 struct，将 JSONCLASSPROVIDED 映射为 typed JSON struct；`TestTriggerInfraInsertOtherStreamBootstrapMatchesInfraInsertOtherStream` 另覆盖 struct/Map/ObjectArray 的 insert-into bootstrap 与 Table route-before-merge、Named Window merge-before-route 顺序。XML、跨表示 shared trace、rollback/transaction、split-stream、持久化和完整 Java trace 继续保持 partial，本轮仍不需要 MySQL Docker。

同时补充 `TestTriggerInfraInsertOtherStreamBootstrapMatchesInfraInsertOtherStream` 与 statement deployment order：Table 的 bootstrap insert 先于 merge，Named Window 的 merge 先于 unique insert route，复现 Java `InfraInsertOtherStream` 的首事件 status=10/0 与后续 10/11 轨迹。该顺序语义已进入 runtime 回归；跨 deployment/module 依赖、rollback/transaction 和完整 route trace 仍保持 partial。

本轮再对照 Java `InfraInvalid`（`java-runtime-99b2d413519187b56232`、`java-runtime-33b1e837bc8b373bb198`）：补充 `TestTriggerInfraInvalidBuildCases`，并修正 merge Build 校验以拒绝不兼容标量/嵌套事件赋值、not-matched 分支中的 `ThenUpdate`、非法 wildcard/action shape、未知 side stream 和 matched target insert。Go 以结构化错误保持 fluent API 边界，不复制 Java EPL 文本诊断；剩余 invalid 矩阵、表示专属诊断、事务/回滚与完整 trace 仍为 partial，本轮不需要 MySQL Docker。

本轮继续对照 Java `EventJsonUnderlying` 的 6 个 runtime（`java-runtime-1818ac2301761eddd97d`、`java-runtime-fbd34cb62b92c3cf59f0`、`java-runtime-2f41e67a017701e5a62c`、`java-runtime-f3ef98516b1e419b0e16`、`java-runtime-2badae05685b3d562aa8`、`java-runtime-d74ba3d2a59266487cb6`）：新增 `TestJSONUnderlyingMapRepresentationMatchesEsper`，覆盖 dynamic/non-dynamic × 0/1/2 declared fields 的 JSON map underlying。静态 schema 丢弃未声明字段，dynamic schema 保留未声明字段，声明但缺失的字段物化为 nil；Go 对照测试覆盖每个 Java execution 的输入序列和 keep-all 等价快照语义，case 已登记为 `case.event-json-underlying` 并映射到 `event.json-typed`。Java `TestSuiteEventJson` 13/13 通过；provided-underlying/class/list、完整 dynamic/lax 矩阵、EventBean metadata 和共享 Java/Go trace 仍保持 partial。本轮不需要 MySQL Docker。

本轮继续对照 Java `EventJsonProvidedUnderlyingClass` 的 4 个正向 runtime（`java-runtime-aae0939c0d67c340b646`、`java-runtime-4126f323a5b47eb54eca`、`java-runtime-80dfca5c409f842b6777`、`java-runtime-c5eb4152c3818ca69a73`）：新增 Go-native `RegisterJSONFor[T]`/`NewJSONSchemaFor[T]` typed struct 对照，覆盖 Users/Clients 的 provided-underlying 与 create-schema 两种入口、嵌套 slice/struct、JSON tag、UUID、`DateOnly`、`time.Time`、`big.Rat`、`Engine.SendJSON` 和 `RenderJSON`。注册阶段现在拒绝 typed schema 的 dynamic/parent/未知字段/字段类型不匹配，避免构造出无法 materialize 的规则。Java `TestSuiteEventJson` 13/13 通过；class-shape invalid、constructor-default primitive、pattern typed-array 三个剩余分支另行登记，Go 不加载 JVM className/annotation。本轮不需要 MySQL Docker。
本轮收尾对照 Java `EventJsonProvidedUnderlyingClass` 的剩余 5 个 runtime（`java-runtime-bd339067efa3ec4748c8`、`java-runtime-b5849c7303cfdd9418b5`、`java-runtime-d8237e0a5ca5bdd69920`、`java-runtime-b2ab1cc201fcd29a7e73`、`java-runtime-354b7f70ff5c84c7b9a6`）：Go 测试补齐 typed JSON schema 的 dynamic/parent/unknown-field/type mismatch 边界、constructor-style primitive default 在直接解析和 null projection route 下的保留、以及 `EventOne -> (EventTwo* until TimerInterval)` 产生 `[]Event` 后的 provided-underlying materialization。为保持 Java `every ... until timer` 语义，修复 MatchUntil 重启子分支继承外层 tag 的缺口，并使 tag history 合并识别已继承前缀，避免第二个匹配事件丢失或历史重复。Java 的 public class、public default constructor、non-static inner class 反射诊断没有 Go 等价物，按 API 差异记录，不伪造 JVM className/annotation 入口；MySQL Docker 本轮仍不需要。

本轮继续对照 Java `EventJsonTypingClassParseWrite` 的六个 execution：`EventJsonTypingClassSimple`、`EventJsonTypingListBuiltinType`、`EventJsonTypingListEnumType`、`EventJsonTypingVMClass`、`EventJsonTypingClassWArrayAndColl`、`EventJsonTypingNestedRecursive`。Go 新增 `json_class_parity_test.go`，以 `NewJSONSchemaFor[T]` 建立 class-shaped typed JSON schema，覆盖递归 struct、嵌套对象的一维/二维数组和 collection、基础类型列表（含 `rune`/Character、`big.Int`/`big.Rat`）、枚举样 string alias、UUID/LocalDate/LocalDateTime/OffsetDateTime/ZonedDateTime/URL/URI 的 scalar/数组/collection，以及 filled/null/empty/missing/partial 状态、property materialization 和 RenderJSON 精确字符串。

为使这组对照真正可复现，`Event` 现在只对 `ParseJSONWithOptions` 产生的事件保留私有 raw JSON tree：typed numeric 字段可在渲染时保留 `50.0` 等词法，Character/rune 可恢复 JSON string 形态，LocalDateTime/ZonedDateTime 可保留 Java 原始时间文本；schema-backed object 使用声明/Go struct 字段顺序输出，普通 map 的动态字段仍稳定排序。`coerceJSONReflect` 对嵌套 struct 的已声明字段转换失败不再静默丢弃，避免 VM-class 非法值被错误物化为零值。Java `TestSuiteEventJson` 13/13 通过，Go `go test ./... -count=1` 通过；本轮仍不需要 MySQL Docker。

本轮只关闭上述 class/list/VM/nested parse-write slice，不等同于 JSON 能力全量完成。后续仍需对照 named nested fragment 的完整 catalog 生命周期与全表示 metadata、provided-class serialization conventions、recursive schema inheritance/metadata、dynamic strict-lax 全矩阵、BigDecimal/BigInteger 在所有 nested/dynamic 路径的格式规则、schema evolution，以及共享 Java/Go trace；JSON capability 继续保持 `partial`。

本轮 Core JSON 复核进一步对照 `EventJsonTypingCoreParse` 的 12 个 execution 与 `EventJsonTypingCoreWrite` 的 14 个 execution：新增 `json_core_parity_test.go`，以 typed struct 固化 scalar、Character 多字符首 code point、nullable wrapper/primitive 一维与二维数组、enum、`big.Int`/`big.Rat`、`Object`/`Object[]`/`Map` 容器、动态数字词法和 nested/nested-array BigDecimal 写回；所有用例同时校验 underlying materialization 与 `RenderJSON` 字段顺序/null/number lexeme。当前 Core slice 的 Go 定向测试通过，`testdata/compat/capability-manifest.json` 已登记新增测试证据；这仍是 JSON 事件类型核心的垂直对照，不代表 independently registered fragment、完整 dynamic strict-lax、schema evolution、provided-class serialization 或共享 Java/Go trace 已完成。本轮不需要启动 MySQL Docker。

本轮命名 fragment catalog 复核继续对照 Java `EventJsonProvidedClassUsersEventWithCreateSchema` 与 `EventJsonProvidedClassClientsEventWithCreateSchema`：新增 `WithNestedPropertySchemaFrom(env, property, schemaName)`，将独立注册的 `Friend`/`User`/`Partner`/`Client` 类似关系显式绑定到 JSON 根 schema；Option 创建阶段验证 Environment、属性名、schema 名和 catalog membership，避免把另一个环境或未注册类型误绑定。`TestJSONNamedNestedSchemaCatalogMatchesJavaCreateSchemaFragments` 固定 Users→User[]→Friend[] 的 schema identity、`Property`/`Getter`、`Event.GetFragments`、深层 fragment 和 JSON round-trip；`TestJSONNamedNestedMapSchemaCatalogSupportsCrossRepresentationFragments` 固定 JSON 根→Map fragment 的读取/渲染；负例覆盖 nil catalog、缺失 schema、空名称和跨 Environment 引用。该切片关闭的是显式 Go catalog 引用和 JSON/Map 子集，不宣称动态注册/卸载、全表示 fragment metadata/identity、Java class-loader/module/path visibility、schema evolution 或共享 Java/Go trace 已完成；本轮仍不需要 MySQL Docker。

本轮补充对照 Java `InfraNWTableFAFSubquery` 的 20 个 runtime execution，避免把 FAF 的普通快照、参数和 on-trigger mutation 测试误当作子查询全覆盖：

| Java execution 分组 | Go 对照证据 | 当前处置 |
|---|---|---|
| `InfraFAFSubquerySimple`（Named Window/Table）、`SimpleJoin`、`Insert`（Named Window/Table） | `TestInfraFAFSubquerySimpleMatchesEsper`、`TestInfraFAFSubquerySimpleJoinMatchesEsper`、`TestInfraFAFSubqueryInsertMatchesEsper`；`SubqueryValue`、二流 Join、FAF insert | 该组功能 execution 已覆盖；总体 case 因另外 4 个 performance/index execution 仍保持 `partial`。保持 Go fluent Plan，不引入 EPL 字符串 |
| `UpdateUncorrelated`、`DeleteUncorrelated`、`SelectCorrelated`、`UpdateCorrelatedSet/Where`、`DeleteCorrelatedWhere` | `TestInfraFAFSubqueryUncorrelatedMutationMatchesEsper`、`TestInfraFAFSubquerySelectCorrelatedMatchesEsper`、`TestInfraFAFSubqueryUpdateDeleteCorrelatedMatchesEsper`；`OuterField`、目标行 snapshot 和 `OnDemand()` | 相关 select/update/delete 的 outer scope 已固定；FAF target-row mutation 才注入 `OuterEvent`，live on-trigger 继续使用入站事件 |
| `ContextBothWindows`、`ContextSelect`、`SelectWhere`、`SelectGroupBy` | `TestInfraFAFSubqueryContextMatchesEsper`、`TestInfraFAFSubquerySelectWhereAndGroupByMatchesEsper`；`WithContext`、Context-local/global source、scalar/grouped rows、inner where/cardinality | 已覆盖行为子集；Context selector 的完整类型/生命周期和更广 grouped/multi-column 组合仍开放 |
| `SelectIndexPerfWSubstitution`（Named Window/Table）、`SelectIndexPerfCorrelated`（Named Window/Table） | 已登记 Java runtime，暂不伪造 timing/index-plan parity | `partial`：后续补 index selection、plan hook、large-cardinality、参数组合和稳定性能预算 |
| `InfraFAFSubqueryInvalid` | `TestInfraFAFSubqueryInvalidMatchesEsper`：event-stream source、source filter、context mismatch | Go 在 FAF 执行边界拒绝这些仅允许 infrastructure snapshot 的子查询；Go 可复用 Query Plan，因此不强行复制 Java EPL 编译期文本诊断和错误时机 |

本轮验证结果：Go `go test ./... -count=1`、`go test -race ./... -run 'TestInfraFAFSubquery|Test(Subquery|OnDemand)' -count=1`、`go vet ./...` 和 `go test ./internal/compat -count=1` 通过；Java `mvn -pl regression-run '-Dtest=TestSuiteInfraNWTable' '-DfailIfNoTests=false' '-Dgpg.skip=true' test` 在 JDK 17/Maven 3.9.16 下为 26/26、0 failures、0 errors。该路径没有 SQL/DB/connector 依赖，本轮不启动 MySQL Docker；需要进入 Historical/SQL/EsperIO 或真实 MySQL round-trip 切片时再按 `ESPER_MYSQL_DSN`/既有 Docker fixture 启动。

本轮新增 Java `InfraNWTableFAFResolve` 的 module name-resolution 切片（`java-runtime-192b97fd8c79b3d51710`、`java-runtime-67e14876d880b2248867`）：`faf_resolve_parity_test.go` 用 Go fluent `RegisterModule`/`Module.RegisterNamedWindow`/`Module.RegisterTable`/`Module.NamedWindow`/`Module.Table` 复现 A/B 两个 module 各自拥有同名 `MyInfra` 的 Named Window/Table；对照 insert、wildcard FAF snapshot、同名对象状态隔离、module-bound runtime lookup、Plan hash/canonical 区分和未知 module Build 失败。实现把模块身份放进编译期 catalog/source node/trigger/subquery/FAF lookup，不把模块名拼进 table row 的 runtime event type，避免破坏既有 source acceptance 语义。Java 的 `ModuleUsesOption`、EPL `module` 文本、protected/public class-loader visibility 和 `CompilerArguments` path 不复制为 Go API；Go 通过显式 Module handle 保留可验证的名称解析边界。该 slice 已登记为 `case.query-fire-and-forget-resolve`，状态为 mapped；Go、Java 回归和 compat 门禁将在本轮末尾重新执行，本轮不需要 MySQL Docker。

本轮继续复核 Java `InfraNWTableFAFSubquery` 的 Context 与 index execution：新增 `faf_subquery_index_parity_test.go`，为 Context 外层 Named Window 配置全局索引化 Named Window/Table 子查询，覆盖 `OuterField` 相关 equality、参数化 equality、`IN`、`exists`、全量与指定 partition selector，以及动态 UDF 不能安全探测时的完整快照回退；另以 Context-scoped Named Window 验证候选查找只访问当前 partition。实现复用同一 `IndexSelection`/声明索引 catalog，但只在根 Named Window/Table、完整 equality/IN key 且无 source filter/window 包装时执行 candidate lookup；`<primary-key>` Table lookup、context partition state、engine-lock mutation 边界和 insertion-order 结果均保留，所有候选仍经过普通 subquery predicate/projection/cardinality 评估。该切片关闭了 Java `SelectIndexPerfWSubstitution`/`SelectIndexPerfCorrelated` 的可观测索引候选语义子集，但不宣称 10k/1M timing、JVM query-plan hook、B-tree range、共享索引的完整形状、复杂 representation 或完整 trace parity；其余范围登记在 `query.fire-and-forget` 的 remaining。Go 定向子查询/FAF、全量测试、compat、vet 和 Java `TestSuiteInfraNWTable` 门禁继续执行；本轮仍不需要 MySQL Docker。

本轮继续实现 `InfraNWTableFAFSubquery` 的 B-tree range 物理候选子集：`indexProbeRangeSpecWithOuter` 复用 equality-prefix + range 的现有 probe 约束，允许当前 outer event 的 `OuterField` 以及 prepared parameter 作为边界；`snapshotFireAndForgetSubquerySourceWithIndex` 对全局 Named Window/Table 调用 ordered lookup，对 Context-scoped Named Window 只访问当前 partition，并在 empty/reversed/Null/动态边界无法证明时回退普通 snapshot。新增 `TestInfraFAFSubqueryContextRangeIndexCandidateParity`（Named Window/Table、相关与参数化上下界）和 `TestInfraFAFSubqueryContextBoundRangeIndexCandidatePartitionParity`（Context 全量/指定 partition selector、partition-local lookup）；候选事件仍进入普通 subquery predicate/projection/cardinality evaluator。该切片关闭的是可观测 range candidate 语义，不宣称 Java 10k/1M timing、index-sharing/cost/selectivity、JVM query-plan hook、复杂 representation、历史/方法源、事务/并发和完整 Java trace parity；本轮不需要 MySQL Docker。

本轮继续实现 Context FAF Join 的两流 outer candidate：`contextJoinIndexShapeAllowed` 现在仅为两流 `LeftOuter`/`RightOuter` 放开物理路径，并由 `contextJoinIndexDriverAllowed` 强制 preserved side 作为驱动源；optional side 继续复用 `joinIndexProbeKeys`/`joinIndexProbeRangeSpecs`，候选为空时保留 `joinTuples` 的 unmatched 语义。`TestInfraFAFContextOuterJoinIndexCandidateRangeParity` 覆盖 Named Window/Table、left/right outer、composite equality-prefix + range、全量/指定 context partition selector、插入顺序、unmatched 行和 index-hit counter；FullOuter、混合/链式 outer、unidirectional 和无法安全提取的 OR/UDF/Null 形状仍走完整 snapshot fallback。该切片关闭的是 Context 两流 outer 的物理候选子集，不宣称 Context mixed/chained/unidirectional、真实 B-tree 二分游标、cost/selectivity、性能阈值或完整 Java trace parity；本轮不需要 MySQL Docker。

本轮继续实现 Context FAF Join 的相邻条件左深 mixed chain candidate：`joinIndexChainedOuterShapeAllowed` 只放开每条边为 `Inner`/`LeftOuter`、且条件仅引用“紧邻前一源 + 新源”的链；根源按 context key 枚举，后续 Named Window/Table 复用 equality/hash 与 equality-prefix + B-tree range candidate，前一 optional 源为空时不错误恢复后续匹配，最终仍由 `joinChainedTuples` 保留中间 unmatched 语义。`TestInfraFAFContextChainedMixedOuterIndexCandidateParity` 覆盖 Named Window/Table、`LeftOuter -> Inner` 与 `LeftOuter -> LeftOuter`、相邻 equality + range、全量/指定 partition selector、稳定插入顺序、缺失中间源和 middle/tail `indexLookups` 计数；RightOuter/FullOuter edge、非相邻条件、unidirectional、OR/UDF/Null/复杂动态边界继续安全回退。本切片不宣称cost/selectivity、性能阈值、Context subquery shared-index 的剩余形状或完整 Java trace parity；本轮不需要 MySQL Docker。

本轮补充 B-tree range 的真实 cursor 子集：`lookupRangeMany` 对每个 equality-prefix + range query 先在有序 `indexEntries` 上二分 lower/upper 半开区间，再在区间内做既有最终匹配和多 query 去重；prefix/range 中遇到 Null、Missing 或不可比较类型时返回不可用并回退完整有序扫描。Table 与 Named Window 共用 `collectIndexRangeCursorPositions`，`TestIndexRangeCursorBoundsAndFallback` 固定边界、inclusive 语义和 Null fallback；现已关闭“仅全量遍历 ordered entries”的实现缺口，但不宣称 Java backing class、成本模型或性能阈值 parity。本轮不需要 MySQL Docker。

本轮补充 Java `InfraNWTableSubqCorrelIndex` 的 equality/hash shared-index 子集：`NamedWindowSubqueryIndexSharing` 允许 Named Window 为相关 equality/IN 子查询按列集合自动创建并复用内部 hash index；`SubqueryUseIndex` 绑定显式 hash/B-tree index，`SubqueryDisableIndexSharing` 仅关闭当前消费者的自动共享，`SubqueryNoIndex` 强制 snapshot。`TestInfraNWTableSubqCorrelIndexSharingParity` 覆盖全局 Named Window 的自动共享、无共享、consumer disable、no-index、显式 index 及两个独立子查询消费者；`TestInfraNWTableSubqCorrelIndexSharingContextPartitionParity` 覆盖 Context partition inheritance、partition-local lookup 和 update/delete rebuild；`TestInfraNWTableSubqCorrelIndexOptionValidationParity` 覆盖 missing index、不可分析 predicate、no-index/explicit-index 冲突和显式 index + disable sharing。该切片登记 Java `InfraNWTableSubqCorrelIndex` 的 20 个 runtime，但只关闭 equality/hash 可观察子集；live statement listener 直接对照、multiple-index hint 选择、B-tree/multikey/array shared index、cost/selectivity、JVM query-plan hook 和 10k/1M 性能阈值仍保持 partial。本轮不需要 MySQL Docker。

本轮复核继续对照同一 Java 类的剩余可观察形状。`TestInfraNWTableSubqCorrelIndexMultipleIndexHintsParity` 在 Named Window/Table 中为同一外层查询的两个相关子查询分别绑定 `I1`/`I2`，断言 hint 不串扰并按 outer row 产生两次独立 candidate probe；`TestInfraNWTableSubqCorrelIndexChoiceParity` 使用结构化 fluent AST 检查复合 equality 优先、equality-prefix + range 选择 B-tree 以及 `SubqueryUseIndex` 显式覆盖，并执行结果确认最终 predicate 仍负责正确性；`TestInfraNWTableSubqIndexShareMultikeyArrayParity` 对照 Java single-array/two-array execution，覆盖 Named Window 的显式数组索引、Go 自动 shared-index、Table primary-key 数组、单/双数组内容相等、顺序/内容不匹配和 Null scalar cardinality。由于 Go 不复制 Java `@Hint` 文本或 `SupportQueryPlanIndexHook`，这些测试以 `SubqueryUseIndex`、`NamedWindowSubqueryIndexSharing` 和内部结构化 selection 作为 Go-style 可验证契约；live listener 轨迹、JVM backing/query-plan hook、Esper 完整 cost/selectivity 模型和 10k/1M timing 仍保持 partial。本轮不需要 MySQL Docker。

本轮门禁结果：Java `mvn -pl regression-run '-Dtest=TestSuiteInfraNWTable' '-DfailIfNoTests=false' '-Dgpg.skip=true' test` 为 26/26、0 failures、0 errors；Go `go test ./... -count=1`、`go test ./internal/compat -count=1`、`go vet ./...` 和 `go test -race . -count=1` 均通过。该切片只使用内存 Named Window/Table 与 Go fluent plan，不需要启动 MySQL Docker。

本轮 Variant 单列转换补充（Draft 2.76，2026-08-08）：对照 Java `EventVariantSingleColumnConversion`，新增 Go `InsertEventIntoNamedWindow`，允许 `Func1`/方法表达式返回已注册 Variant member 的 concrete underlying 或 `Event`，在目标 PREDEFINED Variant Named Window 插入前完成 member schema materialization，保留 concrete `Schema`、Named Window `TypeName` 和窗口 retention。`TestVariantSingleColumnConversionMatchesEsper` 固定 `SupportBean(E1,1)` → `preProcessEvent` → `SupportBean(E2,0)`、`theString='E'` 无匹配及 snapshot identity；Java `TestSuiteEventVariant` 17/17、Go 定向测试和 `go test ./internal/compat -count=1` 均通过。当前 manifest 为 191 个 case，其中 185 个 mapped、2 个 partial、4 个 approved-difference，1,387 Java runtime associations covering 1,377/4,136 个唯一可执行 runtime；本轮不需要 MySQL Docker。

本轮子查询实时 listener 补充（Draft 2.77，2026-08-08）：新增 `TestInfraNWTableSubqCorrelIndexLiveListenerParity`、`TestInfraNWTableSubqCorrelIndexChoiceLiveListenerParity` 和 `TestInfraNWTableSubqIndexShareMultikeyArrayLiveListenerParity`，用 `From`/`FromAny`/`FromNamedWindow`/`FromTable`、`SubqueryValueWithOptions` 和 `Statement.Subscribe` 对照 Java `InfraNWTableSubqCorrelIndexAssertion`、`ShareIndexChoice`/`NoIndexShareIndexChoice` 及 single/two-array execution。覆盖自动 shared index、无共享、显式 index、consumer disable、no-index、复合 equality/range 选择、数组 key、late-start 重部署、new-only listener batch、结果顺序与 physical lookup counter；multiple-index-hint 仍只有 Java JVM plan-hook 证据。没有 SQL/connector 依赖，本轮不启动 MySQL Docker。该 case 目前只剩 JVM query-plan hook、Esper 完整 cost/selectivity 模型及 10k/1M 性能阈值差异。

### 17.5 数据库、方法源与全量覆盖复核（Draft 2.80，2026-08-08）

本次复核将现有 MySQL Docker 纳入可复现实验前提，并重新按运行态 inventory 检查“已有关联”与“已完成”的差距。环境已具备，数据库基础 Go 测试可以执行，但数据库、方法源和全量回归仍远未完成；`case.historical-sql`/`case.esperio-db` 的存在不能关闭整个功能域。

#### 17.5.1 运行态清单硬事实

`testdata/compat/java-execution-inventory.jsonl` 有 4,136 个 `status=ok` 的唯一可执行 runtime；`testdata/compat/capability-manifest.json` 有 191 个 case，其中 185 `mapped`、2 `partial`、4 `approved-difference`。这些状态只代表映射/处置登记，不代表 Java/Go trace 已相等。

| Java 运行域 | inventory runtime | 当前 case 直接关联 | 尚未登记 |
|---|---:|---:|---:|
| `EPLDatabase` | 70 | 34 | 36 |
| `EPLFromClauseMethod` | 57 | 0 | 57 |
| 合计 | 127 | 18 | 109 |

现有 `case.historical-sql` 已把 `EPLDatabaseFAF` 的 10 个 runtime（连同原有数据库/方法源 runtime 共 18 个）逐项登记；测试数量仍不能替代行为 trace 相等，`query.historical-sql` 继续保持 `partial`。

全量域级盘点也显示明显遗漏：

| 域 | 总数/已登记/未登记 | 域 | 总数/已登记/未登记 |
|---|---:|---|---:|
| `epl` | 1,018 / 286 / 732 | `expr` | 657 / 293 / 364 |
| `infra` | 590 / 171 / 419 | `resultset` | 576 / 263 / 313 |
| `client` | 281 / 37 / 244 | `context` | 224 / 40 / 184 |
| `view` | 227 / 51 / 176 | `event` | 294 / 124 / 170 |
| `pattern` | 145 / 39 / 106 | `rowrecog` | 68 / 34 / 34 |
| `multithread` | 56 / 0 / 56 | 合计 | 4,136 / 1,367 / 2,769 |

因此除了 SQL，还必须优先防止遗漏 `client` 管理面、`multithread` 原子性/可见性、Context 生命周期、Infra on-trigger 组合和 resultset 高级访问聚合。

#### 17.5.2 具体遗漏与下一批 planned case

- `EPLDatabaseFAF` 共 10 个 runtime，现已逐项登记并由 Go 对照测试覆盖 simple、column/row hook、prepared、typed substitution、distinct、where、多行、变量、fluent SODA-equivalent plan、SQL text subquery 和 invalid SQL/closed query；Java 的 SQL+SQL Join/Context SQL FAF invalid 与 Go historical/method Join/Context extension、1000 次性能阈值、精确 vendor diagnostic 继续作为差异处置。SQL cache、Null/cardinality 更广矩阵和完整 transaction/dialect/type-code 仍开放。
- `EPLDatabaseJoin`、`EPLDatabase2StreamOuterJoin`、`EPLDatabase3StreamOuterJoin`、`EPLDatabaseJoinOptions`、`EPLDatabaseHintHook`、cache/performance 和 datasource factory 仍需逐项核对 outer preserved side、time-batch、property resolution、insert-into、缓存生命周期和性能 disposition。
- `EPLFromClauseMethod`、`NStream`、`OuterNStream`、`Variable`、`MultikeyWArray`、`JoinPerformance`、`CacheLRU`、`CacheExpiry` 合计 57 个 runtime 全未直接映射；必须覆盖 single/sequence return、参数/变量、dependent 拓扑、N-stream outer、array key、LRU/expiry、异常和副本隔离。
- `case.esperio-db` 代表 DML/Upsert 基础证据，`case.esperio-db-executor` 已补齐 `ExecutorServices`/`ExecutorSameThread`、`RunnableDML`/`RunnableUpsert` 对应的 same-thread/命名 fixed-worker、close/drain、取消、panic/error、异步 retry 和 sink lifecycle re-check；XML/config、连接工厂/JNDI、SQL 类型绑定、duplicate/error handler、listener 结果和完整 Java trace 仍开放。

先登记以下规划 case，再进入实现；名称暂不代表已写入 manifest：

| planned case | 范围 |
|---|---|
| `case.query-database-faf` | 已由 `case.historical-sql` 吸收：SQL FAF、参数/变量、distinct/where、prepared、row/column hook、invalid/context（Draft 2.80） |
| `case.query-database-join-outer` | 2/3 stream inner/outer、time-batch、property、insert-into |
| `case.query-database-cache-options` | query/join cache、options、datasource factory、hint/performance |
| `case.query-method-source` | 57 个 FromClauseMethod runtime 的返回形状、依赖、outer、cache、invalid |
| `case.esperio-db-executor` | EsperIO DB executor、DML/Upsert action、配置、错误恢复和关闭（Draft 2.79 已完成 Go executor/work-queue 子集，Java XML/JNDI/config 仍开放） |

每个 planned case 必须先由运行态 inventory 生成 runtime ID，再登记 Go test、Java source、fixture、状态和差异；不能用同一个 broad Go test 重复制造覆盖率。

#### 17.5.3 环境与固定门禁

| 项目 | 当前值/约束 |
|---|---|
| Java/Maven/Go | OpenJDK 17.0.20、Maven 3.9.16（已安装）、Go 1.25.5 |
| MySQL | `esper-java-mysql`，8.0.46，`127.0.0.1:3306`，image digest `sha256:7dcddc01f13bab2f15cde676d44d01f61fc9f99fe7785e86196dfc07d358ae2b` |
| fixture | `test` 数据库和 `D:\Code\soc\esper\common\etc\regression\create_testdb.sql`；变更型 Java/Go 测试串行或先恢复 fixture |
| 编码/时区 | 当前 Maven 默认编码显示 GBK；oracle 要求显式 UTF-8/UTC，不能依赖 Windows 默认值 |

容器停止时复用现有实例：

```powershell
docker start esper-java-mysql
mvn -pl regression-run '-Dtest=TestSuiteEPLDatabase' '-DfailIfNoTests=false' '-Dgpg.skip=true' '-Dfile.encoding=UTF-8' '-Duser.timezone=UTC' test
$env:ESPER_MYSQL_DSN = 'root:password@tcp(127.0.0.1:3306)/test?parseTime=true&charset=utf8mb4'
go test ./... -run 'Test(SQLHistoricalProvider|SQLSink|SQLHistoricalFireAndForget|DBConnector)MySQLDocker$' -count=1
```

#### 17.5.4 本次实测 disposition

- Java `TestSuiteEPLDatabase`：15 个入口，13 通过、2 失败、0 error。`testEPLDatabaseJoin` 仅 MySQL vendor error location 文本不同（`near ', from ...'` vs `near ' from ...'`）；本次 `testEPLDatabaseJoinPerfNoCache` 为 `delta=1266`，属于 Docker/JDBC 性能阈值抖动，不能算语义 pass。
- Java `esperio-db`：4 个测试，2 通过、2 失败、0 error；`TestConfig` 为 `Properties.toString` 键顺序差异，`TestDBAdapterDML` 为期望/实际值数量差异，`TestDBAdapterUpsert` 通过。两项需保留原始证据并分别决定 approved-difference 或修复。
- Go SQL FAF unit：`database_faf_parity_test.go` 的 10 个对照用例通过，覆盖自定义 database/sql driver 的多行、整行转换、参数声明、prepared reuse/close 和错误传播；这不替代真实数据库门禁。
- Go MySQL Docker：`TestSQLHistoricalProviderMySQLDocker`、`TestSQLSinkMySQLDocker` 和 `TestSQLHistoricalFireAndForgetMySQLDocker` 通过；这证明基础 `database/sql` adapter/sink 及 SQL FAF 真实列/行返回路径可运行，不覆盖上述 109 个未登记 runtime。`connectors/db` 的 executor/work-queue 定向测试和 `go test -race ./connectors/db` 也通过；Go 最终异步错误经 `Task.Wait` 暴露，属于相对 Java 日志吞错的明确 API 差异。

后续顺序固定为：先完成 DB/Method/EsperIO DB runtime-to-case 拆分和 fixture/编码/时区门禁，再做 SQL FAF/outer join、method dependent/outer/cache，随后补 EsperIO DB XML/config、connection factory、SQL type binding 和跨语言 trace。只有 runtime 全部处置为 `mapped`、`approved-difference` 或有评审的 N disposition，并具备正向/无效/边界/关闭/并发证据，才允许提升 capability；当前仍不能宣称 Esper 已完成全量 Go 移植。

Draft 2.79 对前文 EsperIO 汇总表中的“异步 executor”未完成项作修订：Go `connectors/db` 已完成 same-thread/命名 fixed-worker、queue drain、取消、panic/error、异步 retry 和 sink lifecycle re-check 子集；前文所列 DB XML/config、connection factory、完整 Java trace 等未完成项仍然有效。

### 17.5.5 数据库选项、性能与缓存对照（Draft 2.83，2026-08-08）

本轮补齐数据库域剩余可映射 runtime 的 Go 链式 API 对照测试，全部纳入 `case.historical-sql`：

- `EPLDatabaseJoinOptionLowercase` / `EPLDatabaseJoinOptionUppercase`：验证 `SQLColumnCaseLower` / `SQLColumnCaseUpper` 与显式 schema 类型声明如何共同决定结果列名与类型（字符串/整数）。
- `EPLDatabaseNoJoinIteratePerf`：变量驱动的 FAF 查询，`PrepareFireAndForget` + `Execute` 重复执行，验证 `between` + 变量快照结果 `{4, true}`；Java 的 10000 次迭代性能断言不适用于 fake driver，作为 approved-difference 丢弃。
- `EPLDatabaseQueryResultCache`：LRU 缓存的 SQL 历史流与 `SupportBean_S0` 触发 join，验证顺序与重复查询结果。
- `EPLDatabaseJoinPerfNoCache`：100 事件 retained join 正确性对照。
- `EPLDatabaseJoinPerfWithCache` 的 9 个内部 execution（Constants、RangeIndex、KeyAndRangeIndex、SelectLargeResultSet、SelectLargeResultSetCoercion、2StreamOuterJoin、OuterJoinPlusWhere、InKeywordSingleIndex、InKeywordMultiIndex）：全部以功能正确性覆盖，丢弃 Java 的时间阈值；使用 `Join(...).Select(...).Where(...)`、`Between`、`In`、`RightOuter` 和 `KeepAll` 等 Go builder 表达等价语义。

新增 Go 测试文件：

- `database_join_perf_parity_test.go`：包含上述 14 个对照函数及 `mytesttable_large` 1000 行 fixture、`dbJoinPerfS0` / `dbJoinPerfRange` / `dbJoinPerfSupportBean` 辅助类型和若干 fake SQL handler。

本轮新增 14 个 Java runtime 对账/处置证据，manifest 更新后数据库域直接映射明显增加；`EPLDatabaseJoinInsertInto`（pattern+SQL+insert into+time batch+aggregation）仍依赖 pattern timer + SQL 历史流组合，留作后续切片。`go vet .`、`go test ./... -count=1 -timeout 180s`、`go test ./internal/compat/... -count=1` 均通过；`go test -race .` 已按子集通过（小 fixture 约 1.7s，大 fixture 约 69s）。本轮未使用 MySQL Docker，所有测试基于 fake `database/sql` driver。

Draft 3.41（2026-08-10）继续进入 client/deploy 对账，关闭 `ClientDeployVersionMinorCheck`（`java-runtime-c294ea2d462a0503c131`）。`Plan` 增加实际 compiler provenance，`Plan.Manifest` 不再把当前编译器常量伪装成旧计划的来源；`PlanArtifact` 同时序列化 schema/compiler version，缺失或不匹配均返回 `PlanCompatibilityError`。`Engine.Deploy`、`DeployPlans`、`ExecuteFireAndForget`、`PrepareFireAndForget` 与 FAF route 在状态变更或查询求值前调用同一 ownership/version preflight；普通入口没有 index，`DeployPlans` 返回 zero-based `DeploymentPlanIndex`，单项旧计划固定为 0，且失败后 active deployment 为空。Java 用 8.0.0 `EPCompiled` JAR 验证 9.0.0 runtime 的 deploy/rollout/FAF 拒绝；Go 测试从当前 typed Plan 构造 detached legacy compiler/schema contract，不加载 JVM serialization/class provider。此映射只关闭版本拒绝的可观察行为，不宣称 `ClientDeployRollout` 的跨 deployment 依赖、statement ID、重复 deployment ID、替换参数和并发可见性已完成。manifest 达到 78 capabilities、1,776 runtime associations、1,750/4,140 unique runtime（42.27%）和 244 cases（236 mapped / 0 partial / 8 approved-difference）。

Draft 3.42（2026-08-10）继续关闭 deployment runtime option。`Engine.Deploy` 接受 variadic `DeploymentOption`，单 Plan runtime option 不需要退化为批量入口。`DeploymentStatementNameContext` 现在含 index、Plan、original name、显式 deployment ID、TypedDescription 与 detached StatementMetadata；`TestClientDeployStatementNameResolveContextMatchesEsper` 固定 `s0` → `hello`，并验证 resolver context、Deployment lookup、Statement metadata 和事件消费。新增 deployment-time user-object resolver/context，严格在 runtime name 解析后执行；nil、`ABC` 和 native struct 都能覆盖 compile-time default，`Statement.UserObject` 与 `CurrentEvaluationContext.StatementUserObject` 读取同一 runtime value，Plan hash 保持不变，callback error 不留下 deployment。为了在 preflight 期间提供最终 identity，当前对照显式使用 `WithDeploymentID`；自动生成 deployment ID 仍在 callback 后分配，列为 remaining。Java EPL/annotation-array/Serializable 形态分别由 typed description、detached metadata 与任意 Go value 取代。`ClientDeployClassLoaderOptionSimple` 只观察 generated JVM provider class 经过自定义 ClassLoader；Go 无 generated class，故登记 approved-difference，同时验证 Go-native manifest 和 typed Plan deploy/undeploy。manifest 达到 80 capabilities、1,780 runtime associations、1,754/4,140 unique runtime（42.37%）和 246 cases（237 mapped / 0 partial / 9 approved-difference）。

Draft 3.43（2026-08-10）关闭 `ClientDeployResult` 中 `ClientDeployResultSimple`、`ClientDeployGetStmtByDepIdAndName`、`ClientDeploySameDeploymentId` 三条 execution；`ClientDeployStateListener` 因含 rollout item 0/1 事件继续留待原子 rollout 切片。Engine 新增 `IsDeployed`、单 ID `Deployment`、按 deployment order 的 `Deployments` 与 deployment+statement 双键 `Statement`；Deployment 新增 detached `Dependencies`。两 statement typed deployment 固定 active→undeploy lifecycle、empty dependencies、statement order/name 和 TypedDescription；额外 module A/B uses 测试固定真实 dependency ID、order 与 snapshot isolation。A–E 五个显式 deployment ID 中 D/E 可拥有同名 `s3` 而 lookup 不混淆；`StatementName(" stmt0  ")` 在 Build 归一为 `stmt0`，lookup 同样 trim。blank/unknown Go lookup 返回 `(nil,false)`，而非 Java null 参数异常。重复 `ABC` deployment 返回 ErrorDeployment，原 deployment pointer 与 active set 不变。Java resource module 的 create-schema statement 由 Environment registration 取代，StatementProperty.EPL 由 typed Query description 取代。manifest 达到 81 capabilities、1,783 runtime associations、1,757/4,140 unique runtime（42.44%）和 247 cases（238 mapped / 0 partial / 9 approved-difference）。

Draft 3.44（2026-08-10）关闭 `ClientDeployRollout` 3/3 与 `ClientDeployStateListener` 1/1 execution。新增 fluent `RolloutPlans(...).WithOptions(...)`、`Engine.Rollout`、ordered `DeploymentRollout`/result item 与结构化 `DeploymentRolloutError`；所有 item 先完成 ownership/name/user-object/parameter preflight，再在同一 Engine mutex transaction 内按序激活。后续 item 的 active module uses 能解析到前序 deployment；任一 missing dependency、protected module ownership、duplicate/active deployment ID 或 substitution binding 错误都会以 zero-based item index 返回，逆序移除此前 item、恢复 `nextID`/statement sequence，且不发 lifecycle/dataflow/context 通知。两 module 与四 module 对照覆盖 result order、依赖 ID、事件处理、sequence 1..7 和失败后 baseline isolation。`DeploymentStateListener` 提供 Add/Remove/List/RemoveAll，ordinary deploy/undeploy index 为 -1，rollout 成功事件为 0/1；Go 用统一 State 字段和回调替代 Java 两个 subclass/method，并在锁外以 detached statement slice 调用。Go Schema 在 fluent Build 前已注册，因此 Java rollout 的 runtime event-type path missing/duplicate 用 active module-use precondition 与 protected-module ownership 冲突固定同类可观察原子失败。manifest 达到 83 capabilities、1,787 runtime associations、1,761/4,140 unique runtime（42.54%）和 249 cases（240 mapped / 0 partial / 9 approved-difference）。

Draft 3.45（2026-08-10）先关闭 `ClientDeployUndeploy` 的 `ClientUndeployInvalid` 与 `ClientUndeployDependencyChain` 2/11 execution。Engine 在删除 deployment 前扫描其他 active deployment 的 detached dependency-ID snapshot；若 provider 仍被引用，按 deployment order 稳定选择 dependent，返回 `UndeployPreconditionError{DeploymentID,ReferencedBy}`，并以 `ErrorDependency` 支持 errors.Is，statement/deployment/lifecycle 状态均不改变。unknown ID 保持 ErrorDeployment。A/B/C/D typed module-use chain固定 `[] / [A] / [B] / [C]` dependency snapshot、D 投影值 10、A/B 失败不改变四个 active deployment，随后 D→C→B→A 逆序卸载成功。Java execution 用 public variable A/B/C/D；本切片只映射 deployment graph 的可观察 precondition，variable/schema/context/expression/table/Named Window/index/script/class 资源图继续独立处理。manifest 达到 84 capabilities、1,789 runtime associations、1,763/4,140 unique runtime（42.58%）和 250 cases（241 mapped / 0 partial / 9 approved-difference）。
