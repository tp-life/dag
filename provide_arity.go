package dag

import (
	"context"
	"fmt"
)

func (b *Builder) nilHandler(key Key, fn any) bool {
	if !isNil(fn) {
		return false
	}
	b.mu.Lock()
	b.failf("dag: handler for %s is nil", key)
	b.mu.Unlock()
	return true
}

func (b *Builder) provideArity[T any](key Key, deps []Key, after []Key, handler Handler[T]) *Builder {
	all := make([]Key, 0, len(deps)+len(after))
	all = append(all, deps...)
	all = append(all, after...)
	return b.provide(key, handler, true, all...)
}

func loadDep[T any](s *State) (T, error) {
	v, ok := s.LoadData[T]()
	if !ok {
		var zero T
		return zero, fmt.Errorf("dag: dependency %s not found", TypeKey[T]())
	}
	return v, nil
}

// Provide0 注册无依赖的类型化服务
func (b *Builder) Provide0[T any](fn func(context.Context) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	return b.provideArity(TypeKey[T](), nil, after, func(ctx context.Context, _ *State) (T, error) {
		return fn(ctx)
	})
}

// Provide1 注册 1 个依赖的类型化服务，依赖按参数类型自动注入
func (b *Builder) Provide1[T, A any](fn func(context.Context, A) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	return b.provideArity(TypeKey[T](), []Key{TypeKey[A]()}, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a)
	})
}

// Provide2 注册 2 个依赖的类型化服务
func (b *Builder) Provide2[T, A, B any](fn func(context.Context, A, B) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	return b.provideArity(TypeKey[T](), []Key{TypeKey[A](), TypeKey[B]()}, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb)
	})
}

// Provide3 注册 3 个依赖的类型化服务
func (b *Builder) Provide3[T, A, B, C any](fn func(context.Context, A, B, C) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	deps := []Key{TypeKey[A](), TypeKey[B](), TypeKey[C]()}
	return b.provideArity(TypeKey[T](), deps, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		c, err := loadDep[C](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb, c)
	})
}

// Provide4 注册 4 个依赖的类型化服务
func (b *Builder) Provide4[T, A, B, C, D any](fn func(context.Context, A, B, C, D) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	deps := []Key{TypeKey[A](), TypeKey[B](), TypeKey[C](), TypeKey[D]()}
	return b.provideArity(TypeKey[T](), deps, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		c, err := loadDep[C](s)
		if err != nil {
			var zero T
			return zero, err
		}
		d, err := loadDep[D](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb, c, d)
	})
}

// Provide5 注册 5 个依赖的类型化服务
func (b *Builder) Provide5[T, A, B, C, D, E any](fn func(context.Context, A, B, C, D, E) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	deps := []Key{TypeKey[A](), TypeKey[B](), TypeKey[C](), TypeKey[D](), TypeKey[E]()}
	return b.provideArity(TypeKey[T](), deps, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		c, err := loadDep[C](s)
		if err != nil {
			var zero T
			return zero, err
		}
		d, err := loadDep[D](s)
		if err != nil {
			var zero T
			return zero, err
		}
		e, err := loadDep[E](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb, c, d, e)
	})
}

// Provide6 注册 6 个依赖的类型化服务
func (b *Builder) Provide6[T, A, B, C, D, E, F any](fn func(context.Context, A, B, C, D, E, F) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	deps := []Key{TypeKey[A](), TypeKey[B](), TypeKey[C](), TypeKey[D](), TypeKey[E](), TypeKey[F]()}
	return b.provideArity(TypeKey[T](), deps, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		c, err := loadDep[C](s)
		if err != nil {
			var zero T
			return zero, err
		}
		d, err := loadDep[D](s)
		if err != nil {
			var zero T
			return zero, err
		}
		e, err := loadDep[E](s)
		if err != nil {
			var zero T
			return zero, err
		}
		f, err := loadDep[F](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb, c, d, e, f)
	})
}

// Provide7 注册 7 个依赖的类型化服务
func (b *Builder) Provide7[T, A, B, C, D, E, F, G any](fn func(context.Context, A, B, C, D, E, F, G) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	deps := []Key{TypeKey[A](), TypeKey[B](), TypeKey[C](), TypeKey[D](), TypeKey[E](), TypeKey[F](), TypeKey[G]()}
	return b.provideArity(TypeKey[T](), deps, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		c, err := loadDep[C](s)
		if err != nil {
			var zero T
			return zero, err
		}
		d, err := loadDep[D](s)
		if err != nil {
			var zero T
			return zero, err
		}
		e, err := loadDep[E](s)
		if err != nil {
			var zero T
			return zero, err
		}
		f, err := loadDep[F](s)
		if err != nil {
			var zero T
			return zero, err
		}
		g, err := loadDep[G](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb, c, d, e, f, g)
	})
}

// Provide8 注册 8 个依赖的类型化服务
func (b *Builder) Provide8[T, A, B, C, D, E, F, G, H any](fn func(context.Context, A, B, C, D, E, F, G, H) (T, error), after ...Key) *Builder {
	if b.nilHandler(TypeKey[T](), fn) {
		return b
	}
	deps := []Key{TypeKey[A](), TypeKey[B](), TypeKey[C](), TypeKey[D](), TypeKey[E](), TypeKey[F](), TypeKey[G](), TypeKey[H]()}
	return b.provideArity(TypeKey[T](), deps, after, func(ctx context.Context, s *State) (T, error) {
		a, err := loadDep[A](s)
		if err != nil {
			var zero T
			return zero, err
		}
		bb, err := loadDep[B](s)
		if err != nil {
			var zero T
			return zero, err
		}
		c, err := loadDep[C](s)
		if err != nil {
			var zero T
			return zero, err
		}
		d, err := loadDep[D](s)
		if err != nil {
			var zero T
			return zero, err
		}
		e, err := loadDep[E](s)
		if err != nil {
			var zero T
			return zero, err
		}
		f, err := loadDep[F](s)
		if err != nil {
			var zero T
			return zero, err
		}
		g, err := loadDep[G](s)
		if err != nil {
			var zero T
			return zero, err
		}
		h, err := loadDep[H](s)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, a, bb, c, d, e, f, g, h)
	})
}
