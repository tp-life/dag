package dag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestContinueOnError(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		return nil, errors.New("boom")
	})
	b.Provide1(func(ctx context.Context, b *B) (*C, error) {
		return &C{}, nil
	})
	b.ProvideNamed("independent", func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "I"}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background(), ContinueOnError())
	if err == nil {
		t.Fatal("Run err = nil; want failure")
	}

	var runErrs RunErrors
	if !errors.As(err, &runErrs) || len(runErrs) != 1 {
		t.Fatalf("errors.As RunErrors = %v, want 1 error", err)
	}
	if runErrs[0].Key != TypeKey[*B]() {
		t.Fatalf("failed key = %s; want %s", runErrs[0].Key, TypeKey[*B]())
	}

	if a, ok := result.Get[*A](); !ok || a.Val != "A" {
		t.Fatalf("Get[*A] = %v, %v; want success", a, ok)
	}
	if _, ok := result.Get[*B](); ok {
		t.Fatal("Get[*B] returned ok; want failed")
	}
	if _, ok := result.Get[*C](); ok {
		t.Fatal("Get[*C] returned ok; want skipped")
	}
	if a, ok := result.GetNamed[*A]("independent"); !ok || a.Val != "I" {
		t.Fatalf("GetNamed(independent) = %v, %v; want success", a, ok)
	}

	skipped := result.Skipped()
	if len(skipped) != 1 || skipped[0] != TypeKey[*C]() {
		t.Fatalf("Skipped = %v; want [%s]", skipped, TypeKey[*C]())
	}

	report := result.Report()
	if report == nil {
		t.Fatal("Report = nil")
	}
	if got := reportStatus(t, report, TypeKey[*A]()); got != StatusSucceeded {
		t.Fatalf("A status = %s; want succeeded", got)
	}
	if got := reportStatus(t, report, TypeKey[*B]()); got != StatusFailed {
		t.Fatalf("B status = %s; want failed", got)
	}
	if got := reportStatus(t, report, TypeKey[*C]()); got != StatusSkipped {
		t.Fatalf("C status = %s; want skipped", got)
	}
	if got := reportStatus(t, report, NameKey("independent")); got != StatusSucceeded {
		t.Fatalf("independent status = %s; want succeeded", got)
	}
}

func TestFailFastSkipsPending(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return nil, errors.New("boom")
	})
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		return &B{}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err == nil {
		t.Fatal("Run err = nil; want failure")
	}

	skipped := result.Skipped()
	if len(skipped) != 1 || skipped[0] != TypeKey[*B]() {
		t.Fatalf("Skipped = %v; want [%s]", skipped, TypeKey[*B]())
	}
	report := result.Report()
	if got := reportStatus(t, report, TypeKey[*A]()); got != StatusFailed {
		t.Fatalf("A status = %s; want failed", got)
	}
	if got := reportStatus(t, report, TypeKey[*B]()); got != StatusSkipped {
		t.Fatalf("B status = %s; want skipped", got)
	}
}

type recordingHook struct {
	mu     sync.Mutex
	starts []Key
	ends   []Key
}

func (h *recordingHook) OnServiceStart(ctx context.Context, key Key, layer int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.starts = append(h.starts, key)
}

func (h *recordingHook) OnServiceEnd(ctx context.Context, key Key, layer int, d time.Duration, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ends = append(h.ends, key)
}

func TestRunHookAndReport(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		return &B{Val: a.Val}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	hook := &recordingHook{}
	result, err := plan.Run(context.Background(), WithHook(hook))
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}

	if len(hook.starts) != 2 || len(hook.ends) != 2 {
		t.Fatalf("hook events = %d starts, %d ends; want 2, 2", len(hook.starts), len(hook.ends))
	}
	report := result.Report()
	if len(report.Services) != 2 {
		t.Fatalf("report services = %d; want 2", len(report.Services))
	}
	for _, svc := range report.Services {
		if svc.Status != StatusSucceeded {
			t.Fatalf("service %s status = %s; want succeeded", svc.Key, svc.Status)
		}
		if svc.Duration < 0 {
			t.Fatalf("service %s duration = %v", svc.Key, svc.Duration)
		}
	}
	if got := serviceLayer(t, report, TypeKey[*A]()); got != 0 {
		t.Fatalf("A layer = %d; want 0", got)
	}
	if got := serviceLayer(t, report, TypeKey[*B]()); got != 1 {
		t.Fatalf("B layer = %d; want 1", got)
	}
}

type panicHook struct{}

func (panicHook) OnServiceStart(context.Context, Key, int)                     { panic("hook start") }
func (panicHook) OnServiceEnd(context.Context, Key, int, time.Duration, error) { panic("hook end") }

func TestHookPanicDoesNotBreakRun(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{}, nil
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	if _, err := plan.Run(context.Background(), WithHook(panicHook{})); err != nil {
		t.Fatalf("Run err: %v", err)
	}
}

func TestRunServiceTimeout(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	_, err = plan.Run(context.Background(), WithServiceTimeout(30*time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run err = %v; want context.DeadlineExceeded", err)
	}
}

func TestRunMaxConcurrency(t *testing.T) {
	var current, max atomic.Int32
	handler := func(ctx context.Context, _ *State) (*A, error) {
		n := current.Add(1)
		for {
			m := max.Load()
			if n <= m || max.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		current.Add(-1)
		return &A{}, nil
	}

	b := NewBuilder()
	for i := 0; i < 6; i++ {
		b.ProvideNamed(fmt.Sprintf("n%d", i), handler)
	}
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	if _, err := plan.Run(context.Background(), WithMaxConcurrency(2)); err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if got := max.Load(); got > 2 {
		t.Fatalf("max concurrency = %d; want <= 2", got)
	}
	if got := max.Load(); got < 1 {
		t.Fatal("handlers did not run")
	}
}

// TestRunReadyQueue 验证后继节点不等同层慢节点：慢服务阻塞时，快分支的后继仍能启动。
func TestRunReadyQueue(t *testing.T) {
	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	bStarted := make(chan struct{})

	b := NewBuilder()
	b.ProvideNamed("slow", func(ctx context.Context, _ *State) (*C, error) {
		close(slowStarted)
		<-releaseSlow
		return &C{}, nil
	})
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{}, nil
	})
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		close(bStarted)
		return &B{}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := plan.Run(context.Background())
		done <- err
	}()

	<-slowStarted
	select {
	case <-bStarted:
		close(releaseSlow)
	case <-time.After(2 * time.Second):
		close(releaseSlow)
		<-done
		t.Fatal("dependent service did not start while slow sibling was running")
	}

	if err := <-done; err != nil {
		t.Fatalf("Run err: %v", err)
	}
}

func TestRunErrorsAggregation(t *testing.T) {
	errA := errors.New("err-a")
	errB := errors.New("err-b")

	b := NewBuilder()
	b.ProvideNamed("a", func(ctx context.Context, _ *State) (*A, error) {
		return nil, errA
	})
	b.ProvideNamed("b", func(ctx context.Context, _ *State) (*B, error) {
		return nil, errB
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	result, err := plan.Run(context.Background(), ContinueOnError())
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("Run err = %v; want both errA and errB", err)
	}
	if got := len(result.Errors()); got != 2 {
		t.Fatalf("Errors() len = %d; want 2", got)
	}

	msg := err.Error()
	if !strings.Contains(msg, "service a") || !strings.Contains(msg, "service b") {
		t.Fatalf("error message = %q", msg)
	}
}

func TestErrorFormatting(t *testing.T) {
	sentinel := errors.New("boom")

	cases := []struct {
		name string
		err  *Error
		want []string
	}{
		{"named with layer", &Error{Key: NameKey("x"), Layer: 2, Err: sentinel}, []string{"service x", "layer 2", "boom"}},
		{"named without layer", &Error{Key: NameKey("x"), Layer: -1, Err: sentinel}, []string{"service x", "boom"}},
		{"run with layer", &Error{Layer: 2, Err: sentinel}, []string{"layer 2", "boom"}},
		{"run without layer", &Error{Layer: -1, Err: sentinel}, []string{"dag: boom"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, sentinel) {
				t.Fatal("errors.Is(*Error, sentinel) = false")
			}
			msg := tc.err.Error()
			for _, want := range tc.want {
				if !strings.Contains(msg, want) {
					t.Fatalf("Error() = %q; want contains %q", msg, want)
				}
			}
		})
	}

	var nilErr *Error
	if nilErr.Error() != "<nil>" {
		t.Fatalf("nil Error() = %q", nilErr.Error())
	}
	if RunErrors(nil).Error() != "" {
		t.Fatalf("empty RunErrors() = %q", RunErrors(nil).Error())
	}

	runErr := RunErrors{{Key: TypeKey[*A](), Layer: 2, Err: sentinel}}
	if !errors.Is(runErr, sentinel) {
		t.Fatal("errors.Is(RunErrors, sentinel) = false")
	}
}

func TestEmptyPlanCanceled(t *testing.T) {
	plan, err := NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := plan.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err = %v; want context.Canceled", err)
	}
	if result == nil {
		t.Fatal("result = nil")
	}
	if got := len(result.Errors()); got != 1 {
		t.Fatalf("Errors() len = %d; want 1", got)
	}
}

func TestFailFastOption(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{}, nil
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	if _, err := plan.Run(context.Background(), FailFast()); err != nil {
		t.Fatalf("Run err: %v", err)
	}
}

func TestServiceTimeoutOverridesSuccess(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		time.Sleep(40 * time.Millisecond)
		return &A{}, nil
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	_, err = plan.Run(context.Background(), WithServiceTimeout(10*time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run err = %v; want context.DeadlineExceeded", err)
	}
}

func TestReflectNilDependency(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (any, error) {
		return nil, nil
	})
	b.Wrap[*A](func(ctx context.Context, v any) (*A, error) {
		if v != nil {
			return nil, fmt.Errorf("v = %v; want nil", v)
		}
		return &A{}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	if _, err := plan.Run(context.Background()); err != nil {
		t.Fatalf("Run err: %v", err)
	}
}

func TestProvideWithoutValueRun(t *testing.T) {
	b := NewBuilder()
	b.ProvideWithoutValue("x", func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "X"}, nil
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if _, ok := result.GetNamed[*A]("x"); ok {
		t.Fatal("ProvideWithoutValue wrote a value")
	}
	if _, ok := result.Get[*A](); ok {
		t.Fatal("ProvideWithoutValue wrote a type-keyed value")
	}
}

func reportStatus(t *testing.T, report *RunReport, key Key) ServiceStatus {
	t.Helper()
	for _, svc := range report.Services {
		if svc.Key == key {
			return svc.Status
		}
	}
	t.Fatalf("report missing key %s", key)
	return ""
}

func serviceLayer(t *testing.T, report *RunReport, key Key) int {
	t.Helper()
	for _, svc := range report.Services {
		if svc.Key == key {
			return svc.Layer
		}
	}
	t.Fatalf("report missing key %s", key)
	return -1
}
