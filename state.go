package dag

import (
	"reflect"
	"sync"
)

// store 是单次运行的值存储：init 不可变，val 由各服务并发写入。
type store struct {
	mu   sync.RWMutex
	init map[Key]any
	val  map[Key]any
}

func newStore(init map[Key]any) *store {
	initCopy := make(map[Key]any, len(init))
	for k, v := range init {
		initCopy[k] = v
	}
	return &store{
		init: initCopy,
		val:  make(map[Key]any),
	}
}

// get 优先返回 val；val 为 nil 时回退 init；init 也没有时保留 nil 值。
func (s *store) get(k Key) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.val[k]; ok {
		if !isNil(v) {
			return v, true
		}
		if iv, iok := s.init[k]; iok {
			return iv, true
		}
		return v, true
	}
	v, ok := s.init[k]
	return v, ok
}

func (s *store) set(k Key, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.val[k] = v
}

// snapshot 生成运行结果快照，应用与 get 相同的 nil 回退规则
func (s *store) snapshot() map[Key]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[Key]any, len(s.init)+len(s.val))
	for k, v := range s.init {
		out[k] = v
	}
	for k, v := range s.val {
		if isNil(v) {
			if _, ok := s.init[k]; ok {
				continue
			}
		}
		out[k] = v
	}
	return out
}

// State 是 handler 可见的只读运行状态。
// 值只能通过 *Result 或 State 的 Load 系列方法读取，handler 无法修改运行状态。
type State struct {
	store *store
}

// Load 按 Key 读取原始值
func (s *State) Load(k Key) (any, bool) {
	if s == nil || s.store == nil {
		return nil, false
	}
	return s.store.get(k)
}

// LoadData 读取类型 T 的值
func (s *State) LoadData[T any]() (T, bool) {
	return s.LoadByKey[T](TypeKey[T]())
}

// LoadDataByName 读取指定名称的值
func (s *State) LoadDataByName[T any](name string) (T, bool) {
	return s.LoadByKey[T](NameKey(name))
}

// LoadByKey 按 Key 读取并断言类型 T
func (s *State) LoadByKey[T any](k Key) (T, bool) {
	v, ok := s.Load(k)
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}

// set 仅供包内服务写入
func (s *State) set(k Key, v any) {
	s.store.set(k, v)
}

// Result 是一次运行的结果快照；Run 返回后只读，可并发访问。
type Result struct {
	values  map[Key]any
	errs    RunErrors
	skipped []Key
	report  *RunReport
}

// Load 按 Key 读取原始值
func (r *Result) Load(k Key) (any, bool) {
	if r == nil {
		return nil, false
	}
	v, ok := r.values[k]
	return v, ok
}

// Get 读取类型 T 的结果
func (r *Result) Get[T any]() (T, bool) {
	return r.GetByKey[T](TypeKey[T]())
}

// GetNamed 读取指定名称的结果
func (r *Result) GetNamed[T any](name string) (T, bool) {
	return r.GetByKey[T](NameKey(name))
}

// GetByKey 按 Key 读取并断言类型 T
func (r *Result) GetByKey[T any](k Key) (T, bool) {
	v, ok := r.Load(k)
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}

// Values 返回结果副本
func (r *Result) Values() map[Key]any {
	if r == nil {
		return nil
	}
	out := make(map[Key]any, len(r.values))
	for k, v := range r.values {
		out[k] = v
	}
	return out
}

// Errors 返回本次运行的服务错误
func (r *Result) Errors() RunErrors {
	if r == nil {
		return nil
	}
	out := make(RunErrors, len(r.errs))
	copy(out, r.errs)
	return out
}

// Skipped 返回本次运行中被跳过的服务 key
func (r *Result) Skipped() []Key {
	if r == nil {
		return nil
	}
	out := make([]Key, len(r.skipped))
	copy(out, r.skipped)
	return out
}

// Report 返回本次运行的报告副本
func (r *Result) Report() *RunReport {
	if r == nil || r.report == nil {
		return nil
	}
	out := *r.report
	out.Services = append([]ServiceReport(nil), r.report.Services...)
	return &out
}

// isNil 判断接口值是否为 nil（包括带类型的 nil 指针、切片、map 等）
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
