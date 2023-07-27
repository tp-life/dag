package dag

import (
	"fmt"

	"golang.org/x/sync/errgroup"
)

// SafeFn 安全执行。 是一个errgroup 的包装
type safeGo struct {
	eg errgroup.Group
}

// NewSafeGo 创建一个安全的 Go
func NewSafeGo() *safeGo {
	return &safeGo{
		eg: errgroup.Group{},
	}
}

// Go errgroup.Go()
func (sg *safeGo) Go(fn func() error) {
	sg.eg.Go(func() error {
		return SafeFn(fn)
	})
}

// Wait errgroup.Wait()
func (sg *safeGo) Wait() error {
	return sg.eg.Wait()
}

// SafeFn 安全执行
func SafeFn(fn func() error) (err error) {
	defer func() {
		if er := recover(); er != nil {
			err = fmt.Errorf("panic: %v", er)
		}
	}()
	err = fn()
	return
}
