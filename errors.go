package dag

import (
	"fmt"
	"strings"
)

// Error 是单个服务或运行级别的错误。
type Error struct {
	Key   Key // 为空表示运行级错误
	Layer int // 服务所在层；运行级错误为 -1
	Err   error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case !e.Key.IsZero() && e.Layer >= 0:
		return fmt.Sprintf("dag: service %s (layer %d): %v", e.Key, e.Layer, e.Err)
	case !e.Key.IsZero():
		return fmt.Sprintf("dag: service %s: %v", e.Key, e.Err)
	case e.Layer >= 0:
		return fmt.Sprintf("dag: layer %d: %v", e.Layer, e.Err)
	default:
		return fmt.Sprintf("dag: %v", e.Err)
	}
}

// Unwrap 支持 errors.Is/As
func (e *Error) Unwrap() error {
	return e.Err
}

// RunErrors 是一次运行中的多个错误。
// 实现 Unwrap() []error，因此 errors.Is/As 会遍历所有服务错误。
type RunErrors []*Error

func (e RunErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	parts := make([]string, len(e))
	for i, err := range e {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "; ")
}

// Unwrap 支持 errors.Is/As
func (e RunErrors) Unwrap() []error {
	out := make([]error, len(e))
	for i, err := range e {
		out[i] = err
	}
	return out
}
