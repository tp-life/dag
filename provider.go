package dag

import (
	"fmt"
	"reflect"
)

// getFxConcurrenceOrDefault 获取一个服务， 当服务不存在时返回默认的服务
func getFxConcurrenceOrDefault(i *FxDag) *FxDag {
	if i != nil {
		return i
	}
	return DefaultDag
}

// LoadData 从dag里获取数据
func LoadData[T any](f *FxDag) (T, bool) {
	name := GenName[T]()
	return LoadDataByName[T](f, name)
}

// LoadDataByName 从dag里获指定name的数据
func LoadDataByName[T any](f *FxDag, name string) (T, bool) {
	a, b := f.Load(name)
	if !b {
		v := new(T)
		return *v, false
	}
	v, ok := a.(T)
	return v, ok
}

// WrapperDagHandler 是一个用于处理有向无环图中依赖关系的函数。
// 它将给定函数 fn 返回的服务注册到提供的 FxDag f 中。
// 如果调用函数时出现任何问题，或者依赖关系已经存在于 FxDag 中，则该函数返回错误。
func WrapperDagHandler[T any](f *FxDag, fn any) error {
	// 为给定类型 T 生成一个唯一的依赖关系名称。
	name := generateDependenceName[T]()
	return wrapperDagHandlerByName(f, name, fn, true)
}

// WrapperDagHandlerNoName 不提供任何被依赖数项
// 它将给定函数 fn 返回的服务注册到提供的 FxDag f 中。
// 如果调用函数时出现任何问题，或者依赖关系已经存在于 FxDag 中，则该函数返回错误。
func WrapperDagHandlerNoName(f *FxDag, fn any) error {
	name := fmt.Sprintf("%p", fn)
	return wrapperDagHandlerByName(f, name, fn, false)
}

// wrapperDagHandlerByName 是一个用于处理有向无环图中依赖关系的函数。
// 它将给定函数 fn 返回的服务注册到提供的 FxDag f 中。
// 如果调用函数时出现任何问题，或者依赖关系已经存在于 FxDag 中，则该函数返回错误。
func wrapperDagHandlerByName(f *FxDag, name string, fn any, pd bool) error {

	// 使用生成的名称调用函数，并获取服务和任何错误。
	services, err := callFuncParams(fn, name, pd)
	if err != nil {
		return err
	}

	// 获取 FxDag，如果不存在，则使用默认的 FxDag。
	_f := getFxConcurrenceOrDefault(f)

	// 检查 FxDag 中是否已经存在该依赖关系。
	if _f.exists(name) {
		return fmt.Errorf("dependence %s already exists", name)
	}

	// 在 FxDag 中设置该依赖关系的服务。
	_f.set(name, services)

	return nil
}

// Provide 注册依赖关系
func Provide[T any](f *FxDag, handler FxHandler[T], depName ...string) (err error) {
	name := generateDependenceName[T]()
	return ProvideByName[T](f, name, handler, depName...)
}

// ProvideByName 使用给定的name注册依赖关系
// 它接受一个FxDag指针、一个字符串name、一个FxHandler函数和一个可变参数depName（字符串类型）。返回一个错误。
// 该函数检查FxDag中是否已经存在具有给定名称的依赖项。如果存在，则返回一个错误。否则，它使用提供的handler函数创建一个新的服务，并将其设置在具有给定名称和依赖项的FxDag中。
func ProvideByName[T any](f *FxDag, name string, handler FxHandler[T], depName ...string) (err error) {
	_f := getFxConcurrenceOrDefault(f)
	if _f.exists(name) {
		return fmt.Errorf("dependence %s already exists", name)
	}
	dep := make(map[string]struct{})
	for _, v := range depName {
		dep[v] = struct{}{}
	}
	_f.set(name, newServiceT[T](handler, dep, name, true))
	return nil
}

// GenName 获取指定类型的字符串类型
func GenName[T any]() string {
	return generateDependenceName[T]()
}

// generateDependenceName 获取指定类型的字符串值
func generateDependenceName[T any]() string {
	var t T
	// struct
	name := fmt.Sprintf("%T", t)
	if name != "<nil>" {
		return name
	}
	// interface
	return fmt.Sprintf("%T", new(T))
}

// generateName 获取指定类型的字符串值
func generateName(v any) (string, error) {
	name := fmt.Sprintf("%T", v)
	if name != "<nil>" {
		return name, nil
	}
	return "", fmt.Errorf("value is nil")
}

// callFuncParams 获取service。
// 它接受两个参数：fn（任意类型）和produce（字符串类型）。函数首先使用反射检查fn的类型是否为函数类型。如果不是，则返回一个错误。然后，它创建了一个名为dep的空映射。
// 该函数检查fn是否至少有一个参数。如果没有，将返回一个错误，指示fn必须具有context.Context类型的参数。
// 如果fn至少有一个参数，则函数检查第一个参数是否为context.Context类型。如果不是，则返回一个错误。
// 然后，函数遍历fn的剩余参数，并将它们的字符串表示添加到dep映射中。
// 最后，函数返回调用newService函数的结果，其中将fn参数作为reflect.Value传递，将dep映射和produce参数作为参数传递，同时返回nil错误。
func callFuncParams(fn any, produce string, pd bool) (*service, error) {
	_fn := reflect.TypeOf(fn)
	if _fn.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn is not of type reflect.Func")
	}
	dep := make(map[string]struct{})
	if _fn.NumIn() == 0 {
		return nil, fmt.Errorf("fn has no params. fn must has context.Context params")
	}
	if _fn.In(0).String() != "context.Context" {
		return nil, fmt.Errorf(" fn must has context.Context params")
	}
	if _fn.NumOut() > 2 {
		return nil, fmt.Errorf("fn must not have more than 2 outputs")
	}
	// 如果有参数，则最后一个参数需要为error 类型
	if _fn.NumOut() > 0 && !_fn.Out(_fn.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, fmt.Errorf("fn %s last return params must return a error", produce)
	}

	for i := 1; i < _fn.NumIn(); i++ {
		dep[_fn.In(i).String()] = struct{}{}
	}
	return newService(reflect.ValueOf(fn), dep, produce, pd), nil
}
