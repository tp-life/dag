package dag

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

type A struct {
	Val string
}

type B struct {
	Val string
}

type C struct {
	c string
}

// namedInterface 用于验证接口类型的名称生成
type namedInterface interface {
	Name() string
}

func callA(ctx context.Context) (*A, error) {
	return &A{Val: "A"}, nil
}

func callC(ctx context.Context, b *B) {
	_ = b
}

func callB(ctx context.Context, a *A) (*B, error) {
	return &B{Val: a.Val}, nil
}

func TestFxDag_Draw(t *testing.T) {
	dag := New()

	if err := dag.WrapHandlerNoName(callB); err != nil {
		t.Fatalf("WrapHandlerNoName err: %v", err)
	}
	if err := dag.WrapHandler[*A](callA); err != nil {
		t.Fatalf("WrapHandler A err: %v", err)
	}
	if err := dag.WrapHandler[*B](callB); err != nil {
		t.Fatalf("WrapHandler B err: %v", err)
	}
	if err := dag.WrapHandler[*C](callC); err != nil {
		t.Fatalf("WrapHandler C err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	b, ok := dag.LoadData[*B]()
	if !ok || b == nil || b.Val != "A" {
		t.Fatalf("LoadData[*B] = %v, %v; want &B{Val: \"A\"}, true", b, ok)
	}
}

func TestFxDag_Execute(t *testing.T) {
	dag := New()

	if err := dag.WrapHandler[*A](callA); err != nil {
		t.Fatalf("WrapHandler A err: %v", err)
	}
	if err := dag.WrapHandler[*B](callB); err != nil {
		t.Fatalf("WrapHandler B err: %v", err)
	}
	if err := dag.WrapHandler[*C](callC); err != nil {
		t.Fatalf("WrapHandler C err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
}

func TestFxDag_ExecuteHandler(t *testing.T) {
	dag := New()

	if err := dag.ProvideByName("A", func(ctx context.Context, _ *FxDag) (*A, error) {
		return &A{Val: "A"}, nil
	}); err != nil {
		t.Fatalf("ProvideByName A err: %v", err)
	}

	err := dag.ProvideByName("B", func(ctx context.Context, d *FxDag) (*B, error) {
		a, ok := d.LoadDataByName[*A]("A")
		if !ok {
			return nil, context.Canceled
		}
		return &B{Val: a.Val}, nil
	}, "A")
	if err != nil {
		t.Fatalf("ProvideByName B err: %v", err)
	}

	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	b, ok := dag.LoadDataByName[*B]("B")
	if !ok || b == nil || b.Val != "A" {
		t.Fatalf("LoadDataByName[*B] = %v, %v; want &B{Val: \"A\"}, true", b, ok)
	}
}

// TestWrapHandlerNoNamePropagatesError 回归：pd=false 时不能吞掉 handler 的 error。
func TestWrapHandlerNoNamePropagatesError(t *testing.T) {
	dag := New()

	if err := dag.WrapHandlerNoName(func(ctx context.Context) error {
		return context.Canceled
	}); err != nil {
		t.Fatalf("WrapHandlerNoName err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != context.Canceled {
		t.Fatalf("Execute err = %v; want context.Canceled", err)
	}
}

// TestReflectMissingDependencyReturnsError 回归：缺依赖时返回明确错误而不是 reflect.Call panic。
func TestReflectMissingDependencyReturnsError(t *testing.T) {
	dag := New()

	// 以 *A 的类型名注册，但 pd=false，不会写入值
	if err := dag.ProvideByNameWithOut(GenName[*A](), func(ctx context.Context, _ *FxDag) (*A, error) {
		return &A{Val: "A"}, nil
	}); err != nil {
		t.Fatalf("ProvideByNameWithOut err: %v", err)
	}
	if err := dag.WrapHandler[*B](func(ctx context.Context, a *A) (*B, error) {
		return &B{Val: a.Val}, nil
	}); err != nil {
		t.Fatalf("WrapHandler err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}

	err := dag.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err = nil; want missing dependency error")
	}
	t.Logf("Execute err = %v", err)
}

// TestClearResetsDrawQueue 回归：Clear 后 Draw/Execute 不能执行旧队列。
func TestClearResetsDrawQueue(t *testing.T) {
	dag := New()
	var n atomic.Int32

	if err := dag.WrapHandlerNoName(func(ctx context.Context) {
		n.Add(1)
	}); err != nil {
		t.Fatalf("WrapHandlerNoName err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if got := n.Load(); got != 1 {
		t.Fatalf("handler ran %d times; want 1", got)
	}

	dag.Clear()
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw after Clear err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute after Clear err: %v", err)
	}
	if got := n.Load(); got != 1 {
		t.Fatalf("handler ran %d times after Clear; want 1", got)
	}
}

// TestGenNameInterface 回归：接口类型名不能带指针前缀。
func TestGenNameInterface(t *testing.T) {
	if got, want := GenName[namedInterface](), "dag.namedInterface"; got != want {
		t.Fatalf("GenName[namedInterface]() = %q; want %q", got, want)
	}
}

// TestInitValFallbackOnNil 回归：val 中的 nil 值应回退到 initVal。
func TestInitValFallbackOnNil(t *testing.T) {
	dag := New()

	if err := dag.InitParams(&A{Val: "init"}); err != nil {
		t.Fatalf("InitParams err: %v", err)
	}
	if err := dag.Provide(func(ctx context.Context, _ *FxDag) (*A, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("Provide err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	a, ok := dag.LoadData[*A]()
	if !ok || a == nil || a.Val != "init" {
		t.Fatalf("LoadData[*A] = %v, %v; want init value", a, ok)
	}
}

// TestNilHandlerReturnsError 回归：nil handler 应返回错误而不是崩溃。
func TestNilHandlerReturnsError(t *testing.T) {
	dag := New()

	if err := dag.WrapHandler[*A](nil); err == nil {
		t.Fatal("WrapHandler(nil) err = nil; want error")
	}
	if err := dag.WrapHandlerNoName(nil); err == nil {
		t.Fatal("WrapHandlerNoName(nil) err = nil; want error")
	}

	var handler FxHandler[*A]
	if err := dag.Provide(handler); err == nil {
		t.Fatal("Provide(nil) err = nil; want error")
	}
}

// TestWrapHandlerRejectsWrongReturnType 回归：返回类型与 T 不一致时必须报错。
func TestWrapHandlerRejectsWrongReturnType(t *testing.T) {
	dag := New()

	if err := dag.WrapHandler[*A](callB); err == nil {
		t.Fatal("WrapHandler[*A](callB) err = nil; want type mismatch error")
	}
}

// TestProvideDuplicateConcurrent 回归：并发注册同名依赖只能有一个成功。
func TestProvideDuplicateConcurrent(t *testing.T) {
	dag := New()
	var wg sync.WaitGroup
	var success atomic.Int32

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := dag.ProvideByName("same", func(ctx context.Context, _ *FxDag) (*A, error) {
				return &A{}, nil
			})
			if err == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := success.Load(); got != 1 {
		t.Fatalf("successful registrations = %d; want 1", got)
	}
}

// TestStopDuringExecute 回归：Stop 可并发调用，且应阻止后续层执行。
func TestStopDuringExecute(t *testing.T) {
	dag := New()
	started := make(chan struct{})
	release := make(chan struct{})
	var nextRan atomic.Bool

	if err := dag.WrapHandlerNoName(func(ctx context.Context) {
		close(started)
		<-release
	}); err != nil {
		t.Fatalf("WrapHandlerNoName err: %v", err)
	}
	if err := dag.Provide(func(ctx context.Context, _ *FxDag) (*A, error) {
		return &A{}, nil
	}); err != nil {
		t.Fatalf("Provide A err: %v", err)
	}
	if err := dag.ProvideByName("next", func(ctx context.Context, _ *FxDag) (*B, error) {
		nextRan.Store(true)
		return &B{}, nil
	}, GenName[*A]()); err != nil {
		t.Fatalf("ProvideByName next err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}

	go func() {
		<-started
		dag.Stop()
		close(release)
	}()

	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if nextRan.Load() {
		t.Fatal("second layer ran after Stop")
	}
}

// TestDeleteProviderClearsValue 回归：DeleteProvider 后不应再读取到旧值。
func TestDeleteProviderClearsValue(t *testing.T) {
	dag := New()

	if err := dag.Provide(func(ctx context.Context, _ *FxDag) (*A, error) {
		return &A{Val: "A"}, nil
	}); err != nil {
		t.Fatalf("Provide err: %v", err)
	}
	if err := dag.Draw(); err != nil {
		t.Fatalf("Draw err: %v", err)
	}
	if err := dag.Execute(context.Background()); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if _, ok := dag.LoadData[*A](); !ok {
		t.Fatal("LoadData[*A] before delete = false; want true")
	}

	dag.DeleteProvider(GenName[*A]())
	if _, ok := dag.LoadData[*A](); ok {
		t.Fatal("LoadData[*A] after delete = true; want false")
	}
}
