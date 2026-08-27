# Java oracle runner

该目录提供固定 Esper 9.0.0 Java commit 的运行态清单探针和 parity oracle。当前 Linux 工作区以 shell runner 为主；Windows 仍保留 PowerShell 变体。Java/Go 全量行为差分和完整 Go 映射尚未完成。

固定输入：

- Java source checkout：`/root/app/esper`（或 Windows 上的 Esper checkout）。
- Java commit：`9e1b9f1cc9117fea4bf33ab043762c045d73839c`。
- Java 17、Maven 3.9、固定 UTF-8、UTC、英文 locale；shell runner 另外要求 `jq`。

## Runtime inventory

`ExecutionInventory.java`/`run-probe.ps1` 通过外层 suite 的 `executions()` 枚举动态 execution，并覆盖无参、布尔参数变体、direct execution 和 `RegressionExecutionPreConfigured`。输出记录外层类、factory 变体、execution class、`name()`、ordinal、flags 和稳定的 `runtimeId`。探针只构造 execution，不调用 `run(RegressionEnvironment)`。

Windows 探针：

```powershell
pwsh -File .\tools\java-oracle\run-probe.ps1 -SkipBuild
```

输出为 `testdata/compat/java-execution-inventory.jsonl`。当前清单有 4,136 条 `status=ok` runtime、4 条明确 ignored，runtime ID 无重复；静态候选清单仍只是输入，不是 Go parity 证据。

## ContextHash oracle

shell runner 会校验 Esper checkout 的 Git commit，构建 compiler/runtime（省略 `--skip-build` 时），生成 Maven runtime classpath，编译 `ContextHashScenarioOracle.java`，并校验输出是 `esper-parity/v1` trace。已有构建产物时可直接运行：

```sh
./tools/java-oracle/run-context-hash.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/context-hash-segmented.json \
  --output /tmp/context-hash-java.json \
  --skip-build
```

将 Java trace 与 Go trace 比较并生成 evidence：

```sh
go run ./cmd/parity \
  -mode context-hash-diff \
  -scenario testdata/parity/context-hash-segmented.json \
  -java-trace /tmp/context-hash-java.json \
  -evidence /tmp/context-hash.evidence.json
```

checked-in 的场景和 evidence 分别是 `testdata/parity/context-hash-segmented.json` 与 `testdata/parity/context-hash-segmented.evidence.json`；当前 evidence 状态为 `passing`，差异数为 0，覆盖 3 个 Java runtime。它只证明该场景，不代表全量 Esper parity。

Windows PowerShell 变体：

```powershell
pwsh -File .\tools\java-oracle\run-context-hash.ps1 `
  -EsperRoot D:\Code\soc\esper `
  -Scenario testdata\parity\context-hash-segmented.json `
  -Output $env:TEMP\context-hash-java.json `
  -SkipBuild
```

当前 Linux 环境没有 `pwsh`，因此本轮只执行并验证 shell runner；PowerShell 文件保留用于 Windows/CI。

## Filter-window-aggregate oracle

第二个代表性场景使用 `FilterWindowAggregateScenarioOracle.java`（runner：`run-filter-window-aggregate.sh`）。它用 Map 事件类型 `Trade`（symbol/price）双跑 `from Trade#length(2) where price > 10 group by symbol` 与 `output every 3 events` 两个 execution，并输出同一 `esper-parity/v1` trace。Java 语义要点：where 过滤发生在 length 窗口之后（被过滤事件仍淘汰窗口事件），istream 分组聚合批次中新事件组排在淘汰重算组之前；这两点已由 Go 实现复现。带事件数输出策略的 statement，Java `iterator()` 返回上次已发射的每组最新行（pending 更新不可见，尚未输出过的组投影 null 聚合值），Go 已按同一契约实现（`lastOutputGroupRows` + 输出受限快照路径），every-3 case 包含 4 个 snapshot 步骤（首事件后、第二事件后、首次输出后、pending 事件后）且差分一致。

```sh
./tools/java-oracle/run-filter-window-aggregate.sh   --esper-root /root/app/esper   --scenario testdata/parity/filter-window-aggregate-output.json   --output /tmp/fwa-java.json   --skip-build

go run ./cmd/parity \
  -mode filter-window-aggregate-diff \
  -scenario testdata/parity/filter-window-aggregate-output.json \
  -java-trace /tmp/fwa-java.json \
  -evidence /tmp/fwa.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/filter-window-aggregate-output.json`、`testdata/parity/filter-window-aggregate-output.evidence.json`；当前差异数为 0。

## Join-length-window oracle

第三个代表性场景使用 `JoinScenarioOracle.java`（runner：`run-join.sh`）。它注册 Map 事件类型 `OrderEvent(orderId, price)` 与 `PaymentEvent(orderId, amount)`，双跑 `from OrderEvent#length(3) as o, PaymentEvent#length(3) as p where o.orderId = p.orderId order by o.orderId asc, p.amount asc` 以及同语句的 `output every 2 events` 变体，并输出同一 `esper-parity/v1` trace。Java 语义要点：非聚合 join 的 `statement.iterator()` 在带事件数输出策略时仍返回实时 join 状态（pending 行可见），而 listener 输出按 `output every 2 events` 缓冲到两个 accepted 行后以 ORDER BY 顺序整批交付；Go 的 `snapshotJoinBatch` 已按实时状态实现。场景覆盖同键多匹配（O1 的 10/20/40）、length(3) 窗口淘汰边界（O2/5 淘汰 O1/10）和 ORDER BY 多键排序。

```sh
./tools/java-oracle/run-join.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/join-length-window.json \
  --output /tmp/join-java.json \
  --skip-build

go run ./cmd/parity \
  -mode join-length-window-diff \
  -scenario testdata/parity/join-length-window.json \
  -java-trace /tmp/join-java.json \
  -evidence /tmp/join.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/join-length-window.json`、`testdata/parity/join-length-window.evidence.json`；当前差异数为 0，覆盖 `EPLJoinJoinWInnerKeywordWOOnClause` 与 `EPLJoinJoinNoWhereClause` 两个 Java runtime。

## Output-policy-iterator oracle

第四个代表性场景使用 `OutputPolicyScenarioOracle.java`（runner：`run-output-policy.sh`）。它注册 Map 事件类型 `Trade(symbol, price)`，双跑 `from Trade#length(2) where price > 10 group by symbol select symbol, sum(price)` 的 `output all every 3 events`、`output first every 3 events` 与 `output last every 3 events` 三个 execution，每个事件后调用 `statement.iterator()` 输出快照。Java 语义要点：all/first/last every-N 分组聚合的 iterator 返回实时聚合状态并按当前过滤后窗口事件顺序逐组一行；listener 批次顺序（all 为批次内事件序、last 为每组合并后的最新行序）不决定 iterator 顺序。`output first every N events` 在分组查询中按组独立计数（新组立即输出、组内每 N 个事件后再次输出，grouped having 未命中不计入），在非分组查询中先输出首个相关结果、之后按全部已接受输入事件计间隔并在间隔结束后的下一个相关结果输出。Go 的 live 快照路径与 first-every 两条实现已按同一契约对齐。

```sh
./tools/java-oracle/run-output-policy.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/output-policy-iterator.json \
  --output /tmp/output-policy-java.json \
  --skip-build

go run ./cmd/parity \
  -mode output-policy-diff \
  -scenario testdata/parity/output-policy-iterator.json \
  -java-trace /tmp/output-policy-java.json \
  -evidence /tmp/output-policy.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/output-policy-iterator.json`、`testdata/parity/output-policy-iterator.evidence.json`；当前差异数为 0，覆盖 `ResultSetWildcardRowPerGroup`、`ResultSetFirstSimpleHavingAndNoHaving` 与 `ResultSetUnaggregatedOutputFirst` 三个 Java runtime。

## Pattern-timer-interval oracle

第五个代表性场景使用 `PatternTimerScenarioOracle.java`（runner：`run-pattern-timer.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`，双跑 `every a=SupportBean -> timer:interval(intPrimitive seconds)`：t=10s 发送 E1(3) 与 E2(2) 各 arm 一个动态 timer，t=11.999s 无输出，t=12s 输出 E2，t=12.999s 无输出，t=13s 输出 E1。该场景覆盖 pattern every + followed-by + 动态 timer interval + 虚拟时钟边界。

```sh
./tools/java-oracle/run-pattern-timer.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/pattern-timer-interval.json \
  --output /tmp/pattern-timer-java.json \
  --skip-build

go run ./cmd/parity \
  -mode pattern-timer-diff \
  -scenario testdata/parity/pattern-timer-interval.json \
  -java-trace /tmp/pattern-timer-java.json \
  -evidence /tmp/pattern-timer.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/pattern-timer-interval.json`、`testdata/parity/pattern-timer-interval.evidence.json`；当前差异数为 0，覆盖 `PatternIntervalSpecExpressionWithProperty`。

## Subquery-length-window oracle

第六个代表性场景使用 `SubqueryScenarioOracle.java`（runner：`run-subquery.sh`）。它注册 Map 事件类型 `SupportBean_S0(id)` 与 `SupportBean_S1(id)`，双跑 `select (select max(id) from SupportBean_S1#length(3)) as value from SupportBean_S0`：空窗口投影 null，随后 S1 序列 100/200/190/180/170 使 S0 依次得到 100/200/200/200/190，覆盖 length(3) 窗口淘汰边界。

```sh
./tools/java-oracle/run-subquery.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/subquery-length-window.json \
  --output /tmp/subquery-java.json \
  --skip-build

go run ./cmd/parity \
  -mode subquery-diff \
  -scenario testdata/parity/subquery-length-window.json \
  -java-trace /tmp/subquery-java.json \
  -evidence /tmp/subquery.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/subquery-length-window.json`、`testdata/parity/subquery-length-window.evidence.json`；当前差异数为 0，覆盖 `EPLSubselectUngroupedUncorrelatedInSelect`。

## Named-window-mutation oracle

第七个代表性场景使用 `NamedWindowMutationScenarioOracle.java`（runner：`run-named-window-mutation.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)` 与 `SupportBean_S0(id, p00)`，双跑 `create window MyWindow#firstunique(theString) as SupportBean`、`insert into MyWindow select * from SupportBean`、`on SupportBean_S0 delete from MyWindow where p00 = theString` 与 `select count(*) as cnt from MyWindow` 的 create/count 两个 listener：重复键静默忽略、删除以 old-stream 交付给 create consumer、count 随 mutation 更新。

```sh
./tools/java-oracle/run-named-window-mutation.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/named-window-mutation.json \
  --output /tmp/named-window-mutation-java.json \
  --skip-build

go run ./cmd/parity \
  -mode named-window-mutation-diff \
  -scenario testdata/parity/named-window-mutation.json \
  -java-trace /tmp/named-window-mutation-java.json \
  -evidence /tmp/named-window-mutation.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/named-window-mutation.json`、`testdata/parity/named-window-mutation.evidence.json`；当前差异数为 0，覆盖 `InfraFirstUnique`。

## Table-mutation oracle

第八个代表性场景使用 `TableMutationScenarioOracle.java`（runner：`run-table-mutation.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`、`SupportBean_S0(id, p00)` 与 `SupportTwoKeyEvent(k1, k2, newValue)`，双跑 `create table varagg(keyOne string primary key, keyTwo int primary key, p0 long)`、`on SupportBean merge varagg when not matched then insert`、`select varagg[p00,id].p0 from SupportBean_S0` 与 `on SupportTwoKeyEvent update varagg`：缺失 key 读 null、merge insert 后读 1、update listener 交付 new/old、无匹配更新不触发。

```sh
./tools/java-oracle/run-table-mutation.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/table-mutation.json \
  --output /tmp/table-mutation-java.json \
  --skip-build

go run ./cmd/parity \
  -mode table-mutation-diff \
  -scenario testdata/parity/table-mutation.json \
  -java-trace /tmp/table-mutation-java.json \
  -evidence /tmp/table-mutation.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/table-mutation.json`、`testdata/parity/table-mutation.evidence.json`；当前差异数为 0，覆盖 `InfraTableOnUpdateTwoKey`。

## Variable-deploy oracle

第九个代表性场景使用 `VariableDeployScenarioOracle.java`（runner：`run-variable-deploy.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`，双跑 `create variable int var1RTC = 10`、`select var1RTC, theString from SupportBean(theString like 'E%')` 与 `on SupportBean(theString like 'S%') set var1RTC = intPrimitive`：select 先部署并读默认 10，set 后部署后 E 事件读到 3/-1、S 事件 set listener 输出 3/-1。

```sh
./tools/java-oracle/run-variable-deploy.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/variable-deploy.json \
  --output /tmp/variable-deploy-java.json \
  --skip-build

go run ./cmd/parity \
  -mode variable-deploy-diff \
  -scenario testdata/parity/variable-deploy.json \
  -java-trace /tmp/variable-deploy-java.json \
  -evidence /tmp/variable-deploy.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/variable-deploy.json`、`testdata/parity/variable-deploy.evidence.json`；当前差异数为 0，覆盖 `EPLVariableOnSetWDeploy`。

## Context-output-termination oracle

第十个代表性场景使用 `ContextOutputScenarioOracle.java`（runner：`run-context-output.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`，双跑 `create context EveryMinute as initiated by pattern[every timer:at(*, *, *, *, *)] terminated after 1 min` 与 `context EveryMinute select sum(intPrimitive) as c1 from SupportBean output snapshot when terminated`：08:01 分区在 08:02 终止输出 6，08:02 分区在 08:03 终止输出 15。Go runner 必须在部署前把虚拟时钟初始化到 08:00（`WithStartTime`），避免从 epoch 0 追赶 every-minute cron 的全部历史分区。

```sh
./tools/java-oracle/run-context-output.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/context-output-termination.json \
  --output /tmp/context-output-java.json \
  --skip-build

go run ./cmd/parity \
  -mode context-output-diff \
  -scenario testdata/parity/context-output-termination.json \
  -java-trace /tmp/context-output-java.json \
  -evidence /tmp/context-output.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/context-output-termination.json`、`testdata/parity/context-output-termination.evidence.json`；当前差异数为 0，覆盖 `ContextInitTermOutputSnapshotWhenTerminated`。

## Deployment-restart-window oracle

第十一个代表性场景使用 `DeploymentRestartScenarioOracle.java`（runner：`run-deployment-restart.sh`）。它注册 Map 事件类型 `SupportMarketDataBean(symbol, volume)`，双跑 `SupportMarketDataBean(symbol='IBM')#length(3) s0, SupportMarketDataBean(symbol='CSCO')#length(3) s1 where s0.volume=s1.volume` 的三个 case：每个 case 都是一个全新 deployment（undeploy/redeploy 边界），分别覆盖匹配、不匹配、再次匹配；这证明 length(3) join 状态在重启后彻底重置。

```sh
./tools/java-oracle/run-deployment-restart.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/deployment-restart-window.json \
  --output /tmp/deployment-restart-java.json \
  --skip-build

go run ./cmd/parity \
  -mode deployment-restart-diff \
  -scenario testdata/parity/deployment-restart-window.json \
  -java-trace /tmp/deployment-restart-java.json \
  -evidence /tmp/deployment-restart.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/deployment-restart-window.json`、`testdata/parity/deployment-restart-window.evidence.json`；当前差异数为 0，覆盖 `EPLJoinStartStopSceneOne`。

## High-cardinality-context oracle

第十二个代表性场景使用 `HighCardinalityScenarioOracle.java`（runner：`run-high-cardinality.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`，双跑 `create context SegmentedByAString partition by theString from SupportBean` 与 `context SegmentedByAString select sum(intPrimitive) as col1 from SupportBean#keepall`：40 个唯一 key 各建一个分段分区并保留独立聚合，第二次给前 10 组各 +100 后快照前 10 行为 101..110、其余保持 1..40。

```sh
./tools/java-oracle/run-high-cardinality.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/high-cardinality-context.json \
  --output /tmp/high-cardinality-java.json \
  --skip-build

go run ./cmd/parity \
  -mode high-cardinality-diff \
  -scenario testdata/parity/high-cardinality-context.json \
  -java-trace /tmp/high-cardinality-java.json \
  -evidence /tmp/high-cardinality.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/high-cardinality-context.json`、`testdata/parity/high-cardinality-context.evidence.json`；当前差异数为 0，覆盖 `ContextKeySegmentedLargeNumberPartitions`（缩减版）。

## Time-window-long-running oracle

第十三个代表性场景使用 `TimeWindowScenarioOracle.java`（runner：`run-time-window.sh`）。它注册 Map 事件类型 `SupportBean(theString, longBoxed)` 与 `SupportMarketDataBean(symbol)`，双跑 `create window MyWindowTW#time(10 sec)`、insert、`select irstream key, value from MyWindowTW` 与 on-delete：虚拟时钟 1s/5s/10s 进入 E1/E2/E3，11s 淘汰 E1，E4 进入，delete 删除 E2，20s 淘汰 E3，快照只剩 E4，再 delete E4 后快照为空。

```sh
./tools/java-oracle/run-time-window.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/time-window-long-running.json \
  --output /tmp/time-window-java.json \
  --skip-build

go run ./cmd/parity \
  -mode time-window-diff \
  -scenario testdata/parity/time-window-long-running.json \
  -java-trace /tmp/time-window-java.json \
  -evidence /tmp/time-window.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/time-window-long-running.json`、`testdata/parity/time-window-long-running.evidence.json`；当前差异数为 0，覆盖 `InfraTimeWindow`。

## Dataflow-connector-output oracle

第十四个代表性场景使用 `DataflowConnectorScenarioOracle.java`（runner：`run-dataflow-connector.sh`）。它双跑 `create objectarray schema MyEventBeacon(p0 string, p1 long)`、`select p0, p1 from MyEventBeacon` 与 `create dataflow MyDataFlowOne BeaconSource -> BeaconStream<MyEventBeacon> { iterations: 3, p0: 'abc', p1: 1 } EventBusSink(BeaconStream) {}`：BeaconSource 产生 3 个事件，EventBusSink 路由到 event bus，s0 listener 收到 3 行。

```sh
./tools/java-oracle/run-dataflow-connector.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/dataflow-connector-output.json \
  --output /tmp/dataflow-connector-java.json \
  --skip-build

go run ./cmd/parity \
  -mode dataflow-connector-diff \
  -scenario testdata/parity/dataflow-connector-output.json \
  -java-trace /tmp/dataflow-connector-java.json \
  -evidence /tmp/dataflow-connector.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/dataflow-connector-output.json`、`testdata/parity/dataflow-connector-output.evidence.json`；当前差异数为 0，覆盖 `EPLDataflowBeacon`。

## Output-after-last oracle

第十五个代表性场景使用 `OutputAfterScenarioOracle.java`（runner：`run-output-after.sh`）。它双跑 `select sum(intPrimitive) as thesum from SupportBean#keepall output after 4 events last every 2 events`：前 4 个事件激活策略且不输出，第 5 个事件进入 last-every 窗口，第 6 个事件输出最终聚合行 thesum=210。

```sh
./tools/java-oracle/run-output-after.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/output-after-last.json \
  --output /tmp/output-after-java.json \
  --skip-build

go run ./cmd/parity \
  -mode output-after-diff \
  -scenario testdata/parity/output-after-last.json \
  -java-trace /tmp/output-after-java.json \
  -evidence /tmp/output-after.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/output-after-last.json`、`testdata/parity/output-after-last.evidence.json`；当前差异数为 0，覆盖 `ResultSetAfterWithOutputLast`。

## Rollup-output-every oracle

第十六个代表性场景使用 `RollupScenarioOracle.java`（runner：`run-rollup.sh`）。它双跑 `select irstream theString as c0, intPrimitive as c1, sum(longBoxed) as c2 from SupportBean#time(3.5 sec) group by rollup(theString, intPrimitive) output every 1 second`：三个 1 秒输出间隔分别交付 9/9、6/6、3/3 的 new/old rollup 行。

```sh
./tools/java-oracle/run-rollup.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/rollup-output-every.json \
  --output /tmp/rollup-java.json \
  --skip-build

go run ./cmd/parity \
  -mode rollup-diff \
  -scenario testdata/parity/rollup-output-every.json \
  -java-trace /tmp/rollup-java.json \
  -evidence /tmp/rollup.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/rollup-output-every.json`、`testdata/parity/rollup-output-every.evidence.json`；当前差异数为 0，覆盖 `ResultSetOutputDefault{join=false}`。

## Match-recognize-simple oracle

第十七个代表性场景使用 `MatchRecognizeScenarioOracle.java`（runner：`run-match-recognize.sh`）。它双跑 `SupportRecogBean#keepall match_recognize ( measures A.theString as a_string, B.theString as b_string all matches pattern (A B) define B as B.value > A.value ) order by a_string, b_string`：E1/E2 不匹配、E3 匹配 E2-E3、E4/E5 匹配、E6/E7/E8 不再匹配。

```sh
./tools/java-oracle/run-match-recognize.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/match-recognize-simple.json \
  --output /tmp/match-recognize-java.json \
  --skip-build

go run ./cmd/parity \
  -mode match-recognize-diff \
  -scenario testdata/parity/match-recognize-simple.json \
  -java-trace /tmp/match-recognize-java.json \
  -evidence /tmp/match-recognize.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/match-recognize-simple.json`、`testdata/parity/match-recognize-simple.evidence.json`；当前差异数为 0，覆盖 `RowRecogConcatenation`。

## Unidirectional-aggregate-join oracle

第十八个代表性场景使用 `UnidirectionalJoinScenarioOracle.java`（runner：`run-unidirectional-join.sh`）。它注册 Map 事件类型 `SupportMarketDataBean(symbol, volume)` 与 `SupportBean(theString, intPrimitive)`，双跑 `@name('s0') select irstream symbol, count(*) as cnt from SupportMarketDataBean unidirectional, SupportBean#keepall where theString = symbol group by theString, symbol`：被动 SupportBean 不输出，三次 SupportMarketDataBean driver 触发分别交付 new/old {E1,1/0}、{E1,2/0}、{E2,1/0}。Go 用 `Join(...).Unidirectional(JoinLeft)` + `GroupBy(JoinField(...))` + `CountAll` + `WithOldStream` 链式 API 复现同一语义。

```sh
./tools/java-oracle/run-unidirectional-join.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/unidirectional-aggregate-join.json \
  --output /tmp/unidirectional-join-java.json \
  --skip-build

go run ./cmd/parity \
  -mode unidirectional-join-diff \
  -scenario testdata/parity/unidirectional-aggregate-join.json \
  -java-trace /tmp/unidirectional-join-java.json \
  -evidence /tmp/unidirectional-join.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/unidirectional-aggregate-join.json`、`testdata/parity/unidirectional-aggregate-join.evidence.json`；当前差异数为 0，覆盖 `EPLJoin2TableJoinGrouped`。

## Output-first-having oracle

第十九个代表性场景使用 `OutputFirstHavingScenarioOracle.java`（runner：`run-output-first-having.sh`）。它注册 Map 事件类型 `SupportBean(doublePrimitive)`，双跑两个 execution：`select doublePrimitive from SupportBean having doublePrimitive > 1 output first every 2 events`（events case）与 `select sum(doublePrimitive) as val0 from SupportBean#length(5) having sum(doublePrimitive) > 100 output first every 2 seconds`（time case，虚拟时钟）。Java 语义要点：HAVING 未命中事件不进入 first-every 事件计数；time case 在 2.999s/4.999s 边界不输出，3s/5s 后下一个可见事件输出。Go 用 `Aggregate(...).Having(...)` + `OutputFirstEveryEvents`/`OutputFirstEveryTime` 链式 API 复现同一语义。

```sh
./tools/java-oracle/run-output-first-having.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/output-first-having.json \
  --output /tmp/output-first-having-java.json \
  --skip-build

go run ./cmd/parity \
  -mode output-first-having-diff \
  -scenario testdata/parity/output-first-having.json \
  -java-trace /tmp/output-first-having-java.json \
  -evidence /tmp/output-first-having.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/output-first-having.json`、`testdata/parity/output-first-having.evidence.json`；当前差异数为 0，覆盖 `ResultSetHavingNoAvgOutputFirstEvents` 与 `ResultSetHavingNoAvgOutputFirstMinutes`。

## Context-keyed-subquery oracle

第二十个代表性场景使用 `ContextKeyedSubqueryScenarioOracle.java`（runner：`run-context-keyed-subquery.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)` 与 `SupportBean_S0(id, p00)`，双跑 `create context SegmentedByString partition by theString from SupportBean` + `context SegmentedByString select theString, intPrimitive, (select p00 from SupportBean_S0#lastevent as s0 where sb.intPrimitive = s0.id) as val0 from SupportBean as sb`。Java 语义要点：每个 keyed 分区拥有独立的 #lastevent 子查询注册表，新建分区看不到创建前的 inner 事件；Go 用 `CreateKeyContext` + `WithContext` + correlated `SubqueryValue`/`LastEvent` 复现同一语义。

```sh
./tools/java-oracle/run-context-keyed-subquery.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/context-keyed-subquery.json \
  --output /tmp/context-keyed-subquery-java.json \
  --skip-build

go run ./cmd/parity \
  -mode context-keyed-subquery-diff \
  -scenario testdata/parity/context-keyed-subquery.json \
  -java-trace /tmp/context-keyed-subquery-java.json \
  -evidence /tmp/context-keyed-subquery.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/context-keyed-subquery.json`、`testdata/parity/context-keyed-subquery.evidence.json`；当前差异数为 0，覆盖 `ContextKeySegmentedSubqueryFiltered`。

## Rowrecog-aggregation oracle

第二十一个代表性场景使用 `RowRecogAggregationScenarioOracle.java`（runner：`run-rowrecog-aggregation.sh`）。它注册 Map 事件类型 `SupportRecogBean(theString, cat, value)`，双跑 `RowRecogMeasureAggregation`（`pattern (A B* C)` + max/min/first/last/count measures，三次匹配后快照）与 `RowRecogMeasureAggregationPartitioned`（`partition by cat` + `pattern (A B B C C D)` + sum measures，两次匹配后快照）。Java 语义要点：B* 为空时 max/min/first/last 为 null、count 为 0；partitioned 匹配按 cat 分区独立累计。Go 用 `PartitionBy` + `TagMax/TagMin/TagLast/TagFirst/TagCount/TagSum` 链式 API 复现同一语义。

```sh
./tools/java-oracle/run-rowrecog-aggregation.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/rowrecog-aggregation.json \
  --output /tmp/rowrecog-aggregation-java.json \
  --skip-build

go run ./cmd/parity \
  -mode rowrecog-aggregation-diff \
  -scenario testdata/parity/rowrecog-aggregation.json \
  -java-trace /tmp/rowrecog-aggregation-java.json \
  -evidence /tmp/rowrecog-aggregation.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/rowrecog-aggregation.json`、`testdata/parity/rowrecog-aggregation.evidence.json`；当前差异数为 0，覆盖 `RowRecogMeasureAggregation` 与 `RowRecogMeasureAggregationPartitioned`。

## Resultset-grouped-time-window oracle

第二十二个代表性场景使用 `ResultSetGroupedTimeWindowScenarioOracle.java`（runner：`run-resultset-grouped-time-window.sh`）。它注册 Map 事件类型 `SupportMarketDataBean(symbol, volume, price)`，双跑 `select symbol, volume, sum(price) from SupportMarketDataBean#time(5.5 sec) group by symbol`（istream 默认）及其 `having sum(price) > 50` 变体：grouped case 为 0-6s 的 9 条 insert 记录，having case 只输出 IBM 75/97 两条，7s/8s 纯时间淘汰边界不输出。Java 语义要点：默认 istream 分组聚合在纯时间淘汰批次不产生 listener 行；Go 已按同一契约修复并锁定。

```sh
./tools/java-oracle/run-resultset-grouped-time-window.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/resultset-grouped-time-window.json \
  --output /tmp/resultset-grouped-time-window-java.json \
  --skip-build

go run ./cmd/parity \
  -mode resultset-grouped-time-window-diff \
  -scenario testdata/parity/resultset-grouped-time-window.json \
  -java-trace /tmp/resultset-grouped-time-window-java.json \
  -evidence /tmp/resultset-grouped-time-window.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/resultset-grouped-time-window.json`、`testdata/parity/resultset-grouped-time-window.evidence.json`；当前差异数为 0，覆盖 `ResultSet1NoneNoHavingNoJoin` 与 `ResultSet3NoneHavingNoJoin`。

## Resultset-row-per-group-simple oracle

第二十三个代表性场景使用 `ResultSetRowPerGroupSimpleScenarioOracle.java`（runner：`run-resultset-row-per-group-simple.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`，双跑 `select theString as c0, sum(intPrimitive) as c1, min(intPrimitive) as c2, max(intPrimitive) as c3 from SupportBean group by theString`：E1/E2/E3 的 9 条 insert 记录，覆盖 sum/min/max 随组更新的轨迹。Go 用 `GroupBy` + `Sum/Min/Max` 链式 API 复现同一语义。

```sh
./tools/java-oracle/run-resultset-row-per-group-simple.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/resultset-row-per-group-simple.json \
  --output /tmp/resultset-row-per-group-simple-java.json \
  --skip-build

go run ./cmd/parity \
  -mode resultset-row-per-group-simple-diff \
  -scenario testdata/parity/resultset-row-per-group-simple.json \
  -java-trace /tmp/resultset-row-per-group-simple-java.json \
  -evidence /tmp/resultset-row-per-group-simple.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/resultset-row-per-group-simple.json`、`testdata/parity/resultset-row-per-group-simple.evidence.json`；当前差异数为 0，覆盖 `ResultSetQueryTypeRowPerGroupSimple`。

## Resultset-aggregate-count-sum oracle

第二十四个代表性场景使用 `ResultSetAggregateCountSumScenarioOracle.java`（runner：`run-resultset-aggregate-count-sum.sh`）。它注册 Map 事件类型 `SupportMarketDataBean(symbol, volume, price, feed)`、`SupportBeanString(theString)`、`SupportBean(theString, intPrimitive, longBoxed, longPrimitive)`、`SupportBean_A(id)` 与 `SupportBean_B(id)`，双跑 `ResultSetAggregateCountSum.java` 的 9 个 execution：原有 `count-one-view`（irstream 分组 `count(*)/count(distinct volume)/count(all volume)` over `#length(3)`，DELL 50/null/25/25/25 + IBM 1/null/null/null 序列，覆盖 null volume、重复值与窗口淘汰 distinct 重算）、`count-join`（同一语句的 SupportBeanString#length(100) join 变体）、`count-simple`（`count(*)` over `#time(1)` 逐事件 1/2/3）与 `sum-named-window-remove-group`（keepall named window + insert + 按 id on-delete 触发下的分组 sum：组删除输出 null 行、iterator 按 order by theString 逐组快照），以及新增 `count-plus-star`（`select *` 加累积 count）、`count-having`/`sum-having`（无窗口 sum/count 的进入退出）、`count-one-view-om`（SODA model round-trip 与 grouped count 矩阵）和 `nested-avg`（length(3) 分组 `avg(count(*))` 历史前缀）。

```sh
./tools/java-oracle/run-resultset-aggregate-count-sum.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/resultset-aggregate-count-sum.json \
  --output /tmp/resultset-aggregate-count-sum-java.json \
  --skip-build

go run ./cmd/parity \
  -mode resultset-aggregate-count-sum-diff \
  -scenario testdata/parity/resultset-aggregate-count-sum.json \
  -java-trace /tmp/resultset-aggregate-count-sum-java.json \
  -evidence /tmp/resultset-aggregate-count-sum.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/resultset-aggregate-count-sum.json`、`testdata/parity/resultset-aggregate-count-sum.evidence.json`；当前差异数为 0，覆盖 `ResultSetAggregateCountSimple`、`ResultSetAggregateCountPlusStar`、`ResultSetAggregateCountHaving`、`ResultSetAggregateSumHaving`、`ResultSetAggregateCountOneViewOM`、`ResultSetAggregateGroupByCountNestedAggregationAvg`、`ResultSetAggregateCountOneView`、`ResultSetAggregateCountJoin` 与 `ResultSetAggregateSumNamedWindowRemoveGroup`，共 55 条 records。

## Resultset-aggregate-ever oracle

第二十五个代表性场景使用 `ResultSetAggregateFirstEverLastEverScenarioOracle.java`（runner：`run-resultset-aggregate-first-ever-last-ever.sh`）。它注册 struct 事件类型 `SupportBean(theString, intPrimitive, intBoxed, boolPrimitive)` 与 `SupportBean_A(id)`，双跑 `ResultSetAggregateFirstEverLastEver.java` 的 3 个可表示 execution：SODA/EPL `firstever`/`lastever`、窗口 `first`/`last` 与 `countever`/filtered `countever` 的 `length(2)` current/ever 轨迹，以及 keepall named-window on-delete 删除后保留 ever 历史。场景包含 null boxed 值、窗口淘汰和按布尔表达式过滤的 count；`countever(distinct ...)` 的 compile-error execution 由 Go 类型安全 API 保持 implemented-only。

```sh
./tools/java-oracle/run-resultset-aggregate-first-ever-last-ever.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/resultset-aggregate-first-ever-last-ever.json \
  --output /tmp/resultset-aggregate-first-ever-last-ever-java.json \
  --skip-build

go run ./cmd/parity \
  -mode resultset-aggregate-first-ever-last-ever-diff \
  -scenario testdata/parity/resultset-aggregate-first-ever-last-ever.json \
  -java-trace /tmp/resultset-aggregate-first-ever-last-ever-java.json \
  -evidence /tmp/resultset-aggregate-first-ever-last-ever.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/resultset-aggregate-first-ever-last-ever.json`、`testdata/parity/resultset-aggregate-first-ever-last-ever.evidence.json`；当前差异数为 0，覆盖 `ResultSetAggregateFirstLastEver{soda=true}`、`ResultSetAggregateFirstLastEver{soda=false}` 与 `ResultSetAggregateOnDelete`，共 14 条 records。

## Subselect-aggregated-in-exists-any-all oracle

第二十六个代表性场景使用 `EPLSubselectAggregatedInExistsAnyAllScenarioOracle.java`（runner：`run-subselect-aggregated-in-exists-any-all.sh`）。它注册 Map 事件类型 `SupportBean(theString, intPrimitive)`、`SupportValueEvent(value)` 与 `SupportIdAndValueEvent(id, value)`，双跑 `EPLSubselectAggregatedInExistsAnyAll` 的全部 13 个 execution：SupportValueEvent 触发投影 IN/NOT IN、EXISTS/NOT EXISTS 与 ALL/ANY/SOME 量词，对照 SupportBean#keepall 上的聚合子查询（无分组、按 theString 分组、带 `last(theString)/first(theString)` having 过滤），并覆盖 named window + `delete from MyWindow` fire-and-forget 后 EXISTS 复位。Java 语义要点：无分组聚合子查询空输入仍产出 null 聚合行（`10 in (null)` 为 null、EXISTS 为 true），分组空集遵循 SQL 规则（IN/ANY/SOME false、ALL true、EXISTS false），having 过滤掉唯一聚合行时 IN 为 null。Go 以 `SubqueryIn/SubqueryAny/SubqueryAll/SubquerySome/SubqueryExistsValue` 及 `SubqueryHaving` 复现同一语义；

```sh
./tools/java-oracle/run-subselect-aggregated-in-exists-any-all.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/subselect-aggregated-in-exists-any-all.json \
  --output /tmp/subselect-aggregated-java.json \
  --skip-build

go run ./cmd/parity \
  -mode subselect-aggregated-in-exists-any-all-diff \
  -scenario testdata/parity/subselect-aggregated-in-exists-any-all.json \
  -java-trace /tmp/subselect-aggregated-java.json \
  -evidence /tmp/subselect-aggregated.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/subselect-aggregated-in-exists-any-all.json`、`testdata/parity/subselect-aggregated-in-exists-any-all.evidence.json`；当前差异数为 0，覆盖 `EPLSubselectAggregatedInExistsAnyAll` 的 13 个 execution。

## Subselect-aggregated-single-value oracle

第二十七个代表性场景使用 `EPLSubselectAggregatedSingleValueScenarioOracle.java`（runner：`run-subselect-aggregated-single-value.sh`）。它注册 Map 事件类型 `SupportBean_S0(id, p00)`、`SupportBean_S1(id, p10, p11)` 与 `SupportBean(theString, intPrimitive)`，双跑 `EPLSubselectAggregatedSingleValue` 的 13 个 execution：单值聚合子查询覆盖无窗口累积 `sum`、keepall + having、select 子句内 `s0.id + max(s1.id)` 混合投影、length(3) where 过滤、相关 count、where 子句相关子查询（含 `||` 拼接变体）、相关 where + having、分组 scalar 子查询（相关 having、having 内相关 `sum = s0.id`）、`last(theString)` 相关 having，以及 table + into-table 聚合列上的 having 子查询（无键与双键分组）。Go 以 `SubquerySum/SubqueryValue/SubqueryGroupScalar` + `OuterField` + `SubqueryHaving` + `IntoTable`/`FromTable` 复现同一语义；table 子查询的行投影与分组键使用 `Field`（`TableField` 是 trigger/assignment 目标行语义）。

```sh
./tools/java-oracle/run-subselect-aggregated-single-value.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/subselect-aggregated-single-value.json \
  --output /tmp/subselect-single-java.json \
  --skip-build

go run ./cmd/parity \
  -mode subselect-aggregated-single-value-diff \
  -scenario testdata/parity/subselect-aggregated-single-value.json \
  -java-trace /tmp/subselect-single-java.json \
  -evidence /tmp/subselect-single.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/subselect-aggregated-single-value.json`、`testdata/parity/subselect-aggregated-single-value.evidence.json`；当前差异数为 0，覆盖 `EPLSubselectAggregatedSingleValue` 的 13 个 execution。

## Subselect-in oracle

第二十八个代表性场景使用 `EPLSubselectInScenarioOracle.java`（runner：`run-subselect-in.sh`）。它注册 Map 事件类型 `SupportBean_S0(id, p00, p01)`、`SupportBean_S1(id, p10, p11)` 与 `SupportBean(theString, intBoxed, longBoxed)`，双跑 `EPLSubselectIn` 的 14 个 execution：IN/NOT IN 子查询在 select 子句（含 OM/Compile 等价变体）、filter criteria（length(2) 窗口淘汰边界）与 where 子句（含 `3*id in (select 2*id)` 双侧表达式）的位置形态，nullable 字符串（p00/p10）与 boxed 数值（longBoxed in intBoxed 及反向）的 coercion 三值逻辑，null row（`x in (null)` → null、空集 `not in` → true 含 null 外层），以及 keepall 相关 IN 索引形态（`s0.p01 in (s1.p10, s1.p11)` 与 `s1.p11 in (s0.p00, s0.p01)`）。Go 以 `SubqueryIn` + `In` + `Not` 与 boxed 指针值语义（`reflect.DeepEqual` 指针解引用）复现同一行为。

```sh
./tools/java-oracle/run-subselect-in.sh \
  --esper-root /root/app/esper \
  --scenario testdata/parity/subselect-in.json \
  --output /tmp/subselect-in-java.json \
  --skip-build

go run ./cmd/parity \
  -mode subselect-in-diff \
  -scenario testdata/parity/subselect-in.json \
  -java-trace /tmp/subselect-in-java.json \
  -evidence /tmp/subselect-in.evidence.json
```

checked-in 场景与 evidence：`testdata/parity/subselect-in.json`、`testdata/parity/subselect-in.evidence.json`；当前差异数为 0，覆盖 `EPLSubselectIn` 的 14 个 execution。

## Java regression baseline

Java 基线记录在 `testdata/compat/java-regression-baseline.json`。该基线不是 Go parity 证据。需要 MySQL 的 Java fixture、Docker 命令和 ready 检查见 [外部服务集成](../../docs/integration/external-services.md)；Java regression-run 的完整构建仍需在有对应 Maven/JDK 和外部服务的环境执行。
