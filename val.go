package dag

import "sync"

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

func (dv *dagValue) set(key string, v any) error {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	dv.val[key] = v
	return nil
}
