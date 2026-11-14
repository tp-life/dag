# dag

有向无环图（DAG）依赖执行库，基于 Go 1.27 泛型方法实现。

```go
import dag "github.com/tp-life/dag/v2"
```

## 架构

- `Builder`：注册期，收集服务与初始值，并发安全。
- `Plan`：不可变执行计划，可并发多次运行。
- `State`：handler 可见的只读运行状态。
- `Result`：单次运行的结果、错误、跳过项与报告。
- 调度器：就绪队列。依赖完成即启动后继，不等待同层其他节点；支持最大并发与失败策略。

## 快速开始

```go
b := dag.NewBuilder()
b.Init(&Config{})

b.Provide0(func(ctx context.Context) (*A, error) {
    return &A{}, nil
})
b.Provide1(func(ctx context.Context, a *A) (*B, error) {
    return &B{A: a}, nil
})
b.Provide2(func(ctx context.Context, a *A, b *B) (*C, error) {
    return &C{A: a, B: b}, nil
})

plan, err := b.Build()
if err != nil {
    return err
}

result, err := plan.Run(ctx)
c, ok := result.Get[*C]()
```

`Provide0` ~ `Provide8` 会自动把参数类型作为依赖注入，不需要手写 `LoadData`，也没有反射调用。

## 显式依赖与命名 key

```go
b.Provide(func(ctx context.Context, s *dag.State) (*B, error) {
    a, _ := s.LoadData[*A]()
    return &B{A: a}, nil
}, dag.TypeKey[*A]()) // after：只保证执行顺序

b.ProvideNamed("a", func(ctx context.Context, s *dag.State) (*A, error) {
    return &A{}, nil
})
b.ProvideNamed("b", func(ctx context.Context, s *dag.State) (*B, error) {
    a, _ := s.LoadDataByName[*A]("a")
    return &B{A: a}, nil
}, dag.NameKey("a"))

result.GetNamed[*B]("b")
```

类型 key 与命名 key 是两个独立命名空间。

## 任意签名函数（反射自动装配）

```go
b.Wrap[*C](func(ctx context.Context, b *B) (*C, error) {
    return &C{B: b}, nil
})

b.WrapNoName(func(ctx context.Context) {
    // 副作用
})
```

`*State` 参数会注入只读运行状态，不会被当作依赖；使用 `*State` 动态读取时需通过 `after` 声明顺序：

```go
b.Wrap[*C](func(ctx context.Context, s *dag.State) (*C, error) {
    b, _ := s.LoadData[*B]()
    return &C{B: b}, nil
}, dag.TypeKey[*B]())
```

## 运行选项

```go
result, err := plan.Run(ctx,
    dag.ContinueOnError(),                  // 默认 FailFast
    dag.WithMaxConcurrency(8),              // 默认不限制
    dag.WithServiceTimeout(5*time.Second),  // 单服务超时
    dag.WithHook(hook),                     // 观测钩子
)
```

- `FailFast`：首个错误后取消运行，未开始节点标记为 skipped。
- `ContinueOnError`：失败节点的后继标记为 skipped，独立分支继续执行。
- `WithMaxConcurrency`：限制同时执行的服务数。
- `WithServiceTimeout`：为每个服务派生超时 context。

## Hook 与 RunReport

```go
type Hook interface {
    OnServiceStart(ctx context.Context, key dag.Key, layer int)
    OnServiceEnd(ctx context.Context, key dag.Key, layer int, d time.Duration, err error)
}
```

```go
report := result.Report()
for _, svc := range report.Services {
    fmt.Println(svc.Key, svc.Layer, svc.Status, svc.Duration)
}
```

报告按 layer/key 排序；状态包括 `succeeded`、`failed`、`skipped`。

## 错误处理

```go
var runErrs dag.RunErrors
if errors.As(err, &runErrs) {
    for _, e := range runErrs {
        fmt.Println(e.Key, e.Layer, e.Err)
    }
}

var serviceErr *dag.Error
if errors.As(err, &serviceErr) {
    fmt.Println(serviceErr.Key, serviceErr.Layer)
}

result.Errors()   // 本次运行的服务错误
result.Skipped()  // 被跳过的服务 key
```

`dag.Error` 与 `dag.RunErrors` 都支持 `errors.Is/As`；panic 会转换为带堆栈的 error。

## 取消

```go
ctx, cancel := context.WithCancel(context.Background())
go func() {
    // ...
    cancel()
}()
result, err := plan.Run(ctx)
```

## 基准测试

```bash
go test -run '^$' -bench . -benchmem ./...
```

包含 typed、reflection、并行运行、100 节点链式构建与执行的 benchmark。

## 查看依赖图

`Plan.Graph()` 返回结构化快照，`Plan.DOT()` 导出 Graphviz：

```go
for _, node := range plan.Graph().Nodes {
    fmt.Println(node.Layer, node.Key, node.Deps)
}

dot := plan.DOT()
os.WriteFile("dag.dot", []byte(dot), 0o644)
```

```bash
dot -Tsvg dag.dot -o dag.svg
```

节点按 layer/key 排序，同一层使用 `rank=same` 对齐；输出是确定性的，可直接用于 diff。

## 从 v2 迁移

- 模块路径改为 `github.com/tp-life/dag/v2`。
- `Plan.Run` 新增可变参数 `RunOption`，原有 `plan.Run(ctx)` 仍可用。
- 错误现在是 `*dag.Error` / `dag.RunErrors`，`errors.Is/As` 行为保持不变。
- 可以直接使用 `Provide0` ~ `Provide8` 替代手写 `State.LoadData` 的 `Provide` 写法。

## 注意

- 需要 Go 1.27+（方法级类型参数）。
- 声明的依赖必须由会产出值的服务或 `Init` 提供，否则 `Build` 直接报错。
- `Plan` 可并发 `Run`，但要求 handler 本身可并发执行；`Hook` 也必须并发安全。
- `ProvideWithoutValue` 注册的服务不会写入值，不能作为依赖（除非 `Init` 提供了同名值）。
