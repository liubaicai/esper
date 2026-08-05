# ADR-003：Go Module 与阶段 1 边界

状态：Accepted for the initial implementation

## 决策

- Go module 使用 `github.com/liubaicai/esper`，与目标仓库远端保持一致。
- 阶段 1 的公共入口以根包 `esper` 提供，先验证链式 API、AST、编译验证和运行时生命周期；只有在跨域原型证明包边界后再拆分公共子包。
- 规则构造不接受 EPL 字符串，也不经过 EPL 往返解析。
- 阶段 1 默认使用内存状态和显式虚拟时钟；同步 Send 是 parity 默认路径。
- 首批稳定验证能力包括：struct/map/JSON 基础事件、字段/常量/变量/比较/逻辑表达式、Filter、length/time Window、Projection、new/old stream、Listener/Sink、Build/Deploy/Send/AdvanceTime/Undeploy。
- 当前工作树另有 experimental 垂直切片：内连接、基础聚合、内存 Table/Named Window 和基础 Fire-and-Forget；这些能力仍需完整 Java 差分后才能进入稳定契约。

## 暂不承诺

该边界不是最终 Esper 功能范围的缩减。外连接/多流 Join、完整聚合和输出、Pattern、Context、Dataflow、Avro/XML、完整 FAF、连接器和完整配置继续按实施规划扩展；它们不能因为阶段 1 尚未完成而被标记为不适用。

## 原因

Go 不支持方法级的新类型参数，类型变化操作不能机械照搬 Java/Flink 的泛型方法链。阶段 1 用保持类型的方法和顶层类型变化组合器验证可演进的 API 形态，并保留动态 Row/Schema 过渡路径。
