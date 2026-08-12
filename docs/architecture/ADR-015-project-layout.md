# ADR-015：Go 项目目录和包边界

状态：Accepted

## 决策

- 根目录继续提供公共包 `github.com/liubaicai/esper`。这是库模块的稳定导入路径，
  不额外引入 `pkg/esper` 层级。
- 对外连接器保留在 `connectors` 公共子包；仓库开发工具和命令实现放入
  `internal`，由 Go 编译器强制执行私有边界。
- 每个 `cmd/<name>` 只保留一个 `main.go`，负责标准流接线和退出码；实际逻辑放在
  `internal/app/<name>`。
- Java/Go 对账代码放在 `internal/compat`，版本化清单、基线和场景放在
  `testdata/compat` 或 `testdata/parity`。
- 使用 `scripts/check-layout.sh`、`Makefile` 和 `.gitignore` 阻止顶层 `src`、厚命令
  入口、未格式化源码和生成物进入仓库。

## 原因

`golang-standards/project-layout` 是社区参考布局，不是 Go 官方规范，也不要求创建所有
候选目录。对公共库机械使用 `pkg/esper` 会改变现有导入路径并给用户带来无收益的迁移；
根包承载公共 API、`internal` 承载私有实现，才是本模块可由编译器验证的边界。

兼容性代码只由仓库内命令和测试消费，不属于用户 API；兼容性 JSON/JSONL 是测试资产，
也不应与可导入 Go 包混放。命令采用薄入口后，参数解析和执行路径可以直接单元测试。

## 结果

- `github.com/liubaicai/esper` 和公开连接器的导入路径保持兼容。
- 仓库外代码不能再导入对账实现。
- 目录职责可通过 `make check` 持续校验。
- 根包仍然较大；后续只有在形成稳定且无循环依赖的领域 API 后才拆出新的公共子包。
