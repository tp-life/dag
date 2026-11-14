# Changelog

## Unreleased

### Added

- `Plan.Graph()`：返回结构化依赖图快照（节点、层级、依赖边）。
- `Plan.DOT()`：导出 Graphviz DOT，可直接渲染为 SVG/PNG。

## v2.0.0 - 2026-11-14

### Added

- `Builder -> Plan -> State/Result` 生命周期；`Plan` 不可变，可并发多次运行。
- 结构化 `Key`：`TypeKey[T]()` 与 `NameKey(name)` 分属独立命名空间。
- 只读 `State`、独立运行值存储、`Result.Errors/Skipped/Report`。
- `Provide0` ~ `Provide8` 类型安全注入，无反射调用。
- `Hook` 与 `RunReport`：服务开始/结束、耗时、状态、层级。
- `RunOption`：`FailFast`、`ContinueOnError`、`WithMaxConcurrency`、`WithServiceTimeout`、`WithHook`。
- 就绪队列调度器：依赖完成即启动后继，不等待同层其他节点。
- `Error` / `RunErrors`，支持 `errors.Is/As` 与多错误聚合。
- 反射路径注册期预计算参数，支持 `*State` 注入与 `after` 显式依赖。
- benchmark：typed、reflection、并行运行、100 节点链式图。

### Changed

- 模块路径改为 `github.com/tp-life/dag/v2`。
- 错误格式改为 `dag: service <key> (layer n): ...`。
- `Plan` 的 `Run` 支持可变参数 `RunOption`，原有调用保持兼容。

### Removed

- v1 的可变容器 API：`FxDag`、`DefaultDag`、`Draw`、`Execute`、`Clear`、`DeleteProvider`、`Stop`。
- `GenName`（由 `TypeKey` 取代）。
- `golang.org/x/sync` 依赖。

## v1.0.0

- 初始版本：运行时注册、拓扑分层、层内并行执行。
