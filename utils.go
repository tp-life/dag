package dag

import (
	"fmt"
	"runtime/debug"
)

// SafeFn 安全执行；panic 会转换为 error 并附带堆栈
func SafeFn(fn func() error) (err error) {
	defer func() {
		if er := recover(); er != nil {
			err = fmt.Errorf("panic: %v\n%s", er, debug.Stack())
		}
	}()
	err = fn()
	return
}
