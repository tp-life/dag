package dag

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestZeroValueBuilder(t *testing.T) {
	var b Builder
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if a, ok := result.Get[*A](); !ok || a.Val != "A" {
		t.Fatalf("Get[*A] = %v, %v", a, ok)
	}
}

func TestBuilderErrorPaths(t *testing.T) {
	h := func(ctx context.Context, _ *State) (*A, error) { return &A{}, nil }
	fn := func(ctx context.Context) (*A, error) { return &A{}, nil }

	cases := []struct {
		name string
		b    *Builder
		want string
	}{
		{"empty dep key", NewBuilder().Provide(h, Key{}), "empty dependency key"},
		{"invalid wrap fn", NewBuilder().Wrap[*A](1), "not a function"},
		{"duplicate wrap key", NewBuilder().Wrap[*A](fn).Wrap[*A](fn), "duplicate"},
		{"duplicate named/wrap", NewBuilder().ProvideNamed("x", h).WrapNamed[*A]("x", fn), "duplicate"},
		{"nil init", NewBuilder().InitNamed("x", nil), "nil"},
		{"duplicate init", NewBuilder().InitNamed("x", 1).InitNamed("x", 2), "duplicate init"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.b.Build(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Build err = %v; want contains %q", err, tc.want)
			}
		})
	}
}

func TestReflectionDuplicateParams(t *testing.T) {
	b := NewBuilder()
	b.Wrap[*A](func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Wrap[*B](func(ctx context.Context, a1, a2 *A) (*B, error) {
		return &B{Val: a1.Val + a2.Val}, nil
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
	if !ok || bv.Val != "AA" {
		t.Fatalf("Get[*B] = %v, %v; want &B{Val: \"AA\"}", bv, ok)
	}
}

func TestTypeKeyStrings(t *testing.T) {
	cases := []struct {
		key  Key
		want string
	}{
		{TypeKey[[]*A](), "[]*dag.A"},
		{TypeKey[map[string]*A](), "map[string]*dag.A"},
		{TypeKey[[2]*A](), "[2]*dag.A"},
		{TypeKey[chan int](), "chan int"},
	}
	for _, tc := range cases {
		if got := tc.key.String(); got != tc.want {
			t.Errorf("Key.String() = %q; want %q", got, tc.want)
		}
	}
	if got := TypeKey[struct{ X int }]().String(); !strings.Contains(got, "struct") {
		t.Errorf("unnamed struct key = %q; want contains struct", got)
	}
	if keySortString(TypeKey[*A]()) == keySortString(NameKey("*dag.A")) {
		t.Error("type key and name key sort strings must differ")
	}
}

func TestNilGuards(t *testing.T) {
	var state *State
	if _, ok := state.Load(TypeKey[*A]()); ok {
		t.Fatal("nil State.Load returned ok")
	}
	if _, ok := (&State{}).Load(TypeKey[*A]()); ok {
		t.Fatal("zero State.Load returned ok")
	}
	if _, ok := (&State{}).LoadData[*A](); ok {
		t.Fatal("zero State.LoadData returned ok")
	}
	if _, ok := (&State{store: newStore(nil)}).LoadByKey[*A](TypeKey[*A]()); ok {
		t.Fatal("missing State.LoadByKey returned ok")
	}

	var result *Result
	if _, ok := result.Load(TypeKey[*A]()); ok {
		t.Fatal("nil Result.Load returned ok")
	}
	if result.Values() != nil {
		t.Fatal("nil Result.Values != nil")
	}

	var plan *Plan
	if plan.Layers() != 0 {
		t.Fatal("nil Plan.Layers != 0")
	}
	if _, err := plan.Run(context.Background()); err == nil {
		t.Fatal("nil Plan.Run err = nil")
	}

	b := NewBuilder()
	b.Provide(func(ctx context.Context, _ *State) (*A, error) {
		return &A{Val: "A"}, nil
	})
	p, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	var nilCtx context.Context
	if _, err := p.Run(nilCtx); err == nil {
		t.Fatal("Run(nil) err = nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = p.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run canceled err = %v; want context.Canceled", err)
	}
	if result == nil {
		t.Fatal("Run canceled result = nil")
	}

	result, err = p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	values := result.Values()
	if len(values) != 1 {
		t.Fatalf("Values() len = %d; want 1", len(values))
	}
}

func TestStoreBranches(t *testing.T) {
	kInit := NameKey("init")
	kVal := NameKey("val")
	kNilInit := NameKey("nil-init")
	kNil := NameKey("nil")

	st := newStore(map[Key]any{
		kInit:    "init",
		kNilInit: "init2",
	})
	st.set(kVal, "val")
	st.set(kNilInit, (*A)(nil))
	st.set(kNil, (*A)(nil))

	if v, ok := st.get(kVal); !ok || v != "val" {
		t.Fatalf("get(val) = %v, %v", v, ok)
	}
	if v, ok := st.get(kInit); !ok || v != "init" {
		t.Fatalf("get(init) = %v, %v", v, ok)
	}
	if v, ok := st.get(kNilInit); !ok || v != "init2" {
		t.Fatalf("get(nil-init) = %v, %v", v, ok)
	}
	if v, ok := st.get(kNil); !ok || !isNil(v) {
		t.Fatalf("get(nil) = %v, %v; want typed nil, true", v, ok)
	}
	if _, ok := st.get(NameKey("missing")); ok {
		t.Fatal("get(missing) returned ok")
	}
}

func TestServiceInternalErrors(t *testing.T) {
	svc := newService(TypeKey[*A](), nil, true)
	err := svc.execute(context.Background(), &State{store: newStore(nil)})
	if err == nil {
		t.Fatal("service with no runner err = nil")
	}

	rsvc, err := buildReflectService(
		TypeKey[*B](),
		func(ctx context.Context, a *A) (*B, error) { return &B{Val: a.Val}, nil },
		true,
		reflect.TypeFor[*B](),
	)
	if err != nil {
		t.Fatalf("buildReflectService err: %v", err)
	}

	err = rsvc.execute(context.Background(), &State{store: newStore(nil)})
	if err == nil || !strings.Contains(err.Error(), "依赖项不存在") {
		t.Fatalf("missing dependency err = %v", err)
	}

	st := newStore(map[Key]any{TypeKey[*A](): &B{}})
	err = rsvc.execute(context.Background(), &State{store: st})
	if err == nil || !strings.Contains(err.Error(), "类型不匹配") {
		t.Fatalf("type mismatch err = %v", err)
	}
}
