package dag

import (
	"reflect"
	"sync"
)

// dagValue 用于存放dag的值
type dagValue struct {
	mu  sync.RWMutex
	val map[string]any
}

// newDagValue 创建一个新的dagValue
func newDagValue() *dagValue {
	return &dagValue{
		val: make(map[string]any),
	}
}

func (dv *dagValue) get(key string) (any, bool) {
	dv.mu.RLock()
	defer dv.mu.RUnlock()
	v, ok := dv.val[key]
	return v, ok
}

func (dv *dagValue) set(key string, v any) {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	dv.val[key] = v
}

func (dv *dagValue) delete(key string) {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	delete(dv.val, key)
}

func (dv *dagValue) clear() {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	dv.val = make(map[string]any)
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
