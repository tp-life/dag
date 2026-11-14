package dag

import (
	"context"
	"errors"
	"fmt"
)

type exampleA struct {
	Val string
}

type exampleB struct {
	Val string
}

// 基础用法：类型 key + after 声明执行顺序
func ExampleBuilder() {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*exampleA, error) {
		return &exampleA{Val: "A"}, nil
	})
	b.Provide(func(ctx context.Context, s *State) (*exampleB, error) {
		a, _ := s.LoadData[*exampleA]()
		return &exampleB{Val: a.Val}, nil
	}, TypeKey[*exampleA]())

	plan, err := b.Build()
	if err != nil {
		fmt.Println("build:", err)
		return
	}

	result, err := plan.Run(context.Background())
	if err != nil {
		fmt.Println("run:", err)
		return
	}

	bv, _ := result.Get[*exampleB]()
	fmt.Println(bv.Val)

	// Output: A
}

// 命名 key：类型 key 与命名 key 属于不同命名空间
func ExampleBuilder_named() {
	b := NewBuilder()
	b.ProvideNamed("a", func(ctx context.Context, _ *State) (*exampleA, error) {
		return &exampleA{Val: "A"}, nil
	})
	b.ProvideNamed("b", func(ctx context.Context, s *State) (*exampleB, error) {
		a, ok := s.LoadDataByName[*exampleA]("a")
		if !ok {
			return nil, errors.New("a not found")
		}
		return &exampleB{Val: a.Val + "B"}, nil
	}, NameKey("a"))

	plan, err := b.Build()
	if err != nil {
		fmt.Println("build:", err)
		return
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		fmt.Println("run:", err)
		return
	}

	bv, _ := result.GetNamed[*exampleB]("b")
	fmt.Println(bv.Val)

	// Output: AB
}

// 任意签名函数：按参数类型自动装配依赖
func ExampleBuilder_wrap() {
	b := NewBuilder()
	b.Wrap[*exampleA](func(ctx context.Context) (*exampleA, error) {
		return &exampleA{Val: "A"}, nil
	})
	b.Wrap[*exampleB](func(ctx context.Context, a *exampleA) (*exampleB, error) {
		return &exampleB{Val: a.Val + "B"}, nil
	})

	plan, err := b.Build()
	if err != nil {
		fmt.Println("build:", err)
		return
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		fmt.Println("run:", err)
		return
	}

	bv, _ := result.Get[*exampleB]()
	fmt.Println(bv.Val)

	// Output: AB
}

// 只读 State + after 显式声明顺序
func ExampleBuilder_state() {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*exampleA, error) {
		return &exampleA{Val: "A"}, nil
	})
	b.Wrap[*exampleB](func(ctx context.Context, s *State) (*exampleB, error) {
		a, ok := s.LoadData[*exampleA]()
		if !ok {
			return nil, errors.New("a not found")
		}
		return &exampleB{Val: a.Val + "B"}, nil
	}, TypeKey[*exampleA]())

	plan, err := b.Build()
	if err != nil {
		fmt.Println("build:", err)
		return
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		fmt.Println("run:", err)
		return
	}

	bv, _ := result.Get[*exampleB]()
	fmt.Println(bv.Val)

	// Output: AB
}

// 初始值：Init 提供的值可以直接作为依赖，无需注册服务
func ExampleBuilder_init() {
	b := NewBuilder()
	b.Init(&exampleA{Val: "init"})
	b.Provide(func(ctx context.Context, s *State) (*exampleB, error) {
		a, ok := s.LoadData[*exampleA]()
		if !ok {
			return nil, errors.New("a not found")
		}
		return &exampleB{Val: a.Val + "B"}, nil
	}, TypeKey[*exampleA]())

	plan, err := b.Build()
	if err != nil {
		fmt.Println("build:", err)
		return
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		fmt.Println("run:", err)
		return
	}

	bv, _ := result.Get[*exampleB]()
	fmt.Println(bv.Val)

	// Output: initB
}

// 出错时仍可读取已完成节点的结果
func ExampleBuilder_partialResultOnError() {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*exampleA, error) {
		return &exampleA{Val: "A"}, nil
	})
	b.Provide(func(ctx context.Context, _ *State) (*exampleB, error) {
		return nil, errors.New("boom")
	}, TypeKey[*exampleA]())

	plan, err := b.Build()
	if err != nil {
		fmt.Println("build:", err)
		return
	}

	result, err := plan.Run(context.Background())
	a, ok := result.Get[*exampleA]()
	fmt.Println(a.Val, ok, err != nil)

	// Output: A true true
}

// Key 的类型命名空间与命名命名空间
func ExampleKey() {
	fmt.Println(TypeKey[*exampleA]().String())
	fmt.Println(NameKey("a").String())

	// Output:
	// *dag.exampleA
	// a
}
