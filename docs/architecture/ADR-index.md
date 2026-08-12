# ADR 索引与状态

规划文档列出的 ADR 必须和代码、测试、发布材料一起维护。当前只把已经在工作树中有明确证据的决策标为 Accepted；其余条目不能因为有草案文字就视为批准。

| ADR | 主题 | 当前状态 | 证据/下一步 |
|---|---|---|---|
| ADR-001 | GPLv2/商业许可、版权和派生代码发布方式 | Pending review | 当前仓库尚缺正式 LICENSE/NOTICE；复制 Java 源码或 fixture 前必须完成法务门禁 |
| ADR-002 | EPL 文本入口边界 | Proposed | 已确定目标是直接 Go AST/Builder；需补迁移说明和能力矩阵 |
| ADR-003 | Go module、最低版本和阶段 1 边界 | Accepted for initial implementation | `ADR-003-go-module-and-stage1-boundary.md` |
| ADR-004 | Schema、Value、Null/Missing 和数值模型 | Prototype | `value.go`、`schema.go`、对应单测；decimal/BigInteger/提升矩阵仍待 spike |
| ADR-005 | 链式 API、泛型边界和动态事件 API | Prototype | `stream.go`、`pattern.go`、`context.go`；v1 前须完成高级 builder 评审 |
| ADR-006 | Clock、Scheduler、时间精度和时区 | Prototype | `VirtualClock` 已有；Scheduler、DST、cron、微秒精度仍待实现/差分 |
| ADR-007 | Engine 并发、顺序、背压和回调模型 | Prototype | 同步 Send、稳定 statement/listener 顺序、race 已验证；异步/背压仍未冻结 |
| ADR-008 | Plan 工件、函数注册和版本化序列化 | Prototype | `plan_artifact.go` 只有校验封装，不能恢复可部署 Query；函数/Serde 注册尚未完成 |
| ADR-009 | StateStore、索引和资源回收 | Prototype | `state.go` 的内存 Table/Named Window；统一 StateStore、持久化和泄漏门禁仍待实现 |
| ADR-010 | 脚本、插件、JMS 等 Java 专属能力 | Proposed | 默认采用显式注册或进程外 bridge；需逐 capability 形成 approved-difference |
| ADR-011 | 正则、Unicode、collation、decimal/BigInteger | Pending spike | Go RE2/UTF-8 目前只覆盖基础路径；需基于 Java 用例选依赖并评审许可证/安全 |
| ADR-012 | JSON/XML/XSD/XPath/Avro 实现与安全边界 | Prototype | 基础 JSON、严格基础 XML 和 Avro JSON datum 已有；XML 外部实体/XSD/XPath、二进制 Avro、Serde、Schema bomb 和版本矩阵未完成 |
| ADR-013 | API SemVer、Plan/State 格式版本和升级矩阵 | Proposed | Plan schema 已独立版本化；需补 State、向前/向后读写/拒绝测试 |
| ADR-014 | 资源配额、非可信输入、连接器与扩展威胁模型 | Proposed | 规划文档有控制项；需形成 threat model、默认配额和滥用/压力测试 |
| ADR-015 | Go 项目目录和包边界 | Accepted | `ADR-015-project-layout.md`；根公共包、公开连接器、`internal` 私有实现和 `testdata` 资产边界已落地 |

## 维护规则

- `Pending review` 不得作为实现授权或发布承诺。
- `Prototype` 只表示已有代码/测试证据，不表示 Java parity 或 API 稳定。
- 每个 ADR 的状态变化必须同时更新规划看板、capability manifest 和相关测试。
- 许可证、外部依赖、第三方协议、格式兼容和安全边界需要明确 owner 与评审记录，不能由单个实现提交隐式决定。
