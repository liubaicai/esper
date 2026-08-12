# Java oracle runner

该目录保留固定 Esper 9.0.0 Java 对照 runner 的接入约定。当前环境已发现 Java 17，并已取得 Maven 3.9.16 可执行文件；运行态 execution 枚举探针和 Java 全量 regression-run 基线已经接入，但 Java/Go 行为差分和 Go 全量映射仍未完成。

要求：

- Java source：`D:\Code\soc\esper`，commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`。
- Java 17、固定 locale/timezone/charset/seed 和外部服务版本。
- 当前机器的 JDK：`C:\Program Files\Microsoft\jdk-17.0.20.8-hotspot`；Maven 由 Chocolatey 下载，当前可执行文件位于 `C:\Users\baicai\AppData\Local\UniGetUI\Chocolatey\lib-bad\maven\3.9.16\apache-maven-3.9.16`。Chocolatey 最终写入环境变量时因非管理员会话失败，因此 runner 必须显式设置 `JAVA_HOME`/Maven 路径，或在 CI 中使用正式安装的 Maven。
- `ExecutionInventory.java`/`run-probe.ps1` 通过外层 suite 的 `executions()` 枚举动态 execution，并覆盖无参、布尔参数变体、direct execution 和 `RegressionExecutionPreConfigured`；记录外层类、factory 变体、execution class、`name()`、ordinal、flags 和基于外层 ID/变体/ordinal 生成的稳定 `runtimeId`。探针只构造 execution，不调用 `run(RegressionEnvironment)`。
- Java 与 Go 共享 `compat` 中的 `esper-parity/v1` 场景和 trace 规范化协议；typed bean、XML DOM、Avro 等通过 host setup/codec 接入。
- 非 Regression 的 Java 源资产可由 `go run ./cmd/source-manifest -source D:\Code\soc\esper -out testdata/compat/source-test-manifest.json` 生成；当前清单只做静态分类，仍需 Java 运行态枚举和 Go disposition/mapping。

已执行的 Java 环境基线：官方 `common/etc/regression/create_testdb.sql` 夹具已加载到 MySQL 8.0.46 Docker（镜像 digest 见 `testdata/compat/java-regression-baseline.json`，`localhost:3306/test`）；`common` 585/585、`common-avro` 13/13、`common-xmlxsd` 10/10、`compiler` 69/69、`runtime` 50/50、`TestSuitePattern` 28/28、`TestSuiteContext` 17/17 均通过。`regression-run` 全量为 78 个 suite、860 个 JUnit 条目，首轮 857 个无失败/错误、2 个已登记 approved difference（MySQL 诊断文本和容器/JDBC 性能阈值）、1 个多线程上下文计数波动；隔离重跑 `TestSuiteMultithread` 为 41/41。首轮计数和 disposition 固化在 `testdata/compat/java-regression-baseline.json`；Java 工作树的 `regression-run/target/surefire-reports` 保留最近的全量分片/隔离报告，重现首轮结果以 JSON 基线中的命令和差异记录为准。

## MySQL 回归夹具

启动本地容器：

```powershell
docker run --name esper-java-mysql --env MYSQL_ROOT_PASSWORD=password --env MYSQL_DATABASE=test --publish 3306:3306 --detach mysql:8.0
```

等待 `mysqladmin ping` 返回 ready 后，将 `D:\Code\soc\esper\common\etc\regression\create_testdb.sql` 导入 `test` 库。该文件首行是说明性 `//` 注释，导入前需过滤 `^\s*//` 行；当前固定夹具应得到 `mytesttable=10`、`mytesttable_large=1000`、`mytestupsert=0`。Java 数据库测试使用 root/password 和 `jdbc:mysql://localhost/test`。

本地 install 需要 `-Dgpg.skip=true`，因为当前机器没有 `gpg.exe`；这只适用于验证工件安装，发布构建不得取消签名门禁。Maven 还报告源码编码未固定并按 GBK 编译，Java oracle 必须固定 UTF-8、locale、timezone 和 charset。

运行探针（默认使用当前机器的 JDK/Maven 路径；需要构建时去掉 `-SkipBuild`）：

```powershell
pwsh -File .\tools\java-oracle\run-probe.ps1 -SkipBuild
```

输出为 `testdata/compat/java-execution-inventory.jsonl`。本轮结果：静态候选 3,848 条、外层 suite 801 个、运行态记录 4,140 条，其中 4,136 条可枚举、4 条明确 ignored、0 条 probe error；4,136 条 `runtimeId` 无重复。清单包含 3,846 条默认 `executions()` 记录、4 条布尔参数变体、277 条 direct、1 条空集合 direct fallback 和 8 条 preconfigured。静态 `testdata/compat/static-manifest.json` 仍只作为候选输入，最终映射必须以运行态清单和实际 JUnit 入口为准。
