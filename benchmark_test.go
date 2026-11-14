package dag

import (
	"context"
	"fmt"
	"testing"
)

type benchA struct {
	V int
}

type benchB struct {
	V int
}

func BenchmarkPlanRunTyped(b *testing.B) {
	builder := NewBuilder()
	builder.Provide0(func(ctx context.Context) (*benchA, error) {
		return &benchA{V: 1}, nil
	})
	builder.Provide1(func(ctx context.Context, a *benchA) (*benchB, error) {
		return &benchB{V: a.V + 1}, nil
	})
	plan, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plan.Run(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlanRunReflect(b *testing.B) {
	builder := NewBuilder()
	builder.Wrap[*benchA](func(ctx context.Context) (*benchA, error) {
		return &benchA{V: 1}, nil
	})
	builder.Wrap[*benchB](func(ctx context.Context, a *benchA) (*benchB, error) {
		return &benchB{V: a.V + 1}, nil
	})
	plan, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plan.Run(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlanRunParallel(b *testing.B) {
	builder := NewBuilder()
	builder.Provide0(func(ctx context.Context) (*benchA, error) {
		return &benchA{V: 1}, nil
	})
	builder.Provide1(func(ctx context.Context, a *benchA) (*benchB, error) {
		return &benchB{V: a.V + 1}, nil
	})
	plan, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := plan.Run(context.Background()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkBuildChain100(b *testing.B) {
	builder := benchChainBuilder(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := builder.Build(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlanRunChain100(b *testing.B) {
	plan, err := benchChainBuilder(100).Build()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plan.Run(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func benchChainBuilder(n int) *Builder {
	builder := NewBuilder()
	prev := ""
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("n%d", i)
		prevName := prev
		value := i
		var deps []Key
		if prev != "" {
			deps = append(deps, NameKey(prev))
		}
		builder.ProvideNamed(name, func(ctx context.Context, s *State) (*benchA, error) {
			if prevName != "" {
				if _, ok := s.LoadDataByName[*benchA](prevName); !ok {
					return nil, fmt.Errorf("missing %s", prevName)
				}
			}
			return &benchA{V: value}, nil
		}, deps...)
		prev = name
	}
	return builder
}
