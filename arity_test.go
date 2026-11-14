package dag

import (
	"context"
	"strings"
	"testing"
)

type in0 struct{ V int }
type in1 struct{ V int }
type in2 struct{ V int }
type in3 struct{ V int }
type in4 struct{ V int }
type in5 struct{ V int }
type in6 struct{ V int }
type in7 struct{ V int }
type out8 struct{ V int }

type n1 struct{ N int }
type n2 struct{ N int }
type n3 struct{ N int }
type n4 struct{ N int }
type n5 struct{ N int }
type n6 struct{ N int }
type n7 struct{ N int }
type n8 struct{ N int }
type n9 struct{ N int }

func TestProvideArityChain(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (n1, error) { return n1{N: 1}, nil })
	b.Provide1(func(ctx context.Context, a n1) (n2, error) { return n2{N: a.N + 1}, nil })
	b.Provide2(func(ctx context.Context, a n1, b n2) (n3, error) { return n3{N: a.N + b.N + 1}, nil })
	b.Provide3(func(ctx context.Context, a n1, b n2, c n3) (n4, error) {
		return n4{N: a.N + b.N + c.N + 1}, nil
	})
	b.Provide4(func(ctx context.Context, a n1, b n2, c n3, d n4) (n5, error) {
		return n5{N: a.N + b.N + c.N + d.N + 1}, nil
	})
	b.Provide5(func(ctx context.Context, a n1, b n2, c n3, d n4, e n5) (n6, error) {
		return n6{N: a.N + b.N + c.N + d.N + e.N + 1}, nil
	})
	b.Provide6(func(ctx context.Context, a n1, b n2, c n3, d n4, e n5, f n6) (n7, error) {
		return n7{N: a.N + b.N + c.N + d.N + e.N + f.N + 1}, nil
	})
	b.Provide7(func(ctx context.Context, a n1, b n2, c n3, d n4, e n5, f n6, g n7) (n8, error) {
		return n8{N: a.N + b.N + c.N + d.N + e.N + f.N + g.N + 1}, nil
	})
	b.Provide8(func(ctx context.Context, a n1, b n2, c n3, d n4, e n5, f n6, g n7, h n8) (n9, error) {
		return n9{N: a.N + b.N + c.N + d.N + e.N + f.N + g.N + h.N + 1}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	out, ok := result.Get[n9]()
	if !ok || out.N != 256 {
		t.Fatalf("Get[n9] = %v, %v; want 256", out, ok)
	}
}

func TestLoadDepMissing(t *testing.T) {
	if _, err := loadDep[*A](&State{store: newStore(nil)}); err == nil {
		t.Fatal("loadDep missing err = nil")
	}
}

func TestProvideArityInjection(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (in0, error) { return in0{V: 1}, nil })
	b.Provide0(func(ctx context.Context) (in1, error) { return in1{V: 2}, nil })
	b.Provide0(func(ctx context.Context) (in2, error) { return in2{V: 3}, nil })
	b.Provide0(func(ctx context.Context) (in3, error) { return in3{V: 4}, nil })
	b.Provide0(func(ctx context.Context) (in4, error) { return in4{V: 5}, nil })
	b.Provide0(func(ctx context.Context) (in5, error) { return in5{V: 6}, nil })
	b.Provide0(func(ctx context.Context) (in6, error) { return in6{V: 7}, nil })
	b.Provide0(func(ctx context.Context) (in7, error) { return in7{V: 8}, nil })
	b.Provide8(func(ctx context.Context, a in0, bb in1, c in2, d in3, e in4, f in5, g in6, h in7) (*out8, error) {
		return &out8{V: a.V + bb.V + c.V + d.V + e.V + f.V + g.V + h.V}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	out, ok := result.Get[*out8]()
	if !ok || out.V != 36 {
		t.Fatalf("Get[*out8] = %v, %v; want 36", out, ok)
	}
}

func TestProvideCommonArities(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		return &B{Val: a.Val + "B"}, nil
	})
	b.Provide2(func(ctx context.Context, a *A, b *B) (*C, error) {
		return &C{Val: a.Val + b.Val}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}
	result, err := plan.Run(context.Background())
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	c, ok := result.Get[*C]()
	if !ok || c.Val != "AAB" {
		t.Fatalf("Get[*C] = %v, %v; want AAB", c, ok)
	}
}

func TestProvideArityDuplicateParams(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide2(func(ctx context.Context, a1, a2 *A) (*B, error) {
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
	if bv, ok := result.Get[*B](); !ok || bv.Val != "AA" {
		t.Fatalf("Get[*B] = %v, %v; want AA", bv, ok)
	}
}

func TestProvideArityMissingDependency(t *testing.T) {
	b := NewBuilder()
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		return &B{}, nil
	})
	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Build err = %v; want dependency not found", err)
	}
}

func TestProvideArityNilHandler(t *testing.T) {
	b := NewBuilder()
	b.Provide1[*A, *B](nil)
	if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("Build err = %v; want nil handler", err)
	}
}
