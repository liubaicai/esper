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

## Java regression baseline

Java 基线记录在 `testdata/compat/java-regression-baseline.json`。该基线不是 Go parity 证据。需要 MySQL 的 Java fixture、Docker 命令和 ready 检查见 [外部服务集成](../../docs/integration/external-services.md)；Java regression-run 的完整构建仍需在有对应 Maven/JDK 和外部服务的环境执行。
