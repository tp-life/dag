package dag

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

var (
	errorType    = reflect.TypeFor[error]()
	contextType  = reflect.TypeFor[context.Context]()
	statePtrType = reflect.TypeFor[*State]()
)

// Builder 收集服务定义并生成不可变 Plan。
// Builder 并发安全；Build 之后可以继续添加定义，已生成的 Plan 不受影响。
type Builder struct {
	mu     sync.Mutex
	defs   []*service
	byKey  map[Key]*service
	init   map[Key]any
	err    error
	noname uint64
}

// NewBuilder 创建一个 Builder
func NewBuilder() *Builder {
	return &Builder{
		byKey: make(map[Key]*service),
		init:  make(map[Key]any),
	}
}

func (b *Builder) ensure() {
	if b.byKey == nil {
		b.byKey = make(map[Key]*service)
	}
	if b.init == nil {
		b.init = make(map[Key]any)
	}
}

func (b *Builder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

func (b *Builder) failf(format string, args ...any) {
	if b.err == nil {
		b.err = fmt.Errorf(format, args...)
	}
}

// Provide 注册类型为 T 的服务，key 为 TypeKey[T]。
// after 仅用于保证执行顺序；依赖必须由会产出值的服务或 Init 提供。
func (b *Builder) Provide[T any](handler Handler[T], after ...Key) *Builder {
	return b.provide(TypeKey[T](), handler, true, after...)
}

// ProvideNamed 注册命名服务
func (b *Builder) ProvideNamed[T any](name string, handler Handler[T], after ...Key) *Builder {
	return b.provide(NameKey(name), handler, true, after...)
}

// ProvideWithoutValue 注册命名服务，但不产出值。
// 该服务不能作为依赖（除非 Init 提供了同名值）。
func (b *Builder) ProvideWithoutValue[T any](name string, handler Handler[T], after ...Key) *Builder {
	return b.provide(NameKey(name), handler, false, after...)
}

func (b *Builder) provide[T any](key Key, handler Handler[T], produces bool, after ...Key) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensure()

	if b.err != nil {
		return b
	}
	if key.IsZero() {
		b.failf("dag: empty key")
		return b
	}
	if handler == nil {
		b.failf("dag: handler for %s is nil", key)
		return b
	}
	if _, ok := b.byKey[key]; ok {
		b.failf("dag: duplicate key %s", key)
		return b
	}
	deps, err := normalizeKeys(after)
	if err != nil {
		b.fail(err)
		return b
	}
	svc := newService(key, deps, produces).bind(handler)
	b.defs = append(b.defs, svc)
	b.byKey[key] = svc
	return b
}

// Init 注册类型 T 的初始值
func (b *Builder) Init[T any](v T) *Builder {
	return b.initValue(TypeKey[T](), v)
}

// InitNamed 注册指定名称的初始值
func (b *Builder) InitNamed(name string, v any) *Builder {
	return b.initValue(NameKey(name), v)
}

func (b *Builder) initValue(key Key, v any) *Builder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensure()

	if b.err != nil {
		return b
	}
	if key.IsZero() {
		b.failf("dag: empty key")
		return b
	}
	if v == nil {
		b.failf("dag: init value for %s is nil", key)
		return b
	}
	if _, ok := b.init[key]; ok {
		b.failf("dag: duplicate init key %s", key)
		return b
	}
	b.init[key] = v
	return b
}

// Wrap 注册任意签名的函数，并按参数类型自动装配依赖。
// fn 必须返回 (T, error)，第一个参数必须是 context.Context。
// after 用于补充显式依赖（例如 handler 使用 *State 动态读取时）。
func (b *Builder) Wrap[T any](fn any, after ...Key) *Builder {
	return b.wrap(TypeKey[T](), fn, true, reflect.TypeFor[T](), after...)
}

// WrapNamed 注册任意签名的函数，产物使用指定名称
func (b *Builder) WrapNamed[T any](name string, fn any, after ...Key) *Builder {
	return b.wrap(NameKey(name), fn, true, reflect.TypeFor[T](), after...)
}

// WrapNoName 注册不产出值的任意签名函数
func (b *Builder) WrapNoName(fn any, after ...Key) *Builder {
	b.mu.Lock()
	b.noname++
	name := fmt.Sprintf("dag:noname:%d", b.noname)
	b.mu.Unlock()

	return b.wrap(NameKey(name), fn, false, nil, after...)
}

func (b *Builder) wrap(key Key, fn any, produces bool, want reflect.Type, after ...Key) *Builder {
	extra, err := normalizeKeys(after)
	if err != nil {
		b.mu.Lock()
		b.fail(err)
		b.mu.Unlock()
		return b
	}

	svc, err := buildReflectService(key, fn, produces, want)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensure()

	if b.err != nil {
		return b
	}
	if err != nil {
		b.fail(err)
		return b
	}
	if key.IsZero() {
		b.failf("dag: empty key")
		return b
	}
	if _, ok := b.byKey[key]; ok {
		b.failf("dag: duplicate key %s", key)
		return b
	}
	for _, d := range extra {
		svc.deps = appendUniqueKey(svc.deps, d)
	}
	b.defs = append(b.defs, svc)
	b.byKey[key] = svc
	return b
}

// Build 校验依赖图并生成不可变 Plan
func (b *Builder) Build() (*Plan, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensure()

	if b.err != nil {
		return nil, b.err
	}

	byKey := make(map[Key]*service, len(b.byKey))
	for k, v := range b.byKey {
		byKey[k] = v
	}
	defs := make([]*service, len(b.defs))
	copy(defs, b.defs)

	for _, svc := range defs {
		for _, d := range svc.deps {
			dep, ok := byKey[d]
			if !ok {
				if _, ok := b.init[d]; ok {
					continue
				}
				return nil, fmt.Errorf("dag: dependency %s of %s not found", d, svc.key)
			}
			if !dep.produces {
				if _, ok := b.init[d]; ok {
					continue
				}
				return nil, fmt.Errorf("dag: dependency %s of %s is not produced", d, svc.key)
			}
		}
	}

	nodes, layerCount, err := buildNodes(defs)
	if err != nil {
		return nil, err
	}

	initCopy := make(map[Key]any, len(b.init))
	for k, v := range b.init {
		initCopy[k] = v
	}
	return &Plan{nodes: nodes, init: initCopy, layerCount: layerCount}, nil
}

func normalizeKeys(keys []Key) ([]Key, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	seen := make(map[Key]struct{}, len(keys))
	out := make([]Key, 0, len(keys))
	for _, k := range keys {
		if k.IsZero() {
			return nil, fmt.Errorf("dag: empty dependency key")
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out, nil
}

func appendUniqueKey(dst []Key, k Key) []Key {
	for _, existing := range dst {
		if existing == k {
			return dst
		}
	}
	return append(dst, k)
}

// buildReflectService 解析任意函数签名并预计算参数/依赖
func buildReflectService(key Key, fn any, produces bool, want reflect.Type) (*service, error) {
	ft := reflect.TypeOf(fn)
	if ft == nil || ft.Kind() != reflect.Func {
		return nil, fmt.Errorf("dag: fn is not a function")
	}
	rv := reflect.ValueOf(fn)
	if rv.IsNil() {
		return nil, fmt.Errorf("dag: fn is nil")
	}
	if ft.NumIn() == 0 {
		return nil, fmt.Errorf("dag: fn must accept context.Context")
	}
	if ft.In(0) != contextType {
		return nil, fmt.Errorf("dag: fn first param must be context.Context")
	}
	if ft.NumOut() > 2 {
		return nil, fmt.Errorf("dag: fn must not have more than 2 outputs")
	}
	if ft.NumOut() > 0 && !ft.Out(ft.NumOut()-1).Implements(errorType) {
		return nil, fmt.Errorf("dag: fn last return must be error")
	}
	if produces {
		if want == nil {
			return nil, fmt.Errorf("dag: missing result type for %s", key)
		}
		if ft.NumOut() != 2 {
			return nil, fmt.Errorf("dag: fn must return (T, error)")
		}
		if !ft.Out(0).AssignableTo(want) {
			return nil, fmt.Errorf("dag: fn returns %s, want %s", ft.Out(0), want)
		}
	}

	params := make([]paramSpec, 0, ft.NumIn()-1)
	deps := make([]Key, 0, ft.NumIn()-1)
	for i := 1; i < ft.NumIn(); i++ {
		pt := ft.In(i)
		if pt == statePtrType {
			params = append(params, paramSpec{typ: pt, isState: true})
			continue
		}
		k := typeKeyOf(pt)
		params = append(params, paramSpec{key: k, typ: pt})
		deps = appendUniqueKey(deps, k)
	}
	return newReflectService(key, deps, produces, rv, params), nil
}
