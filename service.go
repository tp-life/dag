package dag

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// iservice 执行器（包内接口）
type iservice interface {
	execute(ctx context.Context, dag *FxDag) error
	getDependence() []string
	produces() bool
}

// service 用于记录依赖关系
type service struct {
	dependence map[string]struct{} // 依赖的参数
	produce    string              // 产生的结果类型
	pd         bool                // 是否提供依赖
	run        func(context.Context, *FxDag) (any, error)
	reflectFn  reflect.Value
}

// newService 创建一个新的服务
func newService(dependence map[string]struct{}, produce string, pd bool) *service {
	return &service{
		dependence: dependence,
		produce:    produce,
		pd:         pd,
	}
}

// newReflectService 创建一个基于反射的服务
func newReflectService(fn reflect.Value, dependence map[string]struct{}, produce string, pd bool) *service {
	s := newService(dependence, produce, pd)
	s.reflectFn = fn
	return s
}

// bind 使用泛型方法把强类型 handler 擦除为统一执行函数。
// Go 1.27 之前方法不能声明自己的类型参数，这也是 serviceT[T] 存在的原因。
func (s *service) bind[T any](handler FxHandler[T]) *service {
	s.run = func(ctx context.Context, dag *FxDag) (any, error) {
		return handler(ctx, dag)
	}
	return s
}

// execute 执行服务；错误统一带上服务名，便于定位和 errors.Is/As 判断。
func (s *service) execute(ctx context.Context, dag *FxDag) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("dag: service %s: %w", s.produce, err)
		}
	}()

	if s.reflectFn.IsValid() {
		return s.executeReflect(ctx, dag)
	}
	if s.run == nil {
		return errors.New("fn is not supported")
	}

	v, runErr := s.run(ctx, dag)
	if runErr != nil {
		return runErr
	}
	if !s.pd {
		return nil
	}
	dag.setVal(s.produce, v)
	return nil
}

func (s *service) getDependence() []string {
	d := make([]string, 0, len(s.dependence))
	for k := range s.dependence {
		d = append(d, k)
	}
	return d
}

// produces 表示该服务执行后是否会向 dag 写入值
func (s *service) produces() bool {
	if !s.pd {
		return false
	}
	if s.reflectFn.IsValid() {
		return s.reflectFn.Type().NumOut() > 1
	}
	return s.run != nil
}

// executeReflect 用于执行反射函数
func (s *service) executeReflect(ctx context.Context, dag *FxDag) error {
	if ctx == nil {
		return fmt.Errorf("ctx is nil")
	}

	fType := s.reflectFn.Type()
	params := make([]reflect.Value, 0, fType.NumIn())
	params = append(params, reflect.ValueOf(ctx))

	for i := 1; i < fType.NumIn(); i++ {
		pt := fType.In(i)
		v, ok := dag.getVal(pt.String())
		if !ok {
			return fmt.Errorf("依赖项不存在:%s", pt)
		}
		if v == nil {
			params = append(params, reflect.Zero(pt))
			continue
		}
		rv := reflect.ValueOf(v)
		if !rv.Type().AssignableTo(pt) {
			return fmt.Errorf("依赖项类型不匹配: got %s, want %s", rv.Type(), pt)
		}
		params = append(params, rv)
	}

	// 调用函数
	result := s.reflectFn.Call(params)

	// 先检查最后一个返回值（error），与是否提供依赖无关
	if n := fType.NumOut(); n > 0 {
		if err, ok := result[n-1].Interface().(error); ok && !isNil(err) {
			return err
		}
	}

	// 取第一个值作为提供项；只有一个返回参数时它必须是 error
	if s.pd && fType.NumOut() > 1 {
		dag.setVal(s.produce, result[0].Interface())
	}

	return nil
}
