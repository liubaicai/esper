# 项目目录结构

本仓库遵循 Go Modules、Go 编译器的 `internal` 可见性规则，以及
`golang-standards/project-layout` 中适用于公共 Go 库的目录约定。该参考布局并非
Go 官方规范，也不要求每个项目创建全部目录；本仓库只保留有明确职责的目录。

```text
.
├── cmd/                 可执行程序的薄入口
├── connectors/          对外公开的连接器子包
├── docs/                架构决策与实现文档
├── examples/            公共 API 使用示例
├── internal/app/        命令实现，禁止仓库外导入
├── internal/compat/     Java/Go 对账工具，禁止仓库外导入
├── scripts/             构建与结构校验脚本
├── testdata/compat/     版本化兼容性清单和 Java 基线
├── testdata/parity/     跨实现 parity 场景
├── tools/java-oracle/   Java inventory 探针及说明
├── *.go                 公共 `esper` 库 API 与实现
├── go.mod               模块定义
└── Makefile             统一开发入口
```

## 边界规则

- 根目录保留公共包 `github.com/liubaicai/esper`。将它搬到 `pkg/esper` 会破坏现有
  导入路径；对公共 Go 库而言，根包本身就是常规布局。
- `connectors` 是公开扩展 API，不放入 `internal`。新增公开子包前需要确认稳定性
  和跨包依赖方向。
- `cmd/<name>/main.go` 只负责标准流接线和进程退出码，业务逻辑必须放在
  `internal/app/<name>` 并通过单元测试验证。
- 仓库私有代码放在 `internal`，由 Go 编译器阻止外部导入。不要创建顶层 `src`。
- 固定测试资产放在 `testdata`；覆盖率、日志、profile、测试二进制和构建产物不得
  提交。
- `tools` 只存放项目开发工具，`examples` 只展示公共 API，不承载生产实现。

## 开发入口

```text
make check          # 结构门禁、go vet、全量测试
make test-race      # 全量 race 测试
make fmt            # 格式化 Go 源码
```

`scripts/check-layout.sh` 会检查薄命令入口、禁止目录、生成物、格式化状态和包图。
