package dag

import (
	"context"
	"fmt"
	"reflect"
)

// IService 执行器
type IService interface {
	execute(ctx context.Context, dag *FxDag) error
	getDependence() []string
}

// service 用于记录依赖关系
type service struct {
	dependence map[string]struct{} // 依赖的参数
	fn         any
	produce    string // 产生的结果类型
	pd         bool   // 是否提供依赖
}

// newService 创建一个新的服务
func newService(fn any, dependence map[string]struct{}, produce string, pd bool) *service {
	return &service{
		dependence: dependence,
		fn:         fn,
		produce:    produce,
		pd:         pd,
	}
}

func (s *service) execute(ctx context.Context, dag *FxDag) error {
	_, ok := s.fn.(reflect.Value)
	if ok {
		return s.executeReflect(ctx, dag)
	}

	_, ok = s.fn.(FxHandler[any])
	if ok {
		return s.executeHandler(ctx, dag)
	}

	return fmt.Errorf("fn is not supported")
}

func (s *service) getDependence() []string {
	d := make([]string, 0, len(s.dependence))
	for k := range s.dependence {
		d = append(d, k)
	}
	return d
}

// executeHandler 用于执行handler
func (s *service) executeHandler(ctx context.Context, dag *FxDag) error {
	_fn, ok := s.fn.(FxHandler[any])
	if !ok {
		return fmt.Errorf("fn is not of type FxHandler")
	}

	v, err := _fn(ctx, dag)
	if err != nil {
		return err
	}
	if !s.pd {
		return nil
	}
	return dag.val.set(s.produce, v)
}

// executeReflect 用于执行反射函数
func (s *service) executeReflect(ctx context.Context, dag *FxDag) error {
	_f, ok := s.fn.(reflect.Value)
	if !ok {
		return fmt.Errorf("fn is not of type reflect.Func")
	}
	fType := _f.Type()
	params := make([]reflect.Value, 0, fType.NumIn())
	params = append(params, reflect.ValueOf(ctx))

	if fType.NumIn() > 1 {

		for i := 1; i < fType.NumIn(); i++ {
			if v, ok := dag.getVal(fType.In(i).String()); ok {
				params = append(params, reflect.ValueOf(v))
			}
		}

	}

	// 调用函数
	result := _f.Call(params)

	// 无返回数据以及标记为不需要提供依赖
	if fType.NumOut() == 0 || !s.pd {
		return nil
	}

	// 检测最后一个参数是否是error
	if fType.NumOut() > 0 {
		callErr, ok := result[fType.NumOut()-1].Interface().(error)
		if !ok && callErr != nil {
			return fmt.Errorf("fn %s last return params must return a error", s.produce)
		}

		if callErr != nil {
			return callErr
		}

	}

	// 取第一个值作为提供项，只有一个返回参数时必须为error类型
	if fType.NumOut() > 1 {
		dag.setVal(s.produce, result[0].Interface())
	}

	return nil
}

// serviceT 用于记录依赖关系
type serviceT[T any] struct {
	dependence map[string]struct{} // 依赖的参数
	fn         FxHandler[T]
	produce    string // 产生的结果类型
	pd         bool   // 是否提供依赖
}

// newServiceT 创建一个新的服务
func newServiceT[T any](fn FxHandler[T], dependence map[string]struct{}, produce string, pd bool) *serviceT[T] {
	s := serviceT[T]{
		dependence: dependence,
		fn:         fn,
		produce:    produce,
		pd:         pd,
	}
	return &s
}

func (s *serviceT[T]) execute(ctx context.Context, dag *FxDag) error {
	v, err := s.fn(ctx, dag)
	if err != nil {
		return err
	}
	if !s.pd {
		return nil
	}
	return dag.val.set(s.produce, v)
}

func (s *serviceT[T]) getDependence() []string {
	d := make([]string, 0, len(s.dependence))
	for k := range s.dependence {
		d = append(d, k)
	}
	return d
}
