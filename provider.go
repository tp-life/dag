package dag

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
)

var (
	errorType       = reflect.TypeFor[error]()
	contextType     = reflect.TypeFor[context.Context]()
	nonameServiceID atomic.Uint64
)

// LoadData 从dag里获取数据
func (f *FxDag) LoadData[T any]() (T, bool) {
	return f.LoadDataByName[T](GenName[T]())
}

// LoadDataByName 从dag里获指定name的数据
func (f *FxDag) LoadDataByName[T any](name string) (T, bool) {
	a, ok := f.orDefault().Load(name)
	if !ok {
		var zero T
		return zero, false
	}
	v, ok := a.(T)
	return v, ok
}

// WrapHandler 注册一个任意签名的函数，并按参数类型自动装配依赖。
// T 用于生成结果名称，并校验函数首个返回值的类型。
// NOTE:: fn 必须是一个函数，第一个参数必须是 context.Context。
func (f *FxDag) WrapHandler[T any](fn any) error {
	return f.wrapHandlerByName(GenName[T](), fn, true, reflect.TypeFor[T]())
}

// WrapHandlerNoName 注册一个不提供结果的任意签名函数
// NOTE:: fn 必须是一个函数，第一个参数必须是 context.Context。
func (f *FxDag) WrapHandlerNoName(fn any) error {
	name := fmt.Sprintf("dag:noname:%d", nonameServiceID.Add(1))
	return f.wrapHandlerByName(name, fn, false, nil)
}

func (f *FxDag) wrapHandlerByName(name string, fn any, pd bool, want reflect.Type) error {
	services, err := callFuncParams(fn, name, pd, want)
	if err != nil {
		return err
	}
	return f.orDefault().setIfAbsent(name, services)
}

// Provide 注册依赖关系
func (f *FxDag) Provide[T any](handler FxHandler[T], depName ...string) error {
	return f.ProvideByName(GenName[T](), handler, depName...)
}

// ProvideByName 使用给定的name注册依赖关系
// 它接受一个字符串name、一个FxHandler函数和一个可变参数depName（字符串类型）。返回一个错误。
// 如果依赖名已存在则返回错误；否则创建一个带依赖的服务并注册。
func (f *FxDag) ProvideByName[T any](name string, handler FxHandler[T], depName ...string) error {
	return f.provide(name, handler, true, depName...)
}

// ProvideByNameWithOut 使用给定的name注册依赖关系，但不把结果写入dag。
// 该服务不能作为其他服务的依赖（除非 initVal 提供了同名值）。
// 如果依赖名已存在则返回错误；否则创建一个带依赖的服务并注册。
func (f *FxDag) ProvideByNameWithOut[T any](name string, handler FxHandler[T], depName ...string) error {
	return f.provide(name, handler, false, depName...)
}

func (f *FxDag) provide[T any](name string, handler FxHandler[T], pd bool, depName ...string) error {
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}
	dep := make(map[string]struct{}, len(depName))
	for _, v := range depName {
		dep[v] = struct{}{}
	}
	return f.orDefault().setIfAbsent(name, newService(dep, name, pd).bind(handler))
}

// GenName 获取指定类型的字符串类型
func GenName[T any]() string {
	return reflect.TypeFor[T]().String()
}

// callFuncParams 校验函数签名并创建service。
// fn 必须以 context.Context 作为第一个参数，最多两个返回值；
// 若有返回值，最后一个返回值必须实现 error；want 非nil时校验首个返回值类型。
func callFuncParams(fn any, produce string, pd bool, want reflect.Type) (*service, error) {
	_fn := reflect.TypeOf(fn)
	if _fn == nil || _fn.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn is not of type reflect.Func")
	}
	rv := reflect.ValueOf(fn)
	if rv.IsNil() {
		return nil, fmt.Errorf("fn is nil")
	}

	if _fn.NumIn() == 0 {
		return nil, fmt.Errorf("fn has no params. fn must has context.Context params")
	}
	if _fn.In(0) != contextType {
		return nil, fmt.Errorf("fn must has context.Context params")
	}
	if _fn.NumOut() > 2 {
		return nil, fmt.Errorf("fn must not have more than 2 outputs")
	}

	// 如果有返回值，则最后一个返回值需要实现 error 类型
	if _fn.NumOut() > 0 && !_fn.Out(_fn.NumOut()-1).Implements(errorType) {
		return nil, fmt.Errorf("fn %s last return params must return a error", _fn.Name())
	}

	// 泛型方法注册时校验函数返回类型与 T 一致，避免值以错误的名字写入
	if want != nil && _fn.NumOut() > 1 && !_fn.Out(0).AssignableTo(want) {
		return nil, fmt.Errorf("fn returns %s, want %s", _fn.Out(0), want)
	}

	dep := make(map[string]struct{}, _fn.NumIn()-1)
	for i := 1; i < _fn.NumIn(); i++ {
		dep[_fn.In(i).String()] = struct{}{}
	}
	return newReflectService(rv, dep, produce, pd), nil
}
