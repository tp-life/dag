package dag

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Handler 是类型化的依赖处理函数。state 为只读运行状态。
type Handler[T any] func(ctx context.Context, state *State) (T, error)

// paramSpec 是反射调用时参数与依赖的预计算结果
type paramSpec struct {
	key     Key
	typ     reflect.Type
	isState bool
}

// service 是一个不可变的执行单元
type service struct {
	key       Key
	deps      []Key
	produces  bool
	run       func(context.Context, *State) (any, error)
	reflectFn reflect.Value
	params    []paramSpec
}

func newService(key Key, deps []Key, produces bool) *service {
	return &service{key: key, deps: deps, produces: produces}
}

func newReflectService(key Key, deps []Key, produces bool, fn reflect.Value, params []paramSpec) *service {
	return &service{
		key:       key,
		deps:      deps,
		produces:  produces,
		reflectFn: fn,
		params:    params,
	}
}

// bind 使用泛型方法把强类型 handler 擦除为统一执行函数
func (s *service) bind[T any](handler Handler[T]) *service {
	s.run = func(ctx context.Context, state *State) (any, error) {
		return handler(ctx, state)
	}
	return s
}

// execute 执行服务。错误由运行调度器包装为 *Error，附带 key 与 layer。
func (s *service) execute(ctx context.Context, state *State) error {
	if s.reflectFn.IsValid() {
		return s.executeReflect(ctx, state)
	}
	if s.run == nil {
		return errors.New("fn is not supported")
	}

	v, err := s.run(ctx, state)
	if err != nil {
		return err
	}
	if !s.produces {
		return nil
	}
	state.set(s.key, v)
	return nil
}

func (s *service) executeReflect(ctx context.Context, state *State) error {
	if ctx == nil {
		return fmt.Errorf("ctx is nil")
	}

	params := make([]reflect.Value, 0, len(s.params)+1)
	params = append(params, reflect.ValueOf(ctx))
	for _, p := range s.params {
		if p.isState {
			params = append(params, reflect.ValueOf(state))
			continue
		}
		v, ok := state.store.get(p.key)
		if !ok {
			return fmt.Errorf("依赖项不存在:%s", p.key)
		}
		if v == nil {
			params = append(params, reflect.Zero(p.typ))
			continue
		}
		rv := reflect.ValueOf(v)
		if !rv.Type().AssignableTo(p.typ) {
			return fmt.Errorf("依赖项类型不匹配: got %s, want %s", rv.Type(), p.typ)
		}
		params = append(params, rv)
	}

	result := s.reflectFn.Call(params)

	// 先检查最后一个返回值（error），与是否提供依赖无关
	if n := len(result); n > 0 {
		if err, ok := result[n-1].Interface().(error); ok && !isNil(err) {
			return err
		}
	}

	// 取第一个值作为产物
	if s.produces && len(result) > 1 {
		state.set(s.key, result[0].Interface())
	}
	return nil
}
