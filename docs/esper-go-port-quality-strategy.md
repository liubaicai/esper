# Esper Go 移植质量策略

## 1. 文档职责

本文定义迁移质量口径、Java/Go 差分证据、合成数据、门禁和最终验收标准。执行步骤见 [迁移执行手册](esper-go-port-runbook.md)，当前优先级见 [执行路线图](esper-go-port-roadmap.md)，完整功能范围和架构约束见 [实施规划](esper-go-port-implementation-plan.md)。

质量事实按以下优先级判定：

1. `testdata/compat/capability-manifest.json` 及其校验结果；
2. `testdata/parity` 中可重放的 scenario、trace 和 evidence；
3. 自动化测试、race、Docker 集成和性能产物；
4. 路线图、README 和 CHANGELOG 中的人工说明。

人工文档与机器清单冲突时，以通过校验的机器证据为准，并修正文档。

## 2. Oracle 与范围

- Java Esper 9.0.0 固定 commit 由 capability manifest 的 `javaCommit` 字段定义。
- Java 只作为可观测行为 oracle，不要求复制 JVM 类结构、反射、类加载或序列化实现。
- Go 公共规则 API 使用类型化链式 builder；Java EPL 只用于运行 Java oracle。
- “完整移植”指约定范围内的行为和能力对等，不是 Java 文件或类的一一翻译。
- NEsper、EsperHA 和商业能力不在当前范围，具体边界以实施规划为准。

## 3. 验证状态

状态必须使用 manifest v2 已定义的词汇，不另造近义状态：

| 状态 | 含义 | 最低证据 |
| --- | --- | --- |
| `inventoried` | 已盘点 Java 能力，尚未证明 Go 实现 | Java refs/runtime IDs、remaining |
| `implemented` | Go 行为已有定向测试，但尚无完整自动差分证据 | Go refs/tests、Java mapping |
| `differential-verified` | 相同 scenario 的 Java/Go trace 自动比较为零差异 | runtime IDs、scenario、两端 trace、evidence |
| `intentionally-different` | 有意且经记录的 Go/JVM 或 API 差异 | 双方行为、原因、影响、替代方式、测试 |
| `representative-verified` | 合成组合场景通过 | representative scenario IDs |
| `nfr-verified` | 性能、并发或资源验收通过 | 可重放命令、阈值和结果 |

约束：

- `implemented` 不能计作行为差分通过。
- runtime 只有出现在 `differentialVerifiedRuntimeIds` 中才计入差分验证进度。
- 代表性场景用于发现组合缺陷，不能替代对应 Java runtime 的差分证据。
- 未实现、只实现子集、测试不足或暂时困难不得登记为 `intentionally-different`。
- 单一总百分比不得用于表示项目完成度；至少分别报告 runtime 关联率、差分验证率、代表场景和 NFR 状态。

## 4. Java/Go 差分证据

一个 runtime 或紧密相关 runtime 组达到 `differential-verified`，必须具备：

1. 固定 Java commit 和明确的 Java source/execution/runtime ID；
2. 语言无关、可重放、使用固定输入的 scenario；
3. Java 和 Go 对同一 scenario 生成的规范化 trace；
4. 自动 diff 结果为零差异；
5. manifest 中的证据路径与实际文件一致；
6. 相关 Go 测试和 manifest 校验通过。

Trace 应按能力风险覆盖以下可观测面：

- new/old stream、记录数量、字段值、类型和顺序；
- Null、Missing、空集合、typed nil 和数值转换；
- 虚拟时间、timer、窗口快照和 iterator；
- deploy/undeploy/redeploy、变量和状态生命周期；
- build、deploy、runtime 错误发生阶段与稳定错误类别；
- listener/subscriber/sink 的批次和回调顺序。

人工抄写 Java 期望值可以作为单元回归，但不能单独构成自动差分证据。

Mutation 不是每个 execution 都重复建设。以下情况必须增加或更新 mutation：

- 新增 runner、trace 字段、normalizer 或 diff 规则；
- 新增一种此前没有覆盖的语义类别；
- 修复曾被 diff 漏报的缺陷。

Mutation 至少证明值、顺序、Null 状态、时间边界或记录缺失中的相关变化会被拒绝。

## 5. 合成测试数据

当前没有真实生产数据，合成数据按风险分层，而不是要求每个测试机械覆盖全部组合。

### 5.1 所有能力

- 正常路径和最小有效输入；
- Null、Missing、空集合或零值中适用的情况；
- 一个明确的非法输入或非法生命周期路径；
- 固定随机种子；失败种子最小化后固化为回归测试。

### 5.2 状态、窗口、时间和模式能力

- 长于最小样例的事件序列；
- 窗口边界前、边界点、边界后；
- 重复、乱序、相同时间戳以及适用的迟到事件；
- 无匹配、单匹配、多匹配；
- snapshot/iterator 与增量输出的一致性；
- deploy/undeploy/redeploy 后状态和 timer 清理。

### 5.3 并发、连接器和外部状态

- 重入、并发发送、取消和关闭；
- 重复消息、断线重连和至少一次投递边界；
- Docker fixture 中的真实 round-trip；
- goroutine、连接、事务和缓存资源释放。

Property、metamorphic 和 fuzz 测试优先用于共享解析、类型转换、状态机、窗口、索引和序列化边界，不为追求数量而覆盖简单 getter 或纯转发代码。

## 6. 代表性组合场景

代表性场景跨越多个单元能力，用来发现局部测试无法发现的组合缺陷。场景集合至少覆盖：

- filter + window + aggregate；
- 多流 join；
- pattern + timer；
- context/partition；
- named window/table mutation；
- subquery 和 output policy；
- deployment 依赖、变量与重启边界；
- dataflow/connector 输出；
- 长时间运行和高基数状态。

只有 scenario 可重放、结果有明确断言且对应 evidence 通过时，才能登记 `representative-verified`。

## 7. 本地门禁

项目暂不建设 GitLab CI，直接在 `master` 开发和推送。自动化平台缺失不降低本地门禁。

### 7.1 开发迭代

开发中只运行受影响的包、定向 Go 测试和相关差分，缩短反馈时间。已知失败必须立即处理，不得跨过当前语义继续堆叠实现。

### 7.2 工作单元提交前

至少通过：

```text
gofmt/格式检查
项目结构和生成代码漂移检查
go vet ./...
go test ./... -count=1
受影响的 Java/Go differential tests
manifest/evidence/document consistency checks
```

### 7.3 里程碑收口

完成一个 capability 子域或高风险共享基础设施后，再执行成本较高的门禁：

```text
go test -race ./... -count=1
相关域的完整差分场景
medium/stress 合成负载
相关 benchmark 和资源泄漏检查
受影响的 Docker 外部服务集成测试
```

无相关外部依赖时不强行启动 Docker；有相关依赖时不能以普通测试中的 Skip 代替 Docker 验证。

### 7.4 失败规则

- 任何基础门禁失败时停止新增功能，先恢复绿色基线。
- 不得通过删除测试、放宽断言、缩小关键输入、反复重跑 flaky test 或登记差异绕过失败。
- 环境型 Skip 必须有原因、启用变量和可执行验证命令。
- flaky test 视为缺陷，必须修复测试假设或实现。

## 8. 性能与稳定性

NFR 在共享实现或 capability 子域语义稳定后验证，不要求每个 Java execution 单独跑基准。

至少跟踪：

- events/sec 和 P50/P95/P99 延迟；
- 每事件分配和稳态内存；
- goroutine、timer、连接和状态数量；
- 高基数分组和长窗口的增长曲线；
- deploy/undeploy/redeploy 的资源回收；
- 长序列执行时间是否持续恶化。

尚无生产 SLO 时，先记录 Java baseline、Go baseline、环境和趋势。没有阈值与可重放结果时不得登记 `nfr-verified`。

## 9. 最终验收

只有同时满足以下条件，才能宣称约定范围内完成移植：

- Java runtime inventory 已全部处置；
- 所有声明已实现的适用 runtime 都有自动差分证据；
- 所有 intentionally-different 项都有完整理由和测试；
- 代表性组合场景全部通过；
- 格式、结构、生成、vet、test、race 和相关 Docker 门禁通过；
- 性能、稳定性、并发和资源回收达到书面阈值；
- 没有未解释的 Skip、flaky test、TODO、stub 或失败门禁；
- manifest、evidence、实现和文档一致。

在此之前不得宣称“完整 Esper 对等”“完全兼容”或“可以直接替换 Java Esper”。
