# dag

有向无环图（DAG）依赖执行库，基于 Go 1.27 泛型方法实现。

v2 采用 `Builder → Plan → State/Result` 的分层模型：

- `Builder`：注册期，收集服务与初始值。
- `Plan`：不可变执行计划，可并发多次运行。
- `State`：handler 可见的只读运行状态。
- `Result`：单次运行的结果快照。

## 快速开始

```go
b := dag.NewBuilder()

// 初始值，按类型 key 注册
b.Init(&Config{})

// 类型化 handler；after 用于声明执行顺序
b.Provide(func(ctx context.Context, s *dag.State) (*A, error) {
    return &A{}, nil
})
b.Provide(func(ctx context.Context, s *dag.State) (*B, error) {
    a, _ := s.LoadData[*A]()
    return &B{A: a}, nil
}, dag.TypeKey[*A]())

// Build 会校验缺失依赖、非生产依赖、循环依赖
plan, err := b.Build()
if err != nil {
    return err
}

// 每次 Run 都有独立的值存储，同一个 Plan 可并发运行
result, err := plan.Run(ctx)
bv, ok := result.Get[*B]()
```

## 命名 key

类型 key 与命名 key 是两个独立命名空间：

```go
b.ProvideNamed("a", func(ctx context.Context, s *dag.State) (*A, error) {
    return &A{}, nil
})
b.ProvideNamed("b", func(ctx context.Context, s *dag.State) (*B, error) {
    a, _ := s.LoadDataByName[*A]("a")
    return &B{A: a}, nil
}, dag.NameKey("a"))

result.GetNamed[*B]("b")
```

## 任意签名函数（反射自动装配）

```go
// 按参数类型自动装配依赖；返回 (T, error)
b.Wrap[*C](func(ctx context.Context, b *B) (*C, error) {
    return &C{B: b}, nil
})

// 不产出值
b.WrapNoName(func(ctx context.Context) {
    // ...
})
```

`*State` 参数会直接注入只读运行状态，不会被当作依赖；使用 `*State` 动态读取时，
需要通过 `after` 显式声明顺序：

```go
b.Wrap[*C](func(ctx context.Context, s *dag.State) (*C, error) {
    b, _ := s.LoadData[*B]()
    return &C{B: b}, nil
}, dag.TypeKey[*B]())
```

## 生命周期与并发

- `Builder` 并发安全；`Build` 之后可继续添加定义，已生成的 `Plan` 不受影响。
- `Plan` 不可变，可被多个 goroutine 并发 `Run`；要求 handler 本身可并发执行。
- 每次 `Run` 使用独立值存储；出错时 `Result` 仍包含已完成部分的结果。
- 注册与执行状态分离，不再存在 `Provide` / `Execute` 交错修改同一状态的问题。

## 取消

通过 context 取消运行：

```go
ctx, cancel := context.WithCancel(context.Background())
go func() {
    // ...
    cancel()
}()
result, err := plan.Run(ctx)
```

## 从 v1 迁移

| v1 | v2 |
|---|---|
| `New()` / `DefaultDag` | `NewBuilder()`，显式创建 |
| `Provide` / `ProvideByName` | `Provide` / `ProvideNamed` |
| `ProvideByNameWithOut` | `ProvideWithoutValue` |
| `WrapHandler` / `WrapHandlerNoName` | `Wrap` / `WrapNoName` |
| `InitParams` / `InitParamsByName` | `Init` / `InitNamed` |
| `Draw` | `Build` |
| `Execute` | `Plan.Run` |
| `LoadData` / `LoadDataByName` | `State.LoadData` / `State.LoadDataByName`，或 `Result.Get` / `Result.GetNamed` |
| `Stop` | 取消运行 context |
| `GenName[T]()` | `TypeKey[T]()` |
| `Clear` / `DeleteProvider` | 新建 Builder / Plan |

## 注意

- 需要 Go 1.27+（方法级类型参数）。
- 声明的依赖必须由会产出值的服务提供，或由 `Init` 提供；否则 `Build` 直接报错。
- 执行错误会以 `dag: service <key>: ...` 包装，可用 `errors.Is/As` 判断底层错误。
- 层内任一服务返回错误会取消同层其他服务；层与层之间串行。
- `panic` 会被转换为带堆栈的 error。
