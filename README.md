# dag

有向无环图（DAG）依赖执行库，基于 Go 1.27 泛型方法实现。

## 示例

```go
d := dag.New()

// 设置初始值（按静态类型 T 命名）
_ = d.InitParams(&Config{})

// 注册强类型 handler；T 由 handler 推断
_ = d.Provide(func(ctx context.Context, _ *dag.FxDag) (*A, error) {
    return &A{}, nil
})

// 指定名称，并声明依赖名
_ = d.ProvideByName("B", func(ctx context.Context, d *dag.FxDag) (*B, error) {
    a, _ := d.LoadData[*A]()
    return &B{A: a}, nil
}, dag.GenName[*A]())

// 绘制并执行
_ = d.Draw()
_ = d.Execute(ctx)

b, ok := d.LoadDataByName[*B]("B")
```

## 任意签名函数（反射自动装配）

```go
// 按参数类型自动装配依赖；T 用于结果命名与返回值类型校验
_ = d.WrapHandler[*C](func(ctx context.Context, b *B) (*C, error) {
    return &C{B: b}, nil
})

// 不提供结果
_ = d.WrapHandlerNoName(func(ctx context.Context, b *B) {
    // ...
})
```

## 注意

- 需要 Go 1.27+（方法级类型参数）。
- `Provide` / `Draw` / `Execute` / `Clear` 之间不保证并发安全；`Stop` 可并发调用。
- `Clear` 清空注册、队列与执行结果，保留 `InitParams` 设置的默认值。
- `ProvideByNameWithOut` 注册的服务不会写入值，不能作为其他服务的依赖（除非同名 `initVal` 存在）。
- `Draw` 会校验依赖的生产者是否真正产出值；例如依赖一个 `WithOut` 服务会直接报错。
- 执行错误会以 `dag: service <name>: ...` 包装，可用 `errors.Is/As` 判断底层错误。
- `Stop` 会取消本次执行的 context；handler 应监听 `ctx.Done()` 以尽快返回。
- 层内任一服务返回错误会取消同层其他服务；层与层之间串行。
