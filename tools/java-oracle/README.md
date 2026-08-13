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

## Java regression baseline

Java 基线记录在 `testdata/compat/java-regression-baseline.json`。该基线不是 Go parity 证据。需要 MySQL 的 Java fixture、Docker 命令和 ready 检查见 [外部服务集成](../../docs/integration/external-services.md)；Java regression-run 的完整构建仍需在有对应 Maven/JDK 和外部服务的环境执行。
