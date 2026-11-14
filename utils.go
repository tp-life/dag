package dag

import (
	"context"
	"fmt"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// safeGo 安全执行。 是一个errgroup 的包装
type safeGo struct {
	eg *errgroup.Group
}

// NewSafeGo 创建一个安全的 Go
func NewSafeGo() *safeGo {
	return &safeGo{
		eg: &errgroup.Group{},
	}
}

// newSafeGoWithContext 创建一个带取消的 safeGo，并返回派生的 context
func newSafeGoWithContext(ctx context.Context) (*safeGo, context.Context) {
	eg, ctx := errgroup.WithContext(ctx)
	return &safeGo{eg: eg}, ctx
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
