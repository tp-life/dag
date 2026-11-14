package dag

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
	Val string
}

// namedInterface 用于验证接口类型 key
type namedInterface interface {
	Name() string
}

func TestKeyNamespaces(t *testing.T) {
	tk := TypeKey[*A]()
	nk := NameKey(tk.String())

	if tk == nk {
		t.Fatal("type key and name key must be distinct")
	}
	if !tk.IsType() || nk.IsType() {
		t.Fatal("IsType mismatch")
	}
	if got, want := tk.String(), "*github.com/tp-life/dag/v2.A"; got != want {
		t.Fatalf("TypeKey[*A].String() = %q; want %q", got, want)
	}
	if got, want := TypeKey[namedInterface]().String(), "github.com/tp-life/dag/v2.namedInterface"; got != want {
		t.Fatalf("TypeKey[namedInterface].String() = %q; want %q", got, want)
	}
	if !(Key{}).IsZero() {
		t.Fatal("zero Key.IsZero() = false")
	}
}

func TestBuilderBuildTyped(t *testing.T) {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide(func(ctx context.Context, s *State) (*B, error) {
		a, ok := s.LoadData[*A]()
		if !ok {
			return nil, errors.New("A missing")
		}
		return &B{Val: a.Val}, nil
	}, TypeKey[*A]())

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	if got := plan.Layers(); got != 2 {
		t.Fatalf("plan.Layers() = %d; want 2", got)
	}

	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	bv, ok := result.Get[*B]()
	if !ok || bv == nil || bv.Val != "A" {
		t.Fatalf("Get[*B] = %v, %v; want &B{Val: \"A\"}", bv, ok)
	}
}

func TestProvideNamed(t *testing.T) {
	b := NewBuilder()
	b.ProvideNamed("a", func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.ProvideNamed("b", func(ctx context.Context, s *State) (*B, error) {
		a, ok := s.LoadDataByName[*A]("a")
		if !ok {
			return nil, errors.New("a missing")
		}
		return &B{Val: a.Val}, nil
	}, NameKey("a"))

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	bv, ok := result.GetNamed[*B]("b")
	if !ok || bv == nil || bv.Val != "A" {
		t.Fatalf("GetNamed[*B](b) = %v, %v; want &B{Val: \"A\"}", bv, ok)
	}
	if _, ok := result.Get[*B](); ok {
		t.Fatal("Get[*B] found a named-only value")
	}
}

func TestReflectionAutoWire(t *testing.T) {
	b := NewBuilder()
	b.Wrap[*A](func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Wrap[*B](func(ctx context.Context, a *A, s *State) (*B, error) {
		if s == nil {
			return nil, errors.New("state is nil")
		}
		return &B{Val: a.Val}, nil
	})
	b.WrapNamed[*C]("c", func(ctx context.Context, b *B) (*C, error) {
		return &C{Val: b.Val + "-C"}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	bv, ok := result.Get[*B]()
	if !ok || bv.Val != "A" {
		t.Fatalf("Get[*B] = %v, %v", bv, ok)
	}
	cv, ok := result.GetNamed[*C]("c")
	if !ok || cv.Val != "A-C" {
		t.Fatalf("GetNamed[*C](c) = %v, %v", cv, ok)
	}
}

func TestWrapNoName(t *testing.T) {
	var ran atomic.Bool
	b := NewBuilder()
	b.WrapNoName(func(ctx context.Context) {
		ran.Store(true)
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	if _, err := plan.Run(context.Background()); err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if !ran.Load() {
		t.Fatal("WrapNoName handler did not run")
	}
}

func TestWrapStateWithAfter(t *testing.T) {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Wrap[*B](func(ctx context.Context, s *State) (*B, error) {
		a, ok := s.LoadData[*A]()
		if !ok {
			return nil, errors.New("A missing")
		}
		return &B{Val: a.Val}, nil
	}, TypeKey[*A]())

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if bv, ok := result.Get[*B](); !ok || bv.Val != "A" {
		t.Fatalf("Get[*B] = %v, %v", bv, ok)
	}
}

func TestInitValue(t *testing.T) {
	b := NewBuilder()
	b.Init(&A{Val: "init"})
	b.InitNamed("named", &A{Val: "named-init"})
	b.Provide(func(ctx context.Context, s *State) (*B, error) {
		a, ok := s.LoadData[*A]()
		if !ok {
			return nil, errors.New("A missing")
		}
		return &B{Val: a.Val}, nil
	}, TypeKey[*A]())

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	if a, ok := result.Get[*A](); !ok || a.Val != "init" {
		t.Fatalf("Get[*A] = %v, %v", a, ok)
	}
	if a, ok := result.GetNamed[*A]("named"); !ok || a.Val != "named-init" {
		t.Fatalf("GetNamed[*A](named) = %v, %v", a, ok)
	}
	if bv, ok := result.Get[*B](); !ok || bv.Val != "init" {
		t.Fatalf("Get[*B] = %v, %v", bv, ok)
	}
}

func TestInitNilFallback(t *testing.T) {
	b := NewBuilder()
	b.Init(&A{Val: "init"})
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return nil, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	a, ok := result.Get[*A]()
	if !ok || a == nil || a.Val != "init" {
		t.Fatalf("Get[*A] = %v, %v; want init value", a, ok)
	}
}

func TestBuildMissingDependency(t *testing.T) {
	b := NewBuilder()
	b.Wrap[*B](func(ctx context.Context, a *A) (*B, error) {
		return &B{Val: a.Val}, nil
	})

	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Build err = %v; want dependency not found", err)
	}
}

func TestBuildNonProducingDependency(t *testing.T) {
	b := NewBuilder()
	b.ProvideWithoutValue("a", func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide(func(ctx context.Context, _ *State) (*B, error) {
		return &B{}, nil
	}, NameKey("a"))

	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "not produced") {
		t.Fatalf("Build err = %v; want dependency not produced", err)
	}
}

func TestBuildCycle(t *testing.T) {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{}, nil
	}, TypeKey[*B]())
	b.Provide(func(ctx context.Context, _ *State) (*B, error) {
		return &B{}, nil
	}, TypeKey[*A]())

	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("Build err = %v; want dependency cycle", err)
	}
}

func TestBuildDuplicateKey(t *testing.T) {
	b := NewBuilder()
	h := func(ctx context.Context, _ *State) (*A, error) {
		return &A{}, nil
	}
	b.Provide(h)
	b.Provide(h)

	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Build err = %v; want duplicate key", err)
	}
}

func TestBuildNilHandler(t *testing.T) {
	b := NewBuilder()
	var h Handler[*A]
	b.Provide(h)

	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("Build err = %v; want nil handler error", err)
	}
}

func TestBuildEmptyKey(t *testing.T) {
	h := func(ctx context.Context, _ *State) (*A, error) { return &A{}, nil }

	if _, err := NewBuilder().ProvideNamed("", h).Build(); err == nil {
		t.Fatal("ProvideNamed with empty name: err = nil")
	}
	if _, err := NewBuilder().WrapNamed[*A]("", func(ctx context.Context) (*A, error) { return &A{}, nil }).Build(); err == nil {
		t.Fatal("WrapNamed with empty name: err = nil")
	}
	if _, err := NewBuilder().InitNamed("", 1).Build(); err == nil {
		t.Fatal("InitNamed with empty name: err = nil")
	}
}

func TestBuildReflectServiceValidation(t *testing.T) {
	want := reflect.TypeFor[*A]()
	cases := []struct {
		name     string
		fn       any
		produces bool
	}{
		{"nil", nil, true},
		{"not func", 1, true},
		{"typed nil func", (func(context.Context) (*A, error))(nil), true},
		{"no params", func() (*A, error) { return nil, nil }, true},
		{"wrong first param", func(int) (*A, error) { return nil, nil }, true},
		{"too many outputs", func(context.Context) (*A, *A, error) { return nil, nil, nil }, true},
		{"last not error", func(context.Context) (*A, *A) { return nil, nil }, true},
		{"wrong return", func(context.Context) (*B, error) { return nil, nil }, true},
		{"no result", func(context.Context) error { return nil }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildReflectService(TypeKey[*A](), tc.fn, tc.produces, want); err == nil {
				t.Fatalf("buildReflectService(%v) err = nil; want error", tc.fn)
			}
		})
	}

	if _, err := buildReflectService(NameKey("n"), func(context.Context) {}, false, nil); err != nil {
		t.Fatalf("valid no-output fn err: %v", err)
	}
}

func TestRunContextCancel(t *testing.T) {
	b := NewBuilder()
	started := make(chan struct{})
	b.WrapNoName(func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := plan.Run(ctx)
		done <- err
	}()

	<-started
	cancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err = %v; want context.Canceled", err)
	}
}

func TestRunPanicRecovery(t *testing.T) {
	b := NewBuilder()
	b.WrapNoName(func(ctx context.Context) {
		panic("boom")
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	_, err = plan.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "panic: boom") {
		t.Fatalf("Run err = %v; want panic error", err)
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("Run err missing stack trace: %v", err)
	}
}

func TestResultPartialOnError(t *testing.T) {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide(func(ctx context.Context, _ *State) (*B, error) {
		return nil, context.Canceled
	}, TypeKey[*A]())

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	result, err := plan.Run(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err = %v; want context.Canceled", err)
	}
	if result == nil {
		t.Fatal("result = nil on error; want partial result")
	}
	if a, ok := result.Get[*A](); !ok || a.Val != "A" {
		t.Fatalf("Get[*A] = %v, %v; want partial result", a, ok)
	}
}

func TestPlanConcurrentRuns(t *testing.T) {
	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := plan.Run(context.Background())
			if err != nil {
				errs <- err
				return
			}
			a, ok := result.Get[*A]()
			if !ok || a.Val != "A" {
				errs <- errors.New("unexpected result")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestPlanEmpty(t *testing.T) {
	b := NewBuilder()
	b.Init(&A{Val: "init"})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	if got := plan.Layers(); got != 0 {
		t.Fatalf("plan.Layers() = %d; want 0", got)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if a, ok := result.Get[*A](); !ok || a.Val != "init" {
		t.Fatalf("Get[*A] = %v, %v", a, ok)
	}
}

func TestIsNil(t *testing.T) {
	var p *A
	var m map[string]int
	var s []int
	var fn func()

	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"nil", nil, true},
		{"typed nil pointer", p, true},
		{"typed nil map", m, true},
		{"typed nil slice", s, true},
		{"typed nil func", fn, true},
		{"non-nil pointer", &A{}, false},
		{"zero int", 0, false},
		{"empty string", "", false},
	}
	for _, tc := range cases {
		if got := isNil(tc.v); got != tc.want {
			t.Errorf("isNil(%s) = %v; want %v", tc.name, got, tc.want)
		}
	}
}
